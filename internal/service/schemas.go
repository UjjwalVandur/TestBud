package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/parser"
	"github.com/UjjwalVandur/TestBud/internal/repository"
	"github.com/UjjwalVandur/TestBud/internal/util"
)

type SchemaParser interface {
	Parse(ctx context.Context, raw []byte) (parser.ParsedSchema, error)
}

type TestCaseGenerator interface {
	Generate(ctx context.Context, endpoint models.Endpoint) ([]models.TestCase, error)
}

type SchemaService struct {
	parser    SchemaParser
	repo      repository.SchemaRepository
	generator TestCaseGenerator
}

type UploadSchemaInput struct {
	ProjectID  uuid.UUID
	Version    string
	UploadedBy uuid.UUID
	RawBytes   []byte
}

type UploadSchemaResult struct {
	SchemaID       uuid.UUID `json:"schema_id"`
	SchemaHash     string    `json:"schema_hash"`
	OpenAPIVersion string    `json:"openapi_version"`
	EndpointCount  int       `json:"endpoint_count"`
}

func NewSchemaService(parser SchemaParser, repo repository.SchemaRepository, generator TestCaseGenerator) *SchemaService {
	return &SchemaService{parser: parser, repo: repo, generator: generator}
}

func (s *SchemaService) UploadSchema(ctx context.Context, input UploadSchemaInput) (UploadSchemaResult, error) {
	if input.ProjectID == uuid.Nil {
		return UploadSchemaResult{}, fmt.Errorf("project_id is required")
	}
	if input.UploadedBy == uuid.Nil {
		return UploadSchemaResult{}, fmt.Errorf("uploaded_by is required")
	}
	if input.Version == "" {
		return UploadSchemaResult{}, fmt.Errorf("version is required")
	}

	parsed, err := s.parser.Parse(ctx, input.RawBytes)
	if err != nil {
		return UploadSchemaResult{}, err
	}

	// DEV-2: Short-circuit if an identical schema already exists for this project.
	existing, err := s.repo.FindByProjectAndHash(ctx, input.ProjectID, parsed.SchemaHash)
	if err != nil {
		return UploadSchemaResult{}, fmt.Errorf("dedup check: %w", err)
	}
	if existing != nil {
		return UploadSchemaResult{
			SchemaID:       existing.ID,
			SchemaHash:     existing.SchemaHash,
			OpenAPIVersion: existing.OpenAPIVersion,
			EndpointCount:  len(parsed.Endpoints),
		}, nil
	}

	schema := &models.Schema{
		ProjectID:      input.ProjectID,
		Version:        input.Version,
		RawBytes:       input.RawBytes,
		SchemaHash:     parsed.SchemaHash,
		OpenAPIVersion: parsed.OpenAPIVersion,
		UploadedBy:     input.UploadedBy,
	}
	endpoints, err := toModelEndpoints(parsed.Endpoints)
	if err != nil {
		return UploadSchemaResult{}, err
	}

	// Retrieve the latest schema for deduplication comparison (Week 2 dedup logic)
	prevSchema, err := s.repo.FindLatestSchema(ctx, input.ProjectID)
	if err != nil {
		return UploadSchemaResult{}, fmt.Errorf("dedup previous schema lookup: %w", err)
	}

	prevEndpoints := make(map[string]uuid.UUID)
	prevEndpointsByRoute := make(map[string]models.Endpoint) // Week 5: for auth-change detection
	if prevSchema != nil {
		for _, ep := range prevSchema.Endpoints {
			key := fmt.Sprintf("%s:%s:%s", ep.Method, ep.Path, ep.EndpointHash)
			prevEndpoints[key] = ep.ID
			routeKey := fmt.Sprintf("%s:%s", ep.Method, ep.Path)
			prevEndpointsByRoute[routeKey] = ep
		}
	}

	// Generate new test cases or copy existing ones if unchanged
	for i := range endpoints {
		key := fmt.Sprintf("%s:%s:%s", endpoints[i].Method, endpoints[i].Path, endpoints[i].EndpointHash)
		if oldEndpointID, exists := prevEndpoints[key]; exists {
			// Deduplication hit: endpoint is identical, retrieve and copy existing test cases
			oldCases, err := s.repo.GetTestCasesByEndpoint(ctx, oldEndpointID)
			if err != nil {
				return UploadSchemaResult{}, fmt.Errorf("get identical endpoint test cases: %w", err)
			}
			newCases := make([]models.TestCase, len(oldCases))
			for j, tc := range oldCases {
				newCases[j] = models.TestCase{
					Category:       tc.Category,
					PayloadJSON:    tc.PayloadJSON,
					ExpectedStatus: tc.ExpectedStatus,
					GeneratedAt:    tc.GeneratedAt, // DEV-11: preserve original generation timestamp
				}
			}
			endpoints[i].TestCases = newCases
		} else if oldEp, routeExists := prevEndpointsByRoute[fmt.Sprintf("%s:%s", endpoints[i].Method, endpoints[i].Path)]; routeExists && isAuthOnlyChange(oldEp, endpoints[i]) {
			// Week 5: Auth-only change — copy non-security test cases, regenerate security cases
			oldCases, err := s.repo.GetTestCasesByEndpoint(ctx, oldEp.ID)
			if err != nil {
				return UploadSchemaResult{}, fmt.Errorf("get auth-changed endpoint test cases: %w", err)
			}
			var copiedCases []models.TestCase
			for _, tc := range oldCases {
				if tc.Category != models.CategorySecurity {
					copiedCases = append(copiedCases, models.TestCase{
						Category:       tc.Category,
						PayloadJSON:    tc.PayloadJSON,
						ExpectedStatus: tc.ExpectedStatus,
						GeneratedAt:    tc.GeneratedAt,
					})
				}
			}
			// Generate fresh security cases for the new auth configuration
			allNewCases, err := s.generator.Generate(ctx, endpoints[i])
			if err != nil {
				return UploadSchemaResult{}, fmt.Errorf("generate security test cases: %w", err)
			}
			for _, tc := range allNewCases {
				if tc.Category == models.CategorySecurity {
					copiedCases = append(copiedCases, tc)
				}
			}
			endpoints[i].TestCases = copiedCases
		} else {
			// Endpoint is new or changed: generate new test cases
			newCases, err := s.generator.Generate(ctx, endpoints[i])
			if err != nil {
				return UploadSchemaResult{}, fmt.Errorf("generate test cases: %w", err)
			}
			endpoints[i].TestCases = newCases
		}
	}

	if err := s.repo.CreateSchema(ctx, schema, endpoints); err != nil {
		return UploadSchemaResult{}, err
	}

	return UploadSchemaResult{
		SchemaID:       schema.ID,
		SchemaHash:     schema.SchemaHash,
		OpenAPIVersion: schema.OpenAPIVersion,
		EndpointCount:  len(endpoints),
	}, nil
}

