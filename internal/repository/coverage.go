package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

// GormCoverageRepository handles persistence for coverage reports.
type GormCoverageRepository struct {
	db *gorm.DB
}

// NewGormCoverageRepository creates a new GormCoverageRepository.
func NewGormCoverageRepository(db *gorm.DB) *GormCoverageRepository {
	return &GormCoverageRepository{db: db}
}

// UpsertReport inserts a new coverage report or updates the existing one for
// the same schema_id, using the unique index on schema_id.
func (r *GormCoverageRepository) UpsertReport(ctx context.Context, report *models.CoverageReport) error {
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "schema_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"endpoint_pct", "category_json", "generated_at"}),
		}).
		Create(report).Error
	if err != nil {
		return fmt.Errorf("upsert coverage report: %w", err)
	}
	return nil
}
