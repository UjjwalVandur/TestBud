package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	AppEnv         string
	Port           string
	DatabaseURL    string
	AutoMigrate    bool
	CORSOrigins    []string
	BedrockRegion  string
	BedrockModelID string
	ClerkSecretKey string
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func parseEnvFile(path string) {
	data, err := os.ReadFile(filepath.Join(path, ".env"))
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			if _, exists := os.LookupEnv(parts[0]); !exists {
				// Remove quotes if present
				val := strings.Trim(parts[1], `"'`)
				os.Setenv(parts[0], val)
			}
		}
	}
}

func Load(path string) (Config, error) {
	parseEnvFile(path)

	cfg := Config{
		AppEnv:         getEnv("APP_ENV", "development"),
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		AutoMigrate:    getEnv("AUTO_MIGRATE", "true") == "true",
		BedrockRegion:  os.Getenv("AWS_BEDROCK_REGION"),
		BedrockModelID: os.Getenv("AWS_BEDROCK_MODEL_ID"),
		ClerkSecretKey: os.Getenv("CLERK_SECRET_KEY"),
	}

	cors := getEnv("CORS_ORIGINS", "http://localhost:3000")
	if cors != "" {
		for _, part := range strings.Split(cors, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				cfg.CORSOrigins = append(cfg.CORSOrigins, part)
			}
		}
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}
