package organization

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/apierror"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	result, err := h.service.Update(r.Context(), principal.Actor, input.Name, AuditContext{IPAddress: host, UserAgent: strings.TrimSpace(r.UserAgent())})
	if err != nil {
		switch {
		case errors.Is(err, access.ErrPermissionDenied):
			apierror.Write(w, r, http.StatusForbidden, "permission_denied", "Permission denied")
		case errors.Is(err, ErrInvalidInput):
			apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		default:
			apierror.Write(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}
