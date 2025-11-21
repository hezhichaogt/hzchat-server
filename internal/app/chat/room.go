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
	// unique identifier for the room.
	Code string

	// maximum number of users allowed in the room.
	MaxClients int

	// a map of currently connected clients, keyed by their user ID.
	clients map[string]*Client

	// a buffered channel for incoming messages to be sent to all clients.
	broadcast chan Message

	// a channel for clients requesting to join the room.
	register chan *Client

	// a channel for clients requesting to leave the room.
	unregister chan *Client

	// a write-only channel used to notify the Chat Manager to clean up this room.
	cleanupChan chan<- RoomCleanupMsg

	// used to signal the Room to stop its Run loop immediately.
	stopChan chan struct{}

	// the timer used to track room inactivity.
	shutdownTimer *time.Timer

	// mu protects access to the clients map.
	mu sync.RWMutex

	// structured logger with room context.
	logger zerolog.Logger
}

// NewRoom creates and initializes a new Room instance.
func NewRoom(roomCode string, maxClients int, cleanupChan chan<- RoomCleanupMsg) *Room {
	roomLogger := logx.Logger().With().
		Str("room_code", roomCode).
		Logger()

	return &Room{
		Code:          roomCode,
		MaxClients:    maxClients,
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

// Stop sends a signal to immediately terminate the Room's Run loop.
func (r *Room) Stop() {
	r.logger.Info().Msg("Received stop signal. Stopping room immediately.")

	select {
	case <-r.stopChan:
	default:
		close(r.stopChan)
	}
}

// Run starts the main event loop for the Room.
// It handles client registration, deregistration, message broadcasting, and room shutdown.
func (r *Room) Run() {
	defer func() {
		r.logger.Info().Msg("Room Run loop finished. Notifying Manager for cleanup.")

		if r.shutdownTimer != nil {
			r.shutdownTimer.Stop()
		}

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

		r.mu.Lock()
		for _, client := range r.clients {
			select {
			case <-client.send:
			default:
				close(client.send)
			}
		}
		r.mu.Unlock()

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
	}()

	timerChan := r.shutdownTimer.C

	for {
		select {
		case client := <-r.register:
			r.mu.Lock()

			if existingClient, ok := r.clients[client.user.ID]; ok {
				r.logger.Warn().
					Str("client_id", client.user.ID).
					Msg("Client ID already connected. Closing old connection for replacement.")

				existingClient.Kick("Session replaced by new connection. Check other tabs.")
			}

			if r.shutdownTimer != nil {
				if r.shutdownTimer.Stop() {
					select {
					case <-r.shutdownTimer.C:
					default:
					}
				}
			}

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
				continue
			}

			r.clients[client.user.ID] = client
			r.logger.Info().
				Str("client_id", client.user.ID).
				Int("total_users", len(r.clients)).
				Msg("Client joined room.")

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

			err := client.SendInitData(initDataPayload)
			if err != nil {
				r.unregister <- client
				continue
			}

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

		case client := <-r.unregister:
			r.mu.Lock()

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
			} else if ok && currentClient != client {
				r.logger.Info().
					Str("stale_client_id", client.user.ID).
					Msg("Ignoring unregister for STALE connection.")

			} else {
				r.logger.Warn().
					Str("client_id", client.user.ID).
					Msg("Unregister failed for unknown/already deleted client.")
			}

			if len(r.clients) == 0 {
				r.logger.Info().Msg("Room is empty. Shutting down Room.Run() loop.")
				if r.shutdownTimer.Stop() {
					select {
					case <-r.shutdownTimer.C:
					default:
					}
				}
				r.shutdownTimer.Reset(RoomInactivityTimeout)
			}

			r.mu.Unlock()

		case message := <-r.broadcast:
			if message.Type == TypeError {
				r.logger.Warn().
					Interface("message", message).
					Msg("Received unhandled TypeError in broadcast channel.")
				continue
			}

			messageBytes, err := json.Marshal(message)
			if err != nil {
				r.logger.Error().
					Str("message_id", message.ID).
					Err(err).
					Msg("Error marshaling message for broadcast.")
				continue
			}

			r.mu.RLock()
			senderID := message.Sender.ID
			for _, client := range r.clients {
				if client.user.ID != senderID {
					select {
					case client.send <- messageBytes:
					default:
						r.logger.Warn().
							Str("client_id", client.user.ID).
							Msg("Client send channel full or closed, unregistering.")

						select {
						case r.unregister <- client:
						default:
							r.logger.Warn().Msg("Unregister channel full, skipping client cleanup.")
						}
					}
				}
			}
			r.mu.RUnlock()

		case <-timerChan:
			r.logger.Info().Msgf("Room inactivity timeout (%s) reached. Shutting down Room.Run() loop.", RoomInactivityTimeout)
			return

		case <-r.stopChan:
			r.logger.Info().Msg("Room forced stop initiated.")
			return
		}
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
func (r *Room) IsFull() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

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
