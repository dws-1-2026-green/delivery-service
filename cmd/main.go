package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/config"
	"github.com/jbisss/webhook-manager/delivery-service/internal/consumer"
	_ "github.com/jbisss/webhook-manager/delivery-service/internal/metrics"
	"github.com/jbisss/webhook-manager/delivery-service/internal/processor"
	"github.com/jbisss/webhook-manager/delivery-service/internal/service"
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

	httpClient := client.NewHTTPClient()
	deliveryService := service.NewRetryDeliveryService(httpClient)
	proc := processor.New(deliveryService)

	kafkaConsumer := consumer.New(
		cfg.KafkaBrokers,
		cfg.KafkaTopic,
		cfg.KafkaGroupID,
		proc,
	)

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(config.MetricsAddr, mux); err != nil {
			log.Printf("metrics server: %v", err)
		}
	}()

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
