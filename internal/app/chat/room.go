/*
Package chat contains the core logic for handling real-time chat rooms, user connections, and message broadcasting.

This file defines the Room struct, which is the central hub for a single chat session.
It manages client lifecycles (register/unregister), message broadcasting to all participants,
and automatic shutdown based on inactivity.
*/
package chat

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"hzchat/internal/app/user"
	"hzchat/internal/pkg/logx"

	"github.com/rs/zerolog"
)

const broadcastChannelBuffer = 1024

const (
	// PrivateMaxClients defines the capacity limit for private chat rooms.
	PrivateMaxClients = 2

	// GroupMaxClients defines the capacity limit for standard group chat rooms.
	GroupMaxClients = 10

	// RoomInactivityTimeout is the duration after which an empty room will automatically shut down.
	RoomInactivityTimeout = 5 * time.Minute
)

// Room struct represents a single, active chat room session.
type Room struct {
	Code       string
	MaxClients int
	JWTSecret  string

	// Core state
	clients map[string]*Client

	// Channels for concurrency
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client

	// Control & Synchronization
	cleanupChan   chan<- RoomCleanupMsg
	stopChan      chan struct{}
	shutdownTimer *time.Timer
	mu            sync.RWMutex

	// Context
	logger zerolog.Logger
}

// NewRoom creates and initializes a new Room instance.
func NewRoom(roomCode string, maxClients int, cleanupChan chan<- RoomCleanupMsg, jwtSecret string) *Room {
	roomLogger := logx.Logger().With().
		Str("room_code", roomCode).
		Logger()

	return &Room{
		Code:          roomCode,
		MaxClients:    maxClients,
		JWTSecret:     jwtSecret,
		clients:       make(map[string]*Client),
		broadcast:     make(chan Message, broadcastChannelBuffer),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		cleanupChan:   cleanupChan,
		stopChan:      make(chan struct{}),
		shutdownTimer: time.NewTimer(RoomInactivityTimeout),
		logger:        roomLogger,
	}
}

// Run starts the main event loop for the Room.
// It handles client registration, deregistration, message broadcasting, and room shutdown.
func (r *Room) Run() {
	defer r.cleanupOnExit()

	timerChan := r.shutdownTimer.C

	for {
		select {
		case client := <-r.register:
			r.handleRegister(client)

		case client := <-r.unregister:
			r.handleUnregister(client)

		case message := <-r.broadcast:
			r.handleBroadcast(message)

		case <-timerChan:
			r.logger.Info().Msgf("Room inactivity timeout (%s) reached. Shutting down loop.", RoomInactivityTimeout)
			return

		case <-r.stopChan:
			r.logger.Info().Msg("Room forced stop initiated.")
			return
		}
	}
}

// Stop sends a signal to immediately terminate the Room's Run loop.
func (r *Room) Stop() {
	r.logger.Info().Msg("Received stop signal. Stopping room immediately.")

	select {
	case <-r.stopChan:
	default:
		close(r.stopChan)
	}
}

// handleRegister manages the entire lifecycle logic for a client joining the room.
func (r *Room) handleRegister(client *Client) {
	r.mu.Lock()

	// Check if client already exists, kick old connection if so
	if existingClient, ok := r.clients[client.user.ID]; ok {
		r.logger.Warn().
			Str("client_id", client.user.ID).
			Msg("Client ID already connected. Closing old connection for replacement.")

		existingClient.Kick("Session replaced by new connection. Check other tabs.")
	}

	// stop shutdown timer if running
	if r.shutdownTimer != nil {
		if r.shutdownTimer.Stop() {
			select {
			case <-r.shutdownTimer.C:
			default:
			}
		}
	}

	// check room capacity
	if _, exists := r.clients[client.user.ID]; !exists && r.MaxClients > 0 && len(r.clients) >= r.MaxClients {
		r.logger.Warn().
			Int("max_clients", r.MaxClients).
			Str("client_id", client.user.ID).
			Msg("Room is full. New unique client rejected.")

		client.SendError(fmt.Errorf("room is full"))

		select {
		case <-client.send:
		default:
			close(client.send)
		}

		r.mu.Unlock()
		return
	}

	// Register client
	r.clients[client.user.ID] = client
	r.logger.Info().
		Str("client_id", client.user.ID).
		Int("total_users", len(r.clients)).
		Msg("Client joined room.")

	// Prepare initial data
	onlineUsers := make([]user.User, 0, len(r.clients))
	for _, c := range r.clients {
		onlineUsers = append(onlineUsers, c.user)
	}

	initDataPayload := InitDataPayload{
		CurrentUser: client.user,
		OnlineUsers: onlineUsers,
		MaxUsers:    r.MaxClients,
	}

	r.mu.Unlock()

	// Send initial data
	err := client.SendInitData(initDataPayload)
	if err != nil {
		r.unregister <- client
		return
	}

	// Broadcast join event
	msg, err := NewMessage(TypeUserJoined, r.Code, SystemUser, UserEventPayload{User: client.user})
	if err != nil {
		r.logger.Error().
			Str("client_id", client.user.ID).
			Err(err).
			Msg("Failed to build USER_JOINED message.")
	} else {
		select {
		case r.broadcast <- msg:
		default:
			r.logger.Warn().Msg("Broadcast channel full during USER_JOINED.")
		}
	}
}

