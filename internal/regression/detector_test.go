package regression

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/util"
)

func TestDetect(t *testing.T) {
	baseID := uuid.New()
	targetID := uuid.New()

	tests := []struct {
		name         string
		base         []models.Endpoint
		target       []models.Endpoint
		wantAdded    int
		wantRemoved  int
		wantModified int
		// Optional: check specific flags on the first modified entry.
		checkModified func(t *testing.T, change EndpointChange)
	}{
		{
			name:         "no base endpoints — all added",
			base:         nil,
			target:       makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{}`),
			wantAdded:    1,
			wantRemoved:  0,
			wantModified: 0,
		},
		{
			name:         "no target endpoints — all removed",
			base:         makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{}`),
			target:       nil,
			wantAdded:    0,
			wantRemoved:  1,
			wantModified: 0,
		},
		{
			name:         "identical endpoints — empty report",
			base:         makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{}`),
			target:       makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{}`),
			wantAdded:    0,
			wantRemoved:  0,
			wantModified: 0,
		},
		{
			name:         "added and removed",
			base:         makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{}`),
			target:       makeEndpoints("POST", "/pets", false, `[]`, `{}`, `{}`),
			wantAdded:    1,
			wantRemoved:  1,
			wantModified: 0,
		},
		{
			name:   "parameters changed",
			base:   makeEndpoints("GET", "/pets", false, `[{"name":"page"}]`, `{}`, `{}`),
			target: makeEndpoints("GET", "/pets", false, `[{"name":"page"},{"name":"limit"}]`, `{}`, `{}`),
			wantModified: 1,
			checkModified: func(t *testing.T, change EndpointChange) {
				if !change.ParametersChanged {
					t.Error("expected ParametersChanged=true")
				}
				if change.RequestSchemaChanged || change.ResponseSchemaChanged || change.AuthChanged {
					t.Error("only ParametersChanged should be true")
				}
			},
		},
		{
			name:   "request schema changed",
			base:   makeEndpoints("POST", "/pets", false, `[]`, `{"type":"object"}`, `{}`),
			target: makeEndpoints("POST", "/pets", false, `[]`, `{"type":"object","required":["name"]}`, `{}`),
			wantModified: 1,
			checkModified: func(t *testing.T, change EndpointChange) {
				if !change.RequestSchemaChanged {
					t.Error("expected RequestSchemaChanged=true")
				}
				if change.ParametersChanged || change.ResponseSchemaChanged || change.AuthChanged {
					t.Error("only RequestSchemaChanged should be true")
				}
			},
		},
		{
			name:   "response schema changed",
			base:   makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{"200":{"description":"ok"}}`),
			target: makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{"200":{"description":"success"}}`),
			wantModified: 1,
			checkModified: func(t *testing.T, change EndpointChange) {
				if !change.ResponseSchemaChanged {
					t.Error("expected ResponseSchemaChanged=true")
				}
				if change.ParametersChanged || change.RequestSchemaChanged || change.AuthChanged {
					t.Error("only ResponseSchemaChanged should be true")
				}
			},
		},
		{
			name:   "auth changed only",
			base:   makeEndpoints("GET", "/pets", false, `[]`, `{}`, `{}`),
			target: makeEndpoints("GET", "/pets", true, `[]`, `{}`, `{}`),
			wantModified: 1,
			checkModified: func(t *testing.T, change EndpointChange) {
				if !change.AuthChanged {
					t.Error("expected AuthChanged=true")
				}
				if change.ParametersChanged || change.RequestSchemaChanged || change.ResponseSchemaChanged {
					t.Error("only AuthChanged should be true")
				}
			},
		},
		{
			name:   "JSON formatting differences are not false positives",
			base:   makeEndpoints("GET", "/pets", false, `[ { "name" : "page" } ]`, `{}`, `{}`),
			target: makeEndpoints("GET", "/pets", false, `[{"name":"page"}]`, `{}`, `{}`),
			wantModified: 0,
		},
		{
			name: "multiple changes detected together",
			base: []models.Endpoint{
				makeEndpoint("GET", "/pets", false, `[]`, `{}`, `{}`),
				makeEndpoint("DELETE", "/pets/{id}", true, `[{"name":"id"}]`, `{}`, `{}`),
			},
			target: []models.Endpoint{
				makeEndpoint("GET", "/pets", true, `[{"name":"page"}]`, `{}`, `{}`), // modified: auth + params
				makeEndpoint("POST", "/pets", false, `[]`, `{"type":"object"}`, `{}`), // added
				// DELETE /pets/{id} removed
			},
			wantAdded:    1,
			wantRemoved:  1,
			wantModified: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := Detect(tt.base, tt.target, baseID, targetID)

			if len(report.Added) != tt.wantAdded {
				t.Errorf("Added = %d, want %d", len(report.Added), tt.wantAdded)
			}
			if len(report.Removed) != tt.wantRemoved {
				t.Errorf("Removed = %d, want %d", len(report.Removed), tt.wantRemoved)
			}
			if len(report.Modified) != tt.wantModified {
				t.Errorf("Modified = %d, want %d", len(report.Modified), tt.wantModified)
			}
			if tt.checkModified != nil && len(report.Modified) > 0 {
				tt.checkModified(t, report.Modified[0])
			}

			// Verify schema IDs are set.
			if report.BaseSchemaID != baseID {
				t.Errorf("BaseSchemaID = %v, want %v", report.BaseSchemaID, baseID)
			}
			if report.TargetSchemaID != targetID {
				t.Errorf("TargetSchemaID = %v, want %v", report.TargetSchemaID, targetID)
			}
		})
	}
}

func TestCanonicalJSON(t *testing.T) {
	tests := []struct {
		name string
		a, b []byte
		want bool
	}{
		{"both empty", nil, nil, true},
		{"same bytes", []byte(`{"a":1}`), []byte(`{"a":1}`), true},
		{"key ordering", []byte(`{"b":2,"a":1}`), []byte(`{"a":1,"b":2}`), true},
		{"whitespace", []byte(`{ "a" : 1 }`), []byte(`{"a":1}`), true},
		{"different values", []byte(`{"a":1}`), []byte(`{"a":2}`), false},
		{"one empty", []byte(`{}`), nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := util.JSONBytesEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("jsonEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

// makeEndpoints is a convenience wrapper returning a single-element slice.
func makeEndpoints(method, path string, authRequired bool, params, reqSchema, respSchema string) []models.Endpoint {
	return []models.Endpoint{makeEndpoint(method, path, authRequired, params, reqSchema, respSchema)}
}

func makeEndpoint(method, path string, authRequired bool, params, reqSchema, respSchema string) models.Endpoint {
	return models.Endpoint{
		ID:                 uuid.New(),
		Method:             method,
		Path:               path,
		AuthRequired:       authRequired,
		ParametersJSON:     datatypes.JSON(params),
		RequestSchemaJSON:  datatypes.JSON(reqSchema),
		ResponseSchemaJSON: datatypes.JSON(respSchema),
	}
}
