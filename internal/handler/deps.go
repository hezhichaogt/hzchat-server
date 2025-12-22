package handler

import (
	"hzchat/internal/app/chat"
	db "hzchat/internal/app/db/sqlc"
	"hzchat/internal/app/storage"
	"hzchat/internal/configs"
)

type AppDeps struct {
	Manager        *chat.Manager
	Config         *configs.AppConfig
	StorageService storage.StorageService
	DB             *db.Queries
}
