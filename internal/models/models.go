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
