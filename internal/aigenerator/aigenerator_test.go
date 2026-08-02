package aigenerator

import (
	"context"
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

type mockHTTPClient struct {
	handler func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.handler(req)
}

func TestAIGeneratorSuccess(t *testing.T) {
	mockResponseJSON := `[
		{
			"description": "Negative pet age",
			"expected_status": 400,
			"payload": {
				"body": {"name": "Buddy", "age": -5}
			}
		},
		{
			"description": "Future date created",
			"expected_status": 400,
			"payload": {
				"body": {"name": "Buddy", "created_at": "2099-01-01"}
			}
		}
	]`

	client := &mockHTTPClient{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Errorf("expected POST method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(mockResponseJSON))),
			}, nil
		},
	}

	gen, err := New("us-east-1", "google.gemma-4-31b",
		WithHTTPClient(client),
		WithEndpointURL("http://localhost/mock"),
		WithRateLimit(1*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("unexpected error creating generator: %v", err)
	}

	epID := uuid.New()
	ep := models.Endpoint{
		ID:                 epID,
		Method:             "POST",
		Path:               "/pets",
		AuthRequired:       true,
		RequestSchemaJSON:  datatypes.JSON(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	}

	cases, err := gen.Generate(context.Background(), ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cases) != 2 {
		t.Fatalf("expected 2 test cases, got %d", len(cases))
	}

	if cases[0].Category != models.CategoryAIEnhanced {
		t.Errorf("expected category %s, got %s", models.CategoryAIEnhanced, cases[0].Category)
	}
	if cases[0].ExpectedStatus != 400 {
		t.Errorf("expected status 400, got %d", cases[0].ExpectedStatus)
	}
	if cases[0].EndpointID != epID {
		t.Errorf("expected endpoint ID %s, got %s", epID, cases[0].EndpointID)
	}
}

func TestAIGeneratorBedrockEnvelopeParsing(t *testing.T) {
	mockEnvelope := `{"output_text": "[{\"description\": \"Test\", \"expected_status\": 422, \"payload\": {\"body\": {}}}]"}`

	client := &mockHTTPClient{
		handler: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(mockEnvelope))),
			}, nil
		},
	}

	gen, err := New("us-east-1", "google.gemma-4-31b",
		WithHTTPClient(client),
		WithEndpointURL("http://localhost/mock"),
		WithRateLimit(1*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	cases, err := gen.Generate(context.Background(), models.Endpoint{ID: uuid.New()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(cases))
	}
	if cases[0].ExpectedStatus != 422 {
		t.Errorf("expected status 422, got %d", cases[0].ExpectedStatus)
	}
}

func TestAIGeneratorGracefulDegradationOnAPIError(t *testing.T) {
	client := &mockHTTPClient{
		handler: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":"internal error"}`))),
			}, nil
		},
	}

	gen, err := New("us-east-1", "google.gemma-4-31b",
		WithHTTPClient(client),
		WithEndpointURL("http://localhost/mock"),
		WithRateLimit(1*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	// Should not return an error, but return an empty slice (graceful degradation)
	cases, err := gen.Generate(context.Background(), models.Endpoint{ID: uuid.New()})
	if err != nil {
		t.Fatalf("expected nil error (graceful fallback), got %v", err)
	}
	if len(cases) != 0 {
		t.Errorf("expected 0 cases on error, got %d", len(cases))
	}
}
