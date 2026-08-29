package user

import (
	"encoding/json"
	"errors"
	"net/http"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/apierror"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Roles(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	roles, err := h.service.Roles(r.Context(), principal.Actor)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": roles})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	users, err := h.service.List(r.Context(), principal.Actor)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	created, err := h.service.Create(r.Context(), principal.Actor, input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, created)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, access.ErrPermissionDenied):
		apierror.Write(w, r, http.StatusForbidden, "permission_denied", "Permission denied")
	case errors.Is(err, ErrConflict):
		apierror.Write(w, r, http.StatusConflict, "user_exists", "User already exists")
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrRoleNotFound):
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
