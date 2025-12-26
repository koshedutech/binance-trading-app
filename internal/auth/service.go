package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	mathrand "math/rand"
	"time"

	"binance-trading-bot/internal/database"
)

// EmailService interface for sending emails
type EmailService interface {
	IsSMTPConfigured(ctx context.Context) bool
	SendVerificationEmail(ctx context.Context, to, code string) error
}

// Service handles authentication operations
type Service struct {
	repo            *database.Repository
	jwtManager      *JWTManager
	passwordManager *PasswordManager
	emailService    EmailService
	config          Config
}

// NewService creates a new authentication service
func NewService(repo *database.Repository, config Config) *Service {
	return NewServiceWithEmail(repo, config, nil)
}

// NewServiceWithEmail creates a new authentication service with email support
func NewServiceWithEmail(repo *database.Repository, config Config, emailService EmailService) *Service {
	if config.JWTSecret == "" {
		log.Fatal("JWT secret is required")
	}

	if config.AccessTokenDuration == 0 {
		config.AccessTokenDuration = 15 * time.Minute
	}
	if config.RefreshTokenDuration == 0 {
		config.RefreshTokenDuration = 7 * 24 * time.Hour
	}

	return &Service{
		repo:            repo,
		jwtManager:      NewJWTManager(config.JWTSecret, config.AccessTokenDuration, config.RefreshTokenDuration),
		passwordManager: NewPasswordManager(DefaultBcryptCost, config.MinPasswordLength),
		emailService:    emailService,
		config:          config,
	}
}

