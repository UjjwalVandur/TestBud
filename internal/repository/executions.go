package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

// ExecutionRepository defines persistence operations for test executions.
type ExecutionRepository interface {
	// CreateExecution persists a single execution result. Designed for streaming
	// results from the worker pool one at a time to avoid buffering everything.
	CreateExecution(ctx context.Context, execution *models.Execution) error

	// DeleteOldExecutions removes executions older than the given timestamp.
	// Returns the number of rows deleted. Used by the 90-day retention cron.
	DeleteOldExecutions(ctx context.Context, before time.Time) (int64, error)

	// GetExecutionsByTestCaseIDs loads all executions for the given test case IDs.
	// Used by coverage computation to determine which test cases were executed.
	GetExecutionsByTestCaseIDs(ctx context.Context, testCaseIDs []uuid.UUID) ([]models.Execution, error)
}

// GormExecutionRepository implements ExecutionRepository via GORM.
type GormExecutionRepository struct {
	db *gorm.DB
}

// NewGormExecutionRepository creates a new GormExecutionRepository.
func NewGormExecutionRepository(db *gorm.DB) *GormExecutionRepository {
	return &GormExecutionRepository{db: db}
}

// CreateExecution persists a single execution result.
func (r *GormExecutionRepository) CreateExecution(ctx context.Context, execution *models.Execution) error {
	if err := r.db.WithContext(ctx).Create(execution).Error; err != nil {
		return fmt.Errorf("create execution: %w", err)
	}
	return nil
}

// DeleteOldExecutions removes executions with ran_at before the given time.
func (r *GormExecutionRepository) DeleteOldExecutions(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("ran_at < ?", before).
		Delete(&models.Execution{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete old executions: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// GetExecutionsByTestCaseIDs loads all executions matching the given test case IDs.
func (r *GormExecutionRepository) GetExecutionsByTestCaseIDs(ctx context.Context, testCaseIDs []uuid.UUID) ([]models.Execution, error) {
	if len(testCaseIDs) == 0 {
		return nil, nil
	}
	var executions []models.Execution
	if err := r.db.WithContext(ctx).
		Where("test_case_id IN ?", testCaseIDs).
		Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("get executions by test case ids: %w", err)
	}
	return executions, nil
}
