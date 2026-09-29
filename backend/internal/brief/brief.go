
package brief

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/oleg3190/Web-studio-img/backend/internal/auth"
	"github.com/oleg3190/Web-studio-img/backend/internal/events"
	"github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
	"github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

var (
	ErrInvalidBrief  = errors.New("invalid creative brief")
	ErrBriefNotFound = errors.New("creative brief not found")
)

type Brief struct {
	ID               uuid.UUID  `json:"id"`
	ProjectID        uuid.UUID  `json:"project_id"`
	UserID           uuid.UUID  `json:"user_id"`
	Version          int        `json:"version"`
	Title            string     `json:"title"`
	Goal             string     `json:"goal"`
	Audience         *string    `json:"audience,omitempty"`
	Deliverable      *string    `json:"deliverable,omitempty"`
	AspectRatio      *string    `json:"aspect_ratio,omitempty"`
	TargetWidth      *int       `json:"target_width,omitempty"`
	TargetHeight     *int       `json:"target_height,omitempty"`
	Subject          *string    `json:"subject,omitempty"`
	MustHave         []string   `json:"must_have"`
	Avoid            []string   `json:"avoid"`
	Mood             *string    `json:"mood,omitempty"`
	RequiredElements []string   `json:"required_elements"`
	Constraints      []string   `json:"constraints"`
	SuccessCriteria  []string   `json:"success_criteria"`
	Deadline         *time.Time `json:"deadline,omitempty"`
	Status            string     `json:"status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Input struct {
	Title            string     `json:"title"`
	Goal             string     `json:"goal"`
	Audience         *string    `json:"audience,omitempty"`
	Deliverable      *string    `json:"deliverable,omitempty"`
	AspectRatio      *string    `json:"aspect_ratio,omitempty"`
	TargetWidth      *int       `json:"target_width,omitempty"`
	TargetHeight     *int       `json:"target_height,omitempty"`
	Subject          *string    `json:"subject,omitempty"`
	MustHave         []string   `json:"must_have,omitempty"`
	Avoid            []string   `json:"avoid,omitempty"`
	Mood             *string    `json:"mood,omitempty"`
	RequiredElements []string   `json:"required_elements,omitempty"`
	Constraints      []string   `json:"constraints,omitempty"`
	SuccessCriteria  []string   `json:"success_criteria,omitempty"`
	Deadline         *time.Time `json:"deadline,omitempty"`
}

type Handler struct {
	db         *sql.DB
	actions    *humanactions.Store
	provenance *provenance.Store
	events     *events.Store
}

func NewHandler(db *sql.DB, actions *humanactions.Store, provenanceStore *provenance.Store, eventStore *events.Store) (*Handler, error) {
	if db == nil {
		return nil, errors.New("creative brief handler requires database")
	}
	return &Handler{db: db, actions: actions, provenance: provenanceStore, events: eventStore}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{project_id}/brief", h.current)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/brief/versions", h.list)
	mux.HandleFunc("GET /api/v1/projects/{project_id}/brief/approved", h.approved)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/brief", h.create)
	mux.HandleFunc("POST /api/v1/projects/{project_id}/brief/{brief_id}/approve", h.approve)
}

func (h *Handler) current(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := h.authProject(w, r)
	if !ok {
		return
	}
	item, err := h.loadCurrent(r.Context(), userID, projectID)
	if errors.Is(err, ErrBriefNotFound) {
		writeError(w, http.StatusNotFound, "brief_not_found", "creative brief not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "brief_get_failed", "could not load creative brief")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) approved(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := h.authProject(w, r)
	if !ok {
		return
	}
	row := h.db.QueryRowContext(r.Context(), `
		SELECT id,project_id,user_id,version,title,goal,audience,deliverable,aspect_ratio,target_width,target_height,
		       subject,must_have,avoid,mood,required_elements,constraints,success_criteria,deadline,status,created_at,updated_at
		FROM creative_briefs
		WHERE project_id=$1 AND user_id=$2 AND status='approved'
		LIMIT 1
	`, projectID, userID)
	item, err := scanBrief(row)
	if errors.Is(err, ErrBriefNotFound) {
		writeError(w, http.StatusNotFound, "approved_brief_not_found", "approved creative brief not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "brief_get_failed", "could not load approved creative brief")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := h.authProject(w, r)
	if !ok {
		return
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT id, project_id, user_id, version, title, goal, audience, deliverable, aspect_ratio,
		       target_width, target_height, subject, must_have, avoid, mood, required_elements,
		       constraints, success_criteria, deadline, status, created_at, updated_at
		FROM creative_briefs
		WHERE project_id=$1 AND user_id=$2
		ORDER BY version DESC
	`, projectID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "brief_list_failed", "could not list creative briefs")
		return
	}
	defer func() { _ = rows.Close() }()

	out := make([]Brief, 0)
	for rows.Next() {
		item, scanErr := scanBrief(rows)
		if scanErr != nil {
			writeError(w, http.StatusInternalServerError, "brief_list_failed", "could not read creative brief")
			return
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "brief_list_failed", "could not read creative briefs")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"briefs": out})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := h.authProject(w, r)
	if !ok {
		return
	}
	var input Input
	if err := decode(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid creative brief payload")
		return
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_brief", err.Error())
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "brief_create_failed", "could not start creative brief transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", "creative-briefs:"+projectID.String()); err != nil {
		writeError(w, http.StatusInternalServerError, "brief_create_failed", "could not lock creative brief version")
		return
	}

	var version int
	if err := tx.QueryRowContext(r.Context(), `
		SELECT COALESCE(MAX(version), 0) + 1 FROM creative_briefs
		WHERE project_id=$1 AND user_id=$2
	`, projectID, userID).Scan(&version); err != nil {
		writeError(w, http.StatusInternalServerError, "brief_create_failed", "could not allocate creative brief version")
		return
	}

	var item Brief
	var rawMust, rawAvoid, rawRequired, rawConstraints, rawSuccess []byte
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO creative_briefs(
			project_id,user_id,version,title,goal,audience,deliverable,aspect_ratio,target_width,target_height,
			subject,must_have,avoid,mood,required_elements,constraints,success_criteria,deadline
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13::jsonb,$14,$15::jsonb,$16::jsonb,$17::jsonb,$18)
		RETURNING id,project_id,user_id,version,title,goal,audience,deliverable,aspect_ratio,target_width,target_height,
		          subject,must_have,avoid,mood,required_elements,constraints,success_criteria,deadline,status,created_at,updated_at
	`, projectID, userID, version, strings.TrimSpace(input.Title), strings.TrimSpace(input.Goal), input.Audience,
		input.Deliverable, input.AspectRatio, input.TargetWidth, input.TargetHeight, input.Subject,
		mustJSON(input.MustHave), mustJSON(input.Avoid), input.Mood, mustJSON(input.RequiredElements),
		mustJSON(input.Constraints), mustJSON(input.SuccessCriteria), input.Deadline,
	).Scan(briefScanArgs(&item, &rawMust, &rawAvoid, &rawRequired, &rawConstraints, &rawSuccess)...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "brief_create_failed", "could not create creative brief")
		return
	}
	decodeArrays(&item, rawMust, rawAvoid, rawRequired, rawConstraints, rawSuccess)

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "brief_create_failed", "could not commit creative brief")
		return
	}
	h.audit(r.Context(), userID, projectID, item.ID, "CREATIVE_BRIEF_CREATED", map[string]any{"brief_id": item.ID, "version": item.Version, "title": item.Title})
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) approve(w http.ResponseWriter, r *http.Request) {
	userID, projectID, ok := h.authProject(w, r)
	if !ok {
		return
	}
	briefID, err := uuid.Parse(r.PathValue("brief_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_brief_id", "invalid brief id")
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "brief_approve_failed", "could not start approval transaction")
		return
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(r.Context(), "SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", "creative-briefs:"+projectID.String()); err != nil {
		writeError(w, http.StatusInternalServerError, "brief_approve_failed", "could not lock creative brief")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE creative_briefs
		SET status='draft', updated_at=updated_at
		WHERE project_id=$1 AND user_id=$2 AND status='approved'
	`, projectID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "brief_approve_failed", "could not reset previous approval")
		return
	}

	var item Brief
	var rawMust, rawAvoid, rawRequired, rawConstraints, rawSuccess []byte
	err = tx.QueryRowContext(r.Context(), `
		UPDATE creative_briefs
		SET status='approved', updated_at=now()
		WHERE id=$1 AND project_id=$2 AND user_id=$3
		RETURNING id,project_id,user_id,version,title,goal,audience,deliverable,aspect_ratio,target_width,target_height,
		          subject,must_have,avoid,mood,required_elements,constraints,success_criteria,deadline,status,created_at,updated_at
	`, briefID, projectID, userID).Scan(briefScanArgs(&item, &rawMust, &rawAvoid, &rawRequired, &rawConstraints, &rawSuccess)...)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "brief_not_found", "creative brief not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "brief_approve_failed", "could not approve creative brief")
		return
	}
	decodeArrays(&item, rawMust, rawAvoid, rawRequired, rawConstraints, rawSuccess)
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "brief_approve_failed", "could not commit approval")
		return
	}

	h.audit(r.Context(), userID, projectID, item.ID, "CREATIVE_BRIEF_APPROVED", map[string]any{"brief_id": item.ID, "version": item.Version})
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) loadCurrent(ctx context.Context, userID, projectID uuid.UUID) (Brief, error) {
	row := h.db.QueryRowContext(ctx, `
		SELECT id,project_id,user_id,version,title,goal,audience,deliverable,aspect_ratio,target_width,target_height,
		       subject,must_have,avoid,mood,required_elements,constraints,success_criteria,deadline,status,created_at,updated_at
		FROM creative_briefs
		WHERE project_id=$1 AND user_id=$2
		ORDER BY version DESC
		LIMIT 1
	`, projectID, userID)
	return scanBrief(row)
}

