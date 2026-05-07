package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	KafkaBrokers       []string
	KafkaTopic         string
	KafkaGroupID       string
	MetricsAddr        string
	DatabaseURL        string
	BackoffBaseDelay   time.Duration
	BackoffMaxDelay    time.Duration
	BackoffMaxAttempts int
	SchedulerWorkers   int
	ConsumerWorkers    int
	DBMaxConns         int
}

func LoadConfig() *Config {
	schedulerWorkers := 10
	consumerWorkers := 10
	return &Config{
		KafkaBrokers:       strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopic:         getEnv("KAFKA_TOPIC", "delivery-events"),
		KafkaGroupID:       getEnv("KAFKA_GROUP_ID", "delivery-group"),
		MetricsAddr:        getEnv("METRICS_ADDR", ":9095"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		BackoffBaseDelay:   5 * time.Second,
		BackoffMaxDelay:    24 * time.Hour,
		BackoffMaxAttempts: 10,
		SchedulerWorkers:   schedulerWorkers,
		ConsumerWorkers:    consumerWorkers,
		DBMaxConns:         schedulerWorkers + consumerWorkers + 2,
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
