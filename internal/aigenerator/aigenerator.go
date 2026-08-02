// Package aigenerator provides LLM-powered test case generation using AWS Bedrock (Gemma 4).
package aigenerator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/UjjwalVandur/TestBud/internal/generator"
	"github.com/UjjwalVandur/TestBud/internal/models"
)

// DefaultRateLimitDelay is the minimum delay between LLM invocations to avoid hitting Bedrock rate limits.
const DefaultRateLimitDelay = 500 * time.Millisecond

// HTTPClient interface allows mocking HTTP calls in unit tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// AIGenerator generates supplementary edge-case test cases using AWS Bedrock.
type AIGenerator struct {
	region     string
	modelID    string
	endpointURL string // Custom endpoint URL for testing or Bedrock Mantle
	httpClient HTTPClient
	logger     *logrus.Logger
	rateLimit  time.Duration
	mu         sync.Mutex
	lastCall   time.Time
}

// Option configures AIGenerator.
type Option func(*AIGenerator)

// WithHTTPClient sets a custom HTTP client (useful for unit tests).
func WithHTTPClient(client HTTPClient) Option {
	return func(g *AIGenerator) {
		g.httpClient = client
	}
}

// WithEndpointURL sets a custom endpoint URL (useful for unit tests or proxies).
func WithEndpointURL(url string) Option {
	return func(g *AIGenerator) {
		g.endpointURL = url
	}
}

// WithLogger sets a custom logger.
func WithLogger(logger *logrus.Logger) Option {
	return func(g *AIGenerator) {
		g.logger = logger
	}
}

// WithRateLimit sets a custom rate-limiting delay between LLM requests.
func WithRateLimit(delay time.Duration) Option {
	return func(g *AIGenerator) {
		g.rateLimit = delay
	}
}

// New creates a new AIGenerator instance.
func New(region, modelID string, opts ...Option) (*AIGenerator, error) {
	if region == "" {
		return nil, fmt.Errorf("aws region is required")
	}
	if modelID == "" {
		return nil, fmt.Errorf("aws model id is required")
	}

	g := &AIGenerator{
		region:     region,
		modelID:    modelID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		logger:     logrus.New(),
		rateLimit:  DefaultRateLimitDelay,
	}

	for _, opt := range opts {
		opt(g)
	}

	if g.endpointURL == "" {
		// Default Bedrock Mantle / Runtime URL format
		g.endpointURL = fmt.Sprintf("https://bedrock-mantle.%s.api.aws/model/%s/invoke", region, modelID)
	}

	return g, nil
}

// rawAITestCase represents the JSON structure returned by the LLM.
type rawAITestCase struct {
	Description    string                 `json:"description"`
	Payload        generator.TestCasePayload `json:"payload"`
	ExpectedStatus int                    `json:"expected_status"`
}