func (h *Handler) authProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	principal, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return uuid.Nil, uuid.Nil, false
	}
	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id")
		return uuid.Nil, uuid.Nil, false
	}
	var exists bool
	if err := h.db.QueryRowContext(r.Context(),
		"SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')",
		projectID, principal.UserID,
	).Scan(&exists); err != nil || !exists {
		writeError(w, http.StatusNotFound, "project_not_found", "project not found")
		return uuid.Nil, uuid.Nil, false
	}
	return principal.UserID, projectID, true
}

func (h *Handler) audit(ctx context.Context, userID, projectID, entityID uuid.UUID, action string, payload map[string]any) {
	if h.actions != nil {
		_, _ = h.actions.Create(ctx, userID, projectID, humanactions.Request{ActionType: action, Payload: payload, NewState: payload})
	}
	if h.provenance != nil {
		_, _ = h.provenance.Append(ctx, provenance.Event{
			UserID: userID, ProjectID: projectID, EntityType: "creative_brief",
			EntityID: entityID, Action: strings.ToLower(action), Payload: payload,
		})
	}
	if h.events != nil {
		_, _ = h.events.Append(ctx, userID, projectID, strings.ToLower(action), "creative_brief", entityID, payload)
	}
}

func (in Input) Validate() error {
	if strings.TrimSpace(in.Title) == "" || len([]rune(in.Title)) > 200 {
		return fmt.Errorf("%w: title is required and must be <= 200 characters", ErrInvalidBrief)
	}
	if strings.TrimSpace(in.Goal) == "" || len([]rune(in.Goal)) > 5000 {
		return fmt.Errorf("%w: goal is required and must be <= 5000 characters", ErrInvalidBrief)
	}
	for _, field := range []*string{in.Audience, in.Deliverable, in.AspectRatio, in.Subject, in.Mood} {
		if field != nil && len([]rune(*field)) > 1000 {
			return fmt.Errorf("%w: text field is too long", ErrInvalidBrief)
		}
	}
	for name, values := range map[string][]string{
		"must_have": in.MustHave, "avoid": in.Avoid, "required_elements": in.RequiredElements,
		"constraints": in.Constraints, "success_criteria": in.SuccessCriteria,
	} {
		if len(values) > 50 {
			return fmt.Errorf("%w: %s contains too many items", ErrInvalidBrief, name)
		}
		for _, value := range values {
			if strings.TrimSpace(value) == "" || len([]rune(value)) > 500 {
				return fmt.Errorf("%w: %s contains an invalid item", ErrInvalidBrief, name)
			}
		}
	}
	if in.AspectRatio != nil && strings.TrimSpace(*in.AspectRatio) != "" {
		parts := strings.Split(*in.AspectRatio, ":")
		if len(parts) != 2 || !validPositiveNumber(parts[0]) || !validPositiveNumber(parts[1]) {
			return fmt.Errorf("%w: aspect_ratio must use W:H format", ErrInvalidBrief)
		}
	}
	if in.TargetWidth != nil && (*in.TargetWidth < 1 || *in.TargetWidth > 100000) {
		return fmt.Errorf("%w: target_width is out of range", ErrInvalidBrief)
	}
	if in.TargetHeight != nil && (*in.TargetHeight < 1 || *in.TargetHeight > 100000) {
		return fmt.Errorf("%w: target_height is out of range", ErrInvalidBrief)
	}
	return nil
}

