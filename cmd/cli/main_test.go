package main

import (
	"bytes"
	"testing"
)

func TestParseFlagsAllRequired(t *testing.T) {
	args := []string{
		"--api-url", "http://localhost:8080",
		"--api-key", "test-key",
		"--project-id", "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"--version", "1.0.0",
		"--schema-file", "openapi.yaml",
		"--target-url", "http://staging.example.com",
	}

	cfg, err := parseFlags(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.apiURL != "http://localhost:8080" {
		t.Errorf("api-url = %q, want %q", cfg.apiURL, "http://localhost:8080")
	}
	if cfg.apiKey != "test-key" {
		t.Errorf("api-key = %q, want %q", cfg.apiKey, "test-key")
	}
	if cfg.failThreshold != 100.0 {
		t.Errorf("fail-threshold = %f, want 100.0", cfg.failThreshold)
	}
}

func TestParseFlagsMissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing all",
			args:    []string{},
			wantErr: "--api-url, --api-key, --project-id, --version, --schema-file, --target-url",
		},
		{
			name: "missing api-key only",
			args: []string{
				"--api-url", "http://localhost",
				"--project-id", "uuid",
				"--version", "1.0",
				"--schema-file", "f.yaml",
				"--target-url", "http://target",
			},
			wantErr: "--api-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseFlags(tt.args)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !bytes.Contains([]byte(err.Error()), []byte(tt.wantErr)) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseFlagsInvalidThreshold(t *testing.T) {
	args := []string{
		"--api-url", "http://localhost:8080",
		"--api-key", "key",
		"--project-id", "uuid",
		"--version", "1.0",
		"--schema-file", "f.yaml",
		"--target-url", "http://target",
		"--fail-threshold", "150",
	}

	_, err := parseFlags(args)
	if err == nil {
		t.Fatal("expected error for threshold > 100")
	}
}

func TestParseFlagsCustomThreshold(t *testing.T) {
	args := []string{
		"--api-url", "http://localhost:8080",
		"--api-key", "key",
		"--project-id", "uuid",
		"--version", "1.0",
		"--schema-file", "f.yaml",
		"--target-url", "http://target",
		"--fail-threshold", "80",
	}

	cfg, err := parseFlags(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.failThreshold != 80.0 {
		t.Errorf("fail-threshold = %f, want 80.0", cfg.failThreshold)
	}
}

func TestPrintResultsAllPass(t *testing.T) {
	var buf bytes.Buffer

	upload := &uploadResult{
		SchemaID:      "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		EndpointCount: 3,
	}
	details := &executionListResult{
		Summary: executionSummary{
			Total:         10,
			Passed:        10,
			Failed:        0,
			AvgResponseMs: 42.5,
		},
		Executions: []executionItem{},
	}
	cfg := config{version: "1.0.0", failThreshold: 100}

	code := printResults(&buf, cfg, upload, details)
	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if !bytes.Contains(buf.Bytes(), []byte("100.0%")) {
		t.Errorf("output should contain 100.0%%, got:\n%s", buf.String())
	}
}

func TestPrintResultsSomeFail(t *testing.T) {
	var buf bytes.Buffer

	upload := &uploadResult{
		SchemaID:      "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		EndpointCount: 2,
	}
	details := &executionListResult{
		Summary: executionSummary{
			Total:         10,
			Passed:        8,
			Failed:        2,
			AvgResponseMs: 55.0,
		},
		Executions: []executionItem{
			{Category: "negative", ExpectedStatus: 400, ActualStatus: 200, ResponseMs: 30, Passed: false},
			{Category: "security", ExpectedStatus: 401, ActualStatus: 200, ResponseMs: 15, Passed: false},
		},
	}
	cfg := config{version: "2.0.0", failThreshold: 100}

	code := printResults(&buf, cfg, upload, details)
	if code != exitTestFail {
		t.Errorf("exit code = %d, want %d", code, exitTestFail)
	}
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("negative")) {
		t.Error("output should list the negative failure")
	}
	if !bytes.Contains([]byte(output), []byte("security")) {
		t.Error("output should list the security failure")
	}
}

func TestPrintResultsCustomThresholdPass(t *testing.T) {
	var buf bytes.Buffer

	upload := &uploadResult{
		SchemaID:      "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		EndpointCount: 2,
	}
	details := &executionListResult{
		Summary: executionSummary{
			Total:  10,
			Passed: 9,
			Failed: 1,
		},
		Executions: []executionItem{
			{Category: "boundary", ExpectedStatus: 400, ActualStatus: 200, ResponseMs: 20, Passed: false},
		},
	}
	cfg := config{version: "1.0.0", failThreshold: 80}

	code := printResults(&buf, cfg, upload, details)
	if code != exitOK {
		t.Errorf("exit code = %d, want %d (90%% >= 80%% threshold)", code, exitOK)
	}
}

func TestRunMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != exitInfra {
		t.Errorf("exit code = %d, want %d", code, exitInfra)
	}
}
