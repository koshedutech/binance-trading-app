# Epic 1: Authentication & User Management Refactoring

## Epic Overview

**Goal:** Streamline the authentication system with hardcoded admin user, SMTP configuration via admin UI, email verification, simplified password policy, per-user API key management, and subscription bypass.

**Business Value:** Enable secure deployment with admin-managed system settings, email-verified user accounts, and user-configurable Binance and AI API keys.

**Priority:** HIGH - Foundation for all other features

**Estimated Complexity:** MEDIUM-HIGH

---

## Current State Analysis

| Component | Current Implementation | Issue |
|-----------|----------------------|-------|
| Admin User | None | Need hardcoded admin for system config |
| SMTP Config | Environment variables | Should be configurable via admin UI |
| Subscription | 4-tier (Free/Trader/Pro/Whale) with Stripe | Over-engineered for single-user |
| API Keys (Binance) | HashiCorp Vault integration | Requires external service |
| AI Keys | Environment variables only | Cannot be changed per-user |
| Default Route | Dashboard (requires auth) | Should redirect to login |
| Email Verification | Optional (disabled by default) | Should be required |
| Password Policy | Complex (3 of 4 char types) | Too restrictive |

## Target State

| Component | Target Implementation |
|-----------|----------------------|
| **Admin User** | **Hardcoded: admin@local / Weber@#2025** |
| **SMTP Config** | **Admin UI configurable, stored in database (encrypted)** |
| Subscription | All users treated as "Whale" tier (bypassed) |
| API Keys (Binance) | Per-user, encrypted in PostgreSQL (AES-256) |
| AI Keys | Per-user, encrypted in PostgreSQL (AES-256) |
| Default Route | Unauthenticated → Login page |
| Email Verification | **REQUIRED** - 6-digit code sent via email |
| Password Policy | **SIMPLIFIED** - Any password accepted (no restrictions) |
| Settings UI | Combined page for Binance + AI keys |
| Billing UI | Hidden from navigation (code preserved) |

---

## Requirements Traceability

### Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| FR-1 | Hardcoded admin user (admin@local / Weber@#2025) | 1.3 |
| FR-2 | Admin settings page for SMTP configuration | 1.3 |
| FR-3 | System settings stored in database (encrypted) | 1.3 |
| FR-4 | Login page as default for unauthenticated users | 1.2 |
| FR-5 | User registration with email/password | 1.1, 1.4 |
| FR-6 | Email verification with 6-digit code | 1.4 |
| FR-7 | Forgot password with email link (verified emails only) | 1.4 |
| FR-8 | Simple password policy (no restrictions) | 1.4 |
| FR-9 | JWT session management with token refresh | 1.1 |
| FR-10 | Bypass all subscription tier checks | 1.5 |
| FR-11 | Per-user Binance API key storage (encrypted) | 1.6 |
| FR-12 | Per-user AI API key storage (encrypted) | 1.7 |
| FR-13 | Combined Settings page for all API keys | 1.8 |
| FR-14 | Hide billing/subscription UI | 1.5 |

### Non-Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| NFR-1 | API keys encrypted at rest (AES-256-GCM) | 1.6, 1.7 |
| NFR-2 | SMTP credentials encrypted in database | 1.3 |
| NFR-3 | Session tokens rotated on refresh | Already implemented |
| NFR-4 | Passwords hashed with bcrypt (cost 12) | Already implemented |
| NFR-5 | No plaintext secrets in logs or responses | All |
| NFR-6 | Verification codes expire after 15 minutes | 1.4 |

---

## Story List

| Story | Title | Priority | Complexity | Dependencies |
|-------|-------|----------|------------|--------------|
| 1.1 | Verify Existing Auth Flow Works | HIGH | LOW | None |
| 1.2 | Default Route to Login Page | HIGH | LOW | 1.1 |
| **1.3** | **Admin User & System Settings** | **HIGH** | **MEDIUM** | **1.1** |
| 1.4 | Email Verification & Simplified Password | HIGH | MEDIUM | 1.3 |
| 1.5 | Bypass Subscription Tier Enforcement | HIGH | MEDIUM | 1.1 |
| 1.6 | Per-User Binance API Key Storage | HIGH | MEDIUM | 1.3, 1.5 |
| 1.7 | Per-User AI API Key Storage | HIGH | MEDIUM | 1.6 |
| 1.8 | Create Combined Settings UI Page | HIGH | MEDIUM | 1.6, 1.7 |
| 1.9 | Integration Testing & Validation | HIGH | LOW | All |

