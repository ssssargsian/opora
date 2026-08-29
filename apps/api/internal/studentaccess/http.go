package studentaccess

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/apierror"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseID(w, r, "studentId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	items, err := h.service.List(r.Context(), principal.Actor, studentID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Set(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseID(w, r, "studentId")
	if !ok {
		return
	}
	var body struct {
		UserID uuid.UUID `json:"userId"`
		Grants []string  `json:"grants"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || body.UserID == uuid.Nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.service.Set(r.Context(), principal.Actor, studentID, body.UserID, body.Grants); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseID(w, r, "studentId")
	if !ok {
		return
	}
	userID, ok := parseID(w, r, "userId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.service.Revoke(r.Context(), principal.Actor, studentID, userID); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_id", "Invalid ID")
		return uuid.Nil, false
	}
	return id, true
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, access.ErrPermissionDenied):
		apierror.Write(w, r, http.StatusForbidden, "permission_denied", "Permission denied")
	case errors.Is(err, ErrNotFound):
		apierror.Write(w, r, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, ErrInvalidGrant):
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
	default:
		apierror.Write(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
