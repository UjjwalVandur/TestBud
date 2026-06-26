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

type fakeRepo struct {
	schema    *models.Schema
	endpoints []models.Endpoint
	err       error
}

func (f *fakeRepo) CreateSchema(_ context.Context, schema *models.Schema, endpoints []models.Endpoint) error {
	if f.err != nil {
		return f.err
	}
	f.schema = schema
	f.endpoints = endpoints
	return nil
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{err: tt.repoErr}
			svc := NewSchemaService(tt.parser, repo)

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
		})
	}
}