---

## Story 1.1: Verify Existing Auth Flow Works

**As a** developer,
**I want** to verify the existing authentication flow is functional,
**So that** I have a working baseline before making changes.

### Acceptance Criteria

**AC-1.1.1: Registration Flow**
- **Given** a new user with valid email and password
- **When** they submit the registration form
- **Then** a new user record is created in the database
- **And** they receive access and refresh tokens

**AC-1.1.2: Login Flow**
- **Given** an existing user with correct credentials
- **When** they submit the login form
- **Then** they receive new access and refresh tokens
- **And** a session is created in user_sessions table

**AC-1.1.3: Token Refresh**
- **Given** a user with an expired access token but valid refresh token
- **When** the frontend calls the refresh endpoint
- **Then** new tokens are issued
- **And** the old session is rotated

**AC-1.1.4: Logout**
- **Given** an authenticated user
- **When** they click logout
- **Then** their session is revoked
- **And** they are redirected to the login page

### Tasks

- [ ] Task 1.1.1: Start the application and verify database migrations run
- [ ] Task 1.1.2: Test registration endpoint `POST /api/auth/register`
- [ ] Task 1.1.3: Test login endpoint `POST /api/auth/login`
- [ ] Task 1.1.4: Test refresh endpoint `POST /api/auth/refresh`
- [ ] Task 1.1.5: Test logout endpoint `POST /api/auth/logout`
- [ ] Task 1.1.6: Verify frontend login/register pages render correctly
- [ ] Task 1.1.7: Document any issues found

### Technical Notes

**Files to verify:**
- `internal/auth/handlers.go` - Auth endpoints
- `internal/auth/service.go` - Business logic
- `internal/auth/jwt.go` - Token management
- `web/src/pages/Login.tsx` - Login UI
- `web/src/pages/Register.tsx` - Registration UI
- `web/src/contexts/AuthContext.tsx` - Auth state management

---

## Story 1.2: Default Route to Login Page

**As a** user,
**I want** to be redirected to the login page when I'm not authenticated,
**So that** I can log in before accessing protected features.

### Acceptance Criteria

**AC-1.2.1: Unauthenticated Access**
- **Given** a user with no valid session
- **When** they navigate to any protected route (/, /dashboard, /futures, etc.)
- **Then** they are redirected to /login
- **And** the intended destination is preserved for post-login redirect

**AC-1.2.2: Root Route Behavior**
- **Given** a user accessing the root URL (/)
- **When** they are not authenticated
- **Then** they see the login page
- **When** they are authenticated
- **Then** they see the dashboard

**AC-1.2.3: Public Routes Accessible**
- **Given** any user (authenticated or not)
- **When** they navigate to /login, /register, /forgot-password, /verify-email
- **Then** they can access these pages without authentication

### Tasks

- [ ] Task 1.2.1: Update `web/src/App.tsx` to redirect "/" to "/login" for unauthenticated users
- [ ] Task 1.2.2: Verify `ProtectedRoute` component redirects correctly
- [ ] Task 1.2.3: Ensure post-login redirect preserves original destination
- [ ] Task 1.2.4: Add /verify-email route as public
- [ ] Task 1.2.5: Test all route scenarios manually

### Technical Notes

**Files to modify:**
- `web/src/App.tsx` - Route definitions
- `web/src/contexts/AuthContext.tsx` - ProtectedRoute component

---

## Story 1.3: Admin User & System Settings (NEW)

**As an** administrator,
**I want** a hardcoded admin account and settings page to configure SMTP,
**So that** I can set up email sending without editing environment variables.

### Acceptance Criteria

**AC-1.3.1: Hardcoded Admin User**
- **Given** a fresh database with no users
- **When** the application starts
- **Then** an admin user is automatically created with:
  - Email: `admin@local`
  - Password: `Weber@#2025` (bcrypt hashed)
  - `is_admin`: true
  - `email_verified`: true (skip verification for admin)
- **And** if admin already exists, no duplicate is created

