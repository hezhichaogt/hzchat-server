/*
Package handler provides the HTTP handler function for WebSocket connection upgrading and initialization.

This file contains the HandleWebSocket function, which is responsible for rate limiting, validating
room and user parameters, upgrading the HTTP connection to WebSocket, and initiating the client lifecycle.
*/
package handler

import (
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"hzchat/internal/app/chat"
	"hzchat/internal/app/user"
	"hzchat/internal/configs"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/limiter"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/resp"
)

// HandleWebSocket creates an HTTP HandlerFunc to process WebSocket connection requests.
func HandleWebSocket(manager *chat.Manager, upgrader websocket.Upgrader, rateLimiter *limiter.IPRateLimiter, cfg *configs.AppConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP address for rate limiting
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}

		if ip == "" {
			ip = "unknown_ip"
		}

		if !rateLimiter.GetLimiter(ip).Allow() {
			logx.Warn("WebSocket connection rejected: Rate limit exceeded.", "ip", ip)
			rateLimitErr := errs.NewError(errs.ErrRateLimitExceeded)
			resp.RespondError(w, r, rateLimitErr)
			return
		}

		// Extract room code from URL parameters
		roomCode := chi.URLParam(r, "code")
		if roomCode == "" {
			logx.Warn("WebSocket request rejected: Missing room code")
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		// Extract and validate JWT token from query parameters
		query := r.URL.Query()
		tokenString := query.Get("token")

		if tokenString == "" {
			logx.Warn("WebSocket request rejected: Missing 'token' query parameter.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		payload, err := jwt.ParseToken(tokenString, cfg.JWTSecret)
		if err != nil {
			logx.Warn("Failed to parse or validate JWT for WebSocket.", "room_code", roomCode, "error", err)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		if payload.Code != roomCode {
			logx.Warn("JWT room code mismatch.", "expected_code", roomCode, "token_code", payload.Code)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		// Check token expiration
		var tokenExpiry time.Time
		if payload.ExpiresAt > 0 {
			tokenExpiry = time.Unix(payload.ExpiresAt, 0)
		} else {
			logx.Warn("JWT is missing a valid Expiration claim (Exp). Connection rejected.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		userID := payload.ID
		userType := payload.UserType
		nickName := query.Get("nn")
		if userID == "" || nickName == "" {
			logx.Warn("WebSocket request rejected: Missing uid or nn query parameters", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		// check if room exists and if it's full
		room := manager.GetRoom(roomCode)

		if room == nil {
			logx.Info("WebSocket connection rejected: Room not found.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		if room.IsFull(userID) {
			logx.Info("WebSocket connection rejected: Room is full.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomIsFull))
			return
		}

		currentUser := user.User{
			ID:       userID,
			Nickname: nickName,
			Avatar:   "",
			UserType: userType,
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logx.Error(err, "Failed to upgrade connection to WebSocket")
			return
		}

		// Create a new chat client
		client := chat.NewClient(room, conn, currentUser, tokenExpiry)

		// Start the client's write pump in a new goroutine
		go client.WritePump()

		logx.Info("WebSocket connection established and client registered", "client_id", userID, "room_code", roomCode)

		// Register the client with the room
		room.RegisterClient(client)

		// Start the client's read pump (blocking call)
		client.ReadPump()
	}
}
