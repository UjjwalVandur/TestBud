package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contextKey string

const userIDKey contextKey = "authenticated_user_id"

// UserLookup resolves an API key to a user ID. Returns uuid.Nil if not found.
type UserLookup interface {
	FindUserIDByAPIKey(ctx context.Context, apiKey string) (uuid.UUID, error)
}

// APIKeyAuth returns middleware that authenticates requests via the X-API-Key header.
// Unauthenticated requests receive 401. If the key is not found, 401 is returned.
func APIKeyAuth(lookup UserLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			// Also accept "Bearer <key>" in Authorization header for flexibility.
			auth := c.GetHeader("Authorization")
			if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
				key = after
			}
		}

		if key == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}

		userID, err := lookup.FindUserIDByAPIKey(c.Request.Context(), key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authentication lookup failed"})
			return
		}
		if userID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		// Store user ID in context for downstream handlers.
		ctx := context.WithValue(c.Request.Context(), userIDKey, userID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// AuthenticatedUserID extracts the authenticated user ID from the request context.
// Returns uuid.Nil if the context does not contain a user ID (e.g., unauthenticated route).
func AuthenticatedUserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userIDKey).(uuid.UUID)
	return id
}