// handleUnregister manages the entire lifecycle logic for a client leaving the room.
func (r *Room) handleUnregister(client *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// delete client if it exists and matches the current connection
	if currentClient, ok := r.clients[client.user.ID]; ok && currentClient == client {
		delete(r.clients, client.user.ID)

		select {
		case <-client.send:
		default:
			close(client.send)
		}

		r.logger.Info().
			Str("client_id", client.user.ID).
			Int("total_users", len(r.clients)).
			Msg("Client left room.")

		// Broadcast leave event
		msg, err := NewMessage(TypeUserLeft, r.Code, SystemUser, UserEventPayload{User: client.user})
		if err != nil {
			r.logger.Error().
				Str("client_id", client.user.ID).
				Err(err).
				Msg("Failed to build USER_LEFT message during cleanup.")
		} else {
			select {
			case r.broadcast <- msg:
			default:
				r.logger.Warn().Msg("Broadcast channel full during USER_LEFT.")
			}
		}

		// 4. Inactivity timer logic
		if len(r.clients) == 0 {
			r.logger.Info().Msg("Room is empty. Restarting shutdown timer.")

			// Stop and drain the old timer signal (if the timer was running), then reset
			if r.shutdownTimer.Stop() {
				select {
				case <-r.shutdownTimer.C:
				default:
				}
			}
			r.shutdownTimer.Reset(RoomInactivityTimeout)
		}

	} else if ok && currentClient != client {
		// Client ID exists but is not the current connection
		r.logger.Info().
			Str("stale_client_id", client.user.ID).
			Msg("Ignoring unregister for STALE connection.")

	} else {
		// Client ID does not exist
		r.logger.Warn().
			Str("client_id", client.user.ID).
			Msg("Unregister failed for unknown/already deleted client.")
	}
}

// handleBroadcast manages the entire logic for marshaling and distributing a message
// to all other clients in the room.
func (r *Room) handleBroadcast(message Message) {
	// check for TypeError messages
	if message.Type == TypeError {
		r.logger.Warn().
			Interface("message", message).
			Msg("Received unhandled TypeError in broadcast channel.")
		return
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		r.logger.Error().
			Str("message_id", message.ID).
			Err(err).
			Msg("Error marshaling message for broadcast.")
		return
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	senderID := message.Sender.ID

	for _, client := range r.clients {
		// Skip sender
		if client.user.ID != senderID {
			select {
			case client.send <- messageBytes:
				// Message sent successfully
			default:
				// Client send channel full or closed, schedule unregister
				r.logger.Warn().
					Str("client_id", client.user.ID).
					Msg("Client send channel full or closed, scheduling unregister.")

				select {
				case r.unregister <- client:
				default:
					r.logger.Warn().Msg("Unregister channel full, skipping client cleanup.")
				}
			}
		}
	}
}

// cleanupOnExit performs necessary cleanup actions when the Room's Run loop exits.
func (r *Room) cleanupOnExit() {
	r.logger.Info().Msg("Room Run loop finished. Notifying Manager for cleanup.")

	// stop the shutdown timer if it's still running
	if r.shutdownTimer != nil {
		r.shutdownTimer.Stop()
	}

	// notify Manager for cleanup
	func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Warn("Recovered from panic during Manager cleanup notification (channel likely closed).")
			}
		}()

		select {
		case r.cleanupChan <- RoomCleanupMsg{RoomCode: r.Code}:
			r.logger.Info().Msg("Sent cleanup notification to Manager.")
		default:
			r.logger.Warn().Msg("Manager cleanup channel blocked/full. Skipping cleanup notification.")
		}
	}()

	// 3. Close all client send channels
	r.mu.Lock()
	for _, client := range r.clients {
		select {
		case <-client.send:
		default:
			close(client.send)
		}
	}
	r.mu.Unlock()

	// 4. Safely close Room's own input channels
	select {
	case <-r.broadcast:
	default:
		close(r.broadcast)
	}
	select {
	case <-r.register:
	default:
		close(r.register)
	}
	select {
	case <-r.unregister:
	default:
		close(r.unregister)
	}
}

// RegisterClient safely adds a client to the registration queue.
func (r *Room) RegisterClient(client *Client) {
	select {
	case r.register <- client:
	default:
		r.logger.Warn().Msg("Room register channel blocked.")
		client.SendError(fmt.Errorf("room is busy, register channel blocked"))
	}
}

// IsFull checks if the room has reached its maximum client capacity.
// If checkID is provided (non-empty string), it first checks if that ID is already in the room.
// Existing clients are allowed to proceed (re-entry exemption) even if the room is technically full.
func (r *Room) IsFull(checkID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Re-entry Exemption Check
	if checkID != "" {
		if _, exists := r.clients[checkID]; exists {
			return false
		}
	}

	// Standard Capacity Check
	currentClients := len(r.clients)
	return r.MaxClients > 0 && currentClients >= r.MaxClients
}

// GetInitDataPayload prepares the InitDataPayload structure for a user joining the room.
func (r *Room) GetInitDataPayload(currentUser user.User) InitDataPayload {
	r.mu.RLock()
	defer r.mu.RUnlock()

	onlineUsers := make([]user.User, 0, len(r.clients))

	for _, client := range r.clients {
		onlineUsers = append(onlineUsers, client.user)
	}

	return InitDataPayload{
		CurrentUser: currentUser,
		OnlineUsers: onlineUsers,
		MaxUsers:    r.MaxClients,
	}
}
