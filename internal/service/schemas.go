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
	if prevSchema != nil {
		for _, ep := range prevSchema.Endpoints {
			key := fmt.Sprintf("%s:%s:%s", ep.Method, ep.Path, ep.EndpointHash)
			prevEndpoints[key] = ep.ID
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
				}
			}
			endpoints[i].TestCases = newCases
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
