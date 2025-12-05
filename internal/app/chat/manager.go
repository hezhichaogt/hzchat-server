/*
Package chat contains the core logic for handling real-time chat rooms, user connections, and message broadcasting.

This file defines the Manager struct, which serves as the central manager for the entire chat system.
It is responsible for creating, tracking, retrieving, and cleaning up all active Room instances.
*/
package chat

import (
	"sync"

	"github.com/rs/zerolog"

	"hzchat/internal/configs"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
)

// Manager struct is responsible for coordinating and managing all active chat rooms.
type Manager struct {
	// rooms stores a map of all Room instances, keyed by RoomCode.
	rooms map[string]*Room

	// Config holds the application's read-only configuration settings.
	config *configs.AppConfig

	// mu protects concurrent access to the rooms map.
	mu sync.RWMutex

	// the channel used by Rooms to notify the Manager to clean up and remove them.
	cleanup chan RoomCleanupMsg

	// wg is used to wait for the runCleanupLoop goroutine to finish during shutdown.
	wg sync.WaitGroup

	// structured logger with Manager context.
	logger zerolog.Logger
}

// NewManager constructs and returns a new Manager instance.
func NewManager(cfg *configs.AppConfig) *Manager {
	managerLogger := logx.Logger().With().Str("component", "Manager").Logger()

	m := &Manager{
		rooms:   make(map[string]*Room),
		cleanup: make(chan RoomCleanupMsg, 10),
		logger:  managerLogger,
		config:  cfg,
	}

	m.wg.Add(1)

	go m.runCleanupLoop()

	return m
}

// runCleanupLoop is a blocking loop that listens on the cleanup channel.
// When a RoomCleanupMsg is received, it calls deleteRoom to remove the corresponding room.
func (m *Manager) runCleanupLoop() {
	defer m.wg.Done()

	m.logger.Info().Msg("Cleanup loop started.")

	for msg := range m.cleanup {
		m.deleteRoom(msg.RoomCode)
	}

	m.logger.Info().Msg("Cleanup loop stopped.")
}

// deleteRoom removes the specified room from the Manager's rooms map.
func (m *Manager) deleteRoom(roomCode string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rooms[roomCode]; ok {
		delete(m.rooms, roomCode)
		m.logger.Info().Str("room_code", roomCode).Msg("Room successfully removed.")
	}
}

// CreateRoom creates a new Room instance, adds it to the managed list, and starts its Run loop.
func (m *Manager) CreateRoom(roomCode string, maxClients int) (*Room, *errs.CustomError) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rooms[roomCode]; ok {
		m.logger.Warn().Str("room_code", roomCode).Msg("Attempted to create existing room.")
		return nil, errs.NewError(errs.ErrRoomCodeExists)
	}

	newRoom := NewRoom(roomCode, maxClients, m.cleanup, m.config.JWTSecret)
	m.rooms[roomCode] = newRoom

	go newRoom.Run()

	m.logger.Info().Str("room_code", roomCode).Int("max_clients", maxClients).Msg("New Room created and started.")
	return newRoom, nil
}

// GetRoom retrieves a Room instance by its room code.
func (m *Manager) GetRoom(roomCode string) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, ok := m.rooms[roomCode]
	if !ok {
		return nil
	}
	return room
}

// Shutdown gracefully shuts down the Manager and all managed rooms.
// It stops all room Run loops, closes the cleanup channel, and waits for the cleanup goroutine to exit.
func (m *Manager) Shutdown() {
	m.logger.Info().Msg("Shutting down Manager cleanup loop...")

	m.mu.Lock()

	for _, room := range m.rooms {
		room.Stop()
	}
	m.rooms = nil

	m.mu.Unlock()

	close(m.cleanup)
	m.wg.Wait()

	m.logger.Info().Msg("Manager shutdown complete.")
}