**AC-1.3.2: Admin Login**
- **Given** the admin user
- **When** they login with `admin@local` / `Weber@#2025`
- **Then** they are authenticated successfully
- **And** they see "Admin Settings" option in the menu

**AC-1.3.3: System Settings Table**
- **Given** the database
- **When** migrations run
- **Then** a `system_settings` table exists with:
  - `key` (VARCHAR, PRIMARY KEY)
  - `value` (TEXT, encrypted for sensitive data)
  - `is_encrypted` (BOOLEAN)
  - `updated_at` (TIMESTAMP)
  - `updated_by` (UUID, FK to users)

**AC-1.3.4: Admin Settings Page**
- **Given** the admin user is logged in
- **When** they navigate to /admin/settings
- **Then** they see the Admin Settings page with:
  - SMTP Configuration section
  - Encryption Key section
- **Given** a non-admin user
- **When** they try to access /admin/settings
- **Then** they receive 403 Forbidden

**AC-1.3.5: SMTP Configuration**
- **Given** admin on Admin Settings page
- **When** they configure SMTP settings:
  - SMTP Host
  - SMTP Port
  - SMTP Username
  - SMTP Password
  - From Email
  - From Name
- **And** click "Save Settings"
- **Then** settings are encrypted and stored in database
- **And** success message is shown

**AC-1.3.6: SMTP Test**
- **Given** SMTP is configured
- **When** admin clicks "Test Email"
- **Then** a test email is sent to admin's email
- **And** success/failure result is shown

**AC-1.3.7: Encryption Key Management**
- **Given** admin on Admin Settings page
- **When** they view Encryption Key section
- **Then** they can see if a key is configured (not the actual key)
- **When** they click "Generate New Key"
- **Then** a new 32-byte key is generated
- **And** warning is shown about invalidating existing API keys
- **When** they confirm and save
- **Then** the new key is stored securely

### Tasks

- [ ] Task 1.3.1: Create `system_settings` table migration
- [ ] Task 1.3.2: Create `SystemSettings` model and repository
- [ ] Task 1.3.3: Add admin user seeding on application startup
- [ ] Task 1.3.4: Create encryption for system settings values
- [ ] Task 1.3.5: Create API handlers for admin settings (`internal/api/handlers_admin.go`)
- [ ] Task 1.3.6: Register admin-only routes with middleware
- [ ] Task 1.3.7: Create frontend `web/src/pages/AdminSettings.tsx`
- [ ] Task 1.3.8: Add admin route `/admin/settings` in App.tsx
- [ ] Task 1.3.9: Add "Admin Settings" to menu for admin users
- [ ] Task 1.3.10: Implement SMTP configuration form
- [ ] Task 1.3.11: Implement test email functionality
- [ ] Task 1.3.12: Implement encryption key management UI

### Technical Notes

**Admin User Seeding (in main.go or startup):**
```go
func seedAdminUser(db *database.Repository) error {
    // Check if admin exists
    _, err := db.GetUserByEmail(context.Background(), "admin@local")
    if err == nil {
        return nil // Admin already exists
    }

    // Create admin user
    hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("Weber@#2025"), 12)
    admin := &database.User{
        ID:            uuid.New(),
        Email:         "admin@local",
        PasswordHash:  string(hashedPassword),
        Name:          "Administrator",
        IsAdmin:       true,
        EmailVerified: true,
        CreatedAt:     time.Now(),
    }
    return db.CreateUser(context.Background(), admin)
}
```

**System Settings Table:**
```sql
CREATE TABLE system_settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL,
    is_encrypted BOOLEAN DEFAULT false,
    updated_at TIMESTAMP DEFAULT NOW(),
    updated_by UUID REFERENCES users(id)
);

-- Example keys:
-- smtp_host, smtp_port, smtp_user, smtp_password, smtp_from_email, smtp_from_name
-- encryption_key (for API keys)
```

**Admin Settings API Endpoints:**
- `GET /api/admin/settings` - Get all system settings (masked for sensitive)
- `PUT /api/admin/settings/smtp` - Update SMTP configuration
- `POST /api/admin/settings/smtp/test` - Send test email
- `PUT /api/admin/settings/encryption-key` - Update encryption key
- `POST /api/admin/settings/encryption-key/generate` - Generate new key

