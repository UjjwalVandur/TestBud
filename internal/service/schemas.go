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

type SchemaService struct {
	parser SchemaParser
	repo   repository.SchemaRepository
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

func NewSchemaService(parser SchemaParser, repo repository.SchemaRepository) *SchemaService {
	return &SchemaService{parser: parser, repo: repo}
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