func validPositiveNumber(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != "0"
}

func mustJSON(values []string) []byte {
	if values == nil {
		values = []string{}
	}
	raw, _ := json.Marshal(values)
	return raw
}

type scanner interface {
	Scan(...any) error
}

func scanBrief(row scanner) (Brief, error) {
	var item Brief
	var rawMust, rawAvoid, rawRequired, rawConstraints, rawSuccess []byte
	err := row.Scan(briefScanArgs(&item, &rawMust, &rawAvoid, &rawRequired, &rawConstraints, &rawSuccess)...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Brief{}, ErrBriefNotFound
		}
		return Brief{}, err
	}
	decodeArrays(&item, rawMust, rawAvoid, rawRequired, rawConstraints, rawSuccess)
	return item, nil
}

func briefScanArgs(item *Brief, mustHave, avoid, required, constraints, success *[]byte) []any {
	return []any{
		&item.ID, &item.ProjectID, &item.UserID, &item.Version, &item.Title, &item.Goal,
		&item.Audience, &item.Deliverable, &item.AspectRatio, &item.TargetWidth, &item.TargetHeight,
		&item.Subject, mustHave, avoid, &item.Mood, required, constraints, success,
		&item.Deadline, &item.Status, &item.CreatedAt, &item.UpdatedAt,
	}
}

func decodeArrays(item *Brief, mustHave, avoid, required, constraints, success []byte) {
	_ = json.Unmarshal(mustHave, &item.MustHave)
	_ = json.Unmarshal(avoid, &item.Avoid)
	_ = json.Unmarshal(required, &item.RequiredElements)
	_ = json.Unmarshal(constraints, &item.Constraints)
	_ = json.Unmarshal(success, &item.SuccessCriteria)
	if item.MustHave == nil {
		item.MustHave = []string{}
	}
	if item.Avoid == nil {
		item.Avoid = []string{}
	}
	if item.RequiredElements == nil {
		item.RequiredElements = []string{}
	}
	if item.Constraints == nil {
		item.Constraints = []string{}
	}
	if item.SuccessCriteria == nil {
		item.SuccessCriteria = []string{}
	}
}

func decode(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("multiple JSON values")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": uuid.NewString()}})
}