// GetJWTManager returns the JWT manager for use in middleware
func (s *Service) GetJWTManager() *JWTManager {
	return s.jwtManager
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*database.User, error) {
	// Only check SMTP if email verification is required
	if s.config.RequireEmailVerification && s.emailService != nil && !s.emailService.IsSMTPConfigured(ctx) {
		return nil, AuthError{
			Code:    "SMTP_NOT_CONFIGURED",
			Message: "Email service is not configured. Please contact administrator.",
		}
	}

	// Check if email exists
	exists, err := s.repo.EmailExists(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if exists {
		return nil, ErrEmailExists
	}

	// Validate password strength
	if err := s.passwordManager.ValidatePasswordStrength(req.Password); err != nil {
		return nil, AuthError{Code: "WEAK_PASSWORD", Message: err.Error()}
	}

	// Hash password
	passwordHash, err := s.passwordManager.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate referral code
	referralCode, err := generateReferralCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate referral code: %w", err)
	}

	// Check referral
	var referredBy *string
	if req.ReferralCode != nil && *req.ReferralCode != "" {
		referrer, err := s.repo.GetUserByReferralCode(ctx, *req.ReferralCode)
		if err != nil {
			return nil, fmt.Errorf("failed to check referral: %w", err)
		}
		if referrer != nil {
			referredBy = &referrer.ID
		}
	}

	// Determine if email verification is required
	requiresVerification := s.emailService != nil && s.config.RequireEmailVerification

	// Create user - All users get Whale tier (no restrictions, subscriptions bypassed)
	user := &database.User{
		Email:              req.Email,
		PasswordHash:       passwordHash,
		Name:               req.Name,
		SubscriptionTier:   database.TierWhale, // All users are Whale tier
		SubscriptionStatus: database.StatusActive,
		APIKeyMode:         database.APIKeyModeUserProvided,
		ProfitSharePct:     5.0, // Whale tier profit share
		ReferralCode:       referralCode,
		ReferredBy:         referredBy,
		EmailVerified:      !requiresVerification,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create default trading config - Whale tier (unlimited access)
	tradingConfig := &database.UserTradingConfig{
		UserID:                   user.ID,
		MaxOpenPositions:         -1, // Unlimited for Whale tier
		MaxRiskPerTrade:          5.0,
		DefaultStopLossPercent:   2.0,
		DefaultTakeProfitPercent: 5.0,
		EnableSpot:               true,
		EnableFutures:            true, // Whale tier gets futures
		FuturesDefaultLeverage:   10,
		FuturesMarginType:        "CROSSED",
		AutopilotEnabled:         true, // Whale tier gets autopilot
		AutopilotRiskLevel:       "moderate",
		AutopilotMinConfidence:   0.65,
		NotificationEmail:        true,
		NotificationPush:         true,
	}

	if err := s.repo.UpsertUserTradingConfig(ctx, tradingConfig); err != nil {
		log.Printf("Warning: failed to create trading config for user %s: %v", user.ID, err)
	}

	// Send verification email if required
	if requiresVerification {
		code, err := s.GenerateVerificationCode(ctx, user.ID)
		if err != nil {
			log.Printf("Warning: failed to generate verification code: %v", err)
		} else {
			if err := s.emailService.SendVerificationEmail(ctx, user.Email, code); err != nil {
				log.Printf("Warning: failed to send verification email: %v", err)
			}
		}
	}

	return user, nil
}

// Login authenticates a user and returns tokens
func (s *Service) Login(ctx context.Context, req LoginRequest, deviceInfo, ipAddress, userAgent string) (*LoginResponse, error) {
	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		log.Printf("Login: GetUserByEmail failed: %v", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		log.Printf("Login: User not found for email: %s", req.Email)
		return nil, ErrInvalidCredentials
	}
	log.Printf("Login: Found user %s for email %s", user.ID, req.Email)

	// Verify password
	if !s.passwordManager.VerifyPassword(req.Password, user.PasswordHash) {
		log.Printf("Login: Password verification failed for user %s", user.ID)
		return nil, ErrInvalidCredentials
	}
	log.Printf("Login: Password verified for user %s", user.ID)

	// Check if account is suspended
	if user.SubscriptionStatus == database.StatusSuspended {
		return nil, ErrAccountSuspended
	}

	// Check email verification if required
	if s.config.RequireEmailVerification && !user.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	// Generate tokens
	claims := UserClaims{
		UserID:           user.ID,
		Email:            user.Email,
		SubscriptionTier: string(user.SubscriptionTier),
		APIKeyMode:       string(user.APIKeyMode),
		IsAdmin:          user.IsAdmin,
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(claims)
	if err != nil {
		log.Printf("Login: Token generation failed: %v", err)
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}
	log.Printf("Login: Tokens generated for user %s", user.ID)

	// Create session
	session := &database.UserSession{
		UserID:           user.ID,
		RefreshTokenHash: HashRefreshToken(tokenPair.RefreshToken),
		DeviceInfo:       deviceInfo,
		IPAddress:        ipAddress,
		UserAgent:        userAgent,
		ExpiresAt:        time.Now().Add(s.jwtManager.GetRefreshTokenDuration()),
	}

	log.Printf("Login: Creating session for user %s with IP: %s", user.ID, ipAddress)
	if err := s.repo.CreateSession(ctx, session); err != nil {
		// Log the error but don't fail login - session creation is optional
		log.Printf("Login: CreateSession failed (continuing without session): %v", err)
	} else {
		log.Printf("Login: Session created for user %s", user.ID)
	}

	// Update last login
	if err := s.repo.UpdateUserLastLogin(ctx, user.ID); err != nil {
		log.Printf("Warning: failed to update last login for user %s: %v", user.ID, err)
	}

	return &LoginResponse{
		User: UserResponse{
			ID:               user.ID,
			Email:            user.Email,
			Name:             user.Name,
			EmailVerified:    user.EmailVerified,
			SubscriptionTier: string(user.SubscriptionTier),
			ProfitSharePct:   user.ProfitSharePct,
			APIKeyMode:       string(user.APIKeyMode),
			IsAdmin:          user.IsAdmin,
			CreatedAt:        user.CreatedAt,
			LastLoginAt:      user.LastLoginAt,
		},
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// RefreshTokens refreshes the access and refresh tokens
func (s *Service) RefreshTokens(ctx context.Context, refreshToken string) (*RefreshResponse, error) {
	// Hash the refresh token to look it up
	tokenHash := HashRefreshToken(refreshToken)

	// Find the session
	session, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrInvalidToken
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrTokenExpired
	}

	// Check if session is revoked
	if session.RevokedAt != nil {
		return nil, ErrSessionRevoked
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	// Check if account is suspended
	if user.SubscriptionStatus == database.StatusSuspended {
		return nil, ErrAccountSuspended
	}

	// Generate new tokens
	claims := UserClaims{
		UserID:           user.ID,
		Email:            user.Email,
		SubscriptionTier: string(user.SubscriptionTier),
		APIKeyMode:       string(user.APIKeyMode),
		IsAdmin:          user.IsAdmin,
	}

	tokenPair, err := s.jwtManager.GenerateTokenPair(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Revoke old session and create new one (refresh token rotation)
	if err := s.repo.RevokeSession(ctx, session.ID); err != nil {
		log.Printf("Warning: failed to revoke old session: %v", err)
	}

	newSession := &database.UserSession{
		UserID:           user.ID,
		RefreshTokenHash: HashRefreshToken(tokenPair.RefreshToken),
		DeviceInfo:       session.DeviceInfo,
		IPAddress:        session.IPAddress,
		UserAgent:        session.UserAgent,
		ExpiresAt:        time.Now().Add(s.jwtManager.GetRefreshTokenDuration()),
	}

	if err := s.repo.CreateSession(ctx, newSession); err != nil {
		return nil, fmt.Errorf("failed to create new session: %w", err)
	}

	return &RefreshResponse{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// Logout revokes a user's session
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := HashRefreshToken(refreshToken)

	session, err := s.repo.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil // Already logged out or invalid token
	}

	return s.repo.RevokeSession(ctx, session.ID)
}

// LogoutAll revokes all sessions for a user
func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.repo.RevokeAllUserSessions(ctx, userID)
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, userID string, req ChangePasswordRequest) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return ErrUserNotFound
	}

	// Verify current password
	if !s.passwordManager.VerifyPassword(req.CurrentPassword, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	// Validate new password strength
	if err := s.passwordManager.ValidatePasswordStrength(req.NewPassword); err != nil {
		return AuthError{Code: "WEAK_PASSWORD", Message: err.Error()}
	}

	// Hash new password
	newHash, err := s.passwordManager.HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := s.repo.UpdateUserPassword(ctx, userID, newHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke all sessions to force re-login
	if err := s.repo.RevokeAllUserSessions(ctx, userID); err != nil {
		log.Printf("Warning: failed to revoke sessions after password change: %v", err)
	}

	return nil
}

// GeneratePasswordResetToken generates a password reset token
func (s *Service) GeneratePasswordResetToken(ctx context.Context, email string) (string, error) {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		// Don't reveal whether email exists
		return "", nil
	}

	token, err := s.jwtManager.GenerateVerificationToken(user.ID, "password_reset", s.config.PasswordResetDuration)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

// ResetPassword resets a user's password using a reset token
func (s *Service) ResetPassword(ctx context.Context, req ResetPasswordRequest) error {
	// Validate token
	userID, err := s.jwtManager.ValidateVerificationToken(req.Token, "password_reset")
	if err != nil {
		return ErrInvalidToken
	}

	// Validate new password
	if err := s.passwordManager.ValidatePasswordStrength(req.NewPassword); err != nil {
		return AuthError{Code: "WEAK_PASSWORD", Message: err.Error()}
	}

	// Hash new password
	newHash, err := s.passwordManager.HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	if err := s.repo.UpdateUserPassword(ctx, userID, newHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Revoke all sessions
	if err := s.repo.RevokeAllUserSessions(ctx, userID); err != nil {
		log.Printf("Warning: failed to revoke sessions after password reset: %v", err)
	}

	return nil
}

// GenerateEmailVerificationToken generates an email verification token
func (s *Service) GenerateEmailVerificationToken(ctx context.Context, userID string) (string, error) {
	return s.jwtManager.GenerateVerificationToken(userID, "email_verification", 24*time.Hour)
}

// VerifyEmail verifies a user's email using a verification token
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	userID, err := s.jwtManager.ValidateVerificationToken(token, "email_verification")
	if err != nil {
		return ErrInvalidToken
	}

	return s.repo.SetEmailVerified(ctx, userID)
}

// GetUserByID retrieves a user by ID
func (s *Service) GetUserByID(ctx context.Context, userID string) (*database.User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// Helper function to generate a referral code
func generateReferralCode() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CleanupExpiredSessions removes expired sessions from the database
func (s *Service) CleanupExpiredSessions(ctx context.Context) error {
	return s.repo.DeleteExpiredSessions(ctx)
}

// GenerateVerificationCode generates a 6-digit verification code
func (s *Service) GenerateVerificationCode(ctx context.Context, userID string) (string, error) {
	// Generate 6-digit code
	mathrand.Seed(time.Now().UnixNano())
	code := fmt.Sprintf("%06d", mathrand.Intn(1000000))

	// Store code in database
	verificationCode := &database.EmailVerificationCode{
		UserID:    userID,
		Code:      code,
		ExpiresAt: time.Now().Add(15 * time.Minute), // 15 minute expiry
	}

	if err := s.repo.CreateEmailVerificationCode(ctx, verificationCode); err != nil {
		return "", fmt.Errorf("failed to store verification code: %w", err)
	}

	return code, nil
}

// VerifyEmailWithCode verifies an email using a 6-digit code
func (s *Service) VerifyEmailWithCode(ctx context.Context, userID, code string) error {
	verified, err := s.repo.VerifyEmailCode(ctx, userID, code)
	if err != nil {
		return fmt.Errorf("failed to verify code: %w", err)
	}

	if !verified {
		return AuthError{
			Code:    "INVALID_CODE",
			Message: "Invalid or expired verification code",
		}
	}

	return nil
}

// ResendVerificationCode generates and sends a new verification code
func (s *Service) ResendVerificationCode(ctx context.Context, userID string) error {
	if s.emailService == nil {
		return AuthError{
			Code:    "EMAIL_DISABLED",
			Message: "Email service is not configured",
		}
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return ErrUserNotFound
	}

	// Check if already verified
	if user.EmailVerified {
		return AuthError{
			Code:    "ALREADY_VERIFIED",
			Message: "Email is already verified",
		}
	}

	// Generate new code
	code, err := s.GenerateVerificationCode(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	// Send email
	if err := s.emailService.SendVerificationEmail(ctx, user.Email, code); err != nil {
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	return nil
}
