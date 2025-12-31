/*
Package errs provides custom error types and application-level error code constants.

This file defines the CustomError struct, which implements the standard Go error interface
and includes a business code, a user-friendly message, and an HTTP status code for unified error reporting.
*/
package errs

import (
	"fmt"
	"net/http"
	"strings"

	"hzchat/internal/pkg/logx"
)

// CustomError is the custom error structure used throughout the application.
// It wraps the Go error interface, adding a business code and HTTP status code.
type CustomError struct {
	// Code is the business error code (see constants definition).
	Code int

	// Message is the user-friendly error description.
	Message string

	// Status is the standard HTTP status code corresponding to this error.
	Status int
}

// Error implements the standard Go error interface. It returns a formatted
// error string containing the error code, HTTP status, and message.
func (e CustomError) Error() string {
	return fmt.Sprintf("Error Code %d (HTTP %d): %s", e.Code, e.Status, e.Message)
}

// NewError constructs and returns a new *CustomError instance based on a predefined error code.
// The optional details parameter allows for formatting arguments (printf-style) to be supplied
// for the error message. If an unknown code is provided, it defaults to returning ErrUnknown.
func NewError(code int, details ...any) *CustomError {
	templateErr, ok := errorMap[code]

	if !ok {
		logx.Error(
			fmt.Errorf("attempted to create an error with an unknown code in errorMap"),
			"Unknown error code requested",
			"requested_code", code,
		)

		unknownErr := errorMap[ErrUnknown]
		return &CustomError{
			Code:    unknownErr.Code,
			Message: unknownErr.Message,
			Status:  unknownErr.Status,
		}
	}

	customErr := templateErr

	if customErr.Status == 0 {
		customErr.Status = http.StatusOK
	}

	if code == ErrUnknown && len(details) > 0 {
		if originalErr, ok := details[0].(error); ok {
			logx.Error(
				originalErr,
				"Handling ErrUnknown with underlying error",
			)
		}
	} else if len(details) > 0 {
		if strings.Contains(customErr.Message, "%") {
			customErr.Message = fmt.Sprintf(customErr.Message, details...)
		} else {
			logx.Warn(
				"Details provided for error, but message template has no formatting placeholders. Details ignored.",
			)
		}
	}

	return &customErr
}
