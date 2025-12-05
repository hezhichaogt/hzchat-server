/*
Package handler provides HTTP handler functions for managing room creation and status checks.
*/
package handler

import (
	"net/http"

	"hzchat/internal/app/chat"
	"hzchat/internal/configs"
	"hzchat/internal/pkg/auth/jwt"
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

// JoinRoomInput defines the JSON input structure received by the room join API endpoint.
type JoinRoomInput struct {
	// Code is the code of the chat room to join.
	Code string `json:"code" validate:"required"`

	// GuestID is the unique identifier for a guest.
	GuestID string `json:"guestID"`
}

// HandleJoinRoom processes the request to join a room.
func HandleJoinRoom(manager *chat.Manager, cfg *configs.AppConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input JoinRoomInput
		if customErr := req.BindJSON(r, &input); customErr != nil {
			logx.Warn("Failed to bind JSON for room join", "error", customErr)
			resp.RespondError(w, r, customErr)
			return
		}

		code := input.Code
		guestID := input.GuestID

		if !randx.IsValidGuestID(guestID) {
			logx.Warn("Invalid GuestID format or length", "guest_id", guestID, "room_code", code)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		if !randx.IsValidRoomCode(code) {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		room := manager.GetRoom(code)
		if room == nil {
			logx.Info("Room not found", "room_code", code)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		if room.IsFull(guestID) {
			logx.Info("Room is full for new joiner", "room_code", code)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomIsFull))
			return
		}

		// Generate token
		payload := &jwt.Payload{
			ID:       guestID,
			Code:     code,
			UserType: "guest",
		}

		tokenString, err := jwt.GenerateToken(payload, cfg.JWTSecret)
		if err != nil {
			logx.Warn("Failed to generate JWT token", "error", err)
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		data := map[string]any{
			"token": tokenString,
		}
		resp.RespondSuccess(w, r, data)
	}
}
