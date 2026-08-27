package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

func GetEnv(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		slog.Warn("Environment variable not set, using default",
			slog.String("variable", key),
			slog.String("default", defaultValue),
		)
		return defaultValue
	}
	return val
}

func GetEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		slog.Warn("Environment variable not set, using default",
			slog.String("variable", key),
			slog.Int("default", defaultValue),
		)
		return defaultValue
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return result
}

func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		slog.Warn("Environment variable not set, using default",
			slog.String("variable", key),
			slog.Duration("default", defaultValue),
		)
		return defaultValue
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return result
}
