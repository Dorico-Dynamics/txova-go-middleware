// Package middleware provides HTTP middleware components for the Txova platform.
package middleware

import (
	"net/http"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
)

// Middleware-specific error codes extending txova-go-core/errors.
const (
	// CodeTokenRequired indicates no authorization header was provided.
	CodeTokenRequired errors.Code = "TOKEN_REQUIRED"
	// CodeRequestTimeout indicates the request exceeded the timeout limit.
	CodeRequestTimeout errors.Code = "REQUEST_TIMEOUT"
	// CodeMaintenanceMode indicates the service is in maintenance mode.
	CodeMaintenanceMode errors.Code = "MAINTENANCE_MODE"
)

// codeHTTPStatus maps middleware error codes to HTTP status codes.
var codeHTTPStatus = map[errors.Code]int{
	CodeTokenRequired:   http.StatusUnauthorized,
	CodeRequestTimeout:  http.StatusServiceUnavailable,
	CodeMaintenanceMode: http.StatusServiceUnavailable,
}

// HTTPStatus returns the HTTP status code for the given middleware error code.
// Returns 500 if the code is not a known middleware code.
func HTTPStatus(code errors.Code) int {
	if status, ok := codeHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Error constructors for middleware-specific errors.

// TokenRequired creates an error indicating no authorization token was provided.
func TokenRequired() *errors.AppError {
	return errors.New(CodeTokenRequired, "authorization token is required")
}

// RequestTimeout creates an error indicating the request exceeded the timeout.
func RequestTimeout() *errors.AppError {
	return errors.New(CodeRequestTimeout, "request timeout exceeded")
}

// MaintenanceMode creates an error indicating the service is in maintenance mode.
func MaintenanceMode(message string) *errors.AppError {
	if message == "" {
		message = "service is temporarily unavailable for maintenance"
	}
	return errors.New(CodeMaintenanceMode, message)
}

// Error checking helpers.

// IsTokenRequired checks if the error is a token required error.
func IsTokenRequired(err error) bool {
	return errors.IsCode(err, CodeTokenRequired)
}

// IsRequestTimeout checks if the error is a request timeout error.
func IsRequestTimeout(err error) bool {
	return errors.IsCode(err, CodeRequestTimeout)
}

// IsMaintenanceMode checks if the error is a maintenance mode error.
func IsMaintenanceMode(err error) bool {
	return errors.IsCode(err, CodeMaintenanceMode)
}
