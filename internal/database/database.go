package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/UjjwalVandur/TestBud/internal/config"
	"github.com/UjjwalVandur/TestBud/internal/models"
)

func Connect(cfg config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	if cfg.AutoMigrate {
		if err := db.AutoMigrate(&models.User{}, &models.Schema{}, &models.Endpoint{}, &models.TestCase{}, &models.Execution{}, &models.CoverageReport{}); err != nil {
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
	}

	return db, nil
}
