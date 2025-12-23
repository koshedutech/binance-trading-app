package license

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// LicenseType defines the type of license
type LicenseType string

const (
	LicenseTypePersonal   LicenseType = "personal"
	LicenseTypePro        LicenseType = "pro"
	LicenseTypeEnterprise LicenseType = "enterprise"
	LicenseTypeTrial      LicenseType = "trial"
	LicenseTypeInvalid    LicenseType = "invalid"
)

// LicenseInfo contains information about the current license
type LicenseInfo struct {
	Key          string      `json:"key"`
	Type         LicenseType `json:"type"`
	ValidUntil   time.Time   `json:"valid_until"`
	MaxSymbols   int         `json:"max_symbols"`
	Features     []string    `json:"features"`
	IsValid      bool        `json:"is_valid"`
	Message      string      `json:"message,omitempty"`
	LastChecked  time.Time   `json:"last_checked"`
	OfflineMode  bool        `json:"offline_mode"`
}

// Validator handles license validation
type Validator struct {
	mu           sync.RWMutex
	licenseInfo  *LicenseInfo
	validatorURL string
	offlineMode  bool
}

// LicenseKeyPattern matches XXX-XXXX-XXXX-XXXX format
var LicenseKeyPattern = regexp.MustCompile(`^[A-Z0-9]{3}-[A-Z0-9]{4}-[A-Z0-9]{4}-[A-Z0-9]{4}$`)

// NewValidator creates a new license validator
func NewValidator(validatorURL string) *Validator {
	return &Validator{
		validatorURL: validatorURL,
		offlineMode:  validatorURL == "",
	}
}

// ValidateLicense validates the license key
func (v *Validator) ValidateLicense(key string) (*LicenseInfo, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	key = strings.ToUpper(strings.TrimSpace(key))

	// Check if key is empty
	if key == "" {
		return v.createTrialLicense(), nil
	}

	// Validate key format
	if !LicenseKeyPattern.MatchString(key) {
		return &LicenseInfo{
			Key:     key,
			Type:    LicenseTypeInvalid,
			IsValid: false,
			Message: "Invalid license key format. Expected: XXX-XXXX-XXXX-XXXX",
		}, fmt.Errorf("invalid license key format")
	}

	// Try online validation first
	if !v.offlineMode {
		info, err := v.validateOnline(key)
		if err == nil {
			v.licenseInfo = info
			return info, nil
		}
		// Fall back to offline validation if online fails
	}

	// Offline validation using checksum
	info := v.validateOffline(key)
	v.licenseInfo = info
	return info, nil
}

// validateOnline validates the license against the server
func (v *Validator) validateOnline(key string) (*LicenseInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", v.validatorURL+"/validate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-License-Key", key)
	req.Header.Set("X-Product", "binance-trading-bot")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("validation failed with status: %d", resp.StatusCode)
	}

	var info LicenseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	info.LastChecked = time.Now()
	info.OfflineMode = false
	return &info, nil
}

// validateOffline validates license using local checksum
func (v *Validator) validateOffline(key string) *LicenseInfo {
	// Extract checksum from key
	parts := strings.Split(key, "-")
	if len(parts) != 4 {
		return &LicenseInfo{
			Key:     key,
			Type:    LicenseTypeInvalid,
			IsValid: false,
			Message: "Invalid license key format",
		}
	}

	// Validate checksum (last 4 chars should be derived from first 12)
	payload := parts[0] + parts[1] + parts[2]
	expectedChecksum := v.generateChecksum(payload)

	if parts[3] != expectedChecksum {
		return &LicenseInfo{
			Key:     key,
			Type:    LicenseTypeInvalid,
			IsValid: false,
			Message: "License key validation failed",
		}
	}

	// Determine license type from prefix
	licenseType := v.getLicenseTypeFromPrefix(parts[0])
	features, maxSymbols := v.getFeaturesForType(licenseType)

	return &LicenseInfo{
		Key:         key,
		Type:        licenseType,
		ValidUntil:  time.Now().AddDate(1, 0, 0), // 1 year validity for offline
		MaxSymbols:  maxSymbols,
		Features:    features,
		IsValid:     true,
		Message:     "License validated (offline mode)",
		LastChecked: time.Now(),
		OfflineMode: true,
	}
}

