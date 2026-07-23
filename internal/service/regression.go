package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/regression"
	"github.com/UjjwalVandur/TestBud/internal/repository"
)

// RegressionService coordinates loading base/target schemas and computing
// the regression diff between them.
type RegressionService struct {
	schemaRepo repository.SchemaRepository
	logger     *logrus.Logger
}

// NewRegressionService creates a new RegressionService.
func NewRegressionService(schemaRepo repository.SchemaRepository, logger *logrus.Logger) *RegressionService {
	return &RegressionService{
		schemaRepo: schemaRepo,
		logger:     logger,
	}
}

// ErrSchemaNotFound is returned when the target schema does not exist.
var ErrSchemaNotFound = fmt.Errorf("schema not found")

// GetRegressionReport computes the diff between the given schema and its
// predecessor in the same project. If no predecessor exists, all target
// endpoints are reported as added.
func (s *RegressionService) GetRegressionReport(ctx context.Context, schemaID uuid.UUID) (*regression.RegressionReport, error) {
	// 1. Load target schema with endpoints.
	target, err := s.schemaRepo.FindByID(ctx, schemaID)
	if err != nil {
		return nil, fmt.Errorf("load target schema: %w", err)
	}
	if target == nil {
		return nil, ErrSchemaNotFound
	}

	// 2. Load predecessor schema with endpoints.
	predecessor, err := s.schemaRepo.FindPredecessorSchema(ctx, target.ProjectID, target.UploadedAt)
	if err != nil {
		return nil, fmt.Errorf("load predecessor schema: %w", err)
	}

	// 3. Compute diff.
	if predecessor != nil {
		report := regression.Detect(predecessor.Endpoints, target.Endpoints, predecessor.ID, target.ID)
		return &report, nil
	}

	// No predecessor — all endpoints are new.
	report := regression.Detect(nil, target.Endpoints, uuid.Nil, target.ID)
	return &report, nil
}
