
package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

type Handler struct{ store *Store }

func NewHandler(store *Store) (*Handler, error) {
	if store == nil { return nil, fmt.Errorf("events handler requires store") }
	return &Handler{store: store}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/events", h.stream)
}

func parseLastEventID(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" { return 0, nil }
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 0 { return 0, fmt.Errorf("invalid Last-Event-ID") }
	return id, nil
}

func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok { http.Error(w, "unauthorized", http.StatusUnauthorized); return }
	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil { http.Error(w, "invalid project id", http.StatusBadRequest); return }
	owned, err := h.store.ProjectOwned(r.Context(), principal.UserID, projectID)
	if err != nil { http.Error(w, "could not verify project", http.StatusInternalServerError); return }
	if !owned { http.Error(w, "project not found", http.StatusNotFound); return }
	flusher, ok := w.(http.Flusher)
	if !ok { http.Error(w, "streaming is not supported", http.StatusInternalServerError); return }
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	lastID, err := parseLastEventID(r.Header.Get("Last-Event-ID"))
	if err != nil { http.Error(w, "invalid Last-Event-ID", http.StatusBadRequest); return }
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()
	for {
		items, err := h.store.ListSince(r.Context(), principal.UserID, projectID, lastID, 100)
		if err != nil { return }
		for _, item := range items {
			fmt.Fprintf(w, "id: %d\n", item.ID)
			fmt.Fprintf(w, "event: %s\n", item.EventType)
			data, _ := json.Marshal(item)
			fmt.Fprintf(w, "data: %s\n\n", data)
			lastID = item.ID
		}
		if len(items) > 0 { flusher.Flush() }
		select {
		case <-r.Context().Done(): return
		case <-ticker.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