// createTrialLicense creates a trial license for users without a key
func (v *Validator) createTrialLicense() *LicenseInfo {
	return &LicenseInfo{
		Key:         "",
		Type:        LicenseTypeTrial,
		ValidUntil:  time.Now().AddDate(0, 0, 7), // 7 day trial
		MaxSymbols:  3,
		Features:    []string{"spot_trading", "basic_signals"},
		IsValid:     true,
		Message:     "Trial mode - 7 days, limited to 3 symbols",
		LastChecked: time.Now(),
		OfflineMode: true,
	}
}

// generateChecksum generates a 4-character checksum from payload
func (v *Validator) generateChecksum(payload string) string {
	// Add a salt for security
	salt := "BINANCE_BOT_2024"
	hash := sha256.Sum256([]byte(payload + salt))
	hexHash := hex.EncodeToString(hash[:])
	// Take first 4 characters and uppercase
	return strings.ToUpper(hexHash[:4])
}

// getLicenseTypeFromPrefix determines license type from key prefix
func (v *Validator) getLicenseTypeFromPrefix(prefix string) LicenseType {
	switch prefix[0] {
	case 'P': // Personal
		return LicenseTypePersonal
	case 'R': // pRo
		return LicenseTypePro
	case 'E': // Enterprise
		return LicenseTypeEnterprise
	default:
		return LicenseTypePersonal
	}
}

// getFeaturesForType returns features and max symbols for license type
func (v *Validator) getFeaturesForType(licenseType LicenseType) ([]string, int) {
	switch licenseType {
	case LicenseTypePersonal:
		return []string{
			"spot_trading",
			"futures_trading",
			"basic_signals",
			"ai_analysis",
		}, 10
	case LicenseTypePro:
		return []string{
			"spot_trading",
			"futures_trading",
			"basic_signals",
			"ai_analysis",
			"ginie_autopilot",
			"advanced_signals",
			"custom_strategies",
		}, 50
	case LicenseTypeEnterprise:
		return []string{
			"spot_trading",
			"futures_trading",
			"basic_signals",
			"ai_analysis",
			"ginie_autopilot",
			"advanced_signals",
			"custom_strategies",
			"api_access",
			"priority_support",
			"white_label",
		}, 999 // Unlimited
	case LicenseTypeTrial:
		return []string{
			"spot_trading",
			"basic_signals",
		}, 3
	default:
		return []string{}, 0
	}
}

// GetLicenseInfo returns current license info
func (v *Validator) GetLicenseInfo() *LicenseInfo {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.licenseInfo
}

// HasFeature checks if a feature is available
func (v *Validator) HasFeature(feature string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.licenseInfo == nil || !v.licenseInfo.IsValid {
		return false
	}

	for _, f := range v.licenseInfo.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// IsExpired checks if the license has expired
func (v *Validator) IsExpired() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.licenseInfo == nil {
		return true
	}
	return time.Now().After(v.licenseInfo.ValidUntil)
}

// GenerateLicenseKey generates a new license key (for admin use)
func GenerateLicenseKey(licenseType LicenseType) string {
	var prefix string
	switch licenseType {
	case LicenseTypePersonal:
		prefix = "PRS"
	case LicenseTypePro:
		prefix = "PRO"
	case LicenseTypeEnterprise:
		prefix = "ENT"
	default:
		prefix = "TRL"
	}

	// Generate random parts
	part2 := generateRandomAlphanumeric(4)
	part3 := generateRandomAlphanumeric(4)

	payload := prefix + part2 + part3
	salt := "BINANCE_BOT_2024"
	hash := sha256.Sum256([]byte(payload + salt))
	hexHash := hex.EncodeToString(hash[:])
	checksum := strings.ToUpper(hexHash[:4])

	return fmt.Sprintf("%s-%s-%s-%s", prefix, part2, part3, checksum)
}

// generateRandomAlphanumeric generates random alphanumeric string
func generateRandomAlphanumeric(length int) string {
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		// Simple random - in production use crypto/rand
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return string(result)
}

// GetLicenseFromEnv reads and validates license from environment
func GetLicenseFromEnv() (*LicenseInfo, error) {
	key := os.Getenv("LICENSE_KEY")
	validatorURL := os.Getenv("LICENSE_VALIDATOR_URL")

	validator := NewValidator(validatorURL)
	return validator.ValidateLicense(key)
}
