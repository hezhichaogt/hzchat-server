/*
Package req provides helper functions for HTTP request parsing and data binding.

It encapsulates the logic for parsing JSON and Multipart Form data, and integrates
error handling to ensure data format correctness and size constraints, facilitating
subsequent business logic processing.
*/
package req

import (
	"encoding/json"
	"net/http"
	"strings"

	"hzchat/internal/pkg/errs"
)

const (
	// MaxFormMemory defines the maximum amount of memory (32 MB) ParseMultipartForm
	// will use to store non-file fields. File fields exceeding this limit are stored in temporary files.
	MaxFormMemory int64 = 32 << 20 // 32 MB

	// MaxRequestFileSize defines the maximum allowed size (20 MB) for the entire request body, including files.
	// This limit is enforced via http.MaxBytesReader.
	MaxRequestFileSize int64 = 20 << 20 // 20 MB
)

// BindJSON attempts to bind the JSON data from the HTTP request body to the destination struct dst.
func BindJSON(r *http.Request, dst any) *errs.CustomError {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		return errs.NewError(errs.ErrUnsupportedMediaType)
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return errs.NewError(errs.ErrInvalidJSONFormat)
	}

	if decoder.More() {
		return errs.NewError(errs.ErrExtraContentInBody)
	}

	return nil
}

// SetupMultipart sets up and parses Multipart Form or URL-encoded form data from the HTTP request.
func SetupMultipart(w http.ResponseWriter, r *http.Request) *errs.CustomError {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestFileSize)

	err := r.ParseMultipartForm(MaxFormMemory)

	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return errs.NewError(errs.ErrRequestEntityTooLarge)
		}

		return errs.NewError(errs.ErrFormParseFailed)
	}

	return nil
}
