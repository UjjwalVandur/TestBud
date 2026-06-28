package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/parser"
)

type fakeParser struct {
	parsed parser.ParsedSchema
	err    error
}

func (f fakeParser) Parse(context.Context, []byte) (parser.ParsedSchema, error) {
	return f.parsed, f.err
}

type fakeGenerator struct {
	cases []models.TestCase
	err   error
}

func (f fakeGenerator) Generate(_ context.Context, _ models.Endpoint) ([]models.TestCase, error) {
	return f.cases, f.err
}

type fakeRepo struct {
	schema    *models.Schema
	endpoints []models.Endpoint
	err       error

	// findResult is returned by FindByProjectAndHash.
	findResult *models.Schema
	findErr    error

	// latestSchema is returned by FindLatestSchema.
	latestSchema *models.Schema
	latestErr    error

	// testCases is returned by GetTestCasesByEndpoint.
	testCases []models.TestCase
	casesErr  error
}

func (f *fakeRepo) CreateSchema(_ context.Context, schema *models.Schema, endpoints []models.Endpoint) error {
	if f.err != nil {
		return f.err
	}
	f.schema = schema
	f.endpoints = endpoints
	return nil
}

func (f *fakeRepo) FindByProjectAndHash(_ context.Context, _ uuid.UUID, _ string) (*models.Schema, error) {
	return f.findResult, f.findErr
}

func (f *fakeRepo) FindLatestSchema(_ context.Context, _ uuid.UUID) (*models.Schema, error) {
	return f.latestSchema, f.latestErr
}

func (f *fakeRepo) GetTestCasesByEndpoint(_ context.Context, _ uuid.UUID) ([]models.TestCase, error) {
	return f.testCases, f.casesErr
}

func TestSchemaServiceUploadSchema(t *testing.T) {
	projectID := uuid.New()
	uploadedBy := uuid.New()

	tests := []struct {
		name    string
		input   UploadSchemaInput
		parser  fakeParser
		repoErr error
		wantErr bool
	}{
		{
			name: "valid schema is persisted",
			input: UploadSchemaInput{
				ProjectID:  projectID,
				Version:    "1.0.0",
				UploadedBy: uploadedBy,
				RawBytes:   []byte(`{"openapi":"3.0.3"}`),
			},
			parser: fakeParser{
				parsed: parser.ParsedSchema{
					OpenAPIVersion: "3.0.3",
					SchemaHash:     "hash",
					Endpoints: []parser.Endpoint{
						{
							Method:             "GET",
							Path:               "/pets",
							EndpointHash:       "endpoint-hash",
							ParametersJSON:     json.RawMessage(`[]`),
							RequestSchemaJSON:  json.RawMessage(`{}`),
							ResponseSchemaJSON: json.RawMessage(`{"200":{"description":"ok"}}`),
						},
					},
				},
			},
		},
		{
			name: "parser error bubbles up",
			input: UploadSchemaInput{
				ProjectID:  projectID,
				Version:    "1.0.0",
				UploadedBy: uploadedBy,
				RawBytes:   []byte(`bad`),
			},
			parser:  fakeParser{err: errors.New("parse failed")},
			wantErr: true,
		},
		{
			name: "missing project id fails",
			input: UploadSchemaInput{
				Version:    "1.0.0",
				UploadedBy: uploadedBy,
				RawBytes:   []byte(`{}`),
			},
			parser:  fakeParser{},
			wantErr: true,
		},
		{
			name: "missing uploaded_by fails",
			input: UploadSchemaInput{
				ProjectID: projectID,
				Version:   "1.0.0",
				RawBytes:  []byte(`{}`),
			},
			parser:  fakeParser{},
			wantErr: true,
		},
		{
			name: "missing version fails",
			input: UploadSchemaInput{
				ProjectID:  projectID,
				UploadedBy: uploadedBy,
				RawBytes:   []byte(`{}`),
			},
			parser:  fakeParser{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{err: tt.repoErr}
			gen := &fakeGenerator{cases: []models.TestCase{{Category: models.CategoryPositive}}}
			svc := NewSchemaService(tt.parser, repo, gen)

			got, err := svc.UploadSchema(context.Background(), tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("UploadSchema() error = %v", err)
			}
			if got.SchemaHash != tt.parser.parsed.SchemaHash {
				t.Fatalf("SchemaHash = %q, want %q", got.SchemaHash, tt.parser.parsed.SchemaHash)
			}
			if got.EndpointCount != len(tt.parser.parsed.Endpoints) {
				t.Fatalf("EndpointCount = %d, want %d", got.EndpointCount, len(tt.parser.parsed.Endpoints))
			}
			if repo.schema == nil {
				t.Fatal("schema was not persisted")
			}
			if len(repo.endpoints) != 1 {
				t.Fatalf("persisted endpoints = %d, want 1", len(repo.endpoints))
			}
			// Verify test cases were generated
			if len(repo.endpoints[0].TestCases) != 1 {
				t.Fatalf("expected 1 generated test case on endpoint, got %d", len(repo.endpoints[0].TestCases))
			}
		})
	}
}

