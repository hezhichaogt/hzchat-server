package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"hzchat/internal/app/chat"
	"hzchat/internal/app/storage"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/req"
	"hzchat/internal/pkg/resp"

	"github.com/google/uuid"
)

// PresignUploadInput defines the JSON input structure for generating upload URL.
type PresignUploadInput struct {
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
}

// HandlePresignUploadURL creates an HTTP HandlerFunc to generate a time-limited,
// pre-signed URL for file upload, scoped to a specific room.
func HandlePresignUploadURL(manager *chat.Manager, storageService storage.StorageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload := jwt.GetPayloadFromContext(r)
		if payload == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		room := manager.GetRoom(payload.Code)
		if room == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		var input PresignUploadInput
		if customErr := req.BindJSON(r, &input); customErr != nil {
			resp.RespondError(w, r, customErr)
			return
		}

		if err := chat.ValidateFileSize(input.FileSize); err != nil {
			resp.RespondError(w, r, err)
			return
		}

		if err := chat.ValidateFileType(input.FileName, input.MimeType); err != nil {
			resp.RespondError(w, r, err)
			return
		}

		fileExt := strings.ToLower(filepath.Ext(input.FileName))
		fileID := uuid.New().String()
		fileKey := fmt.Sprintf("%s/%s%s", payload.Code, fileID, fileExt)

		url, err := storageService.PresignUpload(
			r.Context(),
			fileKey,
			input.MimeType,
			input.FileSize,
			chat.PresignedURLDuration,
		)

		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrFileStorageFailed))
			return
		}

		data := map[string]any{
			"presignedUrl": url,
			"fileKey":      fileKey,
			"fileName":     input.FileName,
		}
		resp.RespondSuccess(w, r, data)
	}
}

// HandlePresignDownloadURL creates an HTTP HandlerFunc to generate a time-limited,
// pre-signed URL for file download, scoped to a specific room.
func HandlePresignDownloadURL(manager *chat.Manager, storageService storage.StorageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload := jwt.GetPayloadFromContext(r)
		if payload == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		fileKey := r.URL.Query().Get("k")
		if fileKey == "" {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		room := manager.GetRoom(payload.Code)
		if room == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		expectedKeyPrefix := fmt.Sprintf("%s/", payload.Code)
		if !strings.HasPrefix(fileKey, expectedKeyPrefix) {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		url, err := storageService.PresignDownload(
			r.Context(),
			fileKey,
			chat.PresignedURLDuration,
		)

		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrFileStorageFailed))
			return
		}

		http.Redirect(w, r, url, http.StatusFound)
	}
}
