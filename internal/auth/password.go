package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultBcryptCost is the default bcrypt cost factor
	DefaultBcryptCost = 12

	// MinPasswordLength is the minimum password length
	MinPasswordLength = 8

	// MaxPasswordLength is the maximum password length (to prevent DoS)
	MaxPasswordLength = 128
)

// PasswordManager handles password hashing and validation
type PasswordManager struct {
	bcryptCost        int
	minPasswordLength int
}

// NewPasswordManager creates a new password manager
func NewPasswordManager(bcryptCost, minLength int) *PasswordManager {
	if bcryptCost < bcrypt.MinCost {
		bcryptCost = DefaultBcryptCost
	}
	if minLength < MinPasswordLength {
		minLength = MinPasswordLength
	}
	return &PasswordManager{
		bcryptCost:        bcryptCost,
		minPasswordLength: minLength,
	}
}

// HashPassword hashes a password using bcrypt
func (p *PasswordManager) HashPassword(password string) (string, error) {
	if len(password) > MaxPasswordLength {
		return "", fmt.Errorf("password too long")
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), p.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(bytes), nil
}

// VerifyPassword verifies a password against a hash
func (p *PasswordManager) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ValidatePasswordStrength checks if a password meets strength requirements
func (p *PasswordManager) ValidatePasswordStrength(password string) error {
	if len(password) < p.minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", p.minPasswordLength)
	}

	if len(password) > MaxPasswordLength {
		return fmt.Errorf("password must be at most %d characters", MaxPasswordLength)
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	// Require at least 3 of 4 character types for strong passwords
	strength := 0
	if hasUpper {
		strength++
	}
	if hasLower {
		strength++
	}
	if hasNumber {
		strength++
	}
	if hasSpecial {
		strength++
	}

	if strength < 3 {
		return fmt.Errorf("password must contain at least 3 of: uppercase, lowercase, numbers, special characters")
	}

	return nil
}

// HashRefreshToken creates a SHA-256 hash of a refresh token for storage
func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// CheckPasswordHistory checks if a password was used recently
// This would require storing password hashes history in the database
func (p *PasswordManager) CheckPasswordHistory(password string, previousHashes []string) bool {
	for _, hash := range previousHashes {
		if p.VerifyPassword(password, hash) {
			return true // Password was used before
		}
	}
	return false
}
