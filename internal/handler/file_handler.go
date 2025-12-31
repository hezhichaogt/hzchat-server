package handler

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"hzchat/internal/app/chat"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/randx"
	"hzchat/internal/pkg/req"
	"hzchat/internal/pkg/resp"

	"github.com/google/uuid"
)

type PresignChatMessageInput struct {
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	FileSize int64  `json:"fileSize"`
}

func HandlePresignChatMessageURL(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := jwt.GetPayloadFromContext(r)

		if identity == nil || !randx.IsValidRoomCode(identity.Code) {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		room := deps.Manager.GetRoom(identity.Code)
		if room == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		var input PresignChatMessageInput
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
		fileKey := fmt.Sprintf("%s/%s%s", identity.Code, fileID, fileExt)

		url, err := deps.PrivateStorage.PresignUpload(
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

		resp.RespondSuccess(w, r, map[string]any{
			"presignedUrl": url,
			"fileKey":      fileKey,
			"fileName":     input.FileName,
		})
	}
}

type PresignAvatarInput struct {
	MimeType string `json:"mimeType"`
	FileSize int64  `json:"fileSize"`
}

func HandlePresignAvatarURL(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := jwt.GetPayloadFromContext(r)
		if identity == nil || identity.UserType != "registered" {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		var input PresignAvatarInput
		if err := req.BindJSON(r, &input); err != nil {
			resp.RespondError(w, r, err)
			return
		}

		if input.MimeType != "image/webp" {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnsupportedMediaType))
			return
		}

		if input.FileSize > 1*1024*1024 {
			resp.RespondError(w, r, errs.NewError(errs.ErrFileSizeTooLarge))
			return
		}

		fileKey := fmt.Sprintf("avatars/%s/%d.webp", identity.ID, time.Now().Unix())

		url, err := deps.PublicStorage.PresignUpload(
			r.Context(),
			fileKey,
			input.MimeType,
			input.FileSize,
			15*time.Minute,
		)

		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrFileStorageFailed))
			return
		}

		resp.RespondSuccess(w, r, map[string]any{
			"presignedUrl": url,
			"fileKey":      fileKey,
		})
	}
}

func HandlePresignDownloadURL(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := jwt.GetPayloadFromContext(r)

		if identity == nil || !randx.IsValidRoomCode(identity.Code) {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		fileKey := r.URL.Query().Get("k")
		if fileKey == "" {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		room := deps.Manager.GetRoom(identity.Code)
		if room == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrRoomNotFound))
			return
		}

		expectedKeyPrefix := fmt.Sprintf("%s/", identity.Code)

		if !strings.HasPrefix(fileKey, expectedKeyPrefix) {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		url, err := deps.PrivateStorage.PresignDownload(
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
