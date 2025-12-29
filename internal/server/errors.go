package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	apikeydomain "github.com/smallbiznis/valora/internal/apikey/domain"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	"github.com/smallbiznis/valora/internal/authorization"
	customerdomain "github.com/smallbiznis/valora/internal/customer/domain"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	organizationdomain "github.com/smallbiznis/valora/internal/organization/domain"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
	pricetierdomain "github.com/smallbiznis/valora/internal/pricetier/domain"
	productdomain "github.com/smallbiznis/valora/internal/product/domain"
	ratingdomain "github.com/smallbiznis/valora/internal/rating/domain"
	signupdomain "github.com/smallbiznis/valora/internal/signup/domain"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"gorm.io/gorm"
)

type ValidationError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ValidationErrors struct {
	Errors []ValidationError `json:"errors"`
}

func (v ValidationErrors) Error() string {
	return "validation error"
}

type errorPayload struct {
	Type    string            `json:"type"`
	Message string            `json:"message"`
	Errors  []ValidationError `json:"errors,omitempty"`
}

type errorResponse struct {
	Error errorPayload `json:"error"`
}

var (
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrConflict           = errors.New("conflict")
	ErrInternal           = errors.New("internal_error")
	ErrNotFound           = errors.New("not_found")
	ErrInvalidRequest     = errors.New("invalid_request")
	ErrServiceUnavailable = errors.New("service_unavailable")
	ErrOrgRequired        = errors.New("org_required")
	ErrRateLimited        = errors.New("rate_limited")
)

func ErrorHandlingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if c.Writer.Written() {
			return
		}

		lastErr := c.Errors.Last()
		if lastErr == nil {
			return
		}

		status, payload := mapError(lastErr.Err)
		c.Header("Content-Type", "application/json")
		c.AbortWithStatusJSON(status, errorResponse{Error: payload})
	}
}

func AbortWithError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	_ = c.Error(err)
	c.Abort()
}

func invalidRequestError() error {
	return newValidationError("request", "invalid_request", "invalid request")
}

func newValidationError(field, code, message string) error {
	return &ValidationErrors{
		Errors: []ValidationError{
			{
				Field:   field,
				Code:    code,
				Message: message,
			},
		},
	}
}

func mapError(err error) (int, errorPayload) {
	if err == nil {
		return http.StatusInternalServerError, errorPayload{
			Type:    "internal_error",
			Message: "internal server error",
		}
	}

	if vErr := asValidationErrors(err); vErr != nil {
		return http.StatusBadRequest, errorPayload{
			Type:    "validation_error",
			Message: "validation error",
			Errors:  vErr.Errors,
		}
	}

	if isValidationError(err) {
		code := validationErrorCode(err)
		return http.StatusBadRequest, errorPayload{
			Type:    "validation_error",
			Message: "validation error",
			Errors: []ValidationError{
				{
					Field:   validationErrorField(code),
					Code:    code,
					Message: validationErrorMessage(code),
				},
			},
		}
	}

	switch {
	case errors.Is(err, ErrUnauthorized),
		errors.Is(err, authdomain.ErrInvalidCredentials),
		errors.Is(err, authdomain.ErrInvalidSession),
		errors.Is(err, authdomain.ErrSessionExpired),
		errors.Is(err, authdomain.ErrSessionRevoked):
		return http.StatusUnauthorized, errorPayload{
			Type:    "unauthorized",
			Message: "unauthorized",
		}
	case errors.Is(err, ErrForbidden),
		errors.Is(err, authorization.ErrForbidden):
		return http.StatusForbidden, errorPayload{
			Type:    "forbidden",
			Message: "forbidden",
		}
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests, errorPayload{
			Type:    "rate_limited",
			Message: "rate limited",
		}
	case errors.Is(err, organizationdomain.ErrForbidden):
		return http.StatusForbidden, errorPayload{
			Type:    "forbidden",
			Message: "forbidden",
		}
	case errors.Is(err, ErrConflict),
		errors.Is(err, authdomain.ErrUserExists):
		return http.StatusConflict, errorPayload{
			Type:    "conflict",
			Message: "conflict",
		}
	case isNotFoundError(err):
		return http.StatusNotFound, errorPayload{
			Type:    "not_found",
			Message: "not found",
		}
	case errors.Is(err, ErrServiceUnavailable):
		return http.StatusServiceUnavailable, errorPayload{
			Type:    "service_unavailable",
			Message: "service unavailable",
		}
	case errors.Is(err, ErrOrgRequired):
		return http.StatusPreconditionRequired, errorPayload{
			Type:    "precondition_required",
			Message: "organization required",
		}
	case errors.Is(err, ErrInternal):
		return http.StatusInternalServerError, errorPayload{
			Type:    "internal_error",
			Message: "internal server error",
		}
	default:
		return http.StatusInternalServerError, errorPayload{
			Type:    "internal_error",
			Message: "internal server error",
		}
	}
}

