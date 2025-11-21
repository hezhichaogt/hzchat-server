/*
Package handler provides the HTTP handler function for WebSocket connection upgrading and initialization.

This file contains the HandleWebSocket function, which is responsible for rate limiting, validating
room and user parameters, upgrading the HTTP connection to WebSocket, and initiating the client lifecycle.
*/
package handler

import (
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"hzchat/internal/app/chat"
	"hzchat/internal/app/user"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/limiter"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/resp"
)

// HandleWebSocket creates an HTTP HandlerFunc to process WebSocket connection requests.
func HandleWebSocket(manager *chat.Manager, upgrader websocket.Upgrader, rateLimiter *limiter.IPRateLimiter) http.HandlerFunc {
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
			rateLimitErr := errs.NewError(errs.ErrRateLimitExceeded)
			resp.RespondError(w, r, rateLimitErr)
			return
		}

		roomCode := chi.URLParam(r, "code")
		if roomCode == "" {
			logx.Warn("WebSocket request rejected: Missing room code")
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		query := r.URL.Query()
		userID := query.Get("uid")
		nickName := query.Get("nn")

		if userID == "" || nickName == "" {
			logx.Warn("WebSocket request rejected: Missing uid or nn query parameters", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		room := manager.GetRoom(roomCode)
		if room == nil {
			logx.Info("WebSocket connection rejected: Room not found.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}
		if room.IsFull() {
			logx.Info("WebSocket connection rejected: Room is full.", "room_code", roomCode)
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomIsFull))
			return
		}

		logx.Info("Attempting to upgrade connection", "room_code", roomCode, "user_id", userID)

		currentUser := user.User{
			ID:       userID,
			Nickname: nickName,
			Avatar:   "",
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logx.Error(err, "Failed to upgrade connection to WebSocket")
			return
		}

		client := chat.NewClient(room, conn, currentUser)

		go client.WritePump()

		logx.Info("WebSocket connection established and client registered", "client_id", userID, "room_code", roomCode)

		room.RegisterClient(client)

		client.ReadPump()
	}
}
