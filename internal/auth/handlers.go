package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers contains the auth HTTP handlers
type Handlers struct {
	service *Service
}

// NewHandlers creates a new Handlers instance
func NewHandlers(service *Service) *Handlers {
	return &Handlers{service: service}
}

// Register handles user registration
// POST /api/auth/register
func (h *Handlers) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	user, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if authErr, ok := err.(AuthError); ok {
			status := http.StatusBadRequest
			if authErr.Code == ErrEmailExists.Code {
				status = http.StatusConflict
			}
			c.JSON(status, gin.H{
				"error":   authErr.Code,
				"message": authErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to register user",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "registration successful",
		"user": UserResponse{
			ID:               user.ID,
			Email:            user.Email,
			Name:             user.Name,
			EmailVerified:    user.EmailVerified,
			SubscriptionTier: string(user.SubscriptionTier),
			ProfitSharePct:   user.ProfitSharePct,
			APIKeyMode:       string(user.APIKeyMode),
			IsAdmin:          user.IsAdmin,
			CreatedAt:        user.CreatedAt,
		},
	})
}

// Login handles user login
// POST /api/auth/login
func (h *Handlers) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	// Get device info
	deviceInfo := c.GetHeader("X-Device-Info")
	ipAddress := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	response, err := h.service.Login(c.Request.Context(), req, deviceInfo, ipAddress, userAgent)
	if err != nil {
		if authErr, ok := err.(AuthError); ok {
			status := http.StatusUnauthorized
			if authErr.Code == ErrAccountSuspended.Code {
				status = http.StatusForbidden
			}
			c.JSON(status, gin.H{
				"error":   authErr.Code,
				"message": authErr.Message,
			})
			return
		}
		// For debugging: include actual error message
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to login",
			"debug":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Refresh handles token refresh
// POST /api/auth/refresh
func (h *Handlers) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	response, err := h.service.RefreshTokens(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if authErr, ok := err.(AuthError); ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   authErr.Code,
				"message": authErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to refresh tokens",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Logout handles user logout
// POST /api/auth/logout
func (h *Handlers) Logout(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Even without refresh token, consider it a successful logout
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
		return
	}

	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		// Log but don't fail - user experience is more important
		// The token will expire anyway
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// LogoutAll handles logging out all sessions
// POST /api/auth/logout-all
func (h *Handlers) LogoutAll(c *gin.Context) {
	userID := GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   ErrUnauthorized.Code,
			"message": ErrUnauthorized.Message,
		})
		return
	}

	if err := h.service.LogoutAll(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to logout all sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "all sessions logged out"})
}

// ChangePassword handles password change
// POST /api/auth/change-password
func (h *Handlers) ChangePassword(c *gin.Context) {
	userID := GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   ErrUnauthorized.Code,
			"message": ErrUnauthorized.Message,
		})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), userID, req); err != nil {
		if authErr, ok := err.(AuthError); ok {
			status := http.StatusBadRequest
			if authErr.Code == ErrInvalidCredentials.Code {
				status = http.StatusUnauthorized
			}
			c.JSON(status, gin.H{
				"error":   authErr.Code,
				"message": authErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to change password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

// ForgotPassword handles password reset request
// POST /api/auth/forgot-password
func (h *Handlers) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	token, err := h.service.GeneratePasswordResetToken(c.Request.Context(), req.Email)
	if err != nil {
		// Log but don't expose
	}

	// Always return success to prevent email enumeration
	// In production, send email with reset link containing token
	_ = token // TODO: Send email with token

	c.JSON(http.StatusOK, gin.H{
		"message": "if an account exists with this email, a password reset link has been sent",
	})
}

// ResetPassword handles password reset confirmation
// POST /api/auth/reset-password
func (h *Handlers) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": err.Error(),
		})
		return
	}

	if err := h.service.ResetPassword(c.Request.Context(), req); err != nil {
		if authErr, ok := err.(AuthError); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   authErr.Code,
				"message": authErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to reset password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}

// VerifyEmail handles email verification with 6-digit code
// POST /api/auth/verify-email
func (h *Handlers) VerifyEmail(c *gin.Context) {
	userID := GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   ErrUnauthorized.Code,
			"message": ErrUnauthorized.Message,
		})
		return
	}

	var req struct {
		Code string `json:"code" binding:"required,len=6"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "VALIDATION_ERROR",
			"message": "6-digit code is required",
		})
		return
	}

	if err := h.service.VerifyEmailWithCode(c.Request.Context(), userID, req.Code); err != nil {
		if authErr, ok := err.(AuthError); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   authErr.Code,
				"message": authErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to verify email",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "email verified successfully"})
}

// ResendVerification resends verification email with new 6-digit code
// POST /api/auth/resend-verification
func (h *Handlers) ResendVerification(c *gin.Context) {
	userID := GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   ErrUnauthorized.Code,
			"message": ErrUnauthorized.Message,
		})
		return
	}

	err := h.service.ResendVerificationCode(c.Request.Context(), userID)
	if err != nil {
		if authErr, ok := err.(AuthError); ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   authErr.Code,
				"message": authErr.Message,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "failed to resend verification code",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "verification code sent to your email"})
}

// GetMe returns the current user's profile
// GET /api/auth/me
func (h *Handlers) GetMe(c *gin.Context) {
	userID := GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   ErrUnauthorized.Code,
			"message": ErrUnauthorized.Message,
		})
		return
	}

	user, err := h.service.GetUserByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   ErrUserNotFound.Code,
			"message": ErrUserNotFound.Message,
		})
		return
	}

	c.JSON(http.StatusOK, UserResponse{
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
	})
}

// RegisterRoutes registers all auth routes
func (h *Handlers) RegisterRoutes(router *gin.RouterGroup, jwtManager *JWTManager) {
	// Public routes (no auth required)
	router.POST("/register", h.Register)
	router.POST("/login", h.Login)
	router.POST("/refresh", h.Refresh)
	router.POST("/logout", h.Logout)
	router.POST("/forgot-password", h.ForgotPassword)
	router.POST("/reset-password", h.ResetPassword)

	// Protected routes (auth required)
	protected := router.Group("")
	protected.Use(Middleware(jwtManager))
	{
		protected.GET("/me", h.GetMe)
		protected.POST("/logout-all", h.LogoutAll)
		protected.POST("/change-password", h.ChangePassword)
		protected.POST("/verify-email", h.VerifyEmail)
		protected.POST("/resend-verification", h.ResendVerification)
	}
}
