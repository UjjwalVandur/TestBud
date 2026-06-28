package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

type SchemaRepository interface {
	CreateSchema(ctx context.Context, schema *models.Schema, endpoints []models.Endpoint) error
	FindByProjectAndHash(ctx context.Context, projectID uuid.UUID, schemaHash string) (*models.Schema, error)
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

// FindByProjectAndHash returns the existing schema for the given project and
// hash, or nil if none exists. Used for dedup on re-upload (DEV-2).
func (r *GormSchemaRepository) FindByProjectAndHash(ctx context.Context, projectID uuid.UUID, schemaHash string) (*models.Schema, error) {
	var schema models.Schema
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND schema_hash = ?", projectID, schemaHash).
		First(&schema).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find schema by project and hash: %w", err)
	}
	return &schema, nil
}
