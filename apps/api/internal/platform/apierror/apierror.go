// Package apierror writes the stable public API error envelope.
package apierror

import (
	"encoding/json"
	"net/http"

	"opora.local/api/internal/platform/requestid"
)

type envelope struct {
	Error detail `json:"error"`
}

type detail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId,omitempty"`
}

// Write sends a safe error without exposing internal implementation details.
func Write(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: detail{
		Code: code, Message: message, RequestID: requestid.FromContext(r.Context()),
	}})
}