**Admin Middleware:**
```go
func RequireAdmin() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := GetClaimsFromContext(r.Context())
            if claims == nil || !claims.IsAdmin {
                http.Error(w, "Forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## Story 1.4: Email Verification & Simplified Password

**As a** user,
**I want** to verify my email with a 6-digit code during registration,
**So that** my account is secure and I can recover my password if needed.

### Acceptance Criteria

**AC-1.4.1: Registration Creates Unverified Account**
- **Given** a new user submitting the registration form
- **When** they provide name, email, and password
- **Then** user is created with `email_verified = false`
- **And** a 6-digit verification code is generated
- **And** code is stored in database with 15-minute expiry
- **And** email is sent using SMTP settings from database

**AC-1.4.2: Email Verification Page**
- **Given** a user who just registered
- **When** they are redirected to /verify-email
- **Then** they see a page with 6 input boxes for the code
- **And** they see their email address displayed
- **And** they can request a new code (with 60-second cooldown)

**AC-1.4.3: Code Verification**
- **Given** a user entering their 6-digit code
- **When** the code is correct and not expired
- **Then** `email_verified = true` is set in database
- **And** user is logged in and redirected to dashboard
- **When** the code is wrong or expired
- **Then** error message is shown

**AC-1.4.4: Simplified Password Policy**
- **Given** a user creating a password
- **When** they enter any password (minimum 1 character)
- **Then** the password is accepted
- **And** no complexity requirements are enforced

**AC-1.4.5: Forgot Password (Verified Users Only)**
- **Given** a user with `email_verified = true`
- **When** they request password reset
- **Then** reset link is sent to their email
- **Given** a user with `email_verified = false`
- **When** they request password reset
- **Then** error message indicates they must verify email first

**AC-1.4.6: SMTP Not Configured Handling**
- **Given** SMTP is not configured in system settings
- **When** a user tries to register
- **Then** registration fails with message: "Email service not configured. Please contact administrator."

### Tasks

- [ ] Task 1.4.1: Create email service that reads SMTP from database (`internal/email/service.go`)
- [ ] Task 1.4.2: Create email templates for verification and password reset
- [ ] Task 1.4.3: Add `email_verification_codes` table to database
- [ ] Task 1.4.4: Modify registration to check SMTP config first
- [ ] Task 1.4.5: Modify registration to generate and send verification code
- [ ] Task 1.4.6: Create verification endpoint `POST /api/auth/verify-email`
- [ ] Task 1.4.7: Create resend code endpoint `POST /api/auth/resend-verification`
- [ ] Task 1.4.8: **Simplify password validation** - remove complexity requirements
- [ ] Task 1.4.9: Update forgot password to require verified email
- [ ] Task 1.4.10: Create frontend `web/src/pages/VerifyEmail.tsx` page
- [ ] Task 1.4.11: Update registration flow to redirect to verification page

### Technical Notes

**Email Service reads from database:**
```go
func (s *EmailService) GetSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
    settings, err := s.repo.GetSystemSettings(ctx, []string{
        "smtp_host", "smtp_port", "smtp_user", "smtp_password",
        "smtp_from_email", "smtp_from_name",
    })
    if err != nil || settings["smtp_host"] == "" {
        return nil, errors.New("SMTP not configured")
    }
    // Decrypt password
    password, _ := crypto.Decrypt(settings["smtp_password"], s.encryptionKey)
    return &SMTPConfig{
        Host:     settings["smtp_host"],
        Port:     settings["smtp_port"],
        User:     settings["smtp_user"],
        Password: password,
        FromEmail: settings["smtp_from_email"],
        FromName:  settings["smtp_from_name"],
    }, nil
}
```

**Verification codes table:**
```sql
CREATE TABLE email_verification_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX idx_verification_user ON email_verification_codes(user_id);
```

**Password validation change in `internal/auth/password.go`:**
```go
func ValidatePasswordStrength(password string) error {
    if len(password) < 1 {
        return errors.New("password cannot be empty")
    }
    if len(password) > 128 {
        return errors.New("password too long")
    }
    return nil // Accept any password
}
```

---

## Story 1.5: Bypass Subscription Tier Enforcement

**As a** user,
**I want** full access to all features without subscription restrictions,
**So that** I can use the complete trading functionality.

### Acceptance Criteria

**AC-1.5.1: Tier Middleware Bypassed**
- **Given** any authenticated user (regardless of subscription_tier field)
- **When** they access any API endpoint
- **Then** tier checks always pass (treat as Whale tier)
- **And** no "upgrade required" errors occur

**AC-1.5.2: Feature Limits Removed**
- **Given** any user
- **When** they use trading features
- **Then** they have Whale-tier limits:
  - Max positions: unlimited (1000+)
  - Max leverage: 50x
  - Futures access: enabled
  - Autopilot: enabled

**AC-1.5.3: Billing UI Hidden**
- **Given** any authenticated user
- **When** they view the navigation menu
- **Then** "Billing" / "Subscription" menu items are not visible
- **And** /billing route redirects to dashboard

**AC-1.5.4: Code Preserved**
- **Given** the subscription/billing code
- **When** changes are made
- **Then** no code is deleted, only bypassed
- **And** code can be re-enabled via environment variable if needed

### Tasks

- [ ] Task 1.5.1: Add `SUBSCRIPTION_ENABLED=false` environment variable check
- [ ] Task 1.5.2: Modify `internal/auth/middleware.go:RequireTier()` to bypass when disabled
- [ ] Task 1.5.3: Modify tier limit checks in trading logic to use Whale limits
- [ ] Task 1.5.4: Hide billing navigation in frontend
- [ ] Task 1.5.5: Add redirect from /billing to /dashboard when subscription disabled
- [ ] Task 1.5.6: Document the bypass mechanism

### Technical Notes

**Bypass pattern in middleware.go:**
```go
func (m *Middleware) RequireTier(minTier string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !m.config.SubscriptionEnabled {
                next.ServeHTTP(w, r)
                return
            }
            // ... existing tier check logic
        })
    }
}
```

---

## Story 1.6: Per-User Binance API Key Storage

**As a** user,
**I want** my Binance API keys stored securely in the database,
**So that** I don't need external services like HashiCorp Vault.

### Acceptance Criteria

**AC-1.6.1: Database Storage**
- **Given** a user adding a new Binance API key
- **When** they submit the API key and secret
- **Then** keys are encrypted with encryption key from system settings
- **And** stored in the `user_api_keys` table
- **And** only the last 4 characters of API key are stored in plaintext

**AC-1.6.2: Key Retrieval**
- **Given** the trading system needs Binance credentials
- **When** it retrieves the user's API key
- **Then** the key is decrypted using system encryption key
- **And** decrypted value is never logged

**AC-1.6.3: Vault Bypass**
- **Given** the existing Vault integration code
- **When** Vault is not configured (no VAULT_ADDR)
- **Then** system uses database-only storage
- **And** Vault code path is skipped

**AC-1.6.4: API Endpoints**
- `GET /api/user/binance-keys` - List user's Binance keys (masked)
- `POST /api/user/binance-keys` - Add new Binance key pair
- `DELETE /api/user/binance-keys/:id` - Remove Binance key
- `POST /api/user/binance-keys/:id/test` - Test connection

**AC-1.6.5: Encryption Key Required**
- **Given** admin has not configured encryption key
- **When** user tries to save Binance API key
- **Then** error message: "Encryption not configured. Please contact administrator."

### Tasks

- [ ] Task 1.6.1: Create encryption utility (`internal/crypto/aes.go`)
- [ ] Task 1.6.2: Add `api_key_encrypted` and `secret_key_encrypted` columns to `user_api_keys`
- [ ] Task 1.6.3: Modify `repository_user.go` to encrypt/decrypt using system encryption key
- [ ] Task 1.6.4: Update API handlers to use database storage
- [ ] Task 1.6.5: Create test connection endpoint
- [ ] Task 1.6.6: Ensure keys work with Binance client

### Technical Notes

**Encryption uses system settings key:**
```go
func (r *Repository) GetEncryptionKey(ctx context.Context) ([]byte, error) {
    settings, err := r.GetSystemSettings(ctx, []string{"encryption_key"})
    if err != nil || settings["encryption_key"] == "" {
        return nil, errors.New("encryption key not configured")
    }
    return base64.StdEncoding.DecodeString(settings["encryption_key"])
}
```

---

## Story 1.7: Per-User AI API Key Storage

**As a** user,
**I want** to configure my own AI API keys (DeepSeek, OpenAI, Claude),
**So that** I can use my preferred AI provider for trading signals.

### Acceptance Criteria

**AC-1.7.1: Database Schema**
- **Given** the database schema
- **When** migrations run
- **Then** a new `user_ai_keys` table exists with:
  - user_id (FK to users)
  - provider (deepseek/openai/claude)
  - api_key_encrypted (AES-256 encrypted)
  - is_active (boolean)
  - created_at, updated_at

**AC-1.7.2: API Endpoints**
- `GET /api/user/ai-keys` - List configured AI keys (masked)
- `POST /api/user/ai-keys` - Add/update AI key for provider
- `DELETE /api/user/ai-keys/:provider` - Remove AI key
- `POST /api/user/ai-keys/:provider/test` - Test connection

**AC-1.7.3: LLM Client Integration**
- **Given** a user with AI keys configured
- **When** the LLM client makes API calls
- **Then** it uses the user's key (decrypted at runtime)
- **When** user has no key configured
- **Then** it falls back to environment variable (global key)

**AC-1.7.4: Key Validation**
- **Given** a user submitting an AI API key
- **When** the key is saved
- **Then** a test API call validates the key works
- **And** validation status is stored (valid/invalid)

### Tasks

- [ ] Task 1.7.1: Create `user_ai_keys` table migration
- [ ] Task 1.7.2: Add `UserAIKey` model to `models_user.go`
- [ ] Task 1.7.3: Add repository methods for AI key CRUD
- [ ] Task 1.7.4: Create API handlers (`internal/api/handlers_ai_keys.go`)
- [ ] Task 1.7.5: Register routes in `server.go`
- [ ] Task 1.7.6: Modify LLM client to check user keys first
- [ ] Task 1.7.7: Add key validation on save

### Technical Notes

**New table:**
```sql
CREATE TABLE user_ai_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    api_key_encrypted TEXT NOT NULL,
    api_key_last_four VARCHAR(4),
    is_active BOOLEAN DEFAULT true,
    validation_status VARCHAR(20) DEFAULT 'pending',
    last_validated_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, provider)
);
```

---

## Story 1.8: Create Combined Settings UI Page

**As a** user,
**I want** a single Settings page to manage all my API keys,
**So that** I can configure Binance and AI providers in one place.

### Acceptance Criteria

**AC-1.8.1: Settings Page Access**
- **Given** an authenticated user
- **When** they click on the user menu (avatar/name)
- **Then** they see "Settings" option
- **When** they click "Settings"
- **Then** they navigate to /settings

**AC-1.8.2: Binance API Keys Section**
- **Given** user on Settings page
- **When** the page loads
- **Then** they see "Binance API Keys" section with:
  - Current key (masked, last 4 chars) if configured
  - Status indicator (Connected/Not Connected)
  - Network indicator (Mainnet/Testnet)
  - Test Connection button
  - Edit/Delete buttons

**AC-1.8.3: AI API Keys Section**
- **Given** user on Settings page
- **When** the page loads
- **Then** they see "AI API Keys" section with cards for:
  - DeepSeek (configured/not configured)
  - OpenAI (configured/not configured)
  - Claude (configured/not configured)

**AC-1.8.4: Add/Edit Key Modal**
- **Given** user clicks "Configure" or "Edit"
- **When** modal opens
- **Then** they see input fields for the key
- **When** they save
- **Then** key is encrypted and stored
- **And** connection is tested automatically

**AC-1.8.5: Security Section**
- **Given** user on Settings page
- **When** they scroll to Security section
- **Then** they see "Change Password" option

### Tasks

- [ ] Task 1.8.1: Create `web/src/pages/Settings.tsx` component
- [ ] Task 1.8.2: Add route `/settings` in App.tsx
- [ ] Task 1.8.3: Update user menu to show "Settings" option
- [ ] Task 1.8.4: Implement Binance API Keys section
- [ ] Task 1.8.5: Implement AI API Keys section with provider cards
- [ ] Task 1.8.6: Implement Add/Edit key modal
- [ ] Task 1.8.7: Implement delete confirmation dialog
- [ ] Task 1.8.8: Implement test connection functionality
- [ ] Task 1.8.9: Implement password change section
- [ ] Task 1.8.10: Add API service methods

### Technical Notes

**User Menu Structure:**
```tsx
<DropdownMenu>
  <DropdownMenuItem onClick={() => navigate('/settings')}>
    Settings
  </DropdownMenuItem>
  {isAdmin && (
    <DropdownMenuItem onClick={() => navigate('/admin/settings')}>
      Admin Settings
    </DropdownMenuItem>
  )}
  <DropdownMenuSeparator />
  <DropdownMenuItem onClick={logout}>
    Logout
  </DropdownMenuItem>
