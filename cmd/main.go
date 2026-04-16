package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jbisss/webhook-manager/delivery-service/internal/client"
	"github.com/jbisss/webhook-manager/delivery-service/internal/config"
	"github.com/jbisss/webhook-manager/delivery-service/internal/consumer"
	"github.com/jbisss/webhook-manager/delivery-service/internal/processor"
	"github.com/jbisss/webhook-manager/delivery-service/internal/service"
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