func TestSchemaServiceUploadSchema_Dedup(t *testing.T) {
	projectID := uuid.New()
	uploadedBy := uuid.New()
	existingID := uuid.New()

	repo := &fakeRepo{
		findResult: &models.Schema{
			ID:             existingID,
			SchemaHash:     "existing-hash",
			OpenAPIVersion: "3.0.3",
		},
	}
	svc := NewSchemaService(fakeParser{
		parsed: parser.ParsedSchema{
			OpenAPIVersion: "3.0.3",
			SchemaHash:     "existing-hash",
			Endpoints:      []parser.Endpoint{{Method: "GET", Path: "/pets"}},
		},
	}, repo, &fakeGenerator{})

	got, err := svc.UploadSchema(context.Background(), UploadSchemaInput{
		ProjectID:  projectID,
		Version:    "1.0.0",
		UploadedBy: uploadedBy,
		RawBytes:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("UploadSchema() error = %v", err)
	}
	if got.SchemaID != existingID {
		t.Fatalf("SchemaID = %v, want %v (existing)", got.SchemaID, existingID)
	}
	if repo.schema != nil {
		t.Fatal("schema should not have been created (dedup)")
	}
}

func TestSchemaServiceUploadSchema_EndpointDedupCopy(t *testing.T) {
	projectID := uuid.New()
	uploadedBy := uuid.New()
	oldEndpointID := uuid.New()

	// Setup repo returning a previous schema with a matching endpoint hash
	repo := &fakeRepo{
		latestSchema: &models.Schema{
			ID: uuid.New(),
			Endpoints: []models.Endpoint{
				{
					ID:           oldEndpointID,
					Method:       "GET",
					Path:         "/pets",
					EndpointHash: "identical-hash",
				},
			},
		},
		// Mock test cases on the old endpoint
		testCases: []models.TestCase{
			{
				Category:       models.CategoryPositive,
				PayloadJSON:    []byte(`{"body":"copied"}`),
				ExpectedStatus: 200,
			},
		},
	}

	// Parser returns endpoint with matching hash
	mockParser := fakeParser{
		parsed: parser.ParsedSchema{
			OpenAPIVersion: "3.0.3",
			SchemaHash:     "new-schema-hash",
			Endpoints: []parser.Endpoint{
				{
					Method:       "GET",
					Path:         "/pets",
					EndpointHash: "identical-hash",
				},
			},
		},
	}

	svc := NewSchemaService(mockParser, repo, &fakeGenerator{})

	_, err := svc.UploadSchema(context.Background(), UploadSchemaInput{
		ProjectID:  projectID,
		Version:    "1.1.0",
		UploadedBy: uploadedBy,
		RawBytes:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("UploadSchema() error = %v", err)
	}

	if len(repo.endpoints) != 1 {
		t.Fatalf("expected 1 endpoint persisted, got %d", len(repo.endpoints))
	}

	// Verify test cases were copied instead of generated
	eps := repo.endpoints[0]
	if len(eps.TestCases) != 1 {
		t.Fatalf("expected 1 test case on endpoint, got %d", len(eps.TestCases))
	}
	if string(eps.TestCases[0].PayloadJSON) != `{"body":"copied"}` {
		t.Errorf("expected copied payload, got %s", string(eps.TestCases[0].PayloadJSON))
	}
}
