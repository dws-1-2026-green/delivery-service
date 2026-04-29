package config

import "os"

type Config struct {
	DatabaseURL string
	Addr        string
}

func Load() *Config {
	return &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Addr:        getEnv("ADDR", ":9096"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
