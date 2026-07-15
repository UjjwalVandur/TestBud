package coverage

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/generator"
	"github.com/UjjwalVandur/TestBud/internal/models"
)

// --- helpers ---

func mustJSON(v interface{}) datatypes.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return datatypes.JSON(b)
}

// makePayloadJSON creates a test case payload JSON blob.
func makePayloadJSON(headers, query, path map[string]string, body interface{}) datatypes.JSON {
	p := generator.TestCasePayload{
		Headers:     headers,
		QueryParams: query,
		PathParams:  path,
		Body:        body,
	}
	return mustJSON(p)
}

// --- Tests ---

func TestComputeCoverage_Empty(t *testing.T) {
	pct, details := ComputeCoverage(nil, nil)

	if pct != 0 {
		t.Errorf("endpoint pct: got %v, want 0", pct)
	}
	if len(details.Categories) != 0 {
		t.Errorf("categories: got %d entries, want 0 for empty input", len(details.Categories))
	}
	if len(details.ResponseCodes) != 0 {
		t.Errorf("response codes: got %d, want 0", len(details.ResponseCodes))
	}
	if details.Fields.Total != 0 {
		t.Errorf("fields total: got %d, want 0", details.Fields.Total)
	}
}

func TestComputeCoverage_NoExecutions(t *testing.T) {
	epID := uuid.New()
	tc1ID := uuid.New()
	tc2ID := uuid.New()

	endpoints := []models.Endpoint{
		{
			ID:             epID,
			ParametersJSON: datatypes.JSON(`[]`),
			TestCases: []models.TestCase{
				{ID: tc1ID, EndpointID: epID, Category: models.CategoryPositive, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
				{ID: tc2ID, EndpointID: epID, Category: models.CategoryNegative, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
			},
		},
	}

	pct, details := ComputeCoverage(endpoints, nil)

	if pct != 0 {
		t.Errorf("endpoint pct: got %v, want 0 (no executions)", pct)
	}
	if details.Categories["positive"].Pct != 0 {
		t.Errorf("positive pct: got %v, want 0", details.Categories["positive"].Pct)
	}
	if details.Categories["positive"].Total != 1 {
		t.Errorf("positive total: got %d, want 1", details.Categories["positive"].Total)
	}
	if details.Categories["negative"].Total != 1 {
		t.Errorf("negative total: got %d, want 1", details.Categories["negative"].Total)
	}
	if len(details.ResponseCodes) != 0 {
		t.Errorf("response codes: got %d, want 0", len(details.ResponseCodes))
	}
}

func TestComputeCoverage_FullRun(t *testing.T) {
	ep1ID := uuid.New()
	ep2ID := uuid.New()
	tc1ID := uuid.New()
	tc2ID := uuid.New()
	tc3ID := uuid.New()

	endpoints := []models.Endpoint{
		{
			ID:             ep1ID,
			ParametersJSON: datatypes.JSON(`[]`),
			TestCases: []models.TestCase{
				{ID: tc1ID, EndpointID: ep1ID, Category: models.CategoryPositive, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
				{ID: tc2ID, EndpointID: ep1ID, Category: models.CategoryNegative, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
			},
		},
		{
			ID:             ep2ID,
			ParametersJSON: datatypes.JSON(`[]`),
			TestCases: []models.TestCase{
				{ID: tc3ID, EndpointID: ep2ID, Category: models.CategorySecurity, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
			},
		},
	}

	executions := []models.Execution{
		{TestCaseID: tc1ID, ActualStatus: 200, Passed: true},
		{TestCaseID: tc2ID, ActualStatus: 400, Passed: true},
		{TestCaseID: tc3ID, ActualStatus: 401, Passed: true},
	}

	epPct, details := ComputeCoverage(endpoints, executions)

	if epPct != 100 {
		t.Errorf("endpoint pct: got %v, want 100", epPct)
	}
	if details.Categories["positive"].Pct != 100 {
		t.Errorf("positive pct: got %v, want 100", details.Categories["positive"].Pct)
	}
	if details.Categories["negative"].Pct != 100 {
		t.Errorf("negative pct: got %v, want 100", details.Categories["negative"].Pct)
	}
	if details.Categories["security"].Pct != 100 {
		t.Errorf("security pct: got %v, want 100", details.Categories["security"].Pct)
	}
	// Boundary has 0 total, so 0%.
	if details.Categories["boundary"].Pct != 0 {
		t.Errorf("boundary pct: got %v, want 0 (no boundary test cases)", details.Categories["boundary"].Pct)
	}

	// Response code distribution.
	if details.ResponseCodes[200] != 1 {
		t.Errorf("response code 200: got %d, want 1", details.ResponseCodes[200])
	}
	if details.ResponseCodes[400] != 1 {
		t.Errorf("response code 400: got %d, want 1", details.ResponseCodes[400])
	}
	if details.ResponseCodes[401] != 1 {
		t.Errorf("response code 401: got %d, want 1", details.ResponseCodes[401])
	}
}

func TestComputeCoverage_PartialRun(t *testing.T) {
	ep1ID := uuid.New()
	ep2ID := uuid.New()
	tc1ID := uuid.New()
	tc2ID := uuid.New()
	tc3ID := uuid.New()
	tc4ID := uuid.New()

	endpoints := []models.Endpoint{
		{
			ID:             ep1ID,
			ParametersJSON: datatypes.JSON(`[]`),
			TestCases: []models.TestCase{
				{ID: tc1ID, EndpointID: ep1ID, Category: models.CategoryPositive, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
				{ID: tc2ID, EndpointID: ep1ID, Category: models.CategoryNegative, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
			},
		},
		{
			ID:             ep2ID,
			ParametersJSON: datatypes.JSON(`[]`),
			TestCases: []models.TestCase{
				{ID: tc3ID, EndpointID: ep2ID, Category: models.CategoryPositive, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
				{ID: tc4ID, EndpointID: ep2ID, Category: models.CategoryBoundary, PayloadJSON: makePayloadJSON(nil, nil, nil, nil)},
			},
		},
	}

	// Only tc1 executed — so ep1 is covered, ep2 is not.
	executions := []models.Execution{
		{TestCaseID: tc1ID, ActualStatus: 200, Passed: true},
	}

	epPct, details := ComputeCoverage(endpoints, executions)

	if epPct != 50 {
		t.Errorf("endpoint pct: got %v, want 50", epPct)
	}
	// positive: 2 total, 1 executed = 50%
	if details.Categories["positive"].Pct != 50 {
		t.Errorf("positive pct: got %v, want 50", details.Categories["positive"].Pct)
	}
	// negative: 1 total, 0 executed = 0%
	if details.Categories["negative"].Pct != 0 {
		t.Errorf("negative pct: got %v, want 0", details.Categories["negative"].Pct)
	}
}

func TestExtractDefinedFields(t *testing.T) {
	// Build ParametersJSON with query and path parameters.
	paramsJSON := mustJSON([]map[string]interface{}{
		{"in": "query", "name": "page", "schema": map[string]interface{}{"type": "integer"}},
		{"in": "path", "name": "id", "schema": map[string]interface{}{"type": "string"}},
		{"in": "header", "name": "X-Request-ID", "schema": map[string]interface{}{"type": "string"}},
	})

	// Build RequestSchemaJSON with body properties.
	reqSchemaJSON := mustJSON(map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name":  map[string]interface{}{"type": "string"},
					"email": map[string]interface{}{"type": "string"},
				},
			},
		},
	})

	ep := models.Endpoint{
		ParametersJSON:    paramsJSON,
		RequestSchemaJSON: reqSchemaJSON,
	}

	fields := extractDefinedFields(ep)

	expected := map[string]bool{
		"query:page":          true,
		"path:id":             true,
		"header:X-Request-ID": true,
		"body:name":           true,
		"body:email":          true,
	}

	if len(fields) != len(expected) {
		t.Fatalf("extractDefinedFields: got %d fields, want %d. Fields: %v", len(fields), len(expected), fields)
	}

	fieldSet := make(map[string]bool)
	for _, f := range fields {
		fieldSet[f] = true
	}
	for want := range expected {
		if !fieldSet[want] {
			t.Errorf("missing field %q in extracted fields: %v", want, fields)
		}
	}
}

func TestExtractPayloadFields(t *testing.T) {
	payload := makePayloadJSON(
		map[string]string{"X-Request-ID": "abc123"},
		map[string]string{"page": "1"},
		map[string]string{"id": "42"},
		map[string]interface{}{"name": "Alice", "email": "alice@example.com"},
	)

	fields := extractPayloadFields(payload)

	expected := map[string]bool{
		"header:X-Request-ID": true,
		"query:page":          true,
		"path:id":             true,
		"body:name":           true,
		"body:email":          true,
	}

	if len(fields) != len(expected) {
		t.Fatalf("extractPayloadFields: got %d fields, want %d. Fields: %v", len(fields), len(expected), fields)
	}

	fieldSet := make(map[string]bool)
	for _, f := range fields {
		fieldSet[f] = true
	}
	for want := range expected {
		if !fieldSet[want] {
			t.Errorf("missing field %q in payload fields: %v", want, fields)
		}
	}
}

func TestComputeCoverage_FieldCoverage(t *testing.T) {
	epID := uuid.New()
	tc1ID := uuid.New()
	tc2ID := uuid.New()

	// Endpoint defines 3 fields: query:page, path:id, body:name
	paramsJSON := mustJSON([]map[string]interface{}{
		{"in": "query", "name": "page", "schema": map[string]interface{}{"type": "integer"}},
		{"in": "path", "name": "id", "schema": map[string]interface{}{"type": "string"}},
	})
	reqSchemaJSON := mustJSON(map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": "string"},
				},
			},
		},
	})

	// tc1 covers query:page and path:id, but NOT body:name
	tc1Payload := makePayloadJSON(
		nil,
		map[string]string{"page": "1"},
		map[string]string{"id": "42"},
		nil,
	)
	// tc2 covers body:name only
	tc2Payload := makePayloadJSON(
		nil, nil, nil,
		map[string]interface{}{"name": "Alice"},
	)

	endpoints := []models.Endpoint{
		{
			ID:                epID,
			ParametersJSON:    paramsJSON,
			RequestSchemaJSON: reqSchemaJSON,
			TestCases: []models.TestCase{
				{ID: tc1ID, EndpointID: epID, Category: models.CategoryPositive, PayloadJSON: tc1Payload},
				{ID: tc2ID, EndpointID: epID, Category: models.CategoryNegative, PayloadJSON: tc2Payload},
			},
		},
	}

	// Both test cases executed.
	executions := []models.Execution{
		{TestCaseID: tc1ID, ActualStatus: 200},
		{TestCaseID: tc2ID, ActualStatus: 400},
	}

	_, details := ComputeCoverage(endpoints, executions)

	// All 3 defined fields are covered across the two test cases.
	if details.Fields.Total != 3 {
		t.Errorf("field total: got %d, want 3", details.Fields.Total)
	}
	if details.Fields.Covered != 3 {
		t.Errorf("field covered: got %d, want 3", details.Fields.Covered)
	}
	if details.Fields.Pct != 100 {
		t.Errorf("field pct: got %v, want 100", details.Fields.Pct)
	}
}
