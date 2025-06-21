package errors

import "net/http"

// Error codes
const (
	ErrInternal = iota
	ErrUnauthorized
	ErrForbidden
	ErrNotFound
	ErrConflict
	ErrBadRequest
)

// AppError represents a custom application error
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	return e.Message
}

// NewAppError creates a new AppError
func NewAppError(errType int, message string) *AppError {
	var httpCode int
	switch errType {
	case ErrInternal:
		httpCode = http.StatusInternalServerError
	case ErrUnauthorized:
		httpCode = http.StatusUnauthorized
	case ErrForbidden:
		httpCode = http.StatusForbidden
	case ErrNotFound:
		httpCode = http.StatusNotFound
	case ErrConflict:
		httpCode = http.StatusConflict
	case ErrBadRequest:
		httpCode = http.StatusBadRequest
	default:
		httpCode = http.StatusInternalServerError
	}

	return &AppError{
		Code:    httpCode,
		Message: message,
	}
}