// Package main provides the TestBud CLI for CI/CD integration.
//
// Usage:
//
//	testbud-cli \
//	  --api-url https://testbud.example.com \
//	  --api-key YOUR_KEY \
//	  --project-id UUID \
//	  --version 1.0.0 \
//	  --schema-file ./openapi.yaml \
//	  --target-url https://staging.example.com
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// Exit codes.
const (
	exitOK       = 0
	exitTestFail = 1
	exitInfra    = 2
)

// ---------- DTOs (mirror server responses) ----------

type uploadResult struct {
	SchemaID       string `json:"schema_id"`
	SchemaHash     string `json:"schema_hash"`
	OpenAPIVersion string `json:"openapi_version"`
	EndpointCount  int    `json:"endpoint_count"`
}

type executeResult struct {
	SchemaID string `json:"schema_id"`
	Total    int    `json:"total"`
	Passed   int    `json:"passed"`
	Failed   int    `json:"failed"`
}

type executionItem struct {
	ExecutionID    string `json:"execution_id"`
	Category       string `json:"category"`
	ExpectedStatus int    `json:"expected_status"`
	ActualStatus   int    `json:"actual_status"`
	ResponseMs     int64  `json:"response_ms"`
	Passed         bool   `json:"passed"`
	RanAt          string `json:"ran_at"`
}

type executionSummary struct {
	Total         int     `json:"total"`
	Passed        int     `json:"passed"`
	Failed        int     `json:"failed"`
	AvgResponseMs float64 `json:"avg_response_ms"`
}

