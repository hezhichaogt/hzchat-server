/*
Package handler provides HTTP handler functions for user authentication and management.
*/
package handler

import (
	"net/http"
	"regexp"
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
			"user": map[string]string{
				"id":       user.ID.String(),
				"nickname": user.Nickname.String,
				"avatar":   "",
				"userType": "registered",
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

		payload := &jwt.Payload{
			ID:       dbUser.ID.String(),
			UserType: "registered",
			Nickname: dbUser.Nickname.String,
			Avatar:   dbUser.AvatarUrl.String,
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
				"id":       dbUser.ID.String(),
				"nickname": dbUser.Nickname.String,
				"avatar":   dbUser.AvatarUrl.String,
				"userType": "registered",
			},
		})
	}
}
