package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jbisss/webhook-manager/delivery-service/internal/backoff"
	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/config"
	"github.com/jbisss/webhook-manager/delivery-service/internal/consumer"
	_ "github.com/jbisss/webhook-manager/delivery-service/internal/metrics"
	"github.com/jbisss/webhook-manager/delivery-service/internal/processor"
	"github.com/jbisss/webhook-manager/delivery-service/internal/scheduler"
	"github.com/jbisss/webhook-manager/delivery-service/internal/service"
	"github.com/jbisss/webhook-manager/delivery-service/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func setupLogger() {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}

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
	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

func main() {
	setupLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.LoadConfig()

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		slog.Info("metrics server starting", slog.String("addr", cfg.MetricsAddr))
		if err := http.ListenAndServe(cfg.MetricsAddr, mux); err != nil {
			slog.Error("metrics server error", slog.Any("error", err))
		}
	}()

	backoffCfg := backoff.Config{
		BaseDelay:   cfg.BackoffBaseDelay,
		MaxDelay:    cfg.BackoffMaxDelay,
		MaxAttempts: cfg.BackoffMaxAttempts,
	}

	var deliveryStore store.DeliveryStore = store.NopStore{}
	if cfg.DatabaseURL != "" {
		poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
		if err != nil {
			slog.Error("failed to parse database url", slog.Any("error", err))
			os.Exit(1)
		}
		poolCfg.MaxConns = int32(cfg.DBMaxConns)
		pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
		if err != nil {
			slog.Error("failed to connect to postgres", slog.Any("error", err))
			os.Exit(1)
		}
		defer pool.Close()

		ps, err := store.NewPostgresStore(ctx, pool)
		if err != nil {
			slog.Error("failed to init delivery store", slog.Any("error", err))
			os.Exit(1)
		}
		deliveryStore = ps
		slog.Info("delivery store initialized (postgres)")
	} else {
		slog.Info("delivery store disabled (DATABASE_URL not set)")
	}

	httpClient := client.NewHTTPClient()

	sched := scheduler.New(deliveryStore, httpClient, backoffCfg, cfg.SchedulerWorkers)
	go sched.Run(ctx)

	deliveryService := service.NewRetryDeliveryService(httpClient, deliveryStore, backoffCfg)
	proc := processor.New(deliveryService)

	kafkaConsumer := consumer.New(
		cfg.KafkaBrokers,
		cfg.KafkaTopic,
		cfg.KafkaGroupID,
		proc,
		cfg.ConsumerWorkers,
	)

	go func() {
		if err := kafkaConsumer.Start(ctx); err != nil {
			slog.Error("consumer stopped", slog.Any("error", err))
			cancel()
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	slog.Info("shutting down")
}
