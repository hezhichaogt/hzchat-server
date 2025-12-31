/*
Package handler provides the HTTP handlers and routing setup for the HZ Chat Server.

This file defines the main Router, applying necessary middleware like logging, CORS,
and IP-based rate limiting before delegating requests to specific handlers (API and WebSocket).
*/
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"golang.org/x/time/rate"

	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/limiter"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/resp"
)

const (
	CreateRate  = 0.05
	CreateBurst = 2
	JoinRate    = 0.2
	JoinBurst   = 5
)

// Router sets up the main HTTP routing table (chi.Router) for the application.
// It initializes IP-based rate limiters, configures CORS, and applies global and per-route middleware.
// It requires the chat.Manager for business logic and the AppConfig for settings (like allowed origins).
func Router(deps *AppDeps) http.Handler {
	createLimiter := limiter.NewIPRateLimiter(rate.Limit(CreateRate), CreateBurst)
	joinLimiter := limiter.NewIPRateLimiter(rate.Limit(JoinRate), JoinBurst)

	r := chi.NewRouter()

	allowedOrigins := make(map[string]struct{})
	for _, origin := range deps.Config.AllowedOrigins {
		allowedOrigins[origin] = struct{}{}
	}

	var wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			if deps.Config.Environment == "development" {
				return true
			}

			origin := r.Header.Get("Origin")
			if _, ok := allowedOrigins[origin]; ok {
				return true
			}

			logx.Warn("WebSocket connection rejected: Origin not allowed.", "origin", origin)
			return false
		},
	}

	corsAllowedOrigins := []string{}
	if deps.Config.Environment == "development" {
		corsAllowedOrigins = []string{"*"}
	} else if len(deps.Config.AllowedOrigins) > 0 {
		corsAllowedOrigins = deps.Config.AllowedOrigins
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   corsAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(c.Handler)

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(logx.RequestLogger())
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		logx.Info("Health check endpoint hit")

		data := map[string]string{
			"status":  "ok",
			"service": "HZ Chat Server",
		}
		resp.RespondSuccess(w, r, data)
	})

	r.Route("/api", func(api chi.Router) {
		api.Use(jwt.IdentityExtractorMiddleware(deps.Config.JWTSecret))

		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", HandleRegister(deps))
			auth.Post("/login", HandleLogin(deps))
			auth.Post("/change-password", HandleChangePassword(deps))
		})

		api.Route("/user", func(user chi.Router) {
			user.Get("/profile", HandleGetUserProfile(deps))
			user.Post("/avatar/presign", HandlePresignAvatarURL(deps))
			user.Post("/profile", HandleUpdateUserProfile(deps))
		})

		rateLimitedCreateHandler := createLimiter.Middleware(HandleCreateRoom(deps))
		api.Post("/chat/create", http.HandlerFunc(rateLimitedCreateHandler.ServeHTTP))
		api.Post("/chat/join", HandleJoinRoom(deps))

		api.Post("/file/presign-upload", HandlePresignChatMessageURL(deps))
		api.Get("/file/presign-download", HandlePresignDownloadURL(deps))
	})

	r.Get("/ws/{code}", HandleWebSocket(wsUpgrader, joinLimiter, deps))

	return r
}
