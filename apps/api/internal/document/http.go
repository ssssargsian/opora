package document

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"opora.local/api/internal/access"
	"opora.local/api/internal/auth"
	"opora.local/api/internal/platform/apierror"
)

type Handler struct {
	documents  *Service
	onlyOffice *OnlyOfficeService
	maxBytes   int64
	logger     *slog.Logger
}

func NewHandler(documents *Service, onlyOffice *OnlyOfficeService, maxBytes int64, logger *slog.Logger) *Handler {
	return &Handler{documents: documents, onlyOffice: onlyOffice, maxBytes: maxBytes, logger: logger}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseID(w, r, "studentId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	documents, err := h.documents.List(r.Context(), principal.Actor, studentID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": mapDocuments(documents)})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	documentID, ok := parseID(w, r, "documentId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	document, err := h.documents.Get(r.Context(), principal.Actor, documentID, requestAudit(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapDocument(document))
}

func (h *Handler) Versions(w http.ResponseWriter, r *http.Request) {
	documentID, ok := parseID(w, r, "documentId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	versions, err := h.documents.Versions(r.Context(), principal.Actor, documentID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": mapVersions(versions)})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	studentID, ok := parseID(w, r, "studentId")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+1024*1024)
	// #nosec G120 -- MaxBytesReader bounds the complete request before multipart parsing.
	if err := r.ParseMultipartForm(h.maxBytes + 1024*1024); err != nil {
		apierror.Write(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Maximum file size is 25 MB")
		return
	}
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "file_required", "File is required")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, h.maxBytes+1))
	if err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_file", "File could not be read")
		return
	}
	if int64(len(data)) > h.maxBytes {
		apierror.Write(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Maximum file size is 25 MB")
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	documentType := optionalForm(r.FormValue("documentType"))
	document, err := h.documents.Upload(r.Context(), principal.Actor, studentID, UploadInput{
		Title: r.FormValue("title"), DocumentType: documentType,
		ConfidentialityLevel: r.FormValue("confidentialityLevel"), OriginalFilename: header.Filename,
		Bytes: data, Audit: requestAudit(r),
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.logger.InfoContext(r.Context(), "document uploaded", "organization_id", principal.Actor.OrganizationID, "student_id", studentID, "document_id", document.ID, "version", document.VersionNumber)
	writeJSON(w, http.StatusCreated, mapDocument(document))
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	h.download(w, r, uuid.Nil)
}

func (h *Handler) DownloadVersion(w http.ResponseWriter, r *http.Request) {
	versionID, ok := parseID(w, r, "versionId")
	if !ok {
		return
	}
	h.download(w, r, versionID)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	documentID, ok := parseID(w, r, "documentId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	_, version, reader, err := h.documents.OpenPDFPreview(r.Context(), principal.Actor, documentID, requestAudit(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	defer func() { _ = reader.Close() }()
	w.Header().Set("Content-Type", MIMEPDF)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", version.Size))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": version.OriginalFilename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, reader)
}

func (h *Handler) download(w http.ResponseWriter, r *http.Request, versionID uuid.UUID) {
	documentID, ok := parseID(w, r, "documentId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	var document Document
	var version Version
	var reader io.ReadCloser
	var err error
	if versionID == uuid.Nil {
		document, version, reader, err = h.documents.OpenCurrent(r.Context(), principal.Actor, documentID, requestAudit(r))
	} else {
		document, version, reader, err = h.documents.OpenVersion(r.Context(), principal.Actor, documentID, versionID, requestAudit(r))
	}
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	defer func() { _ = reader.Close() }()
	w.Header().Set("Content-Type", version.MIMEType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", version.Size))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": version.OriginalFilename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = document
	if _, err := io.Copy(w, reader); err != nil {
		return
	}
}

func (h *Handler) Editor(w http.ResponseWriter, r *http.Request) {
	documentID, ok := parseID(w, r, "documentId")
	if !ok {
		return
	}
	principal, _ := auth.PrincipalFromContext(r.Context())
	config, err := h.onlyOffice.Editor(r.Context(), principal.Actor, documentID, requestAudit(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) InternalFile(w http.ResponseWriter, r *http.Request) {
	documentID, ok := parseID(w, r, "documentId")
	if !ok {
		return
	}
	versionID, ok := parseID(w, r, "versionId")
	if !ok {
		return
	}
	organizationID, err := h.onlyOffice.VerifyFileToken(r.URL.Query().Get("token"), documentID, versionID)
	if err != nil {
		apierror.Write(w, r, http.StatusUnauthorized, "invalid_token", "Invalid token")
		return
	}
	version, reader, err := h.documents.OpenInternalVersion(r.Context(), organizationID, documentID, versionID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	defer func() { _ = reader.Close() }()
	w.Header().Set("Content-Type", version.MIMEType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", version.Size))
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, reader)
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	documentID, ok := parseID(w, r, "documentId")
	if !ok {
		return
	}
	claims, err := h.onlyOffice.VerifyCallbackToken(r.URL.Query().Get("token"), documentID)
	if err != nil || h.onlyOffice.VerifyDocumentServerToken(r.Header.Get("AuthorizationJwt")) != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]int{"error": 1})
		return
	}
	if h.onlyOffice.AuthorizeCallback(r.Context(), claims) != nil {
		writeJSON(w, http.StatusForbidden, map[string]int{"error": 1})
		return
	}
	var callback struct {
		Status int    `json:"status"`
		URL    string `json:"url"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024))
	if decoder.Decode(&callback) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]int{"error": 1})
		return
	}
	if callback.Status != 2 && callback.Status != 6 {
		writeJSON(w, http.StatusOK, map[string]int{"error": 0})
		return
	}
	data, err := h.onlyOffice.FetchEdited(r.Context(), callback.URL)
	if err == nil {
		_, err = h.documents.SaveEditedVersion(r.Context(), claims.OrganizationID, claims.ActorID, documentID, claims.VersionID, data, requestAudit(r))
	}
	if err != nil {
		h.logger.WarnContext(r.Context(), "ONLYOFFICE save rejected", "document_id", documentID, "status", callback.Status, "error_type", safeErrorType(err))
		writeJSON(w, http.StatusOK, map[string]int{"error": 1})
		return
	}
	h.logger.InfoContext(r.Context(), "ONLYOFFICE version saved", "organization_id", claims.OrganizationID, "document_id", documentID)
	writeJSON(w, http.StatusOK, map[string]int{"error": 0})
}

func safeErrorType(err error) string {
	switch {
	case errors.Is(err, ErrVersionConflict):
		return "version_conflict"
	case errors.Is(err, ErrFileTooLarge):
		return "file_too_large"
	case errors.Is(err, ErrUnsupportedFile):
		return "unsupported_file"
	case errors.Is(err, ErrMalwareDetected):
		return "unsafe_file"
	default:
		return "integration_error"
	}
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, access.ErrPermissionDenied):
		apierror.Write(w, r, http.StatusForbidden, "permission_denied", "Permission denied")
	case errors.Is(err, ErrStudentNotFound), errors.Is(err, ErrNotFound):
		apierror.Write(w, r, http.StatusNotFound, "not_found", "Resource not found")
	case errors.Is(err, ErrFileTooLarge):
		apierror.Write(w, r, http.StatusRequestEntityTooLarge, "file_too_large", "Maximum file size is 25 MB")
	case errors.Is(err, ErrUnsupportedFile):
		apierror.Write(w, r, http.StatusUnsupportedMediaType, "unsupported_file", "Unsupported file format")
	case errors.Is(err, ErrMalwareDetected):
		apierror.Write(w, r, http.StatusUnprocessableEntity, "unsafe_file", "File failed security validation")
	case errors.Is(err, ErrVersionConflict):
		apierror.Write(w, r, http.StatusConflict, "version_conflict", "Document has a newer version")
	default:
		apierror.Write(w, r, http.StatusServiceUnavailable, "document_service_unavailable", "Document service is unavailable")
	}
}

func mapDocuments(documents []Document) []map[string]any {
	result := make([]map[string]any, 0, len(documents))
	for _, document := range documents {
		result = append(result, mapDocument(document))
	}
	return result
}

func mapDocument(document Document) map[string]any {
	return map[string]any{
		"id": document.ID, "studentId": document.StudentID, "title": document.Title,
		"documentType": document.DocumentType, "confidentialityLevel": document.ConfidentialityLevel,
		"currentVersion": map[string]any{"id": document.CurrentVersionID, "versionNumber": document.VersionNumber,
			"originalFilename": document.OriginalFilename, "mimeType": document.MIMEType, "size": document.Size,
			"sha256": document.SHA256, "changedBy": document.ChangedBy, "createdAt": document.UpdatedAt},
		"createdAt": document.CreatedAt, "updatedAt": document.UpdatedAt,
	}
}

func mapVersions(versions []Version) []map[string]any {
	result := make([]map[string]any, 0, len(versions))
	for _, version := range versions {
		result = append(result, map[string]any{"id": version.ID, "versionNumber": version.VersionNumber,
			"originalFilename": version.OriginalFilename, "mimeType": version.MIMEType, "size": version.Size,
			"sha256": version.SHA256, "changedBy": version.ChangedBy, "createdAt": version.CreatedAt})
	}
	return result
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		apierror.Write(w, r, http.StatusBadRequest, "invalid_id", "Invalid resource ID")
		return uuid.Nil, false
	}
	return id, true
}

func requestAudit(r *http.Request) AuditContext {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = ""
	}
	return AuditContext{IPAddress: host, UserAgent: strings.TrimSpace(r.UserAgent())}
}

func optionalForm(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
