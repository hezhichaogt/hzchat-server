package handler

import (
	"fmt"
	"hzchat/internal/app/chat"
	db "hzchat/internal/app/db/sqlc"
	"hzchat/internal/app/storage"
	"hzchat/internal/configs"
	"strings"
)

type AppDeps struct {
	Manager        *chat.Manager
	Config         *configs.AppConfig
	PublicStorage  storage.StorageService
	PrivateStorage storage.StorageService
	DB             *db.Queries
}

func (deps *AppDeps) FullAssetURL(key string) string {
	if key == "" {
		return ""
	}

	if strings.HasPrefix(key, "http") {
		return key
	}

	base := strings.TrimRight(deps.Config.S3PublicBaseURL, "/")
	path := strings.TrimLeft(key, "/")

	return base + "/" + path
}

func (deps *AppDeps) NormalizeAssetKey(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	if strings.HasPrefix(input, "http") {
		baseURL := strings.TrimRight(deps.Config.S3PublicBaseURL, "/") + "/"

		if strings.HasPrefix(input, baseURL) {
			return strings.TrimPrefix(input, baseURL), nil
		}

		return "", fmt.Errorf("invalid asset url: domain mismatch or unauthorized source")
	}

	return input, nil
}