type executionListResult struct {
	SchemaID   string           `json:"schema_id"`
	Summary    executionSummary `json:"summary"`
	Executions []executionItem  `json:"executions"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// ---------- Config ----------

type config struct {
	apiURL         string
	apiKey         string
	projectID      string
	version        string
	schemaFile     string
	targetURL      string
	authHeaders    string
	altAuthHeaders string
	failThreshold  float64
}

func parseFlags(args []string) (config, error) {
	fs := flag.NewFlagSet("testbud-cli", flag.ContinueOnError)
	var cfg config

	fs.StringVar(&cfg.apiURL, "api-url", "", "TestBud backend URL (required)")
	fs.StringVar(&cfg.apiKey, "api-key", "", "API key for authentication (required)")
	fs.StringVar(&cfg.projectID, "project-id", "", "Project UUID (required)")
	fs.StringVar(&cfg.version, "version", "", "Schema version string (required)")
	fs.StringVar(&cfg.schemaFile, "schema-file", "", "Path to OpenAPI/Swagger file (required)")
	fs.StringVar(&cfg.targetURL, "target-url", "", "Target API URL to test against (required)")
	fs.StringVar(&cfg.authHeaders, "auth-headers", "", "JSON string of auth headers (optional)")
	fs.StringVar(&cfg.altAuthHeaders, "alt-auth-headers", "", "JSON string of alt auth headers (optional)")
	fs.Float64Var(&cfg.failThreshold, "fail-threshold", 100.0, "Minimum pass rate % (default 100)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	var missing []string
	if cfg.apiURL == "" {
		missing = append(missing, "--api-url")
	}
	if cfg.apiKey == "" {
		missing = append(missing, "--api-key")
	}
	if cfg.projectID == "" {
		missing = append(missing, "--project-id")
	}
	if cfg.version == "" {
		missing = append(missing, "--version")
	}
	if cfg.schemaFile == "" {
		missing = append(missing, "--schema-file")
	}
	if cfg.targetURL == "" {
		missing = append(missing, "--target-url")
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing required flags: %s", strings.Join(missing, ", "))
	}

	if cfg.failThreshold < 0 || cfg.failThreshold > 100 {
		return cfg, fmt.Errorf("--fail-threshold must be between 0 and 100, got %.1f", cfg.failThreshold)
	}

	return cfg, nil
}

// ---------- HTTP helpers ----------

func apiRequest(method, url, apiKey string, body io.Reader, contentType string) ([]byte, int, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return data, resp.StatusCode, nil
}

func checkAPIError(data []byte, statusCode int, action string) error {
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}
	var errResp errorResponse
	if err := json.Unmarshal(data, &errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("%s failed (HTTP %d): %s", action, statusCode, errResp.Error)
	}
	return fmt.Errorf("%s failed (HTTP %d): %s", action, statusCode, string(data))
}

// ---------- Steps ----------

func stepUpload(cfg config) (*uploadResult, error) {
	fileBytes, err := os.ReadFile(cfg.schemaFile)
	if err != nil {
		return nil, fmt.Errorf("read schema file: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if err := writer.WriteField("project_id", cfg.projectID); err != nil {
		return nil, fmt.Errorf("write project_id field: %w", err)
	}
	if err := writer.WriteField("version", cfg.version); err != nil {
		return nil, fmt.Errorf("write version field: %w", err)
	}

	part, err := writer.CreateFormFile("file", filepath.Base(cfg.schemaFile))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(fileBytes); err != nil {
		return nil, fmt.Errorf("write file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := strings.TrimRight(cfg.apiURL, "/") + "/api/schemas"
	data, status, err := apiRequest(http.MethodPost, url, cfg.apiKey, &buf, writer.FormDataContentType())
	if err != nil {
		return nil, err
	}
	if err := checkAPIError(data, status, "upload"); err != nil {
		return nil, err
	}

	var result uploadResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}
	return &result, nil
}

func stepExecute(cfg config, schemaID string) (*executeResult, error) {
	body := map[string]interface{}{
		"target_url": cfg.targetURL,
	}
	if cfg.authHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(cfg.authHeaders), &headers); err != nil {
			return nil, fmt.Errorf("parse --auth-headers JSON: %w", err)
		}
		body["auth_headers"] = headers
	}
	if cfg.altAuthHeaders != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(cfg.altAuthHeaders), &headers); err != nil {
			return nil, fmt.Errorf("parse --alt-auth-headers JSON: %w", err)
		}
		body["alt_auth_headers"] = headers
	}

	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal execute body: %w", err)
	}

	url := strings.TrimRight(cfg.apiURL, "/") + "/api/schemas/" + schemaID + "/executions"
	data, status, err := apiRequest(http.MethodPost, url, cfg.apiKey, bytes.NewReader(jsonBytes), "application/json")
	if err != nil {
		return nil, err
	}
	if err := checkAPIError(data, status, "execute"); err != nil {
		return nil, err
	}

	var result executeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse execute response: %w", err)
	}
	return &result, nil
}

func stepFetchDetails(cfg config, schemaID string) (*executionListResult, error) {
	url := strings.TrimRight(cfg.apiURL, "/") + "/api/schemas/" + schemaID + "/executions"
	data, status, err := apiRequest(http.MethodGet, url, cfg.apiKey, nil, "")
	if err != nil {
		return nil, err
	}
	if err := checkAPIError(data, status, "fetch details"); err != nil {
		return nil, err
	}

	var result executionListResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse execution list response: %w", err)
	}
	return &result, nil
}

// ---------- Output ----------

func printResults(w io.Writer, cfg config, upload *uploadResult, details *executionListResult) int {
	fmt.Fprintf(w, "\nTestBud CI — Schema v%s\n", cfg.version)
	fmt.Fprintln(w, "══════════════════════════════════════════")
	fmt.Fprintf(w, "Upload:    ✓ schema_id=%s (%d endpoints)\n", upload.SchemaID[:8]+"…", upload.EndpointCount)
	fmt.Fprintf(w, "Execute:   ✓ %d tests completed\n\n", details.Summary.Total)

	passRate := float64(0)
	if details.Summary.Total > 0 {
		passRate = float64(details.Summary.Passed) / float64(details.Summary.Total) * 100
	}

	fmt.Fprintln(w, "Results:")
	fmt.Fprintf(w, "  Total:    %d\n", details.Summary.Total)
	fmt.Fprintf(w, "  Passed:   %d  (%.1f%%)\n", details.Summary.Passed, passRate)
	fmt.Fprintf(w, "  Failed:   %d  (%.1f%%)\n", details.Summary.Failed, 100-passRate)
	fmt.Fprintf(w, "  Avg Resp: %.0fms\n", details.Summary.AvgResponseMs)

	// Print failed tests table.
	var failures []executionItem
	for _, ex := range details.Executions {
		if !ex.Passed {
			failures = append(failures, ex)
		}
	}

	if len(failures) > 0 {
		fmt.Fprintf(w, "\nFailed Tests (%d):\n", len(failures))
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  CATEGORY\tEXPECTED\tACTUAL\tRESPONSE\tTIME")
		fmt.Fprintln(tw, "  --------\t--------\t------\t--------\t----")
		for _, f := range failures {
			fmt.Fprintf(tw, "  %s\t%d\t%d\t%dms\t%s\n",
				f.Category, f.ExpectedStatus, f.ActualStatus, f.ResponseMs, f.RanAt)
		}
		tw.Flush()
	}

	fmt.Println()
	if passRate >= cfg.failThreshold {
		fmt.Fprintf(w, "Exit code: 0 (pass rate %.1f%% >= threshold %.1f%%)\n", passRate, cfg.failThreshold)
		return exitOK
	}
	fmt.Fprintf(w, "Exit code: 1 (pass rate %.1f%% < threshold %.1f%%)\n", passRate, cfg.failThreshold)
	return exitTestFail
}

// ---------- Main ----------

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := parseFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitInfra
	}

	// Step 1: Upload schema.
	fmt.Fprintf(stdout, "→ Uploading %s to %s…\n", filepath.Base(cfg.schemaFile), cfg.apiURL)
	upload, err := stepUpload(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitInfra
	}
	fmt.Fprintf(stdout, "  ✓ Uploaded schema %s (%d endpoints)\n", upload.SchemaID[:8]+"…", upload.EndpointCount)

	// Step 2: Execute tests.
	fmt.Fprintf(stdout, "→ Executing tests against %s…\n", cfg.targetURL)
	execResult, err := stepExecute(cfg, upload.SchemaID)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitInfra
	}
	fmt.Fprintf(stdout, "  ✓ %d/%d passed\n", execResult.Passed, execResult.Total)

	// Step 3: Fetch detailed results.
	fmt.Fprintf(stdout, "→ Fetching detailed results…\n")
	details, err := stepFetchDetails(cfg, upload.SchemaID)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return exitInfra
	}

	return printResults(stdout, cfg, upload, details)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
