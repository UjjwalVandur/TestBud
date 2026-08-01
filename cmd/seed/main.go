// Package main provides a database seed script that creates a demo user
// with a well-known API key for local development and Docker demos.
//
// The seed is idempotent — it skips creation if the user already exists.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/config"
	"github.com/UjjwalVandur/TestBud/internal/database"
	"github.com/UjjwalVandur/TestBud/internal/models"
)

const (
	demoEmail    = "demo@testbud.local"
	demoAPIKey   = "testbud-demo-key-2026"
	demoPassword = "not-a-real-password-hash"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load(".")
	if err != nil {
		logger.WithError(err).Fatal("load config")
	}

	db, err := database.Connect(cfg)
	if err != nil {
		logger.WithError(err).Fatal("connect database")
	}

	// Check if the demo user already exists.
	var count int64
	db.Model(&models.User{}).Where("email = ?", demoEmail).Count(&count)
	if count > 0 {
		logger.Info("demo user already exists, skipping seed")
		os.Exit(0)
	}

	user := models.User{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Email:        demoEmail,
		PasswordHash: demoPassword,
		CreatedAt:    time.Now().UTC(),
		APIKey:       demoAPIKey,
	}

	if err := db.Create(&user).Error; err != nil {
		logger.WithError(err).Fatal("create demo user")
	}

	fmt.Printf("✓ Demo user created: email=%s api_key=%s\n", demoEmail, demoAPIKey)
}
