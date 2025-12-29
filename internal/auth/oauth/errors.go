package oauth

import "errors"

var (
	ErrProviderNotFound     = errors.New("oauth provider not found")
	ErrInvalidProvider      = errors.New("oauth provider invalid")
	ErrInvalidRequest       = errors.New("oauth invalid request")
	ErrUnauthorized         = errors.New("oauth unauthorized")
	ErrServiceUnavailable   = errors.New("oauth service unavailable")
	ErrProviderNotSupported = errors.New("oauth provider not supported")
)
