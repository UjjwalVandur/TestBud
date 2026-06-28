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
			if payload.IsOversizedProbe {
				hasOversized = true
				if tc.ExpectedStatus != 413 {
					t.Errorf("expected oversized ExpectedStatus = 413, got %d", tc.ExpectedStatus)
				}
				if payload.OversizedBytes != 5<<20 {
					t.Errorf("expected OversizedBytes = %d, got %d", 5<<20, payload.OversizedBytes)
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

func TestGenerator_EmptyEndpoint(t *testing.T) {
	endpoint := models.Endpoint{
		ID:                 uuid.New(),
		Method:             "GET",
		Path:               "/ping",
		AuthRequired:       false,
		ParametersJSON:     datatypes.JSON(`[]`),
		RequestSchemaJSON:  datatypes.JSON(`{}`),
		ResponseSchemaJSON: datatypes.JSON(`{}`),
	}

	gen := NewGenerator()
	cases, err := gen.Generate(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Should still produce positive, negative, boundary, and security cases
	if len(cases) == 0 {
		t.Fatal("Generate() returned 0 test cases for empty endpoint")
	}

	// AuthRequired=false → no auth bypass or authz boundary probes
	for _, tc := range cases {
		var payload TestCasePayload
		if err := json.Unmarshal(tc.PayloadJSON, &payload); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if payload.OmitAuth {
			t.Error("auth bypass probe should not be generated when AuthRequired=false")
		}
		if payload.UseOtherUserAuth {
			t.Error("authz boundary probe should not be generated when AuthRequired=false")
		}
	}
}

func TestGenerator_MalformedParametersJSON(t *testing.T) {
	endpoint := models.Endpoint{
		ID:                 uuid.New(),
		Method:             "GET",
		Path:               "/bad",
		ParametersJSON:     datatypes.JSON(`not valid json`),
		RequestSchemaJSON:  datatypes.JSON(`{}`),
		ResponseSchemaJSON: datatypes.JSON(`{}`),
	}

	gen := NewGenerator()
	_, err := gen.Generate(context.Background(), endpoint)
	if err == nil {
		t.Fatal("expected error for malformed ParametersJSON, got nil")
	}
}

func TestGenerator_MalformedRequestSchemaJSON(t *testing.T) {
	endpoint := models.Endpoint{
		ID:                 uuid.New(),
		Method:             "POST",
		Path:               "/bad",
		ParametersJSON:     datatypes.JSON(`[]`),
		RequestSchemaJSON:  datatypes.JSON(`{{{`),
		ResponseSchemaJSON: datatypes.JSON(`{}`),
	}

	gen := NewGenerator()
	_, err := gen.Generate(context.Background(), endpoint)
	if err == nil {
		t.Fatal("expected error for malformed RequestSchemaJSON, got nil")
	}
}

func TestGenerator_GranularNegative(t *testing.T) {
	endpoint := models.Endpoint{
		ID:                 uuid.New(),
		Method:             "POST",
		Path:               "/users",
		AuthRequired:       false,
		ParametersJSON:     datatypes.JSON(testParametersJSON),
		RequestSchemaJSON:  datatypes.JSON(testRequestSchemaJSON),
		ResponseSchemaJSON: datatypes.JSON(`{}`),
	}

	gen := NewGenerator()
	cases, err := gen.Generate(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Counting expected negative test cases:
	// 1. General type mismatch / invalid object type case.
	// 2. Omitted required query parameter ("id").
	// 3. Omitted required body field ("name").
	// Total negative cases should be 3.
	var negativeCount int
	var hasOmittedParamId bool
	var hasOmittedFieldName bool
	var hasInvalidObjectType bool

	for _, tc := range cases {
		if tc.Category != models.CategoryNegative {
			continue
		}
		negativeCount++

		var payload TestCasePayload
		if err := json.Unmarshal(tc.PayloadJSON, &payload); err != nil {
			t.Fatalf("unmarshal negative: %v", err)
		}

		// Check if it's the general "invalid_object_type" body case
		if bodyStr, ok := payload.Body.(string); ok && bodyStr == "invalid_object_type" {
			hasInvalidObjectType = true
		}

		// Check if required query parameter "id" was omitted
		if _, ok := payload.QueryParams["id"]; !ok {
			hasOmittedParamId = true
		}

		// Check if required body field "name" was omitted
		if bodyMap, ok := payload.Body.(map[string]interface{}); ok {
			if _, ok := bodyMap["name"]; !ok {
				hasOmittedFieldName = true
			}
		}
	}

	if negativeCount < 3 {
		t.Errorf("expected at least 3 negative cases, got %d", negativeCount)
	}
	if !hasInvalidObjectType {
		t.Error("missing general negative case (invalid_object_type)")
	}
	if !hasOmittedParamId {
		t.Error("missing negative case with omitted required parameter 'id'")
	}
	if !hasOmittedFieldName {
		t.Error("missing negative case with omitted required body field 'name'")
	}
}

func TestGenerator_ComplexSchema(t *testing.T) {
	const complexBodySchema = `{
		"application/json": {
			"schema": {
				"type": "object",
				"properties": {
					"role": {
						"type": "string",
						"enum": ["admin", "user"]
					},
					"tags": {
						"type": "array",
						"items": {
							"type": "string"
						}
					},
					"address": {
						"type": "object",
						"properties": {
							"city": {
								"type": "string"
							}
						}
					}
				}
			}
		}
	}`

	endpoint := models.Endpoint{
		ID:                 uuid.New(),
		Method:             "POST",
		Path:               "/complex",
		AuthRequired:       false,
		ParametersJSON:     datatypes.JSON(`[]`),
		RequestSchemaJSON:  datatypes.JSON(complexBodySchema),
		ResponseSchemaJSON: datatypes.JSON(`{}`),
	}

	gen := NewGenerator()
	cases, err := gen.Generate(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var hasComplexBodyAsserted bool
	for _, tc := range cases {
		if tc.Category == models.CategoryPositive {
			var payload TestCasePayload
			if err := json.Unmarshal(tc.PayloadJSON, &payload); err != nil {
				t.Fatalf("unmarshal positive complex: %v", err)
			}
			bodyMap, ok := payload.Body.(map[string]interface{})
			if !ok {
				t.Fatalf("expected body map, got %T", payload.Body)
			}

			// 1. Verify enum value is admin or user
			role, _ := bodyMap["role"].(string)
			if role != "admin" && role != "user" {
				t.Errorf("expected role to be admin or user, got %v", bodyMap["role"])
			}

			// 2. Verify tags array
			tags, _ := bodyMap["tags"].([]interface{})
			if len(tags) != 1 || tags[0] != "sssss" {
				t.Errorf("expected tags to contain sssss, got %v", bodyMap["tags"])
			}

			// 3. Verify nested address object
			address, _ := bodyMap["address"].(map[string]interface{})
			if address == nil || address["city"] != "sssss" {
				t.Errorf("expected city to be sssss, got %v", address)
			}

			hasComplexBodyAsserted = true
		}
	}
	if !hasComplexBodyAsserted {
		t.Error("failed to assert complex body positive case")
	}
}

func TestGenerator_ExclusiveBounds(t *testing.T) {
	const exclusiveBoundsSchema = `{
		"application/json": {
			"schema": {
				"type": "object",
				"properties": {
					"rating": {
						"type": "number",
						"minimum": 1,
						"exclusiveMinimum": true,
						"maximum": 5,
						"exclusiveMaximum": true
					}
				}
			}
		}
	}`

	endpoint := models.Endpoint{
		ID:                 uuid.New(),
		Method:             "POST",
		Path:               "/rating",
		AuthRequired:       false,
		ParametersJSON:     datatypes.JSON(`[]`),
		RequestSchemaJSON:  datatypes.JSON(exclusiveBoundsSchema),
		ResponseSchemaJSON: datatypes.JSON(`{}`),
	}

	gen := NewGenerator()
	cases, err := gen.Generate(context.Background(), endpoint)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var hasMinBelow, hasMinExact, hasMaxAbove, hasMaxExact bool

	for _, tc := range cases {
		if tc.Category != models.CategoryBoundary {
			continue
		}

		var payload TestCasePayload
		if err := json.Unmarshal(tc.PayloadJSON, &payload); err != nil {
			t.Fatalf("unmarshal boundary payload: %v", err)
		}

		bodyMap, ok := payload.Body.(map[string]interface{})
		if !ok {
			continue
		}

		ratingVal, exists := bodyMap["rating"]
		if !exists {
			continue
		}

		rating, _ := ratingVal.(float64)

		// Boundaries are: "min_exact", "min_below", "max_exact", "max_above"
		// Since exclusiveMinimum is true and minimum is 1:
		// - min_exact (value that passes boundary: minimum + 1 = 2)
		// - min_below (value that fails boundary: minimum = 1)
		// Since exclusiveMaximum is true and maximum is 5:
		// - max_exact (value that passes boundary: maximum - 1 = 4)
		// - max_above (value that fails boundary: maximum = 5)
		if rating == 1 {
			hasMinBelow = true
		} else if rating == 2 {
			hasMinExact = true
		} else if rating == 4 {
			hasMaxExact = true
		} else if rating == 5 {
			hasMaxAbove = true
		}
	}

	if !hasMinBelow {
		t.Error("missing boundary case for min_below with exclusiveMinimum")
	}
	if !hasMinExact {
		t.Error("missing boundary case for min_exact with exclusiveMinimum")
	}
	if !hasMaxExact {
		t.Error("missing boundary case for max_exact with exclusiveMaximum")
	}
	if !hasMaxAbove {
		t.Error("missing boundary case for max_above with exclusiveMaximum")
	}
}
