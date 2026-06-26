package parser

import (
	"context"
	"testing"
)

const validOpenAPI = `{
  "openapi": "3.0.3",
  "info": {"title": "Pets", "version": "1.0.0"},
  "paths": {
    "/pets/{id}": {
      "parameters": [
        {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
      ],
      "get": {
        "security": [{"apiKeyAuth": []}],
        "responses": {
          "200": {
            "description": "ok",
            "content": {
              "application/json": {
                "schema": {"type": "object", "properties": {"id": {"type": "string"}}}
              }
            }
          }
        }
      },
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"type": "object", "properties": {"name": {"type": "string"}}}
            }
          }
        },
        "responses": {"201": {"description": "created"}}
      }
    }
  },
  "components": {
    "securitySchemes": {
      "apiKeyAuth": {"type": "apiKey", "name": "X-API-Key", "in": "header"}
    }
  }
}`

const validSwagger = `swagger: "2.0"
info:
  title: Pets
  version: "1.0.0"
paths:
  /pets:
    get:
      responses:
        "200":
          description: ok
          schema:
            type: object
            properties:
              id:
                type: string
`

func TestParserParse(t *testing.T) {
	tests := []struct {
		name          string
		raw           []byte
		wantEndpoints int
		wantErr       bool
	}{
		{
			name:          "valid openapi",
			raw:           []byte(validOpenAPI),
			wantEndpoints: 2,
		},
		{
			name:          "valid swagger",
			raw:           []byte(validSwagger),
			wantEndpoints: 1,
		},
		{
			name:    "invalid schema",
			raw:     []byte(`{"openapi":"3.0.3"}`),
			wantErr: true,
		},
		{
			name:    "empty schema",
			raw:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewParser().Parse(context.Background(), tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got.OpenAPIVersion != "3.0.3" {
				t.Fatalf("OpenAPIVersion = %q, want 3.0.3", got.OpenAPIVersion)
			}
			if got.SchemaHash == "" {
				t.Fatal("SchemaHash is empty")
			}
			if len(got.Endpoints) != tt.wantEndpoints {
				t.Fatalf("len(Endpoints) = %d, want %d", len(got.Endpoints), tt.wantEndpoints)
			}
			for _, endpoint := range got.Endpoints {
				if endpoint.EndpointHash == "" {
					t.Fatalf("%s %s endpoint hash is empty", endpoint.Method, endpoint.Path)
				}
			}
		})
	}
}
