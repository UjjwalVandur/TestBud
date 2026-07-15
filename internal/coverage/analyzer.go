package coverage

import (
	"encoding/json"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/generator"
	"github.com/UjjwalVandur/TestBud/internal/models"
)

// CategoryStats holds coverage metrics for a single test case category.
type CategoryStats struct {
	Total    int     `json:"total"`
	Executed int     `json:"executed"`
	Pct      float64 `json:"pct"`
}

// FieldStats holds coverage metrics for schema-defined request fields.
type FieldStats struct {
	Total   int     `json:"total"`
	Covered int     `json:"covered"`
	Pct     float64 `json:"pct"`
}

// CoverageDetails contains the full breakdown of coverage metrics.
type CoverageDetails struct {
	Categories    map[string]CategoryStats `json:"categories"`
	ResponseCodes map[int]int             `json:"response_codes"`
	Fields        FieldStats              `json:"fields"`
}

// ComputeCoverage calculates endpoint coverage %, category breakdown,
// response code distribution, and field coverage from the given endpoints
// and their associated executions.
func ComputeCoverage(endpoints []models.Endpoint, executions []models.Execution) (float64, CoverageDetails) {
	details := CoverageDetails{
		Categories:    make(map[string]CategoryStats),
		ResponseCodes: make(map[int]int),
	}

	if len(endpoints) == 0 {
		return 0, details
	}

	// Build a set of test case IDs that have at least one execution.
	executedTCIDs := make(map[uuid.UUID]bool, len(executions))
	for _, ex := range executions {
		executedTCIDs[ex.TestCaseID] = true
	}

	// --- Response code distribution ---
	for _, ex := range executions {
		details.ResponseCodes[ex.ActualStatus]++
	}

	// --- Endpoint coverage & category coverage ---
	coveredEndpoints := 0
	// Category accumulators.
	categoryTotal := make(map[string]int)
	categoryExecuted := make(map[string]int)

	// Field coverage accumulators.
	allDefinedFields := make(map[string]bool)
	allCoveredFields := make(map[string]bool)

	for _, ep := range endpoints {
		endpointCovered := false

		// Extract schema-defined fields for this endpoint.
		definedFields := extractDefinedFields(ep)
		for _, f := range definedFields {
			allDefinedFields[f] = true
		}

		for _, tc := range ep.TestCases {
			cat := string(tc.Category)
			categoryTotal[cat]++

			if executedTCIDs[tc.ID] {
				categoryExecuted[cat]++
				endpointCovered = true
			}

			// Extract fields populated in this test case payload.
			payloadFields := extractPayloadFields(tc.PayloadJSON)
			for _, f := range payloadFields {
				// Only count fields that are defined in the schema.
				if allDefinedFields[f] {
					allCoveredFields[f] = true
				}
			}
		}

		if endpointCovered {
			coveredEndpoints++
		}
	}

	// Compute endpoint coverage percentage.
	endpointPct := pct(coveredEndpoints, len(endpoints))

	// Build category stats for all 4 standard categories.
	for _, cat := range []string{"positive", "negative", "boundary", "security"} {
		total := categoryTotal[cat]
		executed := categoryExecuted[cat]
		details.Categories[cat] = CategoryStats{
			Total:    total,
			Executed: executed,
			Pct:      pct(executed, total),
		}
	}

	// Field coverage.
	details.Fields = FieldStats{
		Total:   len(allDefinedFields),
		Covered: len(allCoveredFields),
		Pct:     pct(len(allCoveredFields), len(allDefinedFields)),
	}

	return endpointPct, details
}

// extractDefinedFields parses ParametersJSON and RequestSchemaJSON from an
// endpoint to build a list of schema-defined field identifiers.
// Field identifiers use the format "location:name" for parameters (e.g.
// "query:page", "path:id") and "body:propertyName" for request body properties.
func extractDefinedFields(ep models.Endpoint) []string {
	var fields []string

	// 1. Parameters (query, path, header, cookie).
	var params []*openapi3.ParameterRef
	if len(ep.ParametersJSON) > 0 {
		if err := json.Unmarshal(ep.ParametersJSON, &params); err == nil {
			for _, ref := range params {
				if ref != nil && ref.Value != nil {
					loc := ref.Value.In   // "query", "path", "header", "cookie"
					name := ref.Value.Name // parameter name
					if loc != "" && name != "" {
						fields = append(fields, loc+":"+name)
					}
				}
			}
		}
	}

	// 2. Request body properties (top-level fields of application/json schema).
	if len(ep.RequestSchemaJSON) > 0 {
		var content openapi3.Content
		if err := json.Unmarshal(ep.RequestSchemaJSON, &content); err == nil {
			if mt := content.Get("application/json"); mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
				for propName := range mt.Schema.Value.Properties {
					fields = append(fields, "body:"+propName)
				}
			}
		}
	}

	return fields
}

// extractPayloadFields parses a test case PayloadJSON and returns the field
// identifiers that are populated, using the same "location:name" format as
// extractDefinedFields.
func extractPayloadFields(payloadJSON []byte) []string {
	if len(payloadJSON) == 0 {
		return nil
	}

	var p generator.TestCasePayload
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return nil
	}

	var fields []string
	for name := range p.QueryParams {
		fields = append(fields, "query:"+name)
	}
	for name := range p.PathParams {
		fields = append(fields, "path:"+name)
	}
	for name := range p.Headers {
		fields = append(fields, "header:"+name)
	}

	// Body properties: if body is a map, extract top-level keys.
	if p.Body != nil {
		if bodyMap, ok := p.Body.(map[string]interface{}); ok {
			for key := range bodyMap {
				fields = append(fields, "body:"+key)
			}
		}
	}

	return fields
}

// pct computes a percentage, returning 0 if total is 0.
func pct(covered, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(covered) / float64(total) * 100
}
