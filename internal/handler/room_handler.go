/*
Package handler provides HTTP handler functions for managing room creation and status checks.
*/
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"hzchat/internal/app/chat"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/randx"
	"hzchat/internal/pkg/req"
	"hzchat/internal/pkg/resp"
)

// CreateRoomInput defines the JSON input structure received by the room creation API endpoint.
type CreateRoomInput struct {
	// Type defines the type of room, e.g., "private" or "group".
	Type string `json:"type"`
	// MaxClients defines the maximum number of clients for the room (optional; can be omitted if the type determines the client count).
	MaxClients int `json:"max_clients,omitempty"`
}

// HandleCreateRoom creates an HTTP HandlerFunc to process room creation requests.
func HandleCreateRoom(manager *chat.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input CreateRoomInput

		if customErr := req.BindJSON(r, &input); customErr != nil {
			logx.Warn("Failed to bind JSON for room creation", "error", customErr)
			resp.RespondError(w, r, customErr)
			return
		}

		var maxClients int

		switch input.Type {
		case "private":
			maxClients = chat.PrivateMaxClients
		case "group":
			maxClients = chat.GroupMaxClients
		}

		if maxClients == 0 {
			customErr := errs.NewError(errs.ErrRoomTypeInvalid)
			logx.Warn("Invalid room type received", "type", input.Type)
			resp.RespondError(w, r, customErr)
			return
		}

		logx.Info("Attempting to create new room", "room_type", input.Type, "max_clients", maxClients)

		roomCode, err := randx.RoomCode()
		if err != nil {
			logx.Error(err, "Failed to generate room code")
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		room, createErr := manager.CreateRoom(roomCode, maxClients)
		if createErr != nil {
			logx.Warn("Failed to create room in manager", "room_code", roomCode, "error", createErr)
			resp.RespondError(w, r, createErr)
			return
		}

		logx.Info("Room created successfully", "room_code", room.Code)

		data := map[string]any{
			"chatCode": room.Code,
		}
		resp.RespondSuccess(w, r, data)
	}
}

// HandleCheckRoomStatus creates an HTTP HandlerFunc to check the status of a specified room.
// It validates the room code format, checks for room existence, and determines if the room is full.
func HandleCheckRoomStatus(manager *chat.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomCode := chi.URLParam(r, "code")

		if !randx.IsValidRoomCode(roomCode) {
			logx.Warn("Invalid room code parameter", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		room := manager.GetRoom(roomCode)
		if room == nil {
			logx.Info("Room not found", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		if room.IsFull() {
			logx.Info("Room is full", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomIsFull))
			return
		}

		checkInfo := map[string]any{
			"canJoin": true,
		}
		resp.RespondSuccess(w, r, checkInfo)
	}
}
