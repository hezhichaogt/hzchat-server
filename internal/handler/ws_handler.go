package handler

import (
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"hzchat/internal/app/chat"
	"hzchat/internal/app/user"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/limiter"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/randx"
	"hzchat/internal/pkg/resp"
)

func HandleWebSocket(upgrader websocket.Upgrader, rateLimiter *limiter.IPRateLimiter, deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		if ip == "" {
			ip = "unknown_ip"
		}

		if !rateLimiter.GetLimiter(ip).Allow() {
			logx.Warn("WebSocket connection rejected: Rate limit exceeded.", "ip", ip)
			resp.RespondError(w, r, errs.NewError(errs.ErrRateLimitExceeded))
			return
		}

		roomCode := chi.URLParam(r, "code")
		if !randx.IsValidRoomCode(roomCode) {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		tokenString := r.URL.Query().Get("token")
		payload, err := jwt.ParseToken(tokenString, deps.Config.JWTSecret)

		if err != nil || payload.Code != roomCode {
			logx.Warn("WS connection rejected: Invalid or mismatched token", "room", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		var tokenExpiry time.Time
		if payload.ExpiresAt > 0 {
			tokenExpiry = time.Unix(payload.ExpiresAt, 0)
		} else {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		currentUser := user.User{
			ID:       payload.ID,
			Nickname: payload.Nickname,
			Avatar:   payload.Avatar,
			UserType: payload.UserType,
		}

		if currentUser.ID == "" || currentUser.Nickname == "" {
			logx.Warn("WS connection rejected: Incomplete payload data", "id", currentUser.ID)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		room := deps.Manager.GetRoom(roomCode)

		if room == nil {
			logx.Info("WebSocket connection rejected: Room not found.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		if room.IsFull(currentUser.ID) {
			logx.Info("WebSocket connection rejected: Room is full.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomIsFull))
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logx.Error(err, "Failed to upgrade connection to WebSocket")
			return
		}

		client := chat.NewClient(room, conn, currentUser, tokenExpiry)

		go client.WritePump()

		logx.Info("WebSocket connection established and client registered", "client_id", currentUser.ID, "room_code", roomCode)

		room.RegisterClient(client)

		client.ReadPump()
	}
}
