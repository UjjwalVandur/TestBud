package generator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

// TestCasePayload represents the detailed structure of the generated request payload.
type TestCasePayload struct {
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
	PathParams  map[string]string `json:"path_params,omitempty"`
	Body        interface{}       `json:"body,omitempty"`

	// Execution metadata flags for security probes
	OmitAuth         bool `json:"omit_auth,omitempty"`
	UseOtherUserAuth bool `json:"use_other_user_auth,omitempty"`
	IsRateLimitProbe bool `json:"is_rate_limit_probe,omitempty"`
	IsOversizedProbe bool `json:"is_oversized_probe,omitempty"`
	OversizedBytes   int  `json:"oversized_bytes,omitempty"`
}

// Generator produces positive, negative, boundary, and security test cases.
type Generator struct{}

// NewGenerator creates a new Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate parses the endpoint JSON specs and generates test cases.
func (g *Generator) Generate(ctx context.Context, endpoint models.Endpoint) ([]models.TestCase, error) {
	var params []*openapi3.ParameterRef
	if len(endpoint.ParametersJSON) > 0 {
		if err := json.Unmarshal(endpoint.ParametersJSON, &params); err != nil {
			return nil, fmt.Errorf("unmarshal parameters: %w", err)
		}
	}

	var reqBody openapi3.Content
	if len(endpoint.RequestSchemaJSON) > 0 {
		if err := json.Unmarshal(endpoint.RequestSchemaJSON, &reqBody); err != nil {
			return nil, fmt.Errorf("unmarshal request schema: %w", err)
		}
	}

	var testCases []models.TestCase

	// 1. POSITIVE TEST CASE
	posCase, err := g.generatePositive(endpoint.ID, params, reqBody)
	if err != nil {
		return nil, fmt.Errorf("generate positive test: %w", err)
	}
	testCases = append(testCases, posCase)

	// 2. NEGATIVE TEST CASES
	negCases, err := g.generateNegative(endpoint.ID, params, reqBody)
	if err != nil {
		return nil, fmt.Errorf("generate negative tests: %w", err)
	}
	testCases = append(testCases, negCases...)

	// 3. BOUNDARY TEST CASES
	boundaryCases, err := g.generateBoundaries(endpoint.ID, params, reqBody)
	if err != nil {
		return nil, fmt.Errorf("generate boundary tests: %w", err)
	}
	testCases = append(testCases, boundaryCases...)

	// 4. SECURITY TEST CASES
	securityCases, err := g.generateSecurity(endpoint.ID, endpoint.AuthRequired, params, reqBody)
	if err != nil {
		return nil, fmt.Errorf("generate security tests: %w", err)
	}
	testCases = append(testCases, securityCases...)

	return testCases, nil
}

// generatePositive produces a valid request expected to succeed.
func (g *Generator) generatePositive(endpointID uuid.UUID, params []*openapi3.ParameterRef, reqBody openapi3.Content) (models.TestCase, error) {
	path, query, headers := generateParameters(params, false, "")
	body := generateRequestBody(reqBody, false, "")

	payload := TestCasePayload{
		Headers:     headers,
		QueryParams: query,
		PathParams:  path,
		Body:        body,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return models.TestCase{}, err
	}

	return models.TestCase{
		EndpointID:     endpointID,
		Category:       models.CategoryPositive,
		PayloadJSON:    payloadBytes,
		ExpectedStatus: 200, // or 201, default to 200/2xx
	}, nil
}

// generateNegative produces invalid requests expected to fail with 4xx.
func (g *Generator) generateNegative(endpointID uuid.UUID, params []*openapi3.ParameterRef, reqBody openapi3.Content) ([]models.TestCase, error) {
	var cases []models.TestCase

	// 1. General negative case (invalid types / corrupted structures)
	{
		path, query, headers := generateParametersEx(params, true, "", "")
		body := generateRequestBodyEx(reqBody, true, "", "")

		payload := TestCasePayload{
			Headers:     headers,
			QueryParams: query,
			PathParams:  path,
			Body:        body,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategoryNegative,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: 400,
		})
	}

	// 2. Omit each required parameter one-by-one, keeping others valid
	for _, ref := range params {
		if ref == nil || ref.Value == nil {
			continue
		}
		param := ref.Value
		if param.Required {
			path, query, headers := generateParametersEx(params, false, "", param.Name)
			body := generateRequestBodyEx(reqBody, false, "", "")

			payload := TestCasePayload{
				Headers:     headers,
				QueryParams: query,
				PathParams:  path,
				Body:        body,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				return nil, err
			}

			cases = append(cases, models.TestCase{
				EndpointID:     endpointID,
				Category:       models.CategoryNegative,
				PayloadJSON:    payloadBytes,
				ExpectedStatus: 400,
			})
		}
	}

	// 3. Omit each required request body field one-by-one, keeping others valid
	if len(reqBody) > 0 {
		mediaType := reqBody.Get("application/json")
		if mediaType == nil {
			for _, mt := range reqBody {
				mediaType = mt
				break
			}
		}
		if mediaType != nil && mediaType.Schema != nil && mediaType.Schema.Value != nil {
			schema := mediaType.Schema.Value
			for _, reqFieldName := range schema.Required {
				path, query, headers := generateParametersEx(params, false, "", "")
				body := generateRequestBodyEx(reqBody, true, "", reqFieldName)

				payload := TestCasePayload{
					Headers:     headers,
					QueryParams: query,
					PathParams:  path,
					Body:        body,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					return nil, err
				}

				cases = append(cases, models.TestCase{
					EndpointID:     endpointID,
					Category:       models.CategoryNegative,
					PayloadJSON:    payloadBytes,
					ExpectedStatus: 400,
				})
			}
		}
	}

	return cases, nil
}

