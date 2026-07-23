package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/UjjwalVandur/TestBud/internal/models"
	"github.com/UjjwalVandur/TestBud/internal/repository"
)

// --- Fake SchemaRepository for regression tests ---

type fakeRegSchemaRepo struct {
	repository.SchemaRepository
	findByIDResult    *models.Schema
	findByIDErr       error
	predecessorResult *models.Schema
	predecessorErr    error
}

func (f *fakeRegSchemaRepo) FindByID(_ context.Context, _ uuid.UUID) (*models.Schema, error) {
	return f.findByIDResult, f.findByIDErr
}

func (f *fakeRegSchemaRepo) FindPredecessorSchema(_ context.Context, _ uuid.UUID, _ time.Time) (*models.Schema, error) {
	return f.predecessorResult, f.predecessorErr
}

// --- Tests ---

func TestGetRegressionReport_SchemaNotFound(t *testing.T) {
	svc := NewRegressionService(&fakeRegSchemaRepo{
		findByIDResult: nil,
	}, logrus.New())

	_, err := svc.GetRegressionReport(context.Background(), uuid.New())
	if !errors.Is(err, ErrSchemaNotFound) {
		t.Fatalf("expected ErrSchemaNotFound, got %v", err)
	}
}

func TestGetRegressionReport_NoPredecessor_AllAdded(t *testing.T) {
	targetID := uuid.New()
	projectID := uuid.New()

	svc := NewRegressionService(&fakeRegSchemaRepo{
		findByIDResult: &models.Schema{
			ID:         targetID,
			ProjectID:  projectID,
			UploadedAt: time.Now(),
			Endpoints: []models.Endpoint{
				{ID: uuid.New(), Method: "GET", Path: "/pets", ParametersJSON: datatypes.JSON(`[]`), RequestSchemaJSON: datatypes.JSON(`{}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
				{ID: uuid.New(), Method: "POST", Path: "/pets", ParametersJSON: datatypes.JSON(`[]`), RequestSchemaJSON: datatypes.JSON(`{}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
			},
		},
		predecessorResult: nil,
	}, logrus.New())

	report, err := svc.GetRegressionReport(context.Background(), targetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Added) != 2 {
		t.Errorf("Added = %d, want 2", len(report.Added))
	}
	if len(report.Removed) != 0 {
		t.Errorf("Removed = %d, want 0", len(report.Removed))
	}
	if len(report.Modified) != 0 {
		t.Errorf("Modified = %d, want 0", len(report.Modified))
	}
	if report.TargetSchemaID != targetID {
		t.Errorf("TargetSchemaID = %v, want %v", report.TargetSchemaID, targetID)
	}
	if report.BaseSchemaID != uuid.Nil {
		t.Errorf("BaseSchemaID = %v, want Nil", report.BaseSchemaID)
	}
}

func TestGetRegressionReport_FullDiff(t *testing.T) {
	baseID := uuid.New()
	targetID := uuid.New()
	projectID := uuid.New()
	now := time.Now().UTC()

	svc := NewRegressionService(&fakeRegSchemaRepo{
		findByIDResult: &models.Schema{
			ID:         targetID,
			ProjectID:  projectID,
			UploadedAt: now,
			Endpoints: []models.Endpoint{
				// GET /pets — identical to base
				{ID: uuid.New(), Method: "GET", Path: "/pets", AuthRequired: false, ParametersJSON: datatypes.JSON(`[]`), RequestSchemaJSON: datatypes.JSON(`{}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
				// POST /pets — new (added)
				{ID: uuid.New(), Method: "POST", Path: "/pets", AuthRequired: true, ParametersJSON: datatypes.JSON(`[]`), RequestSchemaJSON: datatypes.JSON(`{"type":"object"}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
				// PUT /pets/{id} — auth changed from false to true (modified)
				{ID: uuid.New(), Method: "PUT", Path: "/pets/{id}", AuthRequired: true, ParametersJSON: datatypes.JSON(`[{"name":"id"}]`), RequestSchemaJSON: datatypes.JSON(`{}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
			},
		},
		predecessorResult: &models.Schema{
			ID:         baseID,
			ProjectID:  projectID,
			UploadedAt: now.Add(-24 * time.Hour),
			Endpoints: []models.Endpoint{
				// GET /pets — identical
				{ID: uuid.New(), Method: "GET", Path: "/pets", AuthRequired: false, ParametersJSON: datatypes.JSON(`[]`), RequestSchemaJSON: datatypes.JSON(`{}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
				// DELETE /pets/{id} — will be removed
				{ID: uuid.New(), Method: "DELETE", Path: "/pets/{id}", AuthRequired: true, ParametersJSON: datatypes.JSON(`[{"name":"id"}]`), RequestSchemaJSON: datatypes.JSON(`{}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
				// PUT /pets/{id} — auth was false, now true
				{ID: uuid.New(), Method: "PUT", Path: "/pets/{id}", AuthRequired: false, ParametersJSON: datatypes.JSON(`[{"name":"id"}]`), RequestSchemaJSON: datatypes.JSON(`{}`), ResponseSchemaJSON: datatypes.JSON(`{}`)},
			},
		},
	}, logrus.New())

	report, err := svc.GetRegressionReport(context.Background(), targetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Added) != 1 {
		t.Errorf("Added = %d, want 1", len(report.Added))
	} else if report.Added[0].Method != "POST" || report.Added[0].Path != "/pets" {
		t.Errorf("Added[0] = %s %s, want POST /pets", report.Added[0].Method, report.Added[0].Path)
	}

	if len(report.Removed) != 1 {
		t.Errorf("Removed = %d, want 1", len(report.Removed))
	} else if report.Removed[0].Method != "DELETE" || report.Removed[0].Path != "/pets/{id}" {
		t.Errorf("Removed[0] = %s %s, want DELETE /pets/{id}", report.Removed[0].Method, report.Removed[0].Path)
	}

	if len(report.Modified) != 1 {
		t.Errorf("Modified = %d, want 1", len(report.Modified))
	} else {
		mod := report.Modified[0]
		if mod.Method != "PUT" || mod.Path != "/pets/{id}" {
			t.Errorf("Modified[0] = %s %s, want PUT /pets/{id}", mod.Method, mod.Path)
		}
		if !mod.AuthChanged {
			t.Error("expected AuthChanged=true on modified endpoint")
		}
		if mod.ParametersChanged || mod.RequestSchemaChanged || mod.ResponseSchemaChanged {
			t.Error("only AuthChanged should be true")
		}
	}

	if report.BaseSchemaID != baseID {
		t.Errorf("BaseSchemaID = %v, want %v", report.BaseSchemaID, baseID)
	}
	if report.TargetSchemaID != targetID {
		t.Errorf("TargetSchemaID = %v, want %v", report.TargetSchemaID, targetID)
	}
}

func TestGetRegressionReport_FindByIDError(t *testing.T) {
	svc := NewRegressionService(&fakeRegSchemaRepo{
		findByIDErr: errors.New("db error"),
	}, logrus.New())

	_, err := svc.GetRegressionReport(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
