package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/UjjwalVandur/TestBud/internal/generator"
	"github.com/UjjwalVandur/TestBud/internal/models"
)

const (
	defaultTimeout       = 10 * time.Second
	rateLimitRequestCount = 100
	defaultOversizedBytes = 5 << 20 // 5 MB
)

// ExecutionInput holds everything the executor needs to run a single test case.
type ExecutionInput struct {
	TargetURL      string
	Endpoint       models.Endpoint
	TestCase       models.TestCase
	AuthHeaders    map[string]string // Standard auth headers for the target API.
	AltAuthHeaders map[string]string // Alternative user's auth headers (authz boundary tests).
}

// Executor executes individual API test cases against a target server.
type Executor struct {
	client *http.Client
}

// NewExecutor creates an Executor with the default HTTP client timeout.
func NewExecutor() *Executor {
	return &Executor{
		client: &http.Client{Timeout: defaultTimeout},
	}
}

// Execute runs a single test case and returns an Execution result.
func (e *Executor) Execute(ctx context.Context, input ExecutionInput) (models.Execution, error) {
	var payload generator.TestCasePayload
	if len(input.TestCase.PayloadJSON) > 0 {
		if err := json.Unmarshal(input.TestCase.PayloadJSON, &payload); err != nil {
			return models.Execution{}, fmt.Errorf("unmarshal payload: %w", err)
		}
	}

	// Rate limit probes fire many requests and return the result.
	if payload.IsRateLimitProbe {
		return e.executeRateLimitProbe(ctx, input, payload)
	}

	return e.executeSingle(ctx, input, payload)
}

// executeSingle builds and sends a single HTTP request for one test case.
func (e *Executor) executeSingle(ctx context.Context, input ExecutionInput, payload generator.TestCasePayload) (models.Execution, error) {
	req, err := e.buildRequest(ctx, input, payload)
	if err != nil {
		return models.Execution{}, fmt.Errorf("build request: %w", err)
	}

	start := time.Now()
	resp, err := e.client.Do(req)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		// Network errors count as failures. Record with status 0.
		return models.Execution{
			TestCaseID:   input.TestCase.ID,
			ActualStatus: 0,
			ResponseMs:   elapsed,
			Passed:       false,
		}, nil
	}
	defer resp.Body.Close()
	// Drain body to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	passed := resp.StatusCode == input.TestCase.ExpectedStatus
	return models.Execution{
		TestCaseID:   input.TestCase.ID,
		ActualStatus: resp.StatusCode,
		ResponseMs:   elapsed,
		Passed:       passed,
	}, nil
}

// executeRateLimitProbe fires rateLimitRequestCount sequential requests and
// returns the first execution that gets a 429, or the last execution if none do.
func (e *Executor) executeRateLimitProbe(ctx context.Context, input ExecutionInput, payload generator.TestCasePayload) (models.Execution, error) {
	var lastExec models.Execution
	for i := 0; i < rateLimitRequestCount; i++ {
		select {
		case <-ctx.Done():
			return models.Execution{}, ctx.Err()
		default:
		}

		exec, err := e.executeSingle(ctx, input, payload)
		if err != nil {
			return models.Execution{}, fmt.Errorf("rate limit probe request %d: %w", i+1, err)
		}
		lastExec = exec
		if exec.ActualStatus == http.StatusTooManyRequests {
			// Got a 429 — this is the expected result for rate limit probes.
			lastExec.Passed = (input.TestCase.ExpectedStatus == http.StatusTooManyRequests)
			return lastExec, nil
		}
	}

	// None of the requests triggered rate limiting.
	lastExec.Passed = (lastExec.ActualStatus == input.TestCase.ExpectedStatus)
	return lastExec, nil
}

// buildRequest constructs the http.Request from the endpoint, payload, and auth config.
func (e *Executor) buildRequest(ctx context.Context, input ExecutionInput, payload generator.TestCasePayload) (*http.Request, error) {
	// Build URL: base + path with param substitution + query params.
	reqPath := input.Endpoint.Path
	for name, val := range payload.PathParams {
		reqPath = strings.ReplaceAll(reqPath, "{"+name+"}", url.PathEscape(val))
	}

	fullURL, err := url.JoinPath(input.TargetURL, reqPath)
	if err != nil {
		return nil, fmt.Errorf("join url path: %w", err)
	}

	// Append query parameters.
	if len(payload.QueryParams) > 0 {
		parsed, err := url.Parse(fullURL)
		if err != nil {
			return nil, fmt.Errorf("parse url: %w", err)
		}
		q := parsed.Query()
		for k, v := range payload.QueryParams {
			q.Set(k, v)
		}
		parsed.RawQuery = q.Encode()
		fullURL = parsed.String()
	}

	// Build request body.
	var body io.Reader
	if payload.IsOversizedProbe {
		size := payload.OversizedBytes
		if size <= 0 {
			size = defaultOversizedBytes
		}
		body = bytes.NewReader(bytes.Repeat([]byte("X"), size))
	} else if payload.Body != nil {
		bodyBytes, err := json.Marshal(payload.Body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		body = bytes.NewReader(bodyBytes)
	}

	method := strings.ToUpper(input.Endpoint.Method)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	// Set Content-Type for requests with a body.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply payload-level headers (from the generated test case).
	for k, v := range payload.Headers {
		req.Header.Set(k, v)
	}

	// Apply auth headers based on probe type.
	if !payload.OmitAuth {
		authHeaders := input.AuthHeaders
		if payload.UseOtherUserAuth {
			authHeaders = input.AltAuthHeaders
		}
		for k, v := range authHeaders {
			req.Header.Set(k, v)
		}
	}

	return req, nil
}
