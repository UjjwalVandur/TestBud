package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/coverage"
	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/repository"
)

// CoverageReportResult is the JSON-friendly response returned by the coverage endpoint.
type CoverageReportResult struct {
	SchemaID      uuid.UUID                    `json:"schema_id"`
	EndpointPct   float64                      `json:"endpoint_pct"`
	Categories    map[string]coverage.CategoryStats `json:"categories"`
	ResponseCodes map[int]int                  `json:"response_codes"`
	Fields        coverage.FieldStats          `json:"fields"`
	GeneratedAt   time.Time                    `json:"generated_at"`
}

// CoverageService orchestrates coverage computation, persistence, and retrieval.
type CoverageService struct {
	schemaRepo   repository.SchemaRepository
	execRepo     repository.ExecutionRepository
	coverageRepo *repository.GormCoverageRepository
	logger       *slog.Logger
}

// NewCoverageService creates a new CoverageService.
func NewCoverageService(
	schemaRepo repository.SchemaRepository,
	execRepo repository.ExecutionRepository,
	coverageRepo *repository.GormCoverageRepository,
	logger *slog.Logger,
) *CoverageService {
	return &CoverageService{
		schemaRepo:   schemaRepo,
		execRepo:     execRepo,
		coverageRepo: coverageRepo,
		logger:       logger,
	}
}

// GetCoverageReport computes a fresh coverage report for the given schema,
// persists it (upsert), and returns the result.
func (s *CoverageService) GetCoverageReport(ctx context.Context, schemaID uuid.UUID) (*CoverageReportResult, error) {
	result, err := s.getCoverageResult(ctx, schemaID)
	if err != nil {
		return nil, err
	}

	// Serialize details into CategoryJSON for storage.
	detailsJSON, err := json.Marshal(coverage.CoverageDetails{
		Categories:    result.Categories,
		ResponseCodes: result.ResponseCodes,
		Fields:        result.Fields,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal coverage details: %w", err)
	}

	// Upsert the coverage report.
	report := &models.CoverageReport{
		SchemaID:     schemaID,
		EndpointPct:  result.EndpointPct,
		CategoryJSON: detailsJSON,
		GeneratedAt:  result.GeneratedAt,
	}
	if err := s.coverageRepo.UpsertReport(ctx, report); err != nil {
		return nil, fmt.Errorf("upsert report: %w", err)
	}

	return result, nil
}

// getCoverageResult loads data and computes coverage without persisting.
func (s *CoverageService) getCoverageResult(ctx context.Context, schemaID uuid.UUID) (*CoverageReportResult, error) {
	// 1. Load endpoints with test cases.
	endpoints, err := s.schemaRepo.GetEndpointsWithTestCases(ctx, schemaID)
	if err != nil {
		return nil, fmt.Errorf("load endpoints: %w", err)
	}

	// 2. Collect all test case IDs.
	var testCaseIDs []uuid.UUID
	for _, ep := range endpoints {
		for _, tc := range ep.TestCases {
			testCaseIDs = append(testCaseIDs, tc.ID)
		}
	}

	// 3. Load executions for those test cases.
	var executions []models.Execution
	if len(testCaseIDs) > 0 {
		executions, err = s.execRepo.GetExecutionsByTestCaseIDs(ctx, testCaseIDs)
		if err != nil {
			return nil, fmt.Errorf("load executions: %w", err)
		}
	}

	// 4. Compute coverage.
	endpointPct, details := coverage.ComputeCoverage(endpoints, executions)

	return &CoverageReportResult{
		SchemaID:      schemaID,
		EndpointPct:   endpointPct,
		Categories:    details.Categories,
		ResponseCodes: details.ResponseCodes,
		Fields:        details.Fields,
		GeneratedAt:   time.Now().UTC(),
	}, nil
}

