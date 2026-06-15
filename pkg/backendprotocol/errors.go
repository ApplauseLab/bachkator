package backendprotocol

import "fmt"

const (
	ErrorInvalidRequest        = "invalid_request"
	ErrorNotInitialized        = "not_initialized"
	ErrorUnsupportedCapability = "unsupported_capability"
	ErrorNotFound              = "not_found"
	ErrorConflict              = "conflict"
	ErrorValidationFailed      = "validation_failed"
	ErrorInternal              = "internal"
)

type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func (e Error) Error() string {
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code string, message string) Error {
	return Error{Code: code, Message: message}
}
