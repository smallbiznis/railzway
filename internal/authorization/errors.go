package authorization

import "errors"

var (
	ErrForbidden           = errors.New("forbidden")
	ErrInvalidActor        = errors.New("invalid_actor")
	ErrInvalidOrganization = errors.New("invalid_organization")
	ErrInvalidObject       = errors.New("invalid_object")
	ErrInvalidAction       = errors.New("invalid_action")
)
