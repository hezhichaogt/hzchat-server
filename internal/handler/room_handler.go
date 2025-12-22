/*
Package handler provides HTTP handler functions for managing room creation and status checks.
*/
package handler

import (
	"net/http"

	"hzchat/internal/app/chat"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/randx"
	"hzchat/internal/pkg/req"
	"hzchat/internal/pkg/resp"

	"github.com/jackc/pgx/v5/pgtype"
)

type CreateRoomInput struct {
	// Type defines the type of room, e.g., "private" or "group".
	Type string `json:"type"`
	// MaxClients defines the maximum number of clients for the room (optional; can be omitted if the type determines the client count).
	MaxClients int `json:"maxClients,omitempty"`
}

// HandleCreateRoom creates an HTTP HandlerFunc to process room creation requests.
func HandleCreateRoom(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input CreateRoomInput

		if customErr := req.BindJSON(r, &input); customErr != nil {
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
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomTypeInvalid))
			return
		}

		roomCode, err := randx.RoomCode()
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		room, createErr := deps.Manager.CreateRoom(roomCode, maxClients)
		if createErr != nil {
			resp.RespondError(w, r, createErr)
			return
		}

		data := map[string]any{
			"chatCode": room.Code,
		}
		resp.RespondSuccess(w, r, data)
	}
}

type JoinRoomInput struct {
	Code     string `json:"code" validate:"required"`
	GuestID  string `json:"guestId,omitempty"`
	Nickname string `json:"nickname,omitempty"`
}

// HandleJoinRoom processes the request to join a room.
func HandleJoinRoom(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := jwt.GetPayloadFromContext(r)

		var input JoinRoomInput
		if customErr := req.BindJSON(r, &input); customErr != nil {
			resp.RespondError(w, r, customErr)
			return
		}

		var finalID string
		var userType string
		var nickName string
		var avatar string

		if identity != nil {
			var userUUID pgtype.UUID

			if err := userUUID.Scan(identity.ID); err != nil {
				logx.Error(err, "Invalid UUID format in identity token", "id", identity.ID)
				resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
				return
			}

			dbUser, err := deps.DB.GetUserByID(r.Context(), userUUID)

			if err != nil {
				logx.Error(err, "Failed to fetch user by UUID", "id", identity.ID)
				resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
				return
			}

			finalID = identity.ID
			userType = "registered"
			nickName = dbUser.Nickname.String
			avatar = dbUser.AvatarUrl.String

		} else {
			if !randx.IsValidGuestID(input.GuestID) {
				logx.Warn("Invalid GuestID format in join request", "guest_id", input.GuestID)
				resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
				return
			}

			if input.Nickname == "" {
				logx.Warn("Guest nickname missing", "guest_id", input.GuestID)
				resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
				return
			}

			finalID = input.GuestID
			userType = "guest"
			nickName = input.Nickname
		}

		if !randx.IsValidRoomCode(input.Code) {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		room := deps.Manager.GetRoom(input.Code)

		if room == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		if room.IsFull(finalID) {
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomIsFull))
			return
		}

		payload := &jwt.Payload{
			ID:       finalID,
			Code:     input.Code,
			UserType: userType,
			Nickname: nickName,
			Avatar:   avatar,
		}

		tokenString, err := jwt.GenerateToken(payload, deps.Config.JWTSecret, jwt.RoomAccessExpiration)
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		resp.RespondSuccess(w, r, map[string]any{
			"token": tokenString,
		})
	}
}
