/*
Package resp provides helper functions for constructing and sending standardized HTTP JSON responses.

It defines a unified JSON response structure, including a business code, message, and optional data,
and offers convenient wrappers for both success and error responses.
*/
package resp

import (
	"encoding/json"
	"net/http"

	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
)

// JSONResponse defines the standardized JSON response structure returned by the application to clients.
type JSONResponse struct {
	// Code is the business status code (0 for success, others for specific errors, see errs package).
	Code int `json:"code"`

	// Message is the client-friendly status description or error message.
	Message string `json:"message"`

	// Data is the optional response payload (e.g., data returned from a successful request).
	Data any `json:"data,omitempty"`
}

// RespondJSON is a generic response function used to set the Content-Type and send the JSON payload.
func RespondJSON(w http.ResponseWriter, r *http.Request, httpStatus int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	response, err := json.Marshal(payload)
	if err != nil {
		logx.Error(
			err,
			"Error encoding JSON response",
			"http_status", httpStatus,
		)

		http.Error(w, "Error encoding JSON response", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(httpStatus)
	w.Write(response)
}

// RespondSuccess sends a successful HTTP response (HTTP 200 OK).
func RespondSuccess(w http.ResponseWriter, r *http.Request, data any) {
	res := JSONResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	}
	RespondJSON(w, r, http.StatusOK, res)
}

// RespondError sends an HTTP response containing custom error information.
func RespondError(w http.ResponseWriter, r *http.Request, customErr *errs.CustomError) {
	if customErr == nil {
		customErr = errs.NewError(errs.ErrUnknown)
	}

	res := JSONResponse{
		Code:    customErr.Code,
		Message: customErr.Message,
		Data:    nil,
	}
	RespondJSON(w, r, customErr.Status, res)
}
