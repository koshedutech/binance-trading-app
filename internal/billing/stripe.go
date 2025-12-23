package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"binance-trading-bot/internal/database"
)

// StripeService handles Stripe payment integration
type StripeService struct {
	secretKey      string
	publishableKey string
	webhookSecret  string
	repo           *database.Repository
	httpClient     *http.Client
	baseURL        string
}

// StripeConfig holds Stripe configuration
type StripeConfig struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
}

// NewStripeService creates a new Stripe service
func NewStripeService(config *StripeConfig, repo *database.Repository) *StripeService {
	return &StripeService{
		secretKey:      config.SecretKey,
		publishableKey: config.PublishableKey,
		webhookSecret:  config.WebhookSecret,
		repo:           repo,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		baseURL:        "https://api.stripe.com/v1",
	}
}

// IsConfigured returns true if Stripe is properly configured
func (s *StripeService) IsConfigured() bool {
	return s.secretKey != "" && s.webhookSecret != ""
}

// CustomerData represents Stripe customer data
type CustomerData struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name,omitempty"`
	DefaultSource string `json:"default_source,omitempty"`
}

// CreateCustomer creates a new Stripe customer
func (s *StripeService) CreateCustomer(ctx context.Context, email, name string) (*CustomerData, error) {
	data := map[string]string{
		"email": email,
		"name":  name,
	}

	resp, err := s.makeRequest(ctx, "POST", "/customers", data)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	var customer CustomerData
	if err := json.Unmarshal(resp, &customer); err != nil {
		return nil, fmt.Errorf("failed to parse customer response: %w", err)
	}

	return &customer, nil
}

// GetOrCreateCustomer gets an existing customer or creates a new one
func (s *StripeService) GetOrCreateCustomer(ctx context.Context, user *database.User) (string, error) {
	// If user already has a Stripe customer ID, return it
	if user.StripeCustomerID != "" {
		return user.StripeCustomerID, nil
	}

	// Create new customer
	customer, err := s.CreateCustomer(ctx, user.Email, user.Name)
	if err != nil {
		return "", err
	}

	// Update user with customer ID
	if err := s.repo.UpdateUserStripeCustomerID(ctx, user.ID, customer.ID); err != nil {
		log.Printf("Warning: failed to save Stripe customer ID: %v", err)
	}

	return customer.ID, nil
}

