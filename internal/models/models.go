package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	APIKey       string    `gorm:"uniqueIndex;not null"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

type Schema struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProjectID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Version        string    `gorm:"not null"`
	RawBytes       []byte    `gorm:"type:bytea;not null"`
	SchemaHash     string    `gorm:"not null;index"`
	OpenAPIVersion string    `gorm:"not null"`
	UploadedAt     time.Time `gorm:"not null"`
	UploadedBy     uuid.UUID `gorm:"type:uuid;not null"`
	UploadedByUser User      `gorm:"foreignKey:UploadedBy;references:ID;constraint:OnDelete:RESTRICT;"`

	Endpoints []Endpoint `gorm:"constraint:OnDelete:CASCADE;"`
}

func (s *Schema) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.UploadedAt.IsZero() {
		s.UploadedAt = time.Now().UTC()
	}
	return nil
}

type Endpoint struct {
	ID                 uuid.UUID      `gorm:"type:uuid;primaryKey"`
	SchemaID           uuid.UUID      `gorm:"type:uuid;not null;index"`
	Method             string         `gorm:"not null"`
	Path               string         `gorm:"not null"`
	EndpointHash       string         `gorm:"not null;index"`
	AuthRequired       bool           `gorm:"not null"`
	ParametersJSON     datatypes.JSON `gorm:"type:jsonb;not null"`
	RequestSchemaJSON  datatypes.JSON `gorm:"type:jsonb;not null"`
	ResponseSchemaJSON datatypes.JSON `gorm:"type:jsonb;not null"`
}

func (e *Endpoint) BeforeCreate(_ *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

// TestCaseCategory is the enum type for test case categories.
type TestCaseCategory string

const (
	CategoryPositive TestCaseCategory = "positive"
	CategoryNegative TestCaseCategory = "negative"
	CategoryBoundary TestCaseCategory = "boundary"
	CategorySecurity TestCaseCategory = "security"
)

type TestCase struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey"`
	EndpointID     uuid.UUID        `gorm:"type:uuid;not null;index:idx_test_cases_endpoint_category"`
	Category       TestCaseCategory `gorm:"type:varchar(20);not null;index:idx_test_cases_endpoint_category"`
	PayloadJSON    datatypes.JSON   `gorm:"type:jsonb;not null"`
	ExpectedStatus int              `gorm:"not null"`
	GeneratedAt    time.Time        `gorm:"not null"`

	Endpoint Endpoint `gorm:"foreignKey:EndpointID;references:ID;constraint:OnDelete:CASCADE;"`
}

func (tc *TestCase) BeforeCreate(_ *gorm.DB) error {
	if tc.ID == uuid.Nil {
		tc.ID = uuid.New()
	}
	if tc.GeneratedAt.IsZero() {
		tc.GeneratedAt = time.Now().UTC()
	}
	return nil
}

type Execution struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	TestCaseID uuid.UUID `gorm:"type:uuid;not null;index"`
	ActualStatus int     `gorm:"not null"`
	ResponseMs   int64   `gorm:"not null"`
	Passed       bool    `gorm:"not null"`
	RanAt        time.Time `gorm:"not null;index"`

	TestCase TestCase `gorm:"foreignKey:TestCaseID;references:ID;constraint:OnDelete:CASCADE;"`
}

func (ex *Execution) BeforeCreate(_ *gorm.DB) error {
	if ex.ID == uuid.Nil {
		ex.ID = uuid.New()
	}
	if ex.RanAt.IsZero() {
		ex.RanAt = time.Now().UTC()
	}
	return nil
}

type CoverageReport struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey"`
	SchemaID     uuid.UUID      `gorm:"type:uuid;not null;index"`
	EndpointPct  float64        `gorm:"type:numeric(5,2);not null"`
	CategoryJSON datatypes.JSON `gorm:"type:jsonb;not null"`
	GeneratedAt  time.Time      `gorm:"not null"`

	Schema Schema `gorm:"foreignKey:SchemaID;references:ID;constraint:OnDelete:CASCADE;"`
}

func (cr *CoverageReport) BeforeCreate(_ *gorm.DB) error {
	if cr.ID == uuid.Nil {
		cr.ID = uuid.New()
	}
	if cr.GeneratedAt.IsZero() {
		cr.GeneratedAt = time.Now().UTC()
	}
	return nil
}
