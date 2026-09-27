package humanactions

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
	store *Store
	events *events.Store
	provenance *provenance.Store
}

func NewHandler(store *Store, ev *events.Store, pv *provenance.Store) (*Handler, error) {
	if store == nil { return nil, errors.New("human actions handler requires store") }
	return &Handler{store: store, events: ev, provenance: pv}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/human-actions", h.list)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/human-actions", h.create)
}

func userID(r *http.Request) (uuid.UUID, bool) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok { return uuid.Nil, false }
	return p.UserID, true
}

func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil { return err }
	var extra any
	if err := d.Decode(&extra); err != io.EOF { return errors.New("multiple JSON values") }
	return nil
}

func writeJSON(w http.ResponseWriter, s int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, s int, c, m string) {
	writeJSON(w, s, map[string]any{"error": map[string]string{"code": c, "message": m, "request_id": uuid.NewString()}})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	u, ok := userID(r)
	if !ok { writeErr(w, 401, "unauthorized", "authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil { writeErr(w, 400, "invalid_project_id", "invalid project id"); return }
	var in Request
	if err := decode(r, &in); err != nil { writeErr(w, 400, "invalid_request", "invalid human action payload"); return }
	item, err := h.store.Create(r.Context(), u, pid, in)
	if errors.Is(err, ErrInvalidAction) { writeErr(w, 400, "invalid_human_action", "human action is invalid"); return }
	if err != nil { writeErr(w, 500, "human_action_create_failed", "could not create human action"); return }
	if h.events != nil { _, _ = h.events.Append(r.Context(), u, pid, "human_action.created", "human_action", item.ID, map[string]any{"action_type":in.ActionType,"payload":in.Payload,"iteration_id":in.IterationID}) }
	if h.provenance != nil { _, _ = h.provenance.Append(r.Context(), provenance.Event{UserID:u, ProjectID:pid, IterationID:in.IterationID, EntityType:"human_action", EntityID:item.ID, Action:in.ActionType, Payload:in.Payload, CreatedAt: item.CreatedAt}) }
	writeJSON(w, 201, item)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	u, ok := userID(r)
	if !ok { writeErr(w, 401, "unauthorized", "authentication required"); return }
	pid, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil { writeErr(w, 400, "invalid_project_id", "invalid project id"); return }
	items, err := h.store.List(r.Context(), u, pid)
	if err != nil { writeErr(w, 500, "human_action_list_failed", "could not list human actions"); return }
	writeJSON(w, 200, map[string]any{"actions": items})
}
