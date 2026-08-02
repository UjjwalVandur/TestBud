package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/UjjwalVandur/TestBud/internal/models"
)

type contextKey string

const userIDKey contextKey = "authenticated_user_id"

// UserLookup resolves an API key to a user ID. Returns uuid.Nil if not found.
type UserLookup interface {
	FindUserIDByAPIKey(ctx context.Context, apiKey string) (uuid.UUID, error)
	GetOrCreateUserByClerkID(ctx context.Context, clerkID, email string) (*models.User, error)
}

// AuthMiddleware returns middleware that authenticates requests via X-API-Key or a Clerk JWT.
func AuthMiddleware(lookup UserLookup, clerkSecret string) gin.HandlerFunc {
	if clerkSecret != "" {
		clerk.SetKey(clerkSecret)
	}
	
	return func(c *gin.Context) {
		// 1. Try X-API-Key first (for CLI/Programmatic access)
		apiKey := c.GetHeader("X-API-Key")
		if apiKey != "" {
			userID, err := lookup.FindUserIDByAPIKey(c.Request.Context(), apiKey)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "authentication lookup failed"})
				return
			}
			if userID != uuid.Nil {
				ctx := context.WithValue(c.Request.Context(), userIDKey, userID)
				c.Request = c.Request.WithContext(ctx)
				c.Next()
				return
			}
		}

		// 2. Try Clerk Bearer Token (for Web UI)
		authHeader := c.GetHeader("Authorization")
		if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok && token != "" && clerkSecret != "" {
			claims, err := jwt.Verify(c.Request.Context(), &jwt.VerifyParams{
				Token: token,
			})
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid clerk token"})
				return
			}
			
			// Try to get user email from clerk API (lazy sync)
			clerkUser, err := user.Get(c.Request.Context(), claims.Subject)
			email := ""
			if err == nil && len(clerkUser.EmailAddresses) > 0 {
				email = clerkUser.EmailAddresses[0].EmailAddress
			} else {
				email = claims.Subject + "@clerk.testbud.local" // fallback
			}

			// Get or create user
			dbUser, err := lookup.GetOrCreateUserByClerkID(c.Request.Context(), claims.Subject, email)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to provision user"})
				return
			}

			ctx := context.WithValue(c.Request.Context(), userIDKey, dbUser.ID)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authentication credentials"})
	}
}

// AuthenticatedUserID extracts the authenticated user ID from the request context.
// Returns uuid.Nil if the context does not contain a user ID (e.g., unauthenticated route).
func AuthenticatedUserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(userIDKey).(uuid.UUID)
	return id
}
