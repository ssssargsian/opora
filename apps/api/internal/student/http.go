package student

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/apierror"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p, _ := auth.PrincipalFromContext(r.Context())
	result, err := h.service.List(r.Context(), p.Actor)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": result})
}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "studentId"))
	if err != nil {
		apierror.Write(w, r, 400, "invalid_id", "Invalid student ID")
		return
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	result, err := h.service.Get(r.Context(), p.Actor, id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LastName   string  `json:"lastName"`
		FirstName  string  `json:"firstName"`
		MiddleName *string `json:"middleName"`
		BirthDate  *string `json:"birthDate"`
		ClassName  *string `json:"className"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil {
		apierror.Write(w, r, 400, "invalid_request", "Invalid request")
		return
	}
	var birth *time.Time
	if body.BirthDate != nil {
		parsed, err := time.Parse("2006-01-02", *body.BirthDate)
		if err != nil {
			apierror.Write(w, r, 400, "invalid_request", "Invalid birth date")
			return
		}
		birth = &parsed
	}
	p, _ := auth.PrincipalFromContext(r.Context())
	result, err := h.service.Create(r.Context(), p.Actor, CreateInput{LastName: body.LastName, FirstName: body.FirstName, MiddleName: body.MiddleName, BirthDate: birth, ClassName: body.ClassName})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, access.ErrPermissionDenied):
		apierror.Write(w, r, 403, "permission_denied", "Permission denied")
	case errors.Is(err, ErrNotFound):
		apierror.Write(w, r, 404, "not_found", "Student not found")
	case errors.Is(err, ErrInvalidInput):
		apierror.Write(w, r, 400, "invalid_request", "Invalid request")
	default:
		apierror.Write(w, r, 500, "internal_error", "Internal server error")
	}
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
