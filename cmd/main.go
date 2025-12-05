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
	// Load configuration from environment variables
	cfg, err := configs.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize global logger
	logx.InitGlobalLogger(cfg.Environment == "development")
	logx.Logger().Info().
		Str("environment", cfg.Environment).
		Int("port", cfg.Port).
		Strs("allowed_origins", cfg.AllowedOrigins).
		Int("pow_difficulty", cfg.PowDifficulty).
		Msg("Configuration loaded successfully")

	// Create a context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize Chat Manager
	manager := chat.NewManager()

	// Setup HTTP server and routes
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

	// Wait for interrupt signal to gracefully shutdown the server with a timeout of 5 seconds.
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