// generateBoundaries scans constraints and generates test cases at/outside limits.
func (g *Generator) generateBoundaries(endpointID uuid.UUID, params []*openapi3.ParameterRef, reqBody openapi3.Content) ([]models.TestCase, error) {
	var cases []models.TestCase

	// We generate boundary test cases for standard constraints:
	// "min_below", "min_exact", "max_above", "max_exact", "min_len_below", "min_len_exact", "max_len_above", "max_len_exact"
	boundaries := []struct {
		name           string
		expectPass     bool
		expectedStatus int
	}{
		{"min_exact", true, 200},
		{"min_below", false, 400},
		{"max_exact", true, 200},
		{"max_above", false, 400},
		{"min_len_exact", true, 200},
		{"min_len_below", false, 400},
		{"max_len_exact", true, 200},
		{"max_len_above", false, 400},
	}

	for _, b := range boundaries {
		path, query, headers := generateParameters(params, false, b.name)
		body := generateRequestBody(reqBody, false, b.name)

		// Check if any boundary was actually triggered (if values changed from standard positive values)
		// To keep it simple, we generate cases for all endpoints, but the execution engine will run them.
		payload := TestCasePayload{
			Headers:     headers,
			QueryParams: query,
			PathParams:  path,
			Body:        body,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategoryBoundary,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: b.expectedStatus,
		})
	}

	return cases, nil
}

// generateSecurity generates the black-box HTTP security probe test cases.
func (g *Generator) generateSecurity(endpointID uuid.UUID, authRequired bool, params []*openapi3.ParameterRef, reqBody openapi3.Content) ([]models.TestCase, error) {
	var cases []models.TestCase

	// 1. Auth Bypass (401)
	if authRequired {
		path, query, headers := generateParameters(params, false, "")
		body := generateRequestBody(reqBody, false, "")
		payload := TestCasePayload{
			Headers:     headers,
			QueryParams: query,
			PathParams:  path,
			Body:        body,
			OmitAuth:    true,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal auth bypass payload: %w", err)
		}
		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategorySecurity,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: 401,
		})

		// 2. Authz Boundary (403)
		payload.OmitAuth = false
		payload.UseOtherUserAuth = true
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal authz boundary payload: %w", err)
		}
		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategorySecurity,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: 403,
		})
	}

	// 3. SQL Injection Probe (expect 400 or safe response, NEVER 500)
	{
		path, query, headers := generateParameters(params, false, "")
		body := generateRequestBody(reqBody, false, "")

		// Inject SQL payload into string values
		sqlPayload := "' OR 1=1 --"
		path = injectMap(path, sqlPayload)
		query = injectMap(query, sqlPayload)
		headers = injectMap(headers, sqlPayload)
		body = injectInterface(body, sqlPayload)

		payload := TestCasePayload{
			Headers:     headers,
			QueryParams: query,
			PathParams:  path,
			Body:        body,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal sqli payload: %w", err)
		}
		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategorySecurity,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: 400, // expecting graceful 400 error, not 500
		})
	}

	// 4. XSS Probe (expect 400 or sanitized response, NEVER 500)
	{
		path, query, headers := generateParameters(params, false, "")
		body := generateRequestBody(reqBody, false, "")

		xssPayload := "<script>alert(1)</script>"
		path = injectMap(path, xssPayload)
		query = injectMap(query, xssPayload)
		headers = injectMap(headers, xssPayload)
		body = injectInterface(body, xssPayload)

		payload := TestCasePayload{
			Headers:     headers,
			QueryParams: query,
			PathParams:  path,
			Body:        body,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal xss payload: %w", err)
		}
		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategorySecurity,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: 400,
		})
	}

	// 5. Oversized Payload Probe (expect 413 or graceful 400)
	// DEV-10: Store a flag + target size instead of the actual 5MB payload
	// to avoid blowing Neon's 512MB storage cap. The execution engine
	// synthesises the body at runtime.
	{
		path, query, headers := generateParameters(params, false, "")

		payload := TestCasePayload{
			Headers:          headers,
			QueryParams:      query,
			PathParams:       path,
			IsOversizedProbe: true,
			OversizedBytes:   5 << 20, // 5MB — execution engine generates body
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal oversized payload: %w", err)
		}
		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategorySecurity,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: 413,
		})
	}

	// 6. Rate Limit Probe (expect 429)
	{
		payload := TestCasePayload{
			IsRateLimitProbe: true,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal rate limit payload: %w", err)
		}
		cases = append(cases, models.TestCase{
			EndpointID:     endpointID,
			Category:       models.CategorySecurity,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: 429,
		})
	}

	return cases, nil
}

