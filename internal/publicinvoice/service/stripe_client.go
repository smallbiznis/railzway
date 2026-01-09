package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	publicinvoicedomain "github.com/smallbiznis/valora/internal/publicinvoice/domain"
)

type stripePaymentIntent struct {
	ID           string `json:"id"`
	ClientSecret string `json:"client_secret"`
	Status       string `json:"status"`
	Amount       int64  `json:"amount"`
	Currency     string `json:"currency"`
}

type stripeErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

type stripeClient struct {
	apiKey    string
	accountID string
	client    *http.Client
}

func newStripeClient(apiKey string, accountID string) *stripeClient {
	return &stripeClient{
		apiKey:    strings.TrimSpace(apiKey),
		accountID: strings.TrimSpace(accountID),
		client:    &http.Client{Timeout: 12 * time.Second},
	}
}

func (c *stripeClient) createPaymentIntent(
	ctx context.Context,
	invoice *publicinvoicedomain.InvoiceRecord,
	amount int64,
) (stripePaymentIntent, error) {
	if invoice == nil {
		return stripePaymentIntent{}, paymentdomain.ErrInvalidConfig
	}
	values := url.Values{}
	values.Set("amount", strconv.FormatInt(amount, 10))
	values.Set("currency", strings.ToLower(invoice.Currency))
	values.Set("automatic_payment_methods[enabled]", "false")
	values.Set("payment_method_types[]", "card")
	values.Set("metadata[invoice_id]", invoice.ID.String())
	values.Set("metadata[invoice_number]", invoice.InvoiceNumber)
	values.Set("metadata[org_id]", invoice.OrgID.String())
	values.Set("metadata[customer_id]", invoice.CustomerID.String())

	return c.doRequest(ctx, http.MethodPost, "/v1/payment_intents", values, "invoice:"+invoice.ID.String())
}

func (c *stripeClient) retrievePaymentIntent(ctx context.Context, intentID string) (stripePaymentIntent, error) {
	return c.doRequest(ctx, http.MethodGet, "/v1/payment_intents/"+intentID, nil, "")
}

func (c *stripeClient) updatePaymentIntentAmount(
	ctx context.Context,
	intentID string,
	amount int64,
) (stripePaymentIntent, error) {
	values := url.Values{}
	values.Set("amount", strconv.FormatInt(amount, 10))
	return c.doRequest(ctx, http.MethodPost, "/v1/payment_intents/"+intentID, values, "")
}

func (c *stripeClient) doRequest(
	ctx context.Context,
	method string,
	path string,
	values url.Values,
	idempotencyKey string,
) (stripePaymentIntent, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return stripePaymentIntent{}, paymentdomain.ErrInvalidConfig
	}
	var bodyReader *strings.Reader
	if values != nil {
		bodyReader = strings.NewReader(values.Encode())
	} else {
		bodyReader = strings.NewReader("")
	}

	req, err := http.NewRequestWithContext(ctx, method, "https://api.stripe.com"+path, bodyReader)
	if err != nil {
		return stripePaymentIntent{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if values != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if c.accountID != "" {
		req.Header.Set("Stripe-Account", c.accountID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return stripePaymentIntent{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		var stripeErr stripeErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&stripeErr); err != nil {
			return stripePaymentIntent{}, errors.New("stripe_request_failed")
		}
		message := strings.TrimSpace(stripeErr.Error.Message)
		if message == "" {
			message = "stripe_request_failed"
		}
		return stripePaymentIntent{}, errors.New(message)
	}

	var intent stripePaymentIntent
	if err := json.NewDecoder(resp.Body).Decode(&intent); err != nil {
		return stripePaymentIntent{}, err
	}
	if intent.ID == "" {
		return stripePaymentIntent{}, errors.New("stripe_response_invalid")
	}
	return intent, nil
}
