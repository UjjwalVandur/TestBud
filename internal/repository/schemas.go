package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

type SchemaRepository interface {
	CreateSchema(ctx context.Context, schema *models.Schema, endpoints []models.Endpoint) error
	FindByProjectAndHash(ctx context.Context, projectID uuid.UUID, schemaHash string) (*models.Schema, error)
	FindByID(ctx context.Context, id uuid.UUID) (*models.Schema, error)
	FindLatestSchema(ctx context.Context, projectID uuid.UUID) (*models.Schema, error)
	FindPredecessorSchema(ctx context.Context, projectID uuid.UUID, currentUploadedAt time.Time) (*models.Schema, error)
	GetTestCasesByEndpoint(ctx context.Context, endpointID uuid.UUID) ([]models.TestCase, error)
	GetEndpointsWithTestCases(ctx context.Context, schemaID uuid.UUID) ([]models.Endpoint, error)
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

// FindLatestSchema returns the latest uploaded schema for the project,
// preloading its endpoints to compare hashes during re-upload deduplication.
func (r *GormSchemaRepository) FindLatestSchema(ctx context.Context, projectID uuid.UUID) (*models.Schema, error) {
	var schema models.Schema
	err := r.db.WithContext(ctx).
		Preload("Endpoints").
		Where("project_id = ?", projectID).
		Order("uploaded_at DESC").
		First(&schema).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find latest schema: %w", err)
	}
	return &schema, nil
}

// GetTestCasesByEndpoint returns all existing test cases for a given endpoint ID.
// Used to copy test cases for identical endpoints during deduplication.
func (r *GormSchemaRepository) GetTestCasesByEndpoint(ctx context.Context, endpointID uuid.UUID) ([]models.TestCase, error) {
	var testCases []models.TestCase
	err := r.db.WithContext(ctx).
		Where("endpoint_id = ?", endpointID).
		Find(&testCases).Error
	if err != nil {
		return nil, fmt.Errorf("get test cases by endpoint: %w", err)
	}
	return testCases, nil
}

// GetEndpointsWithTestCases returns all endpoints for a schema, preloading
// their test cases. Used by the execution engine to dispatch test runs.
func (r *GormSchemaRepository) GetEndpointsWithTestCases(ctx context.Context, schemaID uuid.UUID) ([]models.Endpoint, error) {
	var endpoints []models.Endpoint
	err := r.db.WithContext(ctx).
		Preload("TestCases").
		Where("schema_id = ?", schemaID).
		Find(&endpoints).Error
	if err != nil {
		return nil, fmt.Errorf("get endpoints with test cases: %w", err)
	}
	return endpoints, nil
}

// FindByID returns the schema with the given UUID, preloading its endpoints.
// Returns nil, nil if not found (consistent with FindByProjectAndHash pattern).
func (r *GormSchemaRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Schema, error) {
	var schema models.Schema
	err := r.db.WithContext(ctx).
		Preload("Endpoints").
		First(&schema, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find schema by id: %w", err)
	}
	return &schema, nil
}

// FindPredecessorSchema returns the schema uploaded immediately before
// currentUploadedAt for the given project, preloading its endpoints.
// Returns nil, nil if no predecessor exists.
func (r *GormSchemaRepository) FindPredecessorSchema(ctx context.Context, projectID uuid.UUID, currentUploadedAt time.Time) (*models.Schema, error) {
	var schema models.Schema
	err := r.db.WithContext(ctx).
		Preload("Endpoints").
		Where("project_id = ? AND uploaded_at < ?", projectID, currentUploadedAt).
		Order("uploaded_at DESC").
		First(&schema).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find predecessor schema: %w", err)
	}
	return &schema, nil
}
