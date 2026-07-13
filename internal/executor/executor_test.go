package executor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

// newTestEndpoint creates a minimal Endpoint for tests.
func newTestEndpoint(method, path string) models.Endpoint {
	return models.Endpoint{
		ID:     uuid.New(),
		Method: method,
		Path:   path,
	}
}

// newTestCase creates a TestCase with the given payload and expected status.
func newTestCase(endpointID uuid.UUID, payload interface{}, expectedStatus int, category models.TestCaseCategory) models.TestCase {
	raw, _ := json.Marshal(payload)
	return models.TestCase{
		ID:             uuid.New(),
		EndpointID:     endpointID,
		Category:       category,
		PayloadJSON:    datatypes.JSON(raw),
		ExpectedStatus: expectedStatus,
		GeneratedAt:    time.Now().UTC(),
	}
}

func TestExecutor_SingleTestCase(t *testing.T) {
	// Set up a mock target server that echoes status 200.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path param substitution happened.
		if !strings.Contains(r.URL.Path, "/pets/123") {
			t.Errorf("expected path /pets/123, got %s", r.URL.Path)
		}
		// Verify query param.
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected query param limit=10, got %s", r.URL.Query().Get("limit"))
		}
		// Verify custom header.
		if r.Header.Get("X-Request-ID") != "test-id" {
			t.Errorf("expected header X-Request-ID=test-id, got %s", r.Header.Get("X-Request-ID"))
		}
		// Verify auth header.
		if r.Header.Get("Authorization") != "Bearer valid-token" {
			t.Errorf("expected Authorization=Bearer valid-token, got %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint := newTestEndpoint("GET", "/pets/{petId}")
	payload := map[string]interface{}{
		"path_params":  map[string]string{"petId": "123"},
		"query_params": map[string]string{"limit": "10"},
		"headers":      map[string]string{"X-Request-ID": "test-id"},
	}
	tc := newTestCase(endpoint.ID, payload, http.StatusOK, models.CategoryPositive)

	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), ExecutionInput{
		TargetURL:   server.URL,
		Endpoint:    endpoint,
		TestCase:    tc,
		AuthHeaders: map[string]string{"Authorization": "Bearer valid-token"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true, got false (actual=%d, expected=%d)", result.ActualStatus, tc.ExpectedStatus)
	}
	if result.ActualStatus != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.ActualStatus)
	}
	if result.ResponseMs < 0 {
		t.Errorf("expected non-negative ResponseMs, got %d", result.ResponseMs)
	}
}

func TestExecutor_AuthOmit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header, got %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	endpoint := newTestEndpoint("GET", "/secure")
	endpoint.AuthRequired = true
	payload := map[string]interface{}{
		"omit_auth": true,
	}
	tc := newTestCase(endpoint.ID, payload, http.StatusUnauthorized, models.CategorySecurity)

	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), ExecutionInput{
		TargetURL:   server.URL,
		Endpoint:    endpoint,
		TestCase:    tc,
		AuthHeaders: map[string]string{"Authorization": "Bearer should-not-appear"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true (expected 401, got %d)", result.ActualStatus)
	}
}

func TestExecutor_AltAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer alt-user-token" {
			t.Errorf("expected alt auth header, got %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	endpoint := newTestEndpoint("GET", "/admin/resource")
	endpoint.AuthRequired = true
	payload := map[string]interface{}{
		"use_other_user_auth": true,
	}
	tc := newTestCase(endpoint.ID, payload, http.StatusForbidden, models.CategorySecurity)

	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), ExecutionInput{
		TargetURL:      server.URL,
		Endpoint:       endpoint,
		TestCase:       tc,
		AuthHeaders:    map[string]string{"Authorization": "Bearer primary-token"},
		AltAuthHeaders: map[string]string{"Authorization": "Bearer alt-user-token"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true (expected 403, got %d)", result.ActualStatus)
	}
}

func TestExecutor_OversizedProbe(t *testing.T) {
	var receivedBodySize int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read up to 1MB then stop.
		limited := http.MaxBytesReader(w, r.Body, 1<<20)
		data, _ := io.ReadAll(limited)
		receivedBodySize = len(data)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
	}))
	defer server.Close()

	endpoint := newTestEndpoint("POST", "/upload")
	oversizedSize := 2048 // 2 KB for the test (small enough to be fast)
	payload := map[string]interface{}{
		"is_oversized_probe": true,
		"oversized_bytes":    oversizedSize,
	}
	tc := newTestCase(endpoint.ID, payload, http.StatusRequestEntityTooLarge, models.CategorySecurity)

	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), ExecutionInput{
		TargetURL: server.URL,
		Endpoint:  endpoint,
		TestCase:  tc,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ActualStatus != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", result.ActualStatus)
	}
	// The server should have received some body data.
	if receivedBodySize == 0 {
		t.Error("expected server to receive oversized body, got 0 bytes")
	}
}

func TestExecutor_RateLimitProbe(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount >= 5 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint := newTestEndpoint("GET", "/api/data")
	payload := map[string]interface{}{
		"is_rate_limit_probe": true,
	}
	tc := newTestCase(endpoint.ID, payload, http.StatusTooManyRequests, models.CategorySecurity)

	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), ExecutionInput{
		TargetURL: server.URL,
		Endpoint:  endpoint,
		TestCase:  tc,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ActualStatus != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", result.ActualStatus)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true")
	}
	if callCount < 5 {
		t.Errorf("expected at least 5 requests, got %d", callCount)
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	// Server that blocks forever.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(30 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	endpoint := newTestEndpoint("GET", "/slow")
	tc := newTestCase(endpoint.ID, map[string]interface{}{}, http.StatusOK, models.CategoryPositive)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	exec := NewExecutor()
	result, err := exec.Execute(ctx, ExecutionInput{
		TargetURL: server.URL,
		Endpoint:  endpoint,
		TestCase:  tc,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fail due to context cancellation → status 0.
	if result.Passed {
		t.Error("expected Passed=false for cancelled context")
	}
	if result.ActualStatus != 0 {
		t.Errorf("expected status 0 for network error, got %d", result.ActualStatus)
	}
}

func TestExecutor_PostWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body["name"] != "test-pet" {
			t.Errorf("expected body.name=test-pet, got %v", body["name"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	endpoint := newTestEndpoint("POST", "/pets")
	payload := map[string]interface{}{
		"body": map[string]interface{}{
			"name": "test-pet",
			"tag":  "dog",
		},
	}
	tc := newTestCase(endpoint.ID, payload, http.StatusCreated, models.CategoryPositive)

	exec := NewExecutor()
	result, err := exec.Execute(context.Background(), ExecutionInput{
		TargetURL: server.URL,
		Endpoint:  endpoint,
		TestCase:  tc,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true (expected 201, got %d)", result.ActualStatus)
	}
}

