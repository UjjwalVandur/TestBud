package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

type SchemaRepository interface {
	CreateSchema(ctx context.Context, schema *models.Schema, endpoints []models.Endpoint) error
}

type GormSchemaRepository struct {
	db *gorm.DB
}

func NewGormSchemaRepository(db *gorm.DB) *GormSchemaRepository {
	return &GormSchemaRepository{db: db}
}

func (r *GormSchemaRepository) CreateSchema(ctx context.Context, schema *models.Schema, endpoints []models.Endpoint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(schema).Error; err != nil {
			return fmt.Errorf("create schema: %w", err)
		}

		for i := range endpoints {
			endpoints[i].SchemaID = schema.ID
		}
		if len(endpoints) > 0 {
			if err := tx.Create(&endpoints).Error; err != nil {
				return fmt.Errorf("create endpoints: %w", err)
			}
		}

		return nil
	})
}
