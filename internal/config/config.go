package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HackerOneToken      string
	DatabasePath        string
	WebPort             string
	HealthCheckTimeout  time.Duration
	HealthCheckWorkers  int
	ScanInterval        time.Duration
	SubfinderConfigPath string
}

func Load() (*Config, error) {
	cfg := &Config{
		HackerOneToken:      getEnv("HACKERONE_TOKEN", ""),
		DatabasePath:        getEnv("DATABASE_PATH", "./watchtower.db"),
		WebPort:             getEnv("WEB_PORT", "8080"),
		HealthCheckTimeout:  getDurationEnv("HEALTH_CHECK_TIMEOUT", 10*time.Second),
		HealthCheckWorkers:  getIntEnv("HEALTH_CHECK_WORKERS", 50),
		ScanInterval:        getDurationEnv("SCAN_INTERVAL", 24*time.Hour),
		SubfinderConfigPath: getEnv("SUBFINDER_CONFIG", ""),
	}

	if cfg.HackerOneToken == "" {
		// Try to read from file
		if token, err := os.ReadFile(".hackerone_token"); err == nil {
			cfg.HackerOneToken = strings.TrimSpace(string(token))
		}
	}

	// Trim whitespace from token
	cfg.HackerOneToken = strings.TrimSpace(cfg.HackerOneToken)

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
