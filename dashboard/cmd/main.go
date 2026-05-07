package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jbisss/webhook-manager/delivery-dashboard/internal/config"
	"github.com/jbisss/webhook-manager/delivery-dashboard/internal/handler"
	"github.com/jbisss/webhook-manager/delivery-dashboard/internal/store"
)

func main() {
	setupLogger()

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("postgres ping failed", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("connected to postgres")

	mux := http.NewServeMux()
	mux.Handle("/", handler.New(store.NewPostgresStore(pool)))

	slog.Info("dashboard starting", slog.String("addr", cfg.Addr))
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil {
		slog.Error("server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func setupLogger() {
	levelStr := os.Getenv("LOG_LEVEL")
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}
