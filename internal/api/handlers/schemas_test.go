package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/api/middleware"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

type fakeSchemaUploader struct {
	input service.UploadSchemaInput
	err   error
}

func (f *fakeSchemaUploader) UploadSchema(_ context.Context, input service.UploadSchemaInput) (service.UploadSchemaResult, error) {
	f.input = input
	if f.err != nil {
		return service.UploadSchemaResult{}, f.err
	}
	return service.UploadSchemaResult{
		SchemaID:       uuid.New(),
		SchemaHash:     "schema-hash",
		OpenAPIVersion: "3.0.3",
		EndpointCount:  1,
	}, nil
}

// fakeUserLookup stubs the middleware.UserLookup interface for testing.
type fakeUserLookup struct {
	userID uuid.UUID
}

func (f fakeUserLookup) FindUserIDByAPIKey(_ context.Context, _ string) (uuid.UUID, error) {
	return f.userID, nil
}

func TestSchemaHandlerUpload(t *testing.T) {
	projectID := uuid.New()
	authenticatedUserID := uuid.New()

	tests := []struct {
		name       string
		form       map[string]string
		fileName   string
		fileBody   string
		uploader   *fakeSchemaUploader
		apiKey     string
		userID     uuid.UUID
		wantStatus int
	}{
		{
			name: "valid upload",
			form: map[string]string{
				"project_id": projectID.String(),
				"version":    "1.0.0",
			},
			fileName:   "openapi.json",
			fileBody:   `{"openapi":"3.0.3"}`,
			uploader:   &fakeSchemaUploader{},
			apiKey:     "test-key",
			userID:     authenticatedUserID,
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing file",
			form: map[string]string{
				"project_id": projectID.String(),
				"version":    "1.0.0",
			},
			uploader:   &fakeSchemaUploader{},
			apiKey:     "test-key",
			userID:     authenticatedUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid project id",
			form: map[string]string{
				"project_id": "not-a-uuid",
				"version":    "1.0.0",
			},
			fileName:   "openapi.json",
			fileBody:   `{}`,
			uploader:   &fakeSchemaUploader{},
			apiKey:     "test-key",
			userID:     authenticatedUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "service validation error",
			form: map[string]string{
				"project_id": projectID.String(),
				"version":    "1.0.0",
			},
			fileName:   "openapi.json",
			fileBody:   `{}`,
			uploader:   &fakeSchemaUploader{err: errors.New("parse failed")},
			apiKey:     "test-key",
			userID:     authenticatedUserID,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing api key returns 401",
			form: map[string]string{
				"project_id": projectID.String(),
				"version":    "1.0.0",
			},
			fileName:   "openapi.json",
			fileBody:   `{}`,
			uploader:   &fakeSchemaUploader{},
			apiKey:     "",
			userID:     uuid.Nil,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()

			lookup := fakeUserLookup{userID: tt.userID}
			api := router.Group("/api")
			api.Use(middleware.APIKeyAuth(lookup))
			api.POST("/schemas", NewSchemaHandler(tt.uploader).Upload)

			body, contentType := multipartBody(t, tt.form, tt.fileName, tt.fileBody)
			req := httptest.NewRequest(http.MethodPost, "/api/schemas", body)
			req.Header.Set("Content-Type", contentType)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusCreated {
				var response service.UploadSchemaResult
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if response.EndpointCount != 1 {
					t.Fatalf("EndpointCount = %d, want 1", response.EndpointCount)
				}
				if string(tt.uploader.input.RawBytes) != tt.fileBody {
					t.Fatalf("RawBytes = %q, want %q", string(tt.uploader.input.RawBytes), tt.fileBody)
				}
				// Verify uploaded_by comes from auth context, not form input.
				if tt.uploader.input.UploadedBy != tt.userID {
					t.Fatalf("UploadedBy = %v, want %v (from auth context)", tt.uploader.input.UploadedBy, tt.userID)
				}
			}
		})
	}
}

func multipartBody(t *testing.T, fields map[string]string, fileName, fileBody string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if fileName != "" {
		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte(fileBody)); err != nil {
			t.Fatalf("write form file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return body, writer.FormDataContentType()
}
