package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/reggieanim/jot/internal/modules/users/domain"
)

const (
	// UserIDKey is the gin context key for the authenticated user's ID.
	UserIDKey = "auth_user_id"
	// UserEmailKey is the gin context key for the authenticated user's email.
	UserEmailKey = "auth_user_email"
)

// Middleware returns a gin middleware that validates JWTs.
// Protected routes behind this middleware can read the user ID via auth.GetUserID(c).
func Middleware(issuer *JWTIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization token"})
			return
		}

		claims, err := issuer.Parse(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(UserIDKey, domain.UserID(claims.UserID))
		c.Set(UserEmailKey, claims.Email)
		c.Next()
	}
}

// OptionalMiddleware parses the JWT if present but does not reject unauthenticated requests.
func OptionalMiddleware(issuer *JWTIssuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.Next()
			return
		}
		claims, err := issuer.Parse(tokenStr)
		if err != nil {
			c.Next()
			return
		}
		c.Set(UserIDKey, domain.UserID(claims.UserID))
		c.Set(UserEmailKey, claims.Email)
		c.Next()
	}
}

// GetUserID reads the authenticated user's ID from the gin context.
func GetUserID(c *gin.Context) (domain.UserID, bool) {
	v, exists := c.Get(UserIDKey)
	if !exists {
		return "", false
	}
	uid, ok := v.(domain.UserID)
	return uid, ok
}

func extractToken(c *gin.Context) string {
	// 1. Authorization: Bearer <token>
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	// 2. Cookie fallback (used by the SvelteKit frontend with credentials: 'include')
	if cookie, err := c.Cookie("jot_token"); err == nil && cookie != "" {
		return cookie
	}
	return ""
}