</DropdownMenu>
```

---

## Story 1.9: Integration Testing & Validation

**As a** developer,
**I want** to verify the complete authentication and API key flow works end-to-end,
**So that** we can deploy with confidence.

### Acceptance Criteria

**AC-1.9.1: Admin Setup Flow**
- **Given** a fresh application instance
- **When** admin logs in (admin@local / Weber@#2025)
- **And** configures SMTP settings
- **And** generates encryption key
- **Then** system is ready for user registration

**AC-1.9.2: Complete User Journey**
- **Given** system is configured by admin
- **When** a new user:
  1. Navigates to the app (sees login page)
  2. Clicks "Create Account"
  3. Enters name, email, password
  4. Receives verification email with 6-digit code
  5. Enters code on verification page
  6. Is logged in and redirected to dashboard
  7. Navigates to Settings page
  8. Adds Binance API key
  9. Adds DeepSeek API key
  10. Uses trading features
- **Then** all steps complete successfully

**AC-1.9.3: Security Verification**
- **Given** the database
- **When** inspected directly
- **Then** all API keys are encrypted
- **And** SMTP password is encrypted
- **And** user passwords are hashed
- **And** no plaintext secrets visible

### Tasks

- [ ] Task 1.9.1: Test admin login and setup flow
- [ ] Task 1.9.2: Test SMTP configuration and email sending
- [ ] Task 1.9.3: Test user registration + verification flow
- [ ] Task 1.9.4: Test Binance API key flow
- [ ] Task 1.9.5: Test AI API key flow
- [ ] Task 1.9.6: Verify database encryption
- [ ] Task 1.9.7: Test error scenarios
- [ ] Task 1.9.8: Document setup instructions

---

## Implementation Order

```
Story 1.1 (Verify existing auth)
    ↓
