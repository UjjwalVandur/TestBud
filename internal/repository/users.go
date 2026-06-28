package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/UjjwalVandur/TestBud/internal/models"
)

// UserRepository defines persistence operations for users.
type UserRepository interface {
	FindUserIDByAPIKey(ctx context.Context, apiKey string) (uuid.UUID, error)
}

// GormUserRepository implements UserRepository via GORM.
type GormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository creates a new GormUserRepository.
func NewGormUserRepository(db *gorm.DB) *GormUserRepository {
	return &GormUserRepository{db: db}
}

// FindUserIDByAPIKey returns the user ID for the given API key. Returns uuid.Nil
// if the key does not match any user.
func (r *GormUserRepository) FindUserIDByAPIKey(ctx context.Context, apiKey string) (uuid.UUID, error) {
	var user models.User
	err := r.db.WithContext(ctx).Select("id").Where("api_key = ?", apiKey).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("find user by api key: %w", err)
	}
	return user.ID, nil
}
