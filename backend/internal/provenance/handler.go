package provenance

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

type Handler struct{ store *Store }

func NewHandler(store *Store) (*Handler, error) {
	if store == nil { return nil, errors.New("provenance handler requires store") }
	return &Handler{store: store}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/provenance", h.list)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/provenance/verify", h.verify)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { writeErr(w, 401, "unauthorized", "authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil { writeErr(w, 400, "invalid_project_id", "invalid project id"); return }
	items, err := h.store.List(r.Context(), p.UserID, pid)
	if err != nil { writeErr(w, 404, "project_not_found", "project not found"); return }
	writeJSON(w, http.StatusOK, map[string]any{"events": items})
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { writeErr(w, 401, "unauthorized", "authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil { writeErr(w, 400, "invalid_project_id", "invalid project id"); return }
	result, err := h.store.Verify(r.Context(), p.UserID, pid)
	if err != nil { writeErr(w, 404, "project_not_found", "project not found"); return }
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": uuid.NewString()}})
}
