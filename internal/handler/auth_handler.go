/*
Package handler provides HTTP handler functions for user authentication and management.
*/
package handler

import (
	"context"
	"net/http"
	"regexp"
	"time"
	"unicode/utf8"

	"hzchat/internal/app/db"
	dbc "hzchat/internal/app/db/sqlc"
	"hzchat/internal/pkg/auth/jwt"
	"hzchat/internal/pkg/errs"
	"hzchat/internal/pkg/logx"
	"hzchat/internal/pkg/randx"
	"hzchat/internal/pkg/req"
	"hzchat/internal/pkg/resp"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-z0-9_]{4,20}$`)
)

type RegisterInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// HandleRegister processes the request to create a new user account with only username and password.
func HandleRegister(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if payload := jwt.GetPayloadFromContext(r); payload != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrAlreadyLoggedIn))
			return
		}

		var input RegisterInput
		if customErr := req.BindJSON(r, &input); customErr != nil {
			resp.RespondError(w, r, customErr)
			return
		}

		if !usernameRegex.MatchString(input.Username) {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidUsername))
			return
		}

		passwordLen := utf8.RuneCountInString(input.Password)
		if passwordLen < 6 || passwordLen > 50 {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidPassword))
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		nickname, err := randx.UserNickname()
		if err != nil {
			nickname = "User_X"
		}

		user, err := deps.DB.CreateUser(r.Context(), dbc.CreateUserParams{
			Username:     input.Username,
			PasswordHash: string(hashedPassword),
			Nickname: pgtype.Text{
				String: nickname,
				Valid:  true,
			},
		})

		if err != nil {
			if db.IsUniqueViolation(err) {
				logx.Warn("registration conflict: username already exists", "username", input.Username)
				resp.RespondError(w, r, errs.NewError(errs.ErrUserAlreadyExists))
				return
			}

			logx.Error(err, "failed to create user in database")
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		if err := deps.DB.UpdateLastLogin(r.Context(), user.ID); err != nil {
			logx.Error(err, "register: failed to update last_login_at", "user_id", user.ID)
		}

		payload := &jwt.Payload{
			ID:       user.ID.String(),
			UserType: "registered",
			Nickname: user.Nickname.String,
		}

		tokenString, err := jwt.GenerateToken(payload, deps.Config.JWTSecret, jwt.UserIdentityExpiration)
		if err != nil {
			logx.Error(err, "failed to generate token after registration")
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		resp.RespondSuccess(w, r, map[string]any{
			"token": tokenString,
			"user": map[string]any{
				"id":          user.ID.String(),
				"nickname":    user.Nickname.String,
				"avatar":      "",
				"userType":    "registered",
				"planType":    "FREE",
				"lastLoginAt": time.Now().Format(time.RFC3339),
			},
		})
	}
}

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// HandleLogin verifies user credentials and issues a JWT token.
func HandleLogin(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if identity := jwt.GetPayloadFromContext(r); identity != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrAlreadyLoggedIn))
			return
		}

		var input LoginInput
		if customErr := req.BindJSON(r, &input); customErr != nil {
			resp.RespondError(w, r, customErr)
			return
		}

		dbUser, err := deps.DB.GetUserByUsername(r.Context(), input.Username)
		if err != nil {
			logx.Warn("login: user fetch failed", "username", input.Username, "error", err)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidCredentials))
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(dbUser.PasswordHash), []byte(input.Password)); err != nil {
			logx.Warn("login: password mismatch", "username", input.Username)
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidCredentials))
			return
		}

		if err := deps.DB.UpdateLastLogin(r.Context(), dbUser.ID); err != nil {
			logx.Error(err, "login: failed to update last_login_at", "user_id", dbUser.ID)
		}

		avatarURL := deps.FullAssetURL(dbUser.AvatarUrl.String)

		payload := &jwt.Payload{
			ID:       dbUser.ID.String(),
			UserType: "registered",
			Nickname: dbUser.Nickname.String,
			Avatar:   avatarURL,
		}

		token, err := jwt.GenerateToken(payload, deps.Config.JWTSecret, jwt.UserIdentityExpiration)

		if err != nil {
			logx.Error(err, "login: jwt generation failed")
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		resp.RespondSuccess(w, r, map[string]any{
			"token": token,
			"user": map[string]any{
				"id":          dbUser.ID.String(),
				"nickname":    dbUser.Nickname.String,
				"avatar":      avatarURL,
				"userType":    "registered",
				"planType":    dbUser.PlanType,
				"lastLoginAt": time.Now().Format(time.RFC3339),
			},
		})
	}
}

// HandleGetUserProfile retrieves the current authenticated user's profile and
// updates the last_login_at timestamp if the threshold is met.
func HandleGetUserProfile(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := jwt.GetPayloadFromContext(r)
		if identity == nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		var userUUID pgtype.UUID
		if err := userUUID.Scan(identity.ID); err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		dbUser, err := deps.DB.GetUserByID(r.Context(), userUUID)
		if err != nil {
			logx.Warn("get_user_profile: user not found", "id", identity.ID)
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		var lastLoginResponse any = nil
		if dbUser.LastLoginAt.Valid {
			lastLoginResponse = dbUser.LastLoginAt.Time.Format(time.RFC3339)
		}

		shouldUpdate := !dbUser.LastLoginAt.Valid || time.Since(dbUser.LastLoginAt.Time) > 30*time.Minute

		if shouldUpdate {
			go func(id pgtype.UUID) {
				if err := deps.DB.UpdateLastLogin(context.Background(), id); err != nil {
					logx.Error(err, "get_user_profile: failed to update last_login_at", "user_id", id)
				}
			}(dbUser.ID)
		}

		resp.RespondSuccess(w, r, map[string]any{
			"user": map[string]any{
				"id":          dbUser.ID.String(),
				"nickname":    dbUser.Nickname.String,
				"avatar":      deps.FullAssetURL(dbUser.AvatarUrl.String),
				"userType":    "registered",
				"planType":    dbUser.PlanType,
				"lastLoginAt": lastLoginResponse,
			},
		})
	}
}

type ChangePasswordInput struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func HandleChangePassword(deps *AppDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity := jwt.GetPayloadFromContext(r)
		if identity == nil || identity.UserType != "registered" {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnauthorized))
			return
		}

		var input ChangePasswordInput
		if customErr := req.BindJSON(r, &input); customErr != nil {
			resp.RespondError(w, r, customErr)
			return
		}

		passwordLen := utf8.RuneCountInString(input.NewPassword)
		if passwordLen < 6 || passwordLen > 50 {
			resp.RespondError(w, r, errs.NewError(errs.ErrInvalidPassword))
			return
		}

		var userUUID pgtype.UUID
		_ = userUUID.Scan(identity.ID)
		user, err := deps.DB.GetUserByID(r.Context(), userUUID)
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUserNotFound))
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.OldPassword))
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrOldPasswordInvalid))
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		err = deps.DB.UpdateUserPassword(r.Context(), dbc.UpdateUserPasswordParams{
			ID:           userUUID,
			PasswordHash: string(hashedPassword),
		})
		if err != nil {
			logx.Error(err, "failed to update user password in database", "user_id", identity.ID)
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		newToken, err := jwt.GenerateToken(identity, deps.Config.JWTSecret, jwt.UserIdentityExpiration)
		if err != nil {
			logx.Error(err, "failed to generate token after password change", "user_id", identity.ID)
			resp.RespondError(w, r, errs.NewError(errs.ErrUnknown))
			return
		}

		resp.RespondSuccess(w, r, map[string]any{
			"token": newToken,
		})
	}
}
