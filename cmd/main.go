package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/config"
	"github.com/jbisss/webhook-manager/delivery-service/internal/consumer"
	_ "github.com/jbisss/webhook-manager/delivery-service/internal/metrics"
	"github.com/jbisss/webhook-manager/delivery-service/internal/processor"
	"github.com/jbisss/webhook-manager/delivery-service/internal/service"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := config.LoadConfig()

	client := client.NewHTTPClient()
	deliveryService := service.NewRetryDeliveryService(client)
	processor := processor.New(deliveryService)

	kafkaConsumer := consumer.New(
		config.KafkaBrokers,
		config.KafkaTopic,
		config.KafkaGroupID,
		processor,
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
			log.Fatal(err)
		}
	}()

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Println("shutting down...")
}
