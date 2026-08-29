// Package requestid creates server-controlled request correlation identifiers.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type contextKey struct{}

// Middleware attaches a fresh request ID to the response and request context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newID()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), contextKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the server-generated request ID, if present.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}

func newID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("cryptographic random source unavailable")
	}
	return hex.EncodeToString(value[:])
}
