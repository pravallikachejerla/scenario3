package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrConcurrentUpdate  = errors.New("concurrent update conflict")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrTenantMismatch    = errors.New("tenant mismatch")
	ErrDependencyFailed  = errors.New("dependency call failed")
)