Story 1.2 (Default to login)
    ↓
Story 1.3 (Admin user + SMTP config)  ← NEW - DO THIS EARLY
    ↓
Story 1.4 (Email verification + simplified password)
    ↓
Story 1.5 (Bypass subscription)
    ↓
Story 1.6 (Binance keys in DB - per user)
    ↓
Story 1.7 (AI keys in DB - per user)
    ↓
Story 1.8 (Combined Settings UI)
    ↓
Story 1.9 (Integration testing)
```

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Admin password exposure | LOW | HIGH | Document to change after setup |
| SMTP misconfiguration | MEDIUM | HIGH | Test email button, clear errors |
| Encryption key loss | MEDIUM | HIGH | Document backup procedure |
| Email delivery failures | MEDIUM | MEDIUM | Add retry logic, show errors |
| Breaking existing auth | LOW | HIGH | Verify auth works first |

---

## Definition of Done

- [ ] Admin can login with admin@local / Weber@#2025
- [ ] Admin can configure SMTP settings via UI
- [ ] Admin can generate/set encryption key via UI
- [ ] Email verification flow works end-to-end
- [ ] Simple password policy is implemented
- [ ] API keys encrypted at rest (verified via DB inspection)
- [ ] Fresh user can complete full registration → verification → trading flow
- [ ] Settings page shows both Binance and AI keys
- [ ] Docker rebuild completes successfully
- [ ] No TypeScript or Go compilation errors

---

## Notes

**Admin Credentials (Hardcoded):**
- Email: `admin@local`
- Password: `Weber@#2025`
- **IMPORTANT:** Change this password after first login in production!

**First-Time Setup Order:**
1. Start application
2. Login as admin
3. Go to Admin Settings
4. Configure SMTP
5. Generate encryption key
6. System ready for users

**Password Policy:** No restrictions. Any non-empty password up to 128 characters.

**Preserved Code:** All subscription/billing code is preserved but bypassed.