// SchemaListItem is the lightweight DTO returned by ListSchemas for the dashboard.
type SchemaListItem struct {
	SchemaID       uuid.UUID `json:"schema_id"`
	ProjectID      uuid.UUID `json:"project_id"`
	Version        string    `json:"version"`
	OpenAPIVersion string    `json:"openapi_version"`
	EndpointCount  int       `json:"endpoint_count"`
	UploadedAt     string    `json:"uploaded_at"`
}

// ListSchemas returns all schemas uploaded by the given user, optionally filtered
// by project. Used by the dashboard schema list view.
func (s *SchemaService) ListSchemas(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) ([]SchemaListItem, error) {
	schemas, err := s.repo.FindByUploadedBy(ctx, userID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}

	items := make([]SchemaListItem, 0, len(schemas))
	for _, schema := range schemas {
		// Use GetEndpointsWithTestCases to count endpoints without loading
		// full test case bodies — endpoints are lightweight here.
		endpoints, err := s.repo.GetEndpointsWithTestCases(ctx, schema.ID)
		if err != nil {
			return nil, fmt.Errorf("count endpoints for schema %s: %w", schema.ID, err)
		}
		items = append(items, SchemaListItem{
			SchemaID:       schema.ID,
			ProjectID:      schema.ProjectID,
			Version:        schema.Version,
			OpenAPIVersion: schema.OpenAPIVersion,
			EndpointCount:  len(endpoints),
			UploadedAt:     schema.UploadedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return items, nil
}

// EndpointDetail is a DTO for a single endpoint in the schema detail view.
type EndpointDetail struct {
	EndpointID   uuid.UUID         `json:"endpoint_id"`
	Method       string            `json:"method"`
	Path         string            `json:"path"`
	AuthRequired bool              `json:"auth_required"`
	TestCounts   map[string]int    `json:"test_counts"`
	TotalTests   int               `json:"total_tests"`
}

type TestCaseDetail struct {
	ID             uuid.UUID       `json:"id"`
	Category       string          `json:"category"`
	PayloadJSON    json.RawMessage `json:"payload_json"`
	ExpectedStatus int             `json:"expected_status"`
}

// SchemaDetailResult is the full DTO returned by GetSchemaDetail for the detail view.
type SchemaDetailResult struct {
	SchemaID       uuid.UUID        `json:"schema_id"`
	ProjectID      uuid.UUID        `json:"project_id"`
	Version        string           `json:"version"`
	OpenAPIVersion string           `json:"openapi_version"`
	UploadedAt     string           `json:"uploaded_at"`
	Endpoints      []EndpointDetail `json:"endpoints"`
	TotalEndpoints int              `json:"total_endpoints"`
	TotalTestCases int              `json:"total_test_cases"`
}

// GetTestCasesByEndpoint returns the generated test cases for a specific endpoint.
func (s *SchemaService) GetTestCasesByEndpoint(ctx context.Context, endpointID uuid.UUID) ([]TestCaseDetail, error) {
	testCases, err := s.repo.GetTestCasesByEndpoint(ctx, endpointID)
	if err != nil {
		return nil, fmt.Errorf("get test cases by endpoint: %w", err)
	}

	result := make([]TestCaseDetail, len(testCases))
	for i, tc := range testCases {
		result[i] = TestCaseDetail{
			ID:             tc.ID,
			Category:       string(tc.Category),
			PayloadJSON:    json.RawMessage(tc.PayloadJSON),
			ExpectedStatus: tc.ExpectedStatus,
		}
	}
	return result, nil
}

// GetSchemaDetail returns full schema details with endpoint and test case breakdowns.
func (s *SchemaService) GetSchemaDetail(ctx context.Context, schemaID uuid.UUID) (*SchemaDetailResult, error) {
	schema, err := s.repo.FindByIDWithDetails(ctx, schemaID)
	if err != nil {
		return nil, fmt.Errorf("get schema detail: %w", err)
	}
	if schema == nil {
		return nil, ErrSchemaNotFound
	}

	var totalTestCases int
	endpoints := make([]EndpointDetail, 0, len(schema.Endpoints))
	for _, ep := range schema.Endpoints {
		counts := make(map[string]int)
		for _, tc := range ep.TestCases {
			counts[string(tc.Category)]++
		}
		total := len(ep.TestCases)
		totalTestCases += total
		endpoints = append(endpoints, EndpointDetail{
			EndpointID:   ep.ID,
			Method:       ep.Method,
			Path:         ep.Path,
			AuthRequired: ep.AuthRequired,
			TestCounts:   counts,
			TotalTests:   total,
		})
	}

	return &SchemaDetailResult{
		SchemaID:       schema.ID,
		ProjectID:      schema.ProjectID,
		Version:        schema.Version,
		OpenAPIVersion: schema.OpenAPIVersion,
		UploadedAt:     schema.UploadedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Endpoints:      endpoints,
		TotalEndpoints: len(endpoints),
		TotalTestCases: totalTestCases,
	}, nil
}


func toModelEndpoints(endpoints []parser.Endpoint) ([]models.Endpoint, error) {
	modelEndpoints := make([]models.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		parametersJSON, err := toJSON(endpoint.ParametersJSON)
		if err != nil {
			return nil, fmt.Errorf("%s %s parameters json: %w", endpoint.Method, endpoint.Path, err)
		}
		requestSchemaJSON, err := toJSON(endpoint.RequestSchemaJSON)
		if err != nil {
			return nil, fmt.Errorf("%s %s request schema json: %w", endpoint.Method, endpoint.Path, err)
		}
		responseSchemaJSON, err := toJSON(endpoint.ResponseSchemaJSON)
		if err != nil {
			return nil, fmt.Errorf("%s %s response schema json: %w", endpoint.Method, endpoint.Path, err)
		}

		modelEndpoints = append(modelEndpoints, models.Endpoint{
			Method:             endpoint.Method,
			Path:               endpoint.Path,
			EndpointHash:       endpoint.EndpointHash,
			AuthRequired:       endpoint.AuthRequired,
			ParametersJSON:     parametersJSON,
			RequestSchemaJSON:  requestSchemaJSON,
			ResponseSchemaJSON: responseSchemaJSON,
		})
	}
	return modelEndpoints, nil
}

func toJSON(raw json.RawMessage) (datatypes.JSON, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("invalid json")
	}
	return datatypes.JSON(raw), nil
}

// isAuthOnlyChange returns true when two endpoints at the same Method+Path differ
// only in AuthRequired — their ParametersJSON, RequestSchemaJSON, and
// ResponseSchemaJSON are semantically equal. Used during upload dedup to decide
// whether to regenerate only security test cases (Week 5).
func isAuthOnlyChange(old, new models.Endpoint) bool {
	if old.AuthRequired == new.AuthRequired {
		return false // auth didn't change
	}
	return util.JSONBytesEqual(old.ParametersJSON, new.ParametersJSON) &&
		util.JSONBytesEqual(old.RequestSchemaJSON, new.RequestSchemaJSON) &&
		util.JSONBytesEqual(old.ResponseSchemaJSON, new.ResponseSchemaJSON)
}