// Helpers for value generation based on OpenAPI schemas

func generateParameters(params []*openapi3.ParameterRef, useInvalid bool, boundary string) (map[string]string, map[string]string, map[string]string) {
	return generateParametersEx(params, useInvalid, boundary, "")
}

func generateParametersEx(params []*openapi3.ParameterRef, useInvalid bool, boundary string, omitParamName string) (map[string]string, map[string]string, map[string]string) {
	pathParams := make(map[string]string)
	queryParams := make(map[string]string)
	headers := make(map[string]string)

	for _, ref := range params {
		if ref == nil || ref.Value == nil {
			continue
		}
		param := ref.Value

		if param.Name == omitParamName {
			continue
		}

		// Omit required parameter if simulating invalid request (default behavior when no specific parameter is omitted)
		if omitParamName == "" && useInvalid && param.Required {
			continue
		}

		var valStr string
		valUseInvalid := useInvalid
		if omitParamName != "" {
			valUseInvalid = false
		}

		if param.Schema != nil && param.Schema.Value != nil {
			val := generateSchemaValueEx(param.Schema.Value, valUseInvalid, boundary, "")
			if val != nil {
				valStr = fmt.Sprintf("%v", val)
			}
		} else {
			valStr = "test"
		}

		switch param.In {
		case "path":
			pathParams[param.Name] = valStr
		case "query":
			queryParams[param.Name] = valStr
		case "header":
			headers[param.Name] = valStr
		}
	}
	return pathParams, queryParams, headers
}

func generateRequestBody(content openapi3.Content, useInvalid bool, boundary string) interface{} {
	return generateRequestBodyEx(content, useInvalid, boundary, "")
}

func generateRequestBodyEx(content openapi3.Content, useInvalid bool, boundary string, omitFieldName string) interface{} {
	if len(content) == 0 {
		return nil
	}
	mediaType := content.Get("application/json")
	if mediaType == nil {
		for _, mt := range content {
			mediaType = mt
			break
		}
	}
	if mediaType == nil || mediaType.Schema == nil || mediaType.Schema.Value == nil {
		return nil
	}
	return generateSchemaValueEx(mediaType.Schema.Value, useInvalid, boundary, omitFieldName)
}

func generateSchemaValue(schema *openapi3.Schema, useInvalid bool, boundary string) interface{} {
	return generateSchemaValueEx(schema, useInvalid, boundary, "")
}

func generateSchemaValueEx(schema *openapi3.Schema, useInvalid bool, boundary string, omitFieldName string) interface{} {
	if schema == nil {
		return nil
	}
	if schemaHasType(schema, "string") {
		return generateStringValue(schema, useInvalid, boundary)
	}
	if schemaHasType(schema, "integer") || schemaHasType(schema, "number") {
		return generateNumberValue(schema, useInvalid, boundary)
	}
	if schemaHasType(schema, "boolean") {
		return generateBooleanValue(schema, useInvalid)
	}
	if schemaHasType(schema, "object") {
		return generateObjectValueEx(schema, useInvalid, boundary, omitFieldName)
	}
	if schemaHasType(schema, "array") {
		return generateArrayValueEx(schema, useInvalid, boundary, omitFieldName)
	}

	if len(schema.Properties) > 0 {
		return generateObjectValueEx(schema, useInvalid, boundary, omitFieldName)
	}
	return nil
}

func generateStringValue(schema *openapi3.Schema, useInvalid bool, boundary string) interface{} {
	if useInvalid {
		if len(schema.Enum) > 0 {
			return "invalid_enum_value_xyz"
		}
		return nil
	}

	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}

	// Boundary length checks
	if boundary == "min_len_below" && schema.MinLength > 0 {
		return strings.Repeat("a", int(schema.MinLength)-1)
	}
	if boundary == "min_len_exact" && schema.MinLength > 0 {
		return strings.Repeat("a", int(schema.MinLength))
	}
	if boundary == "max_len_above" && schema.MaxLength != nil {
		return strings.Repeat("a", int(*schema.MaxLength)+1)
	}
	if boundary == "max_len_exact" && schema.MaxLength != nil {
		return strings.Repeat("a", int(*schema.MaxLength))
	}

	switch schema.Format {
	case "date":
		return "2026-06-28"
	case "date-time":
		return time.Now().UTC().Format(time.RFC3339)
	case "email":
		return "test@example.com"
	case "uuid":
		return uuid.New().String()
	}

	length := 5
	if schema.MinLength > 0 {
		length = int(schema.MinLength)
	}
	return strings.Repeat("s", length)
}