// SubscriptionData represents Stripe subscription data
type SubscriptionData struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	CurrentPeriodEnd  int64  `json:"current_period_end"`
	CurrentPeriodStart int64 `json:"current_period_start"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
}

// CreateSubscription creates a new subscription for a customer
func (s *StripeService) CreateSubscription(ctx context.Context, customerID string, tier SubscriptionTier) (*SubscriptionData, error) {
	priceID, err := s.getPriceIDForTier(tier)
	if err != nil {
		return nil, err
	}

	data := map[string]string{
		"customer": customerID,
		"items[0][price]": priceID,
	}

	resp, err := s.makeRequest(ctx, "POST", "/subscriptions", data)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	var sub SubscriptionData
	if err := json.Unmarshal(resp, &sub); err != nil {
		return nil, fmt.Errorf("failed to parse subscription response: %w", err)
	}

	return &sub, nil
}

// CancelSubscription cancels a subscription at period end
func (s *StripeService) CancelSubscription(ctx context.Context, subscriptionID string) error {
	data := map[string]string{
		"cancel_at_period_end": "true",
	}

	_, err := s.makeRequest(ctx, "POST", "/subscriptions/"+subscriptionID, data)
	if err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}

	return nil
}

// InvoiceData represents Stripe invoice data
type InvoiceData struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	AmountDue  int64  `json:"amount_due"`
	AmountPaid int64  `json:"amount_paid"`
	HostedInvoiceURL string `json:"hosted_invoice_url,omitempty"`
}

// CreateProfitShareInvoice creates an invoice for profit share
func (s *StripeService) CreateProfitShareInvoice(ctx context.Context, customerID string, amountUSD float64, periodID string, description string) (*InvoiceData, error) {
	// Convert to cents
	amountCents := int(amountUSD * 100)

	// Create invoice item first
	itemData := map[string]string{
		"customer":    customerID,
		"amount":      strconv.Itoa(amountCents),
		"currency":    "usd",
		"description": description,
	}

	_, err := s.makeRequest(ctx, "POST", "/invoiceitems", itemData)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice item: %w", err)
	}

	// Create and send the invoice
	invoiceData := map[string]string{
		"customer":      customerID,
		"auto_advance":  "true",
		"collection_method": "charge_automatically",
		"metadata[period_id]": periodID,
		"metadata[type]": "profit_share",
	}

	resp, err := s.makeRequest(ctx, "POST", "/invoices", invoiceData)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	var invoice InvoiceData
	if err := json.Unmarshal(resp, &invoice); err != nil {
		return nil, fmt.Errorf("failed to parse invoice response: %w", err)
	}

	// Finalize and send the invoice
	_, err = s.makeRequest(ctx, "POST", fmt.Sprintf("/invoices/%s/finalize", invoice.ID), nil)
	if err != nil {
		log.Printf("Warning: failed to finalize invoice: %v", err)
	}

	return &invoice, nil
}

// GetInvoice retrieves an invoice by ID
func (s *StripeService) GetInvoice(ctx context.Context, invoiceID string) (*InvoiceData, error) {
	resp, err := s.makeRequest(ctx, "GET", "/invoices/"+invoiceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}

	var invoice InvoiceData
	if err := json.Unmarshal(resp, &invoice); err != nil {
		return nil, fmt.Errorf("failed to parse invoice response: %w", err)
	}

	return &invoice, nil
}

// WebhookEvent represents a Stripe webhook event
type WebhookEvent struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
	Created int64           `json:"created"`
}

// WebhookObject represents the object in a webhook event
type WebhookObject struct {
	Object json.RawMessage `json:"object"`
}

// HandleWebhook processes a Stripe webhook event
func (s *StripeService) HandleWebhook(ctx context.Context, payload []byte, signature string) error {
	// Verify webhook signature
	if !s.verifyWebhookSignature(payload, signature) {
		return fmt.Errorf("invalid webhook signature")
	}

	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse webhook event: %w", err)
	}

	log.Printf("Processing Stripe webhook: %s", event.Type)

	switch event.Type {
	case "invoice.paid":
		return s.handleInvoicePaid(ctx, event.Data)
	case "invoice.payment_failed":
		return s.handleInvoicePaymentFailed(ctx, event.Data)
	case "customer.subscription.created":
		return s.handleSubscriptionCreated(ctx, event.Data)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event.Data)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event.Data)
	default:
		log.Printf("Unhandled webhook event type: %s", event.Type)
	}

	return nil
}

// handleInvoicePaid processes a paid invoice webhook
func (s *StripeService) handleInvoicePaid(ctx context.Context, data json.RawMessage) error {
	var obj WebhookObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	var invoice struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
		Metadata struct {
			PeriodID string `json:"period_id"`
			Type     string `json:"type"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(obj.Object, &invoice); err != nil {
		return err
	}

	// If this is a profit share invoice, update the period status
	if invoice.Metadata.Type == "profit_share" && invoice.Metadata.PeriodID != "" {
		if err := s.repo.UpdateProfitPeriodStatus(ctx, invoice.Metadata.PeriodID, string(StatusPaid), &invoice.ID); err != nil {
			log.Printf("Warning: failed to update profit period status: %v", err)
		}
	}

	log.Printf("Invoice paid: %s for customer %s", invoice.ID, invoice.Customer)
	return nil
}