func asValidationErrors(err error) *ValidationErrors {
	var vErr *ValidationErrors
	if errors.As(err, &vErr) && vErr != nil {
		return vErr
	}
	return nil
}

func isValidationError(err error) bool {
	switch {
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, signupdomain.ErrInvalidRequest):
		return true
	case isOrganizationValidationError(err),
		isCustomerValidationError(err),
		isInvoiceValidationError(err),
		isRatingValidationError(err),
		isProductValidationError(err),
		isPriceValidationError(err),
		isPricingValidationError(err),
		isPriceAmountValidationError(err),
		isPriceTierValidationError(err),
		isMeterValidationError(err),
		isSubscriptionValidationError(err),
		isAPIKeyValidationError(err),
		isAuditValidationError(err),
		isAuthorizationValidationError(err):
		return true
	default:
		return false
	}
}

func isNotFoundError(err error) bool {
	switch {
	case errors.Is(err, ErrNotFound),
		errors.Is(err, customerdomain.ErrNotFound),
		errors.Is(err, productdomain.ErrNotFound),
		errors.Is(err, pricedomain.ErrNotFound),
		errors.Is(err, apikeydomain.ErrNotFound),
		errors.Is(err, meterdomain.ErrNotFound),
		errors.Is(err, priceamountdomain.ErrNotFound),
		errors.Is(err, pricetierdomain.ErrNotFound),
		errors.Is(err, invoicedomain.ErrBillingCycleNotFound),
		errors.Is(err, invoicedomain.ErrInvoiceNotFound),
		errors.Is(err, ratingdomain.ErrBillingCycleNotFound),
		errors.Is(err, subscriptiondomain.ErrSubscriptionNotFound),
		errors.Is(err, subscriptiondomain.ErrSubscriptionItemNotFound),
		errors.Is(err, gorm.ErrRecordNotFound):
		return true
	default:
		return false
	}
}

func isInvoiceValidationError(err error) bool {
	switch err {
	case invoicedomain.ErrInvalidOrganization,
		invoicedomain.ErrInvalidBillingCycle,
		invoicedomain.ErrBillingCycleNotClosed,
		invoicedomain.ErrMissingRatingResults,
		invoicedomain.ErrCurrencyMismatch,
		invoicedomain.ErrInvalidInvoiceID,
		invoicedomain.ErrInvoiceNotDraft,
		invoicedomain.ErrInvoiceNotFinalized:
		return true
	default:
		return false
	}
}

func isAPIKeyValidationError(err error) bool {
	switch err {
	case apikeydomain.ErrInvalidOrganization,
		apikeydomain.ErrInvalidName,
		apikeydomain.ErrInvalidKeyID:
		return true
	default:
		return false
	}
}

func isAuditValidationError(err error) bool {
	switch err {
	case auditdomain.ErrInvalidOrganization,
		auditdomain.ErrInvalidPageToken,
		auditdomain.ErrInvalidTimeRange,
		auditdomain.ErrInvalidAction:
		return true
	default:
		return false
	}
}

func isAuthorizationValidationError(err error) bool {
	switch err {
	case authorization.ErrInvalidActor,
		authorization.ErrInvalidOrganization,
		authorization.ErrInvalidObject,
		authorization.ErrInvalidAction:
		return true
	default:
		return false
	}
}

func isRatingValidationError(err error) bool {
	switch err {
	case ratingdomain.ErrInvalidBillingCycle,
		ratingdomain.ErrBillingCycleNotClosing,
		ratingdomain.ErrMissingUsage,
		ratingdomain.ErrMissingPriceAmount,
		ratingdomain.ErrMissingMeter,
		ratingdomain.ErrInvalidQuantity:
		return true
	default:
		return false
	}
}

func validationErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, signupdomain.ErrInvalidRequest):
		return "invalid_request"
	default:
		return err.Error()
	}
}

func validationErrorField(code string) string {
	if strings.HasPrefix(code, "invalid_") {
		return strings.TrimPrefix(code, "invalid_")
	}
	if code == "invalid_request" {
		return "request"
	}
	return ""
}

func validationErrorMessage(code string) string {
	switch code {
	case "invalid_request":
		return "invalid request"
	default:
		return "invalid value"
	}
}
