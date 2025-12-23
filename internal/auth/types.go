package auth

import (
	"time"
)

// UserClaims represents the JWT claims for a user
type UserClaims struct {
	UserID           string `json:"user_id"`
	Email            string `json:"email"`
	SubscriptionTier string `json:"tier"`
	APIKeyMode       string `json:"api_key_mode"`
	IsAdmin          bool   `json:"is_admin"`
}

// TokenPair represents an access and refresh token pair
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`    // Access token expiry in seconds
	TokenType    string `json:"token_type"`    // Always "Bearer"
}

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Email       string  `json:"email" binding:"required,email"`
	Password    string  `json:"password" binding:"required,min=8"`
	Name        string  `json:"name" binding:"required,min=2"`
	ReferralCode *string `json:"referral_code,omitempty"`
}

// LoginRequest represents a user login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	User        UserResponse `json:"user"`
	AccessToken string       `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn   int64        `json:"expires_in"`
}

// UserResponse represents user data returned to the client
type UserResponse struct {
	ID               string     `json:"id"`
	Email            string     `json:"email"`
	Name             string     `json:"name"`
	EmailVerified    bool       `json:"email_verified"`
	SubscriptionTier string     `json:"subscription_tier"`
	ProfitSharePct   float64    `json:"profit_share_pct"`
	APIKeyMode       string     `json:"api_key_mode"`
	IsAdmin          bool       `json:"is_admin"`
	CreatedAt        time.Time  `json:"created_at"`
	LastLoginAt      *time.Time `json:"last_login_at,omitempty"`
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshResponse represents a token refresh response
type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// ForgotPasswordRequest represents a password reset request
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest represents a password reset confirmation
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// VerifyEmailRequest represents an email verification request
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// Config holds authentication configuration
type Config struct {
	// JWT settings
	JWTSecret            string        `json:"jwt_secret"`
	AccessTokenDuration  time.Duration `json:"access_token_duration"`
	RefreshTokenDuration time.Duration `json:"refresh_token_duration"`

	// Password settings
	MinPasswordLength int `json:"min_password_length"`

	// Session settings
	MaxSessionsPerUser int `json:"max_sessions_per_user"`

	// Email verification
	RequireEmailVerification bool   `json:"require_email_verification"`
	EmailVerificationURL     string `json:"email_verification_url"`

	// Password reset
	PasswordResetURL      string        `json:"password_reset_url"`
	PasswordResetDuration time.Duration `json:"password_reset_duration"`
}

// DefaultConfig returns default authentication configuration
func DefaultConfig() Config {
	return Config{
		JWTSecret:                "", // Must be set
		AccessTokenDuration:      15 * time.Minute,
		RefreshTokenDuration:     7 * 24 * time.Hour,
		MinPasswordLength:        8,
		MaxSessionsPerUser:       10,
		RequireEmailVerification: false, // Start without email verification
		PasswordResetDuration:    1 * time.Hour,
	}
}

// Error types for authentication
type AuthError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e AuthError) Error() string {
	return e.Message
}

// Common authentication errors
var (
	ErrInvalidCredentials = AuthError{Code: "INVALID_CREDENTIALS", Message: "invalid email or password"}
	ErrUserNotFound       = AuthError{Code: "USER_NOT_FOUND", Message: "user not found"}
	ErrEmailExists        = AuthError{Code: "EMAIL_EXISTS", Message: "email already registered"}
	ErrInvalidToken       = AuthError{Code: "INVALID_TOKEN", Message: "invalid or expired token"}
	ErrTokenExpired       = AuthError{Code: "TOKEN_EXPIRED", Message: "token has expired"}
	ErrSessionRevoked     = AuthError{Code: "SESSION_REVOKED", Message: "session has been revoked"}
	ErrUnauthorized       = AuthError{Code: "UNAUTHORIZED", Message: "unauthorized access"}
	ErrForbidden          = AuthError{Code: "FORBIDDEN", Message: "access forbidden"}
	ErrAccountSuspended   = AuthError{Code: "ACCOUNT_SUSPENDED", Message: "account has been suspended"}
	ErrEmailNotVerified   = AuthError{Code: "EMAIL_NOT_VERIFIED", Message: "email not verified"}
	ErrWeakPassword       = AuthError{Code: "WEAK_PASSWORD", Message: "password does not meet requirements"}
	ErrRateLimited        = AuthError{Code: "RATE_LIMITED", Message: "too many requests, please try again later"}
)
