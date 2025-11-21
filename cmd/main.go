/*
Package main is the entry point for the HZ Chat application.

It is responsible for loading configuration, initializing the global logging system,
setting up the HTTP server, starting the WebSocket Hub (Chat Manager),
and gracefully handling operating system interrupt signals (SIGINT, SIGTERM)
to ensure a smooth server shutdown.
*/
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hzchat/internal/app/chat"
	"hzchat/internal/configs"
	"hzchat/internal/handler"
	"hzchat/internal/pkg/logx"
)

func main() {
	cfg, err := configs.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logx.InitGlobalLogger(cfg.Environment == "development")

	logx.Logger().Info().
		Str("environment", cfg.Environment).
		Int("port", cfg.Port).
		Strs("allowed_origins", cfg.AllowedOrigins).
		Int("pow_difficulty", cfg.PowDifficulty).
		Msg("Configuration loaded successfully")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	manager := chat.NewManager()

	router := handler.Router(manager, cfg)

	serverAddr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logx.Info(fmt.Sprintf("HZ Chat Server starting on http://localhost%s", serverAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logx.Fatal(err, "Server failed to start")
		}
	}()

	<-ctx.Done()
	logx.Info("Received shutdown signal. Starting graceful shutdown...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logx.Fatal(err, "Server forced to shutdown")
	}

	manager.Shutdown()

	logx.Info("Server gracefully stopped.")
}
