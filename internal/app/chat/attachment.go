package chat

import (
	"encoding/json"
	"hzchat/internal/pkg/errs"
	"path/filepath"
	"strings"
	"time"
)

const (
	// MaxAttachmentSizeMB is the maximum allowed file size in megabytes.
	MaxAttachmentSizeMB = 5

	// MaxAttachmentSize is the maximum allowed file size in bytes.
	MaxAttachmentSize = MaxAttachmentSizeMB * 1024 * 1024

	// PresignedURLDuration is the fixed duration for which the upload URL is valid (5 minutes).
	PresignedURLDuration = 5 * time.Minute
)

// AllowedMIMETypes defines the set of permitted MIME types for file attachments.
var AllowedMIMETypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/gif":  {},
}

// ExtToMIME maps file extensions to their corresponding MIME types.
var ExtToMIME = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
	".gif":  "image/gif",
}

// Attachment represents a file attachment in a chat message.
type Attachment struct {
	Key      string          `json:"fileKey"`
	Name     string          `json:"fileName"`
	MimeType string          `json:"mimeType"`
	Size     int64           `json:"fileSize"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// ValidateFileSize checks if the provided file size is within acceptable limits.
func ValidateFileSize(fileSize int64) *errs.CustomError {
	if fileSize <= 0 {
		return errs.NewError(errs.ErrInvalidParams)
	}

	if fileSize > MaxAttachmentSize {
		return errs.NewError(errs.ErrFileSizeTooLarge)
	}

	return nil
}

// ValidateFileType checks if the provided file name and MIME type are allowed.
func ValidateFileType(fileName string, mimeType string) *errs.CustomError {
	lowerMimeType := strings.ToLower(mimeType)

	if _, ok := AllowedMIMETypes[lowerMimeType]; !ok {
		return errs.NewError(errs.ErrInvalidParams)
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == "" || len(ext) < 2 {
		return errs.NewError(errs.ErrInvalidParams)
	}

	expectedMIME, ok := ExtToMIME[ext]
	if !ok {
		return errs.NewError(errs.ErrInvalidParams)
	}

	if expectedMIME != lowerMimeType {
		return errs.NewError(errs.ErrInvalidParams)
	}

	return nil
}
