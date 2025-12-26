package auth

import (
	"context"
	"fmt"
	"log"
	"time"

	"binance-trading-bot/internal/database"

	"golang.org/x/crypto/bcrypt"
)

const (
	// AdminEmail is the default admin email
	AdminEmail = "admin@binance-bot.local"
	// AdminPassword is the default admin password
	AdminPassword = "Weber@#2025"
	// AdminBcryptCost is the bcrypt cost for admin password
	AdminBcryptCost = 12
)

// SeedAdminUser ensures an admin user exists with proper credentials.
// It creates the admin if missing, or updates the password if it's a placeholder.
func SeedAdminUser(ctx context.Context, db *database.DB) error {
	repo := database.NewRepository(db)

	// Check if admin user exists
	user, err := repo.GetUserByEmail(ctx, AdminEmail)
	if err != nil {
		return fmt.Errorf("failed to check for admin user: %w", err)
	}

	// Hash the admin password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(AdminPassword), AdminBcryptCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	if user == nil {
		// Create admin user
		log.Printf("Admin user not found. Creating admin user: %s", AdminEmail)

		now := time.Now()
		adminUser := &database.User{
			Email:              AdminEmail,
			PasswordHash:       string(hashedPassword),
			Name:               "Administrator",
			EmailVerified:      true,
			EmailVerifiedAt:    &now,
			SubscriptionTier:   database.TierWhale,
			SubscriptionStatus: database.StatusActive,
			APIKeyMode:         database.APIKeyModeUserProvided,
			ProfitSharePct:     0.0, // Admin has no profit share
			IsAdmin:            true,
		}

		if err := repo.CreateUser(ctx, adminUser); err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}

		log.Printf("Admin user created successfully with ID: %s", adminUser.ID)
		return nil
	}

	// User exists - check if password needs updating
	// If password is a placeholder or doesn't verify, update it
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(AdminPassword))
	if err != nil {
		// Password doesn't match - update it
		log.Printf("Admin user exists but password needs updating. Updating password for: %s", AdminEmail)

		if err := repo.UpdateUserPassword(ctx, user.ID, string(hashedPassword)); err != nil {
			return fmt.Errorf("failed to update admin password: %w", err)
		}

		log.Printf("Admin password updated successfully")
	} else {
		log.Printf("Admin user exists with correct password: %s", AdminEmail)
	}

	// Ensure admin flags are set correctly
	if !user.IsAdmin || user.SubscriptionTier != database.TierWhale || !user.EmailVerified {
		log.Printf("Updating admin user flags")

		now := time.Now()
		user.IsAdmin = true
		user.SubscriptionTier = database.TierWhale
		user.SubscriptionStatus = database.StatusActive
		user.EmailVerified = true
		if user.EmailVerifiedAt == nil {
			user.EmailVerifiedAt = &now
		}
		user.ProfitSharePct = 0.0

		if err := repo.UpdateUser(ctx, user); err != nil {
			return fmt.Errorf("failed to update admin user flags: %w", err)
		}

		log.Printf("Admin user flags updated successfully")
	}

	return nil
}
