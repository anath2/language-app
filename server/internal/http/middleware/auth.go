package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/anath2/language-app/internal/config"
)

const sessionCookieName = "session"

type sessionPayload struct {
	Authenticated bool  `json:"authenticated"`
	CreatedAtUnix int64 `json:"created_at_unix"`
}

type SessionManager struct {
	secretKey            []byte
	sessionMaxAgeSeconds int
	secureCookies        bool
}

func NewSessionManager(cfg config.Config) *SessionManager {
	return &SessionManager{
		secretKey:            []byte(cfg.AppSecretKey),
		sessionMaxAgeSeconds: cfg.SessionMaxAgeSeconds,
		secureCookies:        cfg.SecureCookies,
	}
}

func (sm *SessionManager) VerifyPassword(input string, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(input), []byte(expected)) == 1
}

func (sm *SessionManager) SetSessionCookie(w http.ResponseWriter, r *http.Request) error {
	payload := sessionPayload{
		Authenticated: true,
		CreatedAtUnix: time.Now().UTC().Unix(),
	}

	token, err := sm.signPayload(payload)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sm.sessionMaxAgeSeconds,
		HttpOnly: true,
		Secure:   sm.cookieShouldBeSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (sm *SessionManager) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   sm.cookieShouldBeSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (sm *SessionManager) VerifySessionFromRequest(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	return sm.verifyToken(cookie.Value)
}

func (sm *SessionManager) signPayload(payload sessionPayload) (string, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadEncoded := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := sm.signature(payloadEncoded)
	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)
	return payloadEncoded + "." + signatureEncoded, nil
}

func (sm *SessionManager) verifyToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	payloadEncoded := parts[0]
	expectedSignature := sm.signature(payloadEncoded)

	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	if subtle.ConstantTimeCompare(providedSignature, expectedSignature) != 1 {
		return false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return false
	}

	var payload sessionPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return false
	}
	if !payload.Authenticated {
		return false
	}

	age := time.Now().UTC().Unix() - payload.CreatedAtUnix
	return age >= 0 && age <= int64(sm.sessionMaxAgeSeconds)
}

func (sm *SessionManager) signature(payloadEncoded string) []byte {
	mac := hmac.New(sha256.New, sm.secretKey)
	mac.Write([]byte(payloadEncoded))
	return mac.Sum(nil)
}

func (sm *SessionManager) cookieShouldBeSecure(r *http.Request) bool {
	if !sm.secureCookies {
		return false
	}
	if r != nil {
		if r.TLS != nil {
			return true
		}
		if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			return true
		}
		host := r.Host
		if h, _, err := net.SplitHostPort(r.Host); err == nil {
			host = h
		}
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return false
		}
	}
	return sm.secureCookies
}

func Auth(cfg config.Config, sessionManager *SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			if path == "/login" || path == "/health" || strings.HasPrefix(path, "/css/") {
				next.ServeHTTP(w, r)
				return
			}

			if sessionManager.VerifySessionFromRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if strings.Contains(r.Header.Get("Accept"), "text/html") {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"detail":"Not authenticated"}`))
		})
	}
}
