package iterations

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type Handler struct {
	store Store
	events *events.Store
	provenance *provenance.Store
}

func NewHandler(store Store) (*Handler, error) { return NewHandlerWithEvents(store, nil) }

func NewHandlerWithEvents(store Store, eventStore *events.Store) (*Handler, error) {
	return NewHandlerWithEventsAndProvenance(store,eventStore,nil)
}
func NewHandlerWithEventsAndProvenance(store Store,eventStore *events.Store,provenanceStore *provenance.Store)(*Handler,error){
	if store==nil{return nil,errors.New("iteration handler requires store")}
	return &Handler{store:store,events:eventStore,provenance:provenanceStore},nil
}
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/iterations", h.list)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/iterations", h.create)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/iterations/{iteration_id}", h.get)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/iterations/{iteration_id}/restore", h.restore)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeIterationError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseUUID(r.PathValue("project_id"))
	if err != nil { writeIterationError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id"); return }
	items, err := h.store.List(r.Context(), userID, projectID)
	if errors.Is(err, ErrIterationNotFound) { writeIterationError(w, http.StatusNotFound, "project_not_found", "project not found"); return }
	if err != nil { writeIterationError(w, http.StatusInternalServerError, "iteration_list_failed", "could not list iterations"); return }
	writeIterationJSON(w, http.StatusOK, map[string]any{"iterations": items})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeIterationError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseUUID(r.PathValue("project_id"))
	if err != nil { writeIterationError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id"); return }
	var input iterationInput
	if err := decodeIterationInput(r, &input); err != nil {
		writeIterationError(w, http.StatusBadRequest, "invalid_request", "invalid iteration payload"); return
	}
	item, err := h.store.Create(r.Context(), userID, projectID, input.ParentIterationID, input.Type, input.Title, input.Description)
	if errors.Is(err, ErrInvalidIteration) { writeIterationError(w, http.StatusBadRequest, "invalid_iteration", "iteration payload is invalid"); return }
	if errors.Is(err, ErrIterationNotFound) { writeIterationError(w, http.StatusNotFound, "iteration_parent_or_project_not_found", "project or parent iteration not found"); return }
	if err != nil { writeIterationError(w, http.StatusInternalServerError, "iteration_create_failed", "could not create iteration"); return }
	h.recordMutation(r,userID,projectID,item.ID,"iteration.created",map[string]any{"type":item.Type,"parent_iteration_id":item.ParentIterationID})
	writeIterationJSON(w, http.StatusCreated, item)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeIterationError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseUUID(r.PathValue("project_id"))
	if err != nil { writeIterationError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id"); return }
	iterationID, err := parseUUID(r.PathValue("iteration_id"))
	if err != nil { writeIterationError(w, http.StatusBadRequest, "invalid_iteration_id", "invalid iteration id"); return }
	item, err := h.store.Get(r.Context(), userID, iterationID)
	if errors.Is(err, ErrIterationNotFound) || item.ProjectID != projectID {
		writeIterationError(w, http.StatusNotFound, "iteration_not_found", "iteration not found"); return
	}
	if err != nil { writeIterationError(w, http.StatusInternalServerError, "iteration_get_failed", "could not load iteration"); return }
	writeIterationJSON(w, http.StatusOK, item)
}

func (h *Handler) restore(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok { writeIterationError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return }
	projectID, err := parseUUID(r.PathValue("project_id"))
	if err != nil { writeIterationError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id"); return }
	iterationID, err := parseUUID(r.PathValue("iteration_id"))
	if err != nil { writeIterationError(w, http.StatusBadRequest, "invalid_iteration_id", "invalid iteration id"); return }
	source, err := h.store.Get(r.Context(), userID, iterationID)
	if errors.Is(err, ErrIterationNotFound) || source.ProjectID != projectID {
		writeIterationError(w, http.StatusNotFound, "iteration_not_found", "iteration not found"); return
	}
	if err != nil { writeIterationError(w, http.StatusInternalServerError, "iteration_get_failed", "could not load iteration"); return }
	item, err := h.store.Restore(r.Context(), userID, iterationID)
	if err != nil { writeIterationError(w, http.StatusInternalServerError, "iteration_restore_failed", "could not restore iteration"); return }
	h.recordMutation(r,userID,projectID,item.ID,"iteration.restored",map[string]any{"parent_iteration_id":iterationID})
	writeIterationJSON(w, http.StatusCreated, item)
}

type iterationInput struct {
	ParentIterationID *uuid.UUID `json:"parent_iteration_id,omitempty"`
	Type Type `json:"type"`
	Title *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

func decodeIterationInput(r *http.Request, input *iterationInput) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil { return err }
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF { return errors.New("request body must contain one JSON object") }
	return nil
}

func currentUserID(r *http.Request) (uuid.UUID, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok { return uuid.Nil, false }
	return principal.UserID, true
}

func parseUUID(value string) (uuid.UUID, error) { return uuid.Parse(value) }

func (h *Handler) recordMutation(r *http.Request,userID,projectID,entityID uuid.UUID,action string,payload map[string]any){
	if h.events!=nil{_,_=h.events.Append(r.Context(),userID,projectID,action,"iteration",entityID,payload)}
	if h.provenance!=nil{_,_=h.provenance.Append(r.Context(),provenance.Event{UserID:userID,ProjectID:projectID,IterationID:&entityID,EntityType:"iteration",EntityID:entityID,Action:action,Payload:payload})}
	if h.events!=nil&&h.provenance!=nil{_,_=h.events.Append(r.Context(),userID,projectID,"provenance.updated","iteration",entityID,map[string]any{"action":action})}
}

func writeIterationJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeIterationError(w http.ResponseWriter, status int, code, message string) {
	writeIterationJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": uuid.NewString()}})
}