func generateNumberValue(schema *openapi3.Schema, useInvalid bool, boundary string) interface{} {
	if useInvalid {
		return "not_a_number"
	}

	val := 1.0
	if schema.Min != nil {
		val = *schema.Min
	}

	if boundary == "min_below" && schema.Min != nil {
		if isExclusiveMin(schema) {
			return val
		}
		return val - 1
	}
	if boundary == "min_exact" && schema.Min != nil {
		if isExclusiveMin(schema) {
			return val + 1
		}
		return val
	}
	if boundary == "max_above" && schema.Max != nil {
		if isExclusiveMax(schema) {
			return *schema.Max
		}
		return *schema.Max + 1
	}
	if boundary == "max_exact" && schema.Max != nil {
		if isExclusiveMax(schema) {
			return *schema.Max - 1
		}
		return *schema.Max
	}

	return val
}

func generateBooleanValue(schema *openapi3.Schema, useInvalid bool) interface{} {
	if useInvalid {
		return "not_a_boolean"
	}
	return true
}

func generateObjectValue(schema *openapi3.Schema, useInvalid bool, boundary string) interface{} {
	return generateObjectValueEx(schema, useInvalid, boundary, "")
}

func generateObjectValueEx(schema *openapi3.Schema, useInvalid bool, boundary string, omitFieldName string) interface{} {
	if useInvalid && omitFieldName == "" {
		// Simulating invalid type or corrupted structure
		return "invalid_object_type"
	}

	obj := make(map[string]interface{})
	for name, ref := range schema.Properties {
		if ref.Value == nil {
			continue
		}

		if name == omitFieldName {
			continue
		}

		isRequired := false
		for _, req := range schema.Required {
			if req == name {
				isRequired = true
				break
			}
		}

		valUseInvalid := useInvalid
		if omitFieldName != "" {
			valUseInvalid = false
		} else if useInvalid && isRequired {
			continue
		}

		obj[name] = generateSchemaValueEx(ref.Value, valUseInvalid, boundary, "")
	}
	return obj
}

func generateArrayValue(schema *openapi3.Schema, useInvalid bool, boundary string) interface{} {
	return generateArrayValueEx(schema, useInvalid, boundary, "")
}

func generateArrayValueEx(schema *openapi3.Schema, useInvalid bool, boundary string, omitFieldName string) interface{} {
	if useInvalid && omitFieldName == "" {
		return "invalid_array_type"
	}
	var items []interface{}
	if schema.Items != nil && schema.Items.Value != nil {
		items = append(items, generateSchemaValueEx(schema.Items.Value, useInvalid, boundary, omitFieldName))
	}
	return items
}

// Helper methods to inject values for SQLi and XSS security probes

func injectMap(m map[string]string, payload string) map[string]string {
	res := make(map[string]string)
	for k := range m {
		res[k] = payload // replace string parameter value with security probe payload
	}
	return res
}

func injectInterface(val interface{}, payload string) interface{} {
	switch v := val.(type) {
	case string:
		return payload
	case map[string]interface{}:
		res := make(map[string]interface{})
		for k, mv := range v {
			res[k] = injectInterface(mv, payload)
		}
		return res
	case []interface{}:
		var res []interface{}
		for _, sv := range v {
			res = append(res, injectInterface(sv, payload))
		}
		return res
	}
	return val
}

// ComputeHash generates SHA-256 hex string for deduplication comparison.
func ComputeHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// schemaHasType checks if the schema definition contains the specified type.
// Compatible with both OpenAPI 3.0 and 3.1.
func schemaHasType(schema *openapi3.Schema, t string) bool {
	return schema.Type != nil && schema.Type.Includes(t)
}

// isExclusiveMin checks if the minimum value constraint is exclusive.
func isExclusiveMin(schema *openapi3.Schema) bool {
	if schema.ExclusiveMin.Bool != nil {
		return *schema.ExclusiveMin.Bool
	}
	if schema.ExclusiveMin.Value != nil {
		return true
	}
	return false
}

// isExclusiveMax checks if the maximum value constraint is exclusive.
func isExclusiveMax(schema *openapi3.Schema) bool {
	if schema.ExclusiveMax.Bool != nil {
		return *schema.ExclusiveMax.Bool
	}
	if schema.ExclusiveMax.Value != nil {
		return true
	}
	return false
}
