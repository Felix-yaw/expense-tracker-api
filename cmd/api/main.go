// Command api runs the expense-tracker HTTP server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Felix-yaw/expense-tracker-api/internal/api"
	"github.com/Felix-yaw/expense-tracker-api/internal/auth"
	"github.com/Felix-yaw/expense-tracker-api/internal/config"
	"github.com/Felix-yaw/expense-tracker-api/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	if cfg.UsesDefaultSecret() {
		logger.Warn("JWT_SECRET is unset; using an insecure development default")
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open store", "err", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	server := api.New(st, auth.New(cfg.JWTSecret, cfg.JWTTTL), cfg, logger)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server listening", "addr", httpServer.Addr, "db", cfg.DBPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}
