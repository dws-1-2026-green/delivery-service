package config

import (
	"os"
	"strings"
)

type Config struct {
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
	MetricsAddr  string
}

func LoadConfig() *Config {
	return &Config{
		KafkaBrokers: strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "delivery-events"),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "delivery-group"),
		MetricsAddr:  getEnv("METRICS_ADDR", ":9095"),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
