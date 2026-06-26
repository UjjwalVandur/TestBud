package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv      string
	Port        string
	DatabaseURL string
	AutoMigrate bool
}

func Load(path string) (Config, error) {
	v := viper.New()
	v.SetConfigName(".env")
	v.AddConfigPath(path)
	v.SetConfigType("env")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("APP_ENV", "development")
	v.SetDefault("PORT", "8080")
	v.SetDefault("AUTO_MIGRATE", true)

	if err := v.ReadInConfig(); err != nil {
		var cfgNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &cfgNotFound) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := Config{
		AppEnv:      v.GetString("APP_ENV"),
		Port:        v.GetString("PORT"),
		DatabaseURL: v.GetString("DATABASE_URL"),
		AutoMigrate: v.GetBool("AUTO_MIGRATE"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}
