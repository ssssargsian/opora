package user

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

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
		if errors.Is(err, ErrInvitationDelivery) && created.ID != uuid.Nil {
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, http.StatusCreated, created)
			return
		}
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_id", "Invalid user ID")
		return
	}
	var input CreateInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&input); err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	updated, err := h.service.Update(r.Context(), principal.Actor, userID, input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_id", "Invalid user ID")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	item, err := h.service.ResendInvitation(r.Context(), principal.Actor, userID)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	var input AcceptInvitationInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	accepted, err := h.service.AcceptInvitation(r.Context(), input)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, accepted)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var input ProfileInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	profile, err := h.service.UpdateProfile(r.Context(), principal.Actor, input, requestAudit(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var input PasswordInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.service.ChangePassword(r.Context(), principal.Actor, principal.Session.ID, input, requestAudit(r)); err != nil {
		writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, access.ErrPermissionDenied):
		apierror.Write(w, r, http.StatusForbidden, "permission_denied", "Permission denied")
	case errors.Is(err, ErrConflict):
		apierror.Write(w, r, http.StatusConflict, "user_exists", "User already exists")
	case errors.Is(err, ErrEmailConflict):
		apierror.Write(w, r, http.StatusConflict, "email_exists", "Email already exists")
	case errors.Is(err, ErrNotFound):
		apierror.Write(w, r, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, ErrWrongPassword):
		apierror.Write(w, r, http.StatusUnauthorized, "current_password_invalid", "Current password is invalid")
	case errors.Is(err, ErrInvalidInvitation):
		apierror.Write(w, r, http.StatusBadRequest, "invitation_invalid", "Invitation is invalid")
	case errors.Is(err, ErrExpiredInvitation):
		apierror.Write(w, r, http.StatusGone, "invitation_expired", "Invitation has expired")
	case errors.Is(err, ErrUsedInvitation):
		apierror.Write(w, r, http.StatusConflict, "invitation_used", "Invitation has already been used")
	case errors.Is(err, ErrInvitationDelivery):
		apierror.Write(w, r, http.StatusBadGateway, "invitation_delivery_failed", "User was created, but invitation email was not sent")
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrRoleNotFound):
		apierror.Write(w, r, http.StatusBadRequest, "invalid_request", "Invalid request")
	default:
		apierror.Write(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}

func requestAudit(r *http.Request) RequestAudit {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return RequestAudit{IPAddress: host, UserAgent: strings.TrimSpace(r.UserAgent())}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
