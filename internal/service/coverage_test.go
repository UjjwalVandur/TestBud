package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/repository"
)

// --- Fake SchemaRepository (embed interface, override only what's needed) ---

type fakeCovSchemaRepo struct {
	repository.SchemaRepository
	endpoints []models.Endpoint
	err       error
}

func (f *fakeCovSchemaRepo) GetEndpointsWithTestCases(_ context.Context, _ uuid.UUID) ([]models.Endpoint, error) {
	return f.endpoints, f.err
}

// --- Fake ExecutionRepository (embed interface, override only what's needed) ---

type fakeCovExecRepo struct {
	repository.ExecutionRepository
	executions []models.Execution
	err        error
}

func (f *fakeCovExecRepo) GetExecutionsByTestCaseIDs(_ context.Context, _ []uuid.UUID) ([]models.Execution, error) {
	return f.executions, f.err
}

// --- Tests ---

func TestGetCoverageReport_NoEndpoints(t *testing.T) {
	svc := &CoverageService{
		schemaRepo: &fakeCovSchemaRepo{endpoints: nil},
		execRepo:   &fakeCovExecRepo{},
		logger:     slog.Default(),
	}

	result, err := svc.getCoverageResult(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EndpointPct != 0 {
		t.Errorf("endpoint pct: got %v, want 0", result.EndpointPct)
	}
	for cat, stats := range result.Categories {
		if stats.Pct != 0 {
			t.Errorf("category %s pct: got %v, want 0", cat, stats.Pct)
		}
	}
}

func TestGetCoverageReport_NoExecutions(t *testing.T) {
	epID := uuid.New()
	tc1ID := uuid.New()

	svc := &CoverageService{
		schemaRepo: &fakeCovSchemaRepo{
			endpoints: []models.Endpoint{
				{
					ID:             epID,
					ParametersJSON: datatypes.JSON(`[]`),
					TestCases: []models.TestCase{
						{ID: tc1ID, EndpointID: epID, Category: models.CategoryPositive, PayloadJSON: datatypes.JSON(`{}`)},
					},
				},
			},
		},
		execRepo: &fakeCovExecRepo{executions: nil},
		logger:   slog.Default(),
	}

	result, err := svc.getCoverageResult(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EndpointPct != 0 {
		t.Errorf("endpoint pct: got %v, want 0", result.EndpointPct)
	}
	if result.Categories["positive"].Total != 1 {
		t.Errorf("positive total: got %d, want 1", result.Categories["positive"].Total)
	}
	if result.Categories["positive"].Executed != 0 {
		t.Errorf("positive executed: got %d, want 0", result.Categories["positive"].Executed)
	}
}

func TestGetCoverageReport_FullExecution(t *testing.T) {
	ep1ID := uuid.New()
	ep2ID := uuid.New()
	tc1ID := uuid.New()
	tc2ID := uuid.New()

	svc := &CoverageService{
		schemaRepo: &fakeCovSchemaRepo{
			endpoints: []models.Endpoint{
				{
					ID:             ep1ID,
					ParametersJSON: datatypes.JSON(`[]`),
					TestCases: []models.TestCase{
						{ID: tc1ID, EndpointID: ep1ID, Category: models.CategoryPositive, PayloadJSON: datatypes.JSON(`{}`)},
					},
				},
				{
					ID:             ep2ID,
					ParametersJSON: datatypes.JSON(`[]`),
					TestCases: []models.TestCase{
						{ID: tc2ID, EndpointID: ep2ID, Category: models.CategoryNegative, PayloadJSON: datatypes.JSON(`{}`)},
					},
				},
			},
		},
		execRepo: &fakeCovExecRepo{
			executions: []models.Execution{
				{TestCaseID: tc1ID, ActualStatus: 200, Passed: true},
				{TestCaseID: tc2ID, ActualStatus: 400, Passed: true},
			},
		},
		logger: slog.Default(),
	}

	result, err := svc.getCoverageResult(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EndpointPct != 100 {
		t.Errorf("endpoint pct: got %v, want 100", result.EndpointPct)
	}
	if result.Categories["positive"].Pct != 100 {
		t.Errorf("positive pct: got %v, want 100", result.Categories["positive"].Pct)
	}
	if result.Categories["negative"].Pct != 100 {
		t.Errorf("negative pct: got %v, want 100", result.Categories["negative"].Pct)
	}
	if result.ResponseCodes[200] != 1 {
		t.Errorf("response code 200: got %d, want 1", result.ResponseCodes[200])
	}
}

func TestGetCoverageReport_PartialExecution(t *testing.T) {
	ep1ID := uuid.New()
	ep2ID := uuid.New()
	tc1ID := uuid.New()
	tc2ID := uuid.New()

	svc := &CoverageService{
		schemaRepo: &fakeCovSchemaRepo{
			endpoints: []models.Endpoint{
				{
					ID:             ep1ID,
					ParametersJSON: datatypes.JSON(`[]`),
					TestCases: []models.TestCase{
						{ID: tc1ID, EndpointID: ep1ID, Category: models.CategoryPositive, PayloadJSON: datatypes.JSON(`{}`)},
					},
				},
				{
					ID:             ep2ID,
					ParametersJSON: datatypes.JSON(`[]`),
					TestCases: []models.TestCase{
						{ID: tc2ID, EndpointID: ep2ID, Category: models.CategoryBoundary, PayloadJSON: datatypes.JSON(`{}`)},
					},
				},
			},
		},
		execRepo: &fakeCovExecRepo{
			executions: []models.Execution{
				{TestCaseID: tc1ID, ActualStatus: 200, Passed: true},
			},
		},
		logger: slog.Default(),
	}

	result, err := svc.getCoverageResult(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EndpointPct != 50 {
		t.Errorf("endpoint pct: got %v, want 50", result.EndpointPct)
	}
	if result.Categories["positive"].Pct != 100 {
		t.Errorf("positive pct: got %v, want 100", result.Categories["positive"].Pct)
	}
	if result.Categories["boundary"].Pct != 0 {
		t.Errorf("boundary pct: got %v, want 0", result.Categories["boundary"].Pct)
	}
}
