package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// Context keys for user data
	ContextKeyUserID   = "user_id"
	ContextKeyEmail    = "user_email"
	ContextKeyTier     = "user_tier"
	ContextKeyAPIMode  = "user_api_mode"
	ContextKeyIsAdmin  = "user_is_admin"
	ContextKeyClaims   = "user_claims"
)

// Middleware creates a JWT authentication middleware
func Middleware(jwtManager *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Code,
				"message": "missing authorization header",
			})
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Code,
				"message": "invalid authorization header format",
			})
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := jwtManager.ValidateAccessToken(tokenString)
		if err != nil {
			status := http.StatusUnauthorized
			authErr, ok := err.(AuthError)
			if !ok {
				authErr = ErrInvalidToken
			}

			c.AbortWithStatusJSON(status, gin.H{
				"error": authErr.Code,
				"message": authErr.Message,
			})
			return
		}

		// Set user context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyEmail, claims.Email)
		c.Set(ContextKeyTier, claims.SubscriptionTier)
		c.Set(ContextKeyAPIMode, claims.APIKeyMode)
		c.Set(ContextKeyIsAdmin, claims.IsAdmin)
		c.Set(ContextKeyClaims, claims)

		c.Next()
	}
}

// OptionalMiddleware allows requests without auth but sets user context if token is present
func OptionalMiddleware(jwtManager *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		claims, err := jwtManager.ValidateAccessToken(parts[1])
		if err == nil && claims != nil {
			c.Set(ContextKeyUserID, claims.UserID)
			c.Set(ContextKeyEmail, claims.Email)
			c.Set(ContextKeyTier, claims.SubscriptionTier)
			c.Set(ContextKeyAPIMode, claims.APIKeyMode)
			c.Set(ContextKeyIsAdmin, claims.IsAdmin)
			c.Set(ContextKeyClaims, claims)
		}

		c.Next()
	}
}

// RequireAdmin middleware ensures the user is an admin
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get(ContextKeyIsAdmin)
		if !exists || !isAdmin.(bool) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": ErrForbidden.Code,
				"message": "admin access required",
			})
			return
		}
		c.Next()
	}
}

// RequireTier middleware ensures the user has at least the specified tier
func RequireTier(minTier string) gin.HandlerFunc {
	tierOrder := map[string]int{
		"free":   0,
		"trader": 1,
		"pro":    2,
		"whale":  3,
	}

	return func(c *gin.Context) {
		userTier, exists := c.Get(ContextKeyTier)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": ErrForbidden.Code,
				"message": "subscription required",
			})
			return
		}

		userLevel, ok := tierOrder[userTier.(string)]
		if !ok {
			userLevel = 0
		}

		minLevel, ok := tierOrder[minTier]
		if !ok {
			minLevel = 0
		}

		if userLevel < minLevel {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": ErrForbidden.Code,
				"message": "upgrade to " + minTier + " tier required",
			})
			return
		}

		c.Next()
	}
}

// RequireEmailVerified middleware ensures the user's email is verified
func RequireEmailVerified(service *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString(ContextKeyUserID)
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUnauthorized.Code,
				"message": "authentication required",
			})
			return
		}

		user, err := service.repo.GetUserByID(c.Request.Context(), userID)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": ErrUserNotFound.Code,
				"message": ErrUserNotFound.Message,
			})
			return
		}

		if !user.EmailVerified {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": ErrEmailNotVerified.Code,
				"message": ErrEmailNotVerified.Message,
			})
			return
		}

		c.Next()
	}
}

// GetUserID extracts the user ID from the Gin context
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get(ContextKeyUserID); exists {
		return userID.(string)
	}
	return ""
}

// GetUserClaims extracts the full user claims from the Gin context
func GetUserClaims(c *gin.Context) *UserClaims {
	if claims, exists := c.Get(ContextKeyClaims); exists {
		return claims.(*UserClaims)
	}
	return nil
}

// GetUserTier extracts the user tier from the Gin context
func GetUserTier(c *gin.Context) string {
	if tier, exists := c.Get(ContextKeyTier); exists {
		return tier.(string)
	}
	return "free"
}

// IsAdmin checks if the current user is an admin
func IsAdmin(c *gin.Context) bool {
	if isAdmin, exists := c.Get(ContextKeyIsAdmin); exists {
		return isAdmin.(bool)
	}
	return false
}

// GetAPIKeyMode extracts the API key mode from the Gin context
func GetAPIKeyMode(c *gin.Context) string {
	if mode, exists := c.Get(ContextKeyAPIMode); exists {
		return mode.(string)
	}
	return "user_provided"
}
