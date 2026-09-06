package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ochidoma/platform/internal/audit"
	authpkg "github.com/ochidoma/platform/internal/auth"
	"github.com/ochidoma/platform/internal/repository"
)

const refreshCookieName = "ochidoma_refresh"

type Handler struct {
	users            *repository.UserRepo
	accessSecret     string
	accessTTL        time.Duration
	refreshTTL       time.Duration
	secureCookies    bool // false only in local dev over http
}

func NewHandler(users *repository.UserRepo, accessSecret string, accessTTL, refreshTTL time.Duration, secureCookies bool) *Handler {
	return &Handler{
		users:         users,
		accessSecret:  accessSecret,
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
		secureCookies: secureCookies,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	User        struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Role     string `json:"role"`
	} `json:"user"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login failed")
		return
	}
	// Deliberately identical error for "no such user" and "wrong password"
	// so login responses don't leak which emails are registered.
	invalidCreds := func() { writeError(w, http.StatusUnauthorized, "invalid email or password") }

	if user == nil || !user.IsActive {
		invalidCreds()
		return
	}
	ok, err := authpkg.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		invalidCreds()
		return
	}

	accessToken, err := authpkg.IssueAccessToken(h.accessSecret, h.accessTTL, user.ID, user.RoleName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}

	rawRefresh, err := authpkg.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue session")
		return
	}
	expiresAt := time.Now().Add(h.refreshTTL)
	if err := h.users.StoreRefreshToken(r.Context(), user.ID, authpkg.HashRefreshToken(rawRefresh), expiresAt, r.RemoteAddr); err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue session")
		return
	}

	_ = h.users.TouchLastLogin(r.Context(), user.ID)
	_ = audit.Log(r.Context(), h.users.Pool(), audit.Entry{
		UserID: user.ID, Action: audit.ActionAdminLogin,
		EntityType: "user", EntityID: user.ID, IPAddress: r.RemoteAddr,
	})

	http.SetCookie(w, h.refreshCookie(rawRefresh, expiresAt))

	resp := loginResponse{AccessToken: accessToken, ExpiresIn: int(h.accessTTL.Seconds())}
	resp.User.ID = user.ID.String()
	resp.User.Email = user.Email
	resp.User.FullName = user.FullName
	resp.User.Role = user.RoleName

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "no refresh session")
		return
	}

	tokenHash := authpkg.HashRefreshToken(cookie.Value)
	userID, err := h.users.ValidateAndRotateRefreshToken(r.Context(), tokenHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session expired or invalid, please log in again")
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil || user == nil || !user.IsActive {
		writeError(w, http.StatusUnauthorized, "account no longer active")
		return
	}

	accessToken, err := authpkg.IssueAccessToken(h.accessSecret, h.accessTTL, user.ID, user.RoleName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue token")
		return
	}

	rawRefresh, err := authpkg.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue session")
		return
	}
	expiresAt := time.Now().Add(h.refreshTTL)
	if err := h.users.StoreRefreshToken(r.Context(), user.ID, authpkg.HashRefreshToken(rawRefresh), expiresAt, r.RemoteAddr); err != nil {
		writeError(w, http.StatusInternalServerError, "could not issue session")
		return
	}

	http.SetCookie(w, h.refreshCookie(rawRefresh, expiresAt))
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"expires_in":   int(h.accessTTL.Seconds()),
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		_ = h.users.RevokeRefreshToken(r.Context(), authpkg.HashRefreshToken(cookie.Value))
	}
	http.SetCookie(w, h.expiredRefreshCookie())
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (h *Handler) refreshCookie(value string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    value,
		Path:     "/api/auth",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
	}
}

func (h *Handler) expiredRefreshCookie() *http.Cookie {
	return &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/auth",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