// bedrockRequest is the payload sent to Bedrock / Gemma endpoint.
type bedrockRequest struct {
	Prompt      string  `json:"prompt,omitempty"`
	Inputs      string  `json:"inputs,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// bedrockResponse represents a response from Bedrock / Gemma.
type bedrockResponse struct {
	OutputText string `json:"output_text,omitempty"`
	Completion string `json:"completion,omitempty"`
	Generation string `json:"generation,omitempty"`
}

// Generate sends the endpoint spec to Gemma 4 via Bedrock and parses generated test cases.
// On any error (network, rate limit, JSON error), it logs a warning and returns an empty slice
// to ensure graceful degradation.
func (g *AIGenerator) Generate(ctx context.Context, endpoint models.Endpoint) ([]models.TestCase, error) {
	// Rate limiting enforcement
	g.enforceRateLimit(ctx)

	prompt := g.buildPrompt(endpoint)

	reqBody := bedrockRequest{
		Prompt:      prompt,
		Inputs:      prompt,
		Temperature: 0.3,
		MaxTokens:   1024,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		g.logger.WithError(err).Warn("AI generator: failed to marshal prompt body")
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpointURL, bytes.NewReader(bodyBytes))
	if err != nil {
		g.logger.WithError(err).Warn("AI generator: failed to create request")
		return nil, nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.logger.WithError(err).Warn("AI generator: request failed")
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		g.logger.WithFields(logrus.Fields{
			"status": resp.StatusCode,
			"body":   string(respBytes),
		}).Warn("AI generator: Bedrock API returned non-200 status")
		return nil, nil
	}

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		g.logger.WithError(err).Warn("AI generator: failed to read response body")
		return nil, nil
	}

	rawTestCases, err := g.parseLLMResponse(respData)
	if err != nil {
		g.logger.WithError(err).Warn("AI generator: failed to parse LLM response")
		return nil, nil
	}

	var testCases []models.TestCase
	for _, raw := range rawTestCases {
		raw.Payload.Description = raw.Description
		payloadBytes, err := json.Marshal(raw.Payload)
		if err != nil {
			continue
		}

		expectedStatus := raw.ExpectedStatus
		if expectedStatus == 0 {
			expectedStatus = 400
		}

		testCases = append(testCases, models.TestCase{
			EndpointID:     endpoint.ID,
			Category:       models.CategoryAIEnhanced,
			PayloadJSON:    payloadBytes,
			ExpectedStatus: expectedStatus,
			GeneratedAt:    time.Now().UTC(),
		})
	}

	g.logger.WithFields(logrus.Fields{
		"endpoint_id": endpoint.ID,
		"method":      endpoint.Method,
		"path":        endpoint.Path,
		"count":       len(testCases),
	}).Info("AI generator: generated test cases")

	return testCases, nil
}

func (g *AIGenerator) enforceRateLimit(ctx context.Context) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.rateLimit <= 0 {
		return
	}

	elapsed := time.Since(g.lastCall)
	if elapsed < g.rateLimit {
		sleepTime := g.rateLimit - elapsed
		select {
		case <-ctx.Done():
		case <-time.After(sleepTime):
		}
	}
	g.lastCall = time.Now()
}

func (g *AIGenerator) buildPrompt(endpoint models.Endpoint) string {
	var sb strings.Builder
	sb.WriteString("You are an expert API testing engineer. Generate creative edge-case test cases for the following API endpoint.\n\n")
	sb.WriteString(fmt.Sprintf("Method: %s\n", endpoint.Method))
	sb.WriteString(fmt.Sprintf("Path: %s\n", endpoint.Path))
	sb.WriteString(fmt.Sprintf("Auth Required: %v\n", endpoint.AuthRequired))

	if len(endpoint.ParametersJSON) > 0 && string(endpoint.ParametersJSON) != "null" {
		sb.WriteString(fmt.Sprintf("Parameters: %s\n", string(endpoint.ParametersJSON)))
	}
	if len(endpoint.RequestSchemaJSON) > 0 && string(endpoint.RequestSchemaJSON) != "null" {
		sb.WriteString(fmt.Sprintf("Request Schema: %s\n", string(endpoint.RequestSchemaJSON)))
	}

	sb.WriteString(`
Generate 2 to 4 edge-case test cases that test business logic flaws, inter-field dependencies, or unusual semantic values.

Respond ONLY with a JSON array of objects with the following structure:
[
  {
    "description": "Short explanation of the test case",
    "expected_status": 400,
    "payload": {
      "headers": {},
      "query_params": {},
      "path_params": {},
      "body": {}
    }
  }
]

Do not include any Markdown formatting or code block backticks (like ` + "```json" + `). Output raw JSON only.`)

	return sb.String()
}

func (g *AIGenerator) parseLLMResponse(data []byte) ([]rawAITestCase, error) {
	// First try parsing Bedrock envelope
	var bResp bedrockResponse
	if err := json.Unmarshal(data, &bResp); err == nil {
		text := bResp.OutputText
		if text == "" {
			text = bResp.Completion
		}
		if text == "" {
			text = bResp.Generation
		}
		if text != "" {
			data = []byte(text)
		}
	}

	// Clean Markdown code fences if present
	text := strings.TrimSpace(string(data))
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var rawCases []rawAITestCase
	if err := json.Unmarshal([]byte(text), &rawCases); err != nil {
		return nil, fmt.Errorf("unmarshal test cases: %w (raw: %s)", err, text)
	}

	return rawCases, nil
}

// Ensure AIGenerator implements service.TestCaseGenerator interface at compile time.
var _ func(ctx context.Context, endpoint models.Endpoint) ([]models.TestCase, error) = (*AIGenerator)(nil).Generate
