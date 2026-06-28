package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/UjjwalVandur/TestBud/internal/api/middleware"
	"github.com/UjjwalVandur/TestBud/internal/service"
)

const maxSchemaUploadBytes = 5 << 20

type SchemaUploader interface {
	UploadSchema(ctx context.Context, input service.UploadSchemaInput) (service.UploadSchemaResult, error)
}

type SchemaHandler struct {
	service SchemaUploader
}

func NewSchemaHandler(service SchemaUploader) *SchemaHandler {
	return &SchemaHandler{service: service}
}

func (h *SchemaHandler) Upload(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "schema service is not configured"})
		return
	}

	// Derive the uploader identity from the authenticated user in context,
	// not from user-supplied form data (DEV-5 security fix).
	uploadedBy := middleware.AuthenticatedUserID(c.Request.Context())
	if uploadedBy == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	projectID, err := uuid.Parse(c.PostForm("project_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project_id must be a valid UUID"})
		return
	}
	version := c.PostForm("version")
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	if file.Size > maxSchemaUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "schema file exceeds 5MB limit"})
		return
	}

	opened, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "open uploaded file"})
		return
	}
	defer opened.Close()

	raw, err := io.ReadAll(io.LimitReader(opened, maxSchemaUploadBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read uploaded file"})
		return
	}
	if len(raw) > maxSchemaUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "schema file exceeds 5MB limit"})
		return
	}

	result, err := h.service.UploadSchema(c.Request.Context(), service.UploadSchemaInput{
		ProjectID:  projectID,
		Version:    version,
		UploadedBy: uploadedBy,
		RawBytes:   raw,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}
