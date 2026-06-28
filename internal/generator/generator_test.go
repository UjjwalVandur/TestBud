package generator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

const testParametersJSON = `[
  {
    "name": "id",
    "in": "query",
    "required": true,
    "schema": {
      "type": "integer",
      "minimum": 1,
      "maximum": 100
    }
  },
  {
    "name": "email",
    "in": "header",
    "required": false,
    "schema": {
      "type": "string",
      "format": "email"
    }
  }
]`

const testRequestSchemaJSON = `{
  "application/json": {
    "schema": {
      "type": "object",
      "required": ["name"],
      "properties": {
        "name": {
          "type": "string",
          "minLength": 3,
          "maxLength": 10
        }
      }
    }
  }
}`

func TestGenerator_Generate(t *testing.T) {
	endpoint := models.Endpoint{
		ID:                 uuid.New(),
		Method:             "POST",
		Path:               "/users",
		AuthRequired:       true,
		ParametersJSON:     datatypes.JSON(testParametersJSON),
		RequestSchemaJSON:  datatypes.JSON(testRequestSchemaJSON),
		ResponseSchemaJSON: datatypes.JSON(`{}`),
	}

	gen := NewGenerator()
	cases, err := gen.Generate(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(cases) == 0 {
		t.Fatal("Generate() returned 0 test cases")
	}

	// Verify categories are present
	var hasPos, hasNeg, hasBoundary, hasSecurity bool
	var hasBypass, hasSQLi, hasXSS, hasOversized, hasRateLimit bool

	for _, tc := range cases {
		if tc.EndpointID != endpoint.ID {
			t.Errorf("expected EndpointID = %v, got %v", endpoint.ID, tc.EndpointID)
		}

		var payload TestCasePayload
		if err := json.Unmarshal(tc.PayloadJSON, &payload); err != nil {
			t.Fatalf("failed to unmarshal test case payload: %v", err)
		}

		switch tc.Category {
		case models.CategoryPositive:
			hasPos = true
			if tc.ExpectedStatus != 200 {
				t.Errorf("expected positive ExpectedStatus = 200, got %d", tc.ExpectedStatus)
			}
			// Verify valid generation
			if payload.QueryParams["id"] != "1" { // minimum
				t.Errorf("expected positive query parameter id = 1, got %v", payload.QueryParams["id"])
			}
			if payload.Headers["email"] != "test@example.com" {
				t.Errorf("expected positive header email = test@example.com, got %v", payload.Headers["email"])
			}
			bodyMap, ok := payload.Body.(map[string]interface{})
			if !ok {
				t.Fatalf("expected body to be a map, got %T", payload.Body)
			}
			if bodyMap["name"] != "sss" {
				t.Errorf("expected body field name = sss, got %v", bodyMap["name"])
			}

		case models.CategoryNegative:
			hasNeg = true
			if tc.ExpectedStatus != 400 {
				t.Errorf("expected negative ExpectedStatus = 400, got %d", tc.ExpectedStatus)
			}

		case models.CategoryBoundary:
			hasBoundary = true

		case models.CategorySecurity:
			hasSecurity = true
			if payload.OmitAuth {
				hasBypass = true
				if tc.ExpectedStatus != 401 {
					t.Errorf("expected auth bypass ExpectedStatus = 401, got %d", tc.ExpectedStatus)
				}
			}
			if payload.IsRateLimitProbe {
				hasRateLimit = true
				if tc.ExpectedStatus != 429 {
					t.Errorf("expected rate limit ExpectedStatus = 429, got %d", tc.ExpectedStatus)
				}
			}
			if payload.RawBody != "" {
				hasOversized = true
				if tc.ExpectedStatus != 413 {
					t.Errorf("expected oversized ExpectedStatus = 413, got %d", tc.ExpectedStatus)
				}
			}
			// Check SQLi / XSS probes
			if payload.QueryParams["id"] == "' OR 1=1 --" {
				hasSQLi = true
			}
			if payload.QueryParams["id"] == "<script>alert(1)</script>" {
				hasXSS = true
			}
		}
	}

	if !hasPos {
		t.Error("missing positive test case")
	}
	if !hasNeg {
		t.Error("missing negative test case")
	}
	if !hasBoundary {
		t.Error("missing boundary test cases")
	}
	if !hasSecurity {
		t.Error("missing security test cases")
	}
	if !hasBypass {
		t.Error("missing auth bypass security case")
	}
	if !hasSQLi {
		t.Error("missing SQL injection security case")
	}
	if !hasXSS {
		t.Error("missing XSS security case")
	}
	if !hasOversized {
		t.Error("missing oversized payload security case")
	}
	if !hasRateLimit {
		t.Error("missing rate limit security case")
	}
}