// handleInvoicePaymentFailed processes a failed payment webhook
func (s *StripeService) handleInvoicePaymentFailed(ctx context.Context, data json.RawMessage) error {
	var obj WebhookObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	var invoice struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
		Metadata struct {
			PeriodID string `json:"period_id"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(obj.Object, &invoice); err != nil {
		return err
	}

	// Update period status to failed
	if invoice.Metadata.PeriodID != "" {
		if err := s.repo.UpdateProfitPeriodStatus(ctx, invoice.Metadata.PeriodID, string(StatusFailed), &invoice.ID); err != nil {
			log.Printf("Warning: failed to update profit period status: %v", err)
		}
	}

	log.Printf("Invoice payment failed: %s for customer %s", invoice.ID, invoice.Customer)
	return nil
}

// handleSubscriptionCreated processes a new subscription webhook
func (s *StripeService) handleSubscriptionCreated(ctx context.Context, data json.RawMessage) error {
	var obj WebhookObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	var sub struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(obj.Object, &sub); err != nil {
		return err
	}

	log.Printf("Subscription created: %s for customer %s (status: %s)", sub.ID, sub.Customer, sub.Status)
	return nil
}

// handleSubscriptionUpdated processes a subscription update webhook
func (s *StripeService) handleSubscriptionUpdated(ctx context.Context, data json.RawMessage) error {
	var obj WebhookObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	var sub struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(obj.Object, &sub); err != nil {
		return err
	}

	// Find user by Stripe customer ID and update status
	user, err := s.repo.GetUserByStripeCustomerID(ctx, sub.Customer)
	if err != nil || user == nil {
		log.Printf("Warning: could not find user for customer %s", sub.Customer)
		return nil
	}

	// Map Stripe status to our status
	var status database.SubscriptionStatus
	switch sub.Status {
	case "active", "trialing":
		status = database.StatusActive
	case "past_due":
		status = database.StatusPastDue
	case "canceled", "incomplete_expired":
		status = database.StatusCancelled
	case "unpaid":
		status = database.StatusSuspended
	default:
		status = database.StatusActive
	}

	if err := s.repo.UpdateUserSubscriptionStatus(ctx, user.ID, status); err != nil {
		log.Printf("Warning: failed to update user subscription status: %v", err)
	}

	log.Printf("Subscription updated: %s for customer %s (status: %s)", sub.ID, sub.Customer, sub.Status)
	return nil
}

// handleSubscriptionDeleted processes a deleted subscription webhook
func (s *StripeService) handleSubscriptionDeleted(ctx context.Context, data json.RawMessage) error {
	var obj WebhookObject
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	var sub struct {
		ID       string `json:"id"`
		Customer string `json:"customer"`
	}
	if err := json.Unmarshal(obj.Object, &sub); err != nil {
		return err
	}

	// Find user and downgrade to free tier
	user, err := s.repo.GetUserByStripeCustomerID(ctx, sub.Customer)
	if err != nil || user == nil {
		log.Printf("Warning: could not find user for customer %s", sub.Customer)
		return nil
	}

	// Downgrade to free tier
	if err := s.repo.UpdateUserSubscription(ctx, user.ID, database.TierFree, 30.0); err != nil {
		log.Printf("Warning: failed to downgrade user subscription: %v", err)
	}

	log.Printf("Subscription deleted: %s for customer %s - downgraded to free tier", sub.ID, sub.Customer)
	return nil
}

// Helper methods

// makeRequest makes an authenticated request to Stripe API
func (s *StripeService) makeRequest(ctx context.Context, method, path string, data map[string]string) ([]byte, error) {
	url := s.baseURL + path

	var body strings.Builder
	if data != nil {
		for k, v := range data {
			if body.Len() > 0 {
				body.WriteString("&")
			}
			body.WriteString(k)
			body.WriteString("=")
			body.WriteString(v)
		}
	}

	var req *http.Request
	var err error
	if method == "GET" {
		if body.Len() > 0 {
			url += "?" + body.String()
		}
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, strings.NewReader(body.String()))
	}
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(s.secretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respBody []byte
	if _, err := resp.Body.Read(respBody); err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Stripe API error: %s - %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

// verifyWebhookSignature verifies the Stripe webhook signature
func (s *StripeService) verifyWebhookSignature(payload []byte, signatureHeader string) bool {
	if s.webhookSecret == "" {
		return true // Skip verification if no secret configured (dev mode)
	}

	// Parse the signature header
	parts := strings.Split(signatureHeader, ",")
	var timestamp string
	var signatures []string

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return false
	}

	// Compute expected signature
	signedPayload := timestamp + "." + string(payload)
	h := hmac.New(sha256.New, []byte(s.webhookSecret))
	h.Write([]byte(signedPayload))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	// Check if any signature matches
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expectedSig)) {
			return true
		}
	}

	return false
}

// getPriceIDForTier returns the Stripe Price ID for a subscription tier
// These would be configured in your Stripe dashboard
func (s *StripeService) getPriceIDForTier(tier SubscriptionTier) (string, error) {
	// These are placeholder price IDs - replace with actual Stripe Price IDs
	priceIDs := map[SubscriptionTier]string{
		TierFree:   "",                     // No subscription needed
		TierTrader: "price_trader_monthly", // $49/month
		TierPro:    "price_pro_monthly",    // $149/month
		TierWhale:  "price_whale_monthly",  // $499/month
	}

	priceID, ok := priceIDs[tier]
	if !ok || priceID == "" {
		return "", fmt.Errorf("no price ID configured for tier: %s", tier)
	}

	return priceID, nil
}

// CreateCheckoutSession creates a Stripe Checkout session for subscription
func (s *StripeService) CreateCheckoutSession(ctx context.Context, customerID string, tier SubscriptionTier, successURL, cancelURL string) (string, error) {
	priceID, err := s.getPriceIDForTier(tier)
	if err != nil {
		return "", err
	}

	data := map[string]string{
		"customer":                  customerID,
		"mode":                      "subscription",
		"success_url":               successURL,
		"cancel_url":                cancelURL,
		"line_items[0][price]":      priceID,
		"line_items[0][quantity]":   "1",
	}

	resp, err := s.makeRequest(ctx, "POST", "/checkout/sessions", data)
	if err != nil {
		return "", fmt.Errorf("failed to create checkout session: %w", err)
	}

	var session struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resp, &session); err != nil {
		return "", fmt.Errorf("failed to parse session response: %w", err)
	}

	return session.URL, nil
}

// CreatePortalSession creates a Stripe Customer Portal session
func (s *StripeService) CreatePortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	data := map[string]string{
		"customer":   customerID,
		"return_url": returnURL,
	}

	resp, err := s.makeRequest(ctx, "POST", "/billing_portal/sessions", data)
	if err != nil {
		return "", fmt.Errorf("failed to create portal session: %w", err)
	}

	var session struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resp, &session); err != nil {
		return "", fmt.Errorf("failed to parse session response: %w", err)
	}

	return session.URL, nil
}
