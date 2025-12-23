package database

import (
	"time"
)

// License represents a license key in the database
type License struct {
	ID            string    `json:"id" db:"id"`
	Key           string    `json:"key" db:"key"`
	Type          string    `json:"type" db:"type"` // personal, pro, enterprise
	CustomerEmail string    `json:"customer_email" db:"customer_email"`
	CustomerName  string    `json:"customer_name" db:"customer_name"`
	MaxSymbols    int       `json:"max_symbols" db:"max_symbols"`
	Features      string    `json:"features" db:"features"` // JSON array
	IsActive      bool      `json:"is_active" db:"is_active"`
	ActivatedAt   *time.Time `json:"activated_at" db:"activated_at"`
	ExpiresAt     *time.Time `json:"expires_at" db:"expires_at"`
	LastUsedAt    *time.Time `json:"last_used_at" db:"last_used_at"`
	LastUsedIP    string    `json:"last_used_ip" db:"last_used_ip"`
	Notes         string    `json:"notes" db:"notes"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// LicenseUsageLog tracks license usage/validation attempts
type LicenseUsageLog struct {
	ID        string    `json:"id" db:"id"`
	LicenseID string    `json:"license_id" db:"license_id"`
	IP        string    `json:"ip" db:"ip"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	Success   bool      `json:"success" db:"success"`
	Message   string    `json:"message" db:"message"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
