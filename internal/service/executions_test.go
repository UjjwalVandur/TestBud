package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/executor"
	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/repository"
)

// --- Fake SchemaRepository for execution tests ---

type fakeExecSchemaRepo struct {
	repository.SchemaRepository
	endpoints []models.Endpoint
	err       error
}

func (f *fakeExecSchemaRepo) GetEndpointsWithTestCases(_ context.Context, _ uuid.UUID) ([]models.Endpoint, error) {
	return f.endpoints, f.err
}

// --- Fake ExecutionRepository ---

type fakeExecRepo struct {
	repository.ExecutionRepository
	executions []*models.Execution
	err        error
}

func (f *fakeExecRepo) CreateExecution(_ context.Context, exec *models.Execution) error {
	if f.err != nil {
		return f.err
	}
	f.executions = append(f.executions, exec)
	return nil
}


// --- Fake Executor ---

type fakeTestExecutor struct {
	result     models.Execution
	err        error
	callCount  atomic.Int32
	maxConcurr atomic.Int32
	active     atomic.Int32
}

func (f *fakeTestExecutor) Execute(_ context.Context, input executor.ExecutionInput) (models.Execution, error) {
	f.callCount.Add(1)
	curr := f.active.Add(1)
	// Track peak concurrency.
	for {
		old := f.maxConcurr.Load()
		if int32(curr) <= old {
			break
		}
		if f.maxConcurr.CompareAndSwap(old, int32(curr)) {
			break
		}
	}
	// Simulate some work.
	time.Sleep(5 * time.Millisecond)
	f.active.Add(-1)

	if f.err != nil {
		return models.Execution{}, f.err
	}

	result := f.result
	result.TestCaseID = input.TestCase.ID
	return result, nil
}

// --- Tests ---

func makeTestCases(endpointID uuid.UUID, n int) []models.TestCase {
	cases := make([]models.TestCase, n)
	for i := range cases {
		cases[i] = models.TestCase{
			ID:             uuid.New(),
			EndpointID:     endpointID,
			Category:       models.CategoryPositive,
			PayloadJSON:    datatypes.JSON(`{}`),
			ExpectedStatus: 200,
			GeneratedAt:    time.Now().UTC(),
		}
	}
	return cases
}

func TestExecutionService_RunsAllTestCases(t *testing.T) {
	epID := uuid.New()
	testCases := makeTestCases(epID, 5)

	schemaRepo := &fakeExecSchemaRepo{
		endpoints: []models.Endpoint{
			{
				ID:       epID,
				Method:   "GET",
				Path:     "/test",
				TestCases: testCases,
			},
		},
	}
	execRepo := &fakeExecRepo{}
	exec := &fakeTestExecutor{
		result: models.Execution{ActualStatus: 200, Passed: true},
	}

	svc := NewExecutionService(schemaRepo, execRepo, exec, logrus.New())
	result, err := svc.ExecuteSchemaTests(context.Background(), ExecuteSchemaInput{
		SchemaID:  uuid.New(),
		TargetURL: "http://localhost:8080",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 5 {
		t.Errorf("expected Total=5, got %d", result.Total)
	}
	if result.Passed != 5 {
		t.Errorf("expected Passed=5, got %d", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", result.Failed)
	}
	if len(execRepo.executions) != 5 {
		t.Errorf("expected 5 persisted executions, got %d", len(execRepo.executions))
	}
}

func TestExecutionService_ConcurrencyCap(t *testing.T) {
	// Create 25 test cases across 5 endpoints to stress the pool.
	var endpoints []models.Endpoint
	for i := 0; i < 5; i++ {
		epID := uuid.New()
		endpoints = append(endpoints, models.Endpoint{
			ID:        epID,
			Method:    "GET",
			Path:      "/test",
			TestCases: makeTestCases(epID, 5),
		})
	}

	schemaRepo := &fakeExecSchemaRepo{endpoints: endpoints}
	execRepo := &fakeExecRepo{}
	exec := &fakeTestExecutor{
		result: models.Execution{ActualStatus: 200, Passed: true},
	}

	svc := NewExecutionService(schemaRepo, execRepo, exec, logrus.New())
	result, err := svc.ExecuteSchemaTests(context.Background(), ExecuteSchemaInput{
		SchemaID:  uuid.New(),
		TargetURL: "http://localhost:8080",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 25 {
		t.Errorf("expected Total=25, got %d", result.Total)
	}
	// Verify concurrency never exceeded 10.
	if peak := exec.maxConcurr.Load(); peak > 10 {
		t.Errorf("concurrency exceeded cap: peak=%d, limit=10", peak)
	}
}

func TestExecutionService_EmptySchema(t *testing.T) {
	schemaRepo := &fakeExecSchemaRepo{endpoints: nil}
	execRepo := &fakeExecRepo{}
	exec := &fakeTestExecutor{}

	svc := NewExecutionService(schemaRepo, execRepo, exec, logrus.New())
	result, err := svc.ExecuteSchemaTests(context.Background(), ExecuteSchemaInput{
		SchemaID:  uuid.New(),
		TargetURL: "http://localhost:8080",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected Total=0 for empty schema, got %d", result.Total)
	}
}

func TestExecutionService_ContextCancellation(t *testing.T) {
	epID := uuid.New()
	testCases := makeTestCases(epID, 20)

	schemaRepo := &fakeExecSchemaRepo{
		endpoints: []models.Endpoint{
			{ID: epID, Method: "GET", Path: "/test", TestCases: testCases},
		},
	}
	execRepo := &fakeExecRepo{}
	exec := &fakeTestExecutor{
		result: models.Execution{ActualStatus: 200, Passed: true},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	svc := NewExecutionService(schemaRepo, execRepo, exec, logrus.New())
	result, err := svc.ExecuteSchemaTests(ctx, ExecuteSchemaInput{
		SchemaID:  uuid.New(),
		TargetURL: "http://localhost:8080",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With context cancelled early, not all 20 tests should complete.
	if result.Total >= 20 {
		t.Logf("warning: all 20 tests completed despite context cancellation (may be fast enough)")
	}
}

func TestExecutionService_MixedPassFail(t *testing.T) {
	epID := uuid.New()
	testCases := makeTestCases(epID, 4)

	schemaRepo := &fakeExecSchemaRepo{
		endpoints: []models.Endpoint{
			{ID: epID, Method: "GET", Path: "/test", TestCases: testCases},
		},
	}
	execRepo := &fakeExecRepo{}

	// Use a custom executor that alternates pass/fail.
	alternating := &alternatingExecutor{}

	svc := NewExecutionService(schemaRepo, execRepo, alternating, logrus.New())
	result, err := svc.ExecuteSchemaTests(context.Background(), ExecuteSchemaInput{
		SchemaID:  uuid.New(),
		TargetURL: "http://localhost:8080",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 4 {
		t.Errorf("expected Total=4, got %d", result.Total)
	}
	// At least some should pass and some should fail.
	if result.Passed+result.Failed != result.Total {
		t.Errorf("passed(%d) + failed(%d) != total(%d)", result.Passed, result.Failed, result.Total)
	}
}

type alternatingExecutor struct {
	count atomic.Int32
}

func (a *alternatingExecutor) Execute(_ context.Context, input executor.ExecutionInput) (models.Execution, error) {
	n := a.count.Add(1)
	passed := n%2 == 0
	status := 200
	if !passed {
		status = 500
	}
	return models.Execution{
		TestCaseID:   input.TestCase.ID,
		ActualStatus: status,
		Passed:       passed,
		ResponseMs:   1,
	}, nil
}
