package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"binance-trading-bot/internal/database"
)

// Service handles email sending operations
type Service struct {
	repo *database.Repository
}

// NewService creates a new email service
func NewService(repo *database.Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// SMTPConfig holds SMTP configuration
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	FromName string
}

// GetSMTPConfig retrieves SMTP settings from database
func (s *Service) GetSMTPConfig(ctx context.Context) (*SMTPConfig, error) {
	settings, err := s.repo.GetSMTPSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get SMTP settings: %w", err)
	}

	// Check if required settings are present
	required := []string{"smtp_host", "smtp_port", "smtp_username", "smtp_password", "smtp_from"}
	for _, key := range required {
		if settings[key] == "" {
			return nil, fmt.Errorf("SMTP not configured: missing %s", key)
		}
	}

	config := &SMTPConfig{
		Host:     settings["smtp_host"],
		Port:     settings["smtp_port"],
		Username: settings["smtp_username"],
		Password: settings["smtp_password"],
		From:     settings["smtp_from"],
		FromName: settings["smtp_from_name"],
	}

	if config.FromName == "" {
		config.FromName = "Binance Trading Bot"
	}

	return config, nil
}

// IsSMTPConfigured checks if SMTP is configured
func (s *Service) IsSMTPConfigured(ctx context.Context) bool {
	_, err := s.GetSMTPConfig(ctx)
	return err == nil
}

// SendEmail sends an email using SMTP settings from database
func (s *Service) SendEmail(ctx context.Context, to, subject, body string) error {
	config, err := s.GetSMTPConfig(ctx)
	if err != nil {
		return err
	}

	return s.sendEmailWithConfig(config, to, subject, body)
}

// sendEmailWithConfig sends an email with a specific SMTP config
func (s *Service) sendEmailWithConfig(config *SMTPConfig, to, subject, body string) error {
	// Build message
	from := config.From
	if config.FromName != "" {
		from = fmt.Sprintf("%s <%s>", config.FromName, config.From)
	}

	message := []byte(
		"From: " + from + "\r\n" +
			"To: " + to + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	// Setup authentication
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	// Connect and send
	addr := config.Host + ":" + config.Port

	// Log the attempt (without password)
	fmt.Printf("[EMAIL] Attempting to send email to %s via %s (port %s)\n", to, config.Host, config.Port)

	var err error
	// For TLS (port 465)
	if config.Port == "465" {
		err = s.sendEmailTLS(addr, auth, config.From, []string{to}, message)
	} else {
		// For STARTTLS (port 587) or plain (port 25)
		err = smtp.SendMail(addr, auth, config.From, []string{to}, message)
	}

	if err != nil {
		fmt.Printf("[EMAIL] Failed to send email: %v\n", err)
		return fmt.Errorf("SMTP error: %w", err)
	}

	fmt.Printf("[EMAIL] Successfully sent email to %s\n", to)
	return nil
}

// sendEmailTLS sends email using TLS connection (port 465)
func (s *Service) sendEmailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	// Connect with TLS
	host := strings.Split(addr, ":")[0]
	tlsConfig := &tls.Config{
		ServerName: host,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Authenticate
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Add recipients
	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to add recipient: %w", err)
		}
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// SendVerificationEmail sends a verification email with a 6-digit code
func (s *Service) SendVerificationEmail(ctx context.Context, to, code string) error {
	subject := "Verify Your Email Address"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4F46E5; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 5px 5px; }
        .code { font-size: 32px; font-weight: bold; letter-spacing: 8px; color: #4F46E5; text-align: center; margin: 30px 0; padding: 20px; background-color: white; border-radius: 5px; border: 2px dashed #4F46E5; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Email Verification</h1>
        </div>
        <div class="content">
            <p>Thank you for registering with Binance Trading Bot!</p>
            <p>Please enter the following verification code to complete your registration:</p>
            <div class="code">%s</div>
            <p>This code will expire in 15 minutes.</p>
            <p>If you didn't request this verification, please ignore this email.</p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Binance Trading Bot. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, code)

	return s.SendEmail(ctx, to, subject, body)
}

// SendPasswordResetEmail sends a password reset email
func (s *Service) SendPasswordResetEmail(ctx context.Context, to, resetLink string) error {
	subject := "Reset Your Password"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background-color: #4F46E5; color: white; padding: 20px; text-align: center; border-radius: 5px 5px 0 0; }
        .content { background-color: #f9fafb; padding: 30px; border-radius: 0 0 5px 5px; }
        .button { display: inline-block; padding: 12px 30px; background-color: #4F46E5; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset</h1>
        </div>
        <div class="content">
            <p>You requested to reset your password.</p>
            <p>Click the button below to reset your password:</p>
            <p style="text-align: center;">
                <a href="%s" class="button">Reset Password</a>
            </p>
            <p>This link will expire in 1 hour.</p>
            <p>If you didn't request a password reset, please ignore this email.</p>
        </div>
        <div class="footer">
            <p>&copy; 2025 Binance Trading Bot. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, resetLink)

	return s.SendEmail(ctx, to, subject, body)
}
