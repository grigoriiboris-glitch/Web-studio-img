package projects

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

type Handler struct {
	store Store
}

func NewHandler(store Store) (*Handler, error) {
	if store == nil {
		return nil, errors.New("project handler requires store")
	}
	return &Handler{store: store}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects", h.list)
	mux.HandleFunc("POST /api/v1/projects", h.create)
	mux.HandleFunc("GET /api/v1/projects/{project_id}", h.get)
	mux.HandleFunc("PATCH /api/v1/projects/{project_id}", h.update)
	mux.HandleFunc("DELETE /api/v1/projects/{project_id}", h.archive)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	items, err := h.store.List(r.Context(), userID)
	if err != nil {
		writeProjectError(w, http.StatusInternalServerError, "project_list_failed", "could not list projects")
		return
	}
	writeProjectJSON(w, http.StatusOK, map[string]any{"projects": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var input projectInput
	if err := decodeProjectInput(r, &input); err != nil {
		writeProjectError(w, http.StatusBadRequest, "invalid_request", "invalid project payload")
		return
	}
	project, err := h.store.Create(r.Context(), userID, input.Name, input.Description)
	if errors.Is(err, ErrInvalidProject) {
		writeProjectError(w, http.StatusBadRequest, "invalid_project", "project name is invalid")
		return
	}
	if err != nil {
		writeProjectError(w, http.StatusInternalServerError, "project_create_failed", "could not create project")
		return
	}
	writeProjectJSON(w, http.StatusCreated, project)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id")
		return
	}
	project, err := h.store.Get(r.Context(), userID, projectID)
	if errors.Is(err, ErrProjectNotFound) {
		writeProjectError(w, http.StatusNotFound, "project_not_found", "project not found")
		return
	}
	if err != nil {
		writeProjectError(w, http.StatusInternalServerError, "project_get_failed", "could not load project")
		return
	}
	writeProjectJSON(w, http.StatusOK, project)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id")
		return
	}
	var input projectInput
	if err := decodeProjectInput(r, &input); err != nil || input.Status == "" {
		writeProjectError(w, http.StatusBadRequest, "invalid_request", "invalid project payload")
		return
	}
	project, err := h.store.Update(r.Context(), userID, projectID, input.Name, input.Description, Status(input.Status))
	if errors.Is(err, ErrInvalidProject) {
		writeProjectError(w, http.StatusBadRequest, "invalid_project", "project payload is invalid")
		return
	}
	if errors.Is(err, ErrProjectNotFound) {
		writeProjectError(w, http.StatusNotFound, "project_not_found", "project not found")
		return
	}
	if err != nil {
		writeProjectError(w, http.StatusInternalServerError, "project_update_failed", "could not update project")
		return
	}
	writeProjectJSON(w, http.StatusOK, project)
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeProjectError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		writeProjectError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id")
		return
	}
	project, err := h.store.Archive(r.Context(), userID, projectID)
	if errors.Is(err, ErrProjectNotFound) {
		writeProjectError(w, http.StatusNotFound, "project_not_found", "project not found")
		return
	}
	if err != nil {
		writeProjectError(w, http.StatusInternalServerError, "project_archive_failed", "could not archive project")
		return
	}
	writeProjectJSON(w, http.StatusOK, project)
}

type projectInput struct {
	Name string `json:"name"`
	Description *string `json:"description"`
	Status string `json:"status,omitempty"`
}

func decodeProjectInput(r *http.Request, input *projectInput) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func currentUserID(r *http.Request) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	return principal.UserID, true
}

func writeProjectJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProjectError(w http.ResponseWriter, status int, code, message string) {
	writeProjectJSON(w, status, map[string]any{
		"error": map[string]string{
			"code": code,
			"message": message,\n\t\t\t"request_id": uuid.NewString(),
		},
	})
}
