package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"opora.local/api/internal/config"
	"opora.local/api/internal/platform/apierror"
)

type principalKey struct{}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

type Handler struct {
	service *Service
	cfg     config.Auth
	limiter *loginLimiter
}

func NewHandler(service *Service, cfg config.Auth) *Handler {
	return &Handler{service: service, cfg: cfg, limiter: newLoginLimiter(5, 15*time.Minute)}
}

func (h *Handler) OptionalSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(h.cfg.SessionCookieName)
		if err == nil {
			principal, authErr := h.service.Authenticate(r.Context(), cookie.Value)
			if authErr == nil {
				r = r.WithContext(context.WithValue(r.Context(), principalKey{}, principal))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFromContext(r.Context()); !ok {
			apierror.Write(w, r, http.StatusUnauthorized, "authentication_required", "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) RequireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		csrfCookie, err := r.Cookie(h.cfg.CSRFCookieName)
		header := r.Header.Get("X-CSRF-Token")
		if !ok || err != nil || header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(csrfCookie.Value)) != 1 || h.service.ValidateCSRF(principal, header) != nil {
			apierror.Write(w, r, http.StatusForbidden, "csrf_invalid", "Request verification failed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.limiter.Allow(ip, time.Now()) {
		apierror.Write(w, r, http.StatusTooManyRequests, "rate_limited", "Too many login attempts")
		return
	}
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.Email == "" || input.Password == "" {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	result, err := h.service.Login(r.Context(), input.Email, input.Password)
	if err != nil {
		if errors.Is(err, ErrCredentials) {
			apierror.Write(w, r, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
			return
		}
		apierror.Write(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	h.limiter.Reset(ip)
	h.setCookies(w, result)
	writeMe(w, result.Principal)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p, ok := PrincipalFromContext(r.Context())
	if !ok {
		apierror.Write(w, r, http.StatusUnauthorized, "authentication_required", "Authentication required")
		return
	}
	writeMe(w, p)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFromContext(r.Context())
	if err := h.service.Logout(r.Context(), p); err != nil {
		apierror.Write(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	h.clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setCookies(w http.ResponseWriter, result LoginResult) {
	maxAge := int(time.Until(result.Principal.Session.ExpiresAt).Seconds())
	// #nosec G124 -- all security attributes are explicit; Secure is environment-controlled.
	http.SetCookie(w, &http.Cookie{Name: h.cfg.SessionCookieName, Value: result.SessionToken, Path: "/", MaxAge: maxAge,
		Expires: result.Principal.Session.ExpiresAt, HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	// #nosec G124 -- the double-submit CSRF token must be readable by the same-origin frontend.
	http.SetCookie(w, &http.Cookie{Name: h.cfg.CSRFCookieName, Value: result.CSRFToken, Path: "/", MaxAge: maxAge,
		Expires: result.Principal.Session.ExpiresAt, HttpOnly: false, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func (h *Handler) clearCookies(w http.ResponseWriter) {
	for _, cookie := range []struct {
		name     string
		httpOnly bool
	}{{h.cfg.SessionCookieName, true}, {h.cfg.CSRFCookieName, false}} {
		// #nosec G124 -- deletion cookies preserve the configured security attributes.
		http.SetCookie(w, &http.Cookie{Name: cookie.name, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0),
			HttpOnly: cookie.httpOnly, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	}
}

func writeMe(w http.ResponseWriter, p Principal) {
	permissions := make([]string, 0, len(p.Actor.Permissions))
	for permission := range p.Actor.Permissions {
		permissions = append(permissions, string(permission))
	}
	sort.Strings(permissions)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": p.Session.UserID, "email": p.Email, "displayName": p.DisplayName,
		"lastName": p.LastName, "firstName": p.FirstName, "middleName": p.MiddleName,
		"organization": map[string]any{"id": p.Session.OrganizationID, "name": p.OrganizationName},
		"permissions":  permissions,
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type loginAttempt struct {
	count   int
	started time.Time
}
type loginLimiter struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	attempts map[string]loginAttempt
}

func newLoginLimiter(limit int, window time.Duration) *loginLimiter {
	return &loginLimiter{max: limit, window: window, attempts: make(map[string]loginAttempt)}
}
func (l *loginLimiter) Allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	a := l.attempts[key]
	if a.started.IsZero() || now.Sub(a.started) >= l.window {
		a = loginAttempt{started: now}
	}
	a.count++
	l.attempts[key] = a
	return a.count <= l.max
}
func (l *loginLimiter) Reset(key string) { l.mu.Lock(); delete(l.attempts, key); l.mu.Unlock() }

func OriginAllowed(r *http.Request, allowed string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	return origin == "" || origin == allowed
}
