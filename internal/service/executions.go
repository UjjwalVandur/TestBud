package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/executor"
	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/repository"
)

const maxConcurrentWorkers = 10

// ExecuteSchemaInput holds the parameters for running all test cases of a schema.
type ExecuteSchemaInput struct {
	SchemaID       uuid.UUID
	TargetURL      string
	AuthHeaders    map[string]string
	AltAuthHeaders map[string]string
}

// ExecutionRunResult summarises the outcome of a test execution run.
type ExecutionRunResult struct {
	SchemaID uuid.UUID `json:"schema_id"`
	Total    int       `json:"total"`
	Passed   int       `json:"passed"`
	Failed   int       `json:"failed"`
}

// TestExecutor is the interface the service uses to run individual test cases.
// Matches *executor.Executor but allows test doubles.
type TestExecutor interface {
	Execute(ctx context.Context, input executor.ExecutionInput) (models.Execution, error)
}

// ExecutionService orchestrates concurrent test execution.
type ExecutionService struct {
	schemaRepo repository.SchemaRepository
	execRepo   repository.ExecutionRepository
	executor   TestExecutor
	logger     *logrus.Logger
}

// NewExecutionService creates a new ExecutionService.
func NewExecutionService(
	schemaRepo repository.SchemaRepository,
	execRepo repository.ExecutionRepository,
	exec TestExecutor,
	logger *logrus.Logger,
) *ExecutionService {
	return &ExecutionService{
		schemaRepo: schemaRepo,
		execRepo:   execRepo,
		executor:   exec,
		logger:     logger,
	}
}

// ExecuteSchemaTests runs all test cases for the given schema concurrently,
// streaming each result to the database as it completes.
func (s *ExecutionService) ExecuteSchemaTests(ctx context.Context, input ExecuteSchemaInput) (ExecutionRunResult, error) {
	endpoints, err := s.schemaRepo.GetEndpointsWithTestCases(ctx, input.SchemaID)
	if err != nil {
		return ExecutionRunResult{}, fmt.Errorf("load endpoints: %w", err)
	}

	// Flatten all test case work items.
	type workItem struct {
		endpoint models.Endpoint
		testCase models.TestCase
	}
	var items []workItem
	for _, ep := range endpoints {
		for _, tc := range ep.TestCases {
			items = append(items, workItem{endpoint: ep, testCase: tc})
		}
	}

	if len(items) == 0 {
		return ExecutionRunResult{SchemaID: input.SchemaID}, nil
	}

	// Semaphore-based worker pool (hard limit: 10 concurrent goroutines).
	sem := make(chan struct{}, maxConcurrentWorkers)
	resultsCh := make(chan models.Execution, maxConcurrentWorkers)

	// Dispatch workers in a separate goroutine so the collector below can
	// drain resultsCh concurrently — prevents deadlock when the channel fills.
	go func() {
		var wg sync.WaitGroup
		for _, item := range items {
			wg.Add(1)
			sem <- struct{}{} // Acquire semaphore slot.
			go func(ep models.Endpoint, tc models.TestCase) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore slot.

				// Check for context cancellation before executing.
				select {
				case <-ctx.Done():
					return
				default:
				}

				result, err := s.executor.Execute(ctx, executor.ExecutionInput{
					TargetURL:      input.TargetURL,
					Endpoint:       ep,
					TestCase:       tc,
					AuthHeaders:    input.AuthHeaders,
					AltAuthHeaders: input.AltAuthHeaders,
				})
				if err != nil {
					s.logger.WithError(err).WithField("test_case_id", tc.ID).
						Error("execute test case failed")
					return
				}

				resultsCh <- result
			}(item.endpoint, item.testCase)
		}
		wg.Wait()
		close(resultsCh)
	}()

	// Collector: stream each result to the database as it arrives.
	var total, passed, failed int
	for result := range resultsCh {
		total++
		if result.Passed {
			passed++
		} else {
			failed++
		}

		if err := s.execRepo.CreateExecution(ctx, &result); err != nil {
			s.logger.WithError(err).WithField("test_case_id", result.TestCaseID).
				Error("persist execution failed")
		}
	}

	return ExecutionRunResult{
		SchemaID: input.SchemaID,
		Total:    total,
		Passed:   passed,
		Failed:   failed,
	}, nil
}

// ExecutionItem is a single execution record in the dashboard list view.
type ExecutionItem struct {
	ExecutionID    uuid.UUID `json:"execution_id"`
	TestCaseID     uuid.UUID `json:"test_case_id"`
	Category       string    `json:"category"`
	ExpectedStatus int       `json:"expected_status"`
	ActualStatus   int       `json:"actual_status"`
	ResponseMs     int64     `json:"response_ms"`
	Passed         bool      `json:"passed"`
	RanAt          string    `json:"ran_at"`
}

// ExecutionSummary provides aggregate statistics for all executions of a schema.
type ExecutionSummary struct {
	Total         int     `json:"total"`
	Passed        int     `json:"passed"`
	Failed        int     `json:"failed"`
	AvgResponseMs float64 `json:"avg_response_ms"`
}

// ExecutionListResult is the full DTO returned by ListExecutions.
type ExecutionListResult struct {
	SchemaID   uuid.UUID       `json:"schema_id"`
	Summary    ExecutionSummary `json:"summary"`
	Executions []ExecutionItem  `json:"executions"`
}

// ListExecutions returns all execution records for the given schema with summary stats.
// Used by the dashboard execution history view.
func (s *ExecutionService) ListExecutions(ctx context.Context, schemaID uuid.UUID) (*ExecutionListResult, error) {
	executions, err := s.execRepo.GetExecutionsBySchemaID(ctx, schemaID)
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}

	var totalMs int64
	var passed, failed int
	items := make([]ExecutionItem, 0, len(executions))
	for _, ex := range executions {
		if ex.Passed {
			passed++
		} else {
			failed++
		}
		totalMs += ex.ResponseMs

		item := ExecutionItem{
			ExecutionID:  ex.ID,
			TestCaseID:   ex.TestCaseID,
			ActualStatus: ex.ActualStatus,
			ResponseMs:   ex.ResponseMs,
			Passed:       ex.Passed,
			RanAt:        ex.RanAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		// Populate test case context if preloaded.
		if ex.TestCase.ID != uuid.Nil {
			item.Category = string(ex.TestCase.Category)
			item.ExpectedStatus = ex.TestCase.ExpectedStatus
		}
		items = append(items, item)
	}

	var avgMs float64
	if len(executions) > 0 {
		avgMs = float64(totalMs) / float64(len(executions))
	}

	return &ExecutionListResult{
		SchemaID: schemaID,
		Summary: ExecutionSummary{
			Total:         len(executions),
			Passed:        passed,
			Failed:        failed,
			AvgResponseMs: avgMs,
		},
		Executions: items,
	}, nil
}
