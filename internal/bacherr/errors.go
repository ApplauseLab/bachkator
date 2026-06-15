package bacherr

import "errors"

var (
	ErrUsage              = errors.New("usage")
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrValidationFailed   = errors.New("validation failed")
	ErrUnsupported        = errors.New("unsupported")
	ErrCancelled          = errors.New("cancelled")
	ErrMissingInput       = errors.New("missing input")
	ErrInvalidInput       = errors.New("invalid input")
	ErrPlanLedgerConflict = errors.New("plan ledger conflict")
	ErrWaitingApproval    = errors.New("waiting for approval")
)
