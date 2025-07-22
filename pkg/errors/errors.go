package errors

import (
	"fmt"
	"net/http"
)

// APIError represents an error with an associated HTTP status code
type APIError struct {
	Message    string
	StatusCode int
	Code       string
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Message
}

// Common error constructors

// BadRequest creates a 400 error
func BadRequest(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Code:       "BAD_REQUEST",
	}
}

// Unauthorized creates a 401 error
func Unauthorized(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusUnauthorized,
		Code:       "UNAUTHORIZED",
	}
}

// Forbidden creates a 403 error
func Forbidden(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusForbidden,
		Code:       "FORBIDDEN",
	}
}

// NotFound creates a 404 error
func NotFound(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusNotFound,
		Code:       "NOT_FOUND",
	}
}

// Conflict creates a 409 error
func Conflict(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusConflict,
		Code:       "CONFLICT",
	}
}

// UnprocessableEntity creates a 422 error
func UnprocessableEntity(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusUnprocessableEntity,
		Code:       "UNPROCESSABLE_ENTITY",
	}
}

// InternalServerError creates a 500 error
func InternalServerError(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Code:       "INTERNAL_ERROR",
	}
}

// DatabaseError creates a 500 error with database prefix
func DatabaseError(message string) *APIError {
	return &APIError{
		Message:    fmt.Sprintf("database error: %s", message),
		StatusCode: http.StatusInternalServerError,
		Code:       "DATABASE_ERROR",
	}
}

// ValidationError creates a 400 error with validation prefix
func ValidationError(message string) *APIError {
	return &APIError{
		Message:    fmt.Sprintf("validation error: %s", message),
		StatusCode: http.StatusBadRequest,
		Code:       "VALIDATION_ERROR",
	}
}

// CompressionError creates a 500 error for compression failures
func CompressionError(message string) *APIError {
	return &APIError{
		Message:    fmt.Sprintf("compression error: %s", message),
		StatusCode: http.StatusInternalServerError,
		Code:       "COMPRESSION_ERROR",
	}
}

// RateLimitError creates a 429 error
func RateLimitError(message string) *APIError {
	return &APIError{
		Message:    message,
		StatusCode: http.StatusTooManyRequests,
		Code:       "RATE_LIMIT_EXCEEDED",
	}
}

// IsAPIError checks if an error is an APIError
func IsAPIError(err error) (*APIError, bool) {
	apiErr, ok := err.(*APIError)
	return apiErr, ok
}