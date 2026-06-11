package config

import (
	"os"
	"strconv"
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
	schedulerWorkers := getEnvInt("SCHEDULER_WORKERS", 10)
	consumerWorkers := getEnvInt("CONSUMER_WORKERS", 10)
	return &Config{
		KafkaBrokers:       strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopic:         getEnv("KAFKA_TOPIC", "delivery-events"),
		KafkaGroupID:       getEnv("KAFKA_GROUP_ID", "delivery-group"),
		MetricsAddr:        getEnv("METRICS_ADDR", ":9095"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		BackoffBaseDelay:   getEnvDuration("BACKOFF_BASE_DELAY", 5*time.Second),
		BackoffMaxDelay:    getEnvDuration("BACKOFF_MAX_DELAY", 24*time.Hour),
		BackoffMaxAttempts: getEnvInt("BACKOFF_MAX_ATTEMPTS", 10),
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

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return defaultValue
	}
	return d
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return defaultValue
	}
	return n
}
