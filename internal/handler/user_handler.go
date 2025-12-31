/*
Package handler provides HTTP handler functions for user authentication and management.
*/
package handler

import (
	"context"
	"net/http"
	"time"

	dbc "hzchat/internal/app/db/sqlc"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/req"
	"hzchat/internal/pkg/resp"

	"github.com/jackc/pgx/v5/pgtype"
)

type UpdateProfileInput struct {
	Nickname  string `json:"nickname"`
	AvatarUrl string `json:"avatarUrl"`
}

func HandleUpdateUserProfile(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := jwt.GetPayloadFromContext(r)
		if identity == nil || identity.UserType != "registered" {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		var input UpdateProfileInput
		if err := req.BindJSON(r, &input); err != nil {
			resp.RespondError(w, r, err)
			return
		}

		avatarKey, err := deps.NormalizeAssetKey(input.AvatarUrl)
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidParams))
			return
		}

		var userUUID pgtype.UUID
		_ = userUUID.Scan(identity.ID)

		oldUser, err := deps.DB.GetUserByID(r.Context(), userUUID)
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUserNotFound))
			return
		}

		updatedUser, err := deps.DB.UpdateUserProfile(r.Context(), dbc.UpdateUserProfileParams{
			ID:        userUUID,
			Nickname:  pgtype.Text{String: input.Nickname, Valid: true},
			AvatarUrl: pgtype.Text{String: avatarKey, Valid: true},
		})
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		oldKey := oldUser.AvatarUrl.String
		if avatarKey != "" && oldKey != "" && oldKey != avatarKey {
			go func(k string) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = deps.PublicStorage.Delete(ctx, k)
			}(oldKey)
		}

		lastLoginStr := ""
		if oldUser.LastLoginAt.Valid {
			lastLoginStr = oldUser.LastLoginAt.Time.Format(time.RFC3339)
		}

		avatarURL := deps.FullAssetURL(updatedUser.AvatarUrl.String)

		userData := map[string]any{
			"id":          identity.ID,
			"nickname":    updatedUser.Nickname.String,
			"avatar":      avatarURL,
			"userType":    "registered",
			"planType":    oldUser.PlanType,
			"lastLoginAt": lastLoginStr,
		}

		finalResponse := map[string]any{
			"user": userData,
		}

		newPayload := &jwt.Payload{
			ID:       identity.ID,
			UserType: identity.UserType,
			Nickname: updatedUser.Nickname.String,
			Avatar:   avatarURL,
		}

		newToken, err := jwt.GenerateToken(newPayload, deps.Config.JWTSecret, jwt.UserIdentityExpiration)
		if err != nil {
			logx.Error(err, "update_profile: token generation failed, fallback to old token")
		} else {
			finalResponse["token"] = newToken
		}

		resp.RespondSuccess(w, r, finalResponse)
	}
}
