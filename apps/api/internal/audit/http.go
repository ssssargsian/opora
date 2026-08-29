package audit

import (
	"encoding/json"
	"net/http"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/apierror"
)

type Handler struct {
	repository    *Repository
	authorization access.AuthorizationService
}

func NewHandler(repository *Repository) *Handler { return &Handler{repository: repository} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	principal, _ := auth.PrincipalFromContext(r.Context())
	if err := h.authorization.Can(r.Context(), principal.Actor, access.AuditView,
		access.Resource{OrganizationID: principal.Actor.OrganizationID}); err != nil {
		apierror.Write(w, r, http.StatusForbidden, "permission_denied", "Permission denied")
		return
	}
	items, err := h.repository.List(r.Context(), principal.Actor.OrganizationID, 200)
	if err != nil {
		apierror.Write(w, r, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
}
