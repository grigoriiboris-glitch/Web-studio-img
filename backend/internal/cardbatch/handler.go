package cardbatch

import (
  "context"
  "database/sql"
  "encoding/json"
  "errors"
  "io"
  "net/http"
  "strings"
  "time"

  "github.com/google/uuid"
  "github.com/oleg3190/Web-studio-img/backend/internal/auth"
)

const maxJSONBytes = 2 << 20

type Batch struct {
  ID uuid.UUID `json:"id"`
  ProjectID uuid.UUID `json:"project_id"`
  UserID uuid.UUID `json:"user_id"`
  Name string `json:"name"`
  SourceFile string `json:"source_file"`
  Sheet string `json:"sheet"`
  Mapping map[string]any `json:"mapping"`
  State map[string]any `json:"state"`
  Version int64 `json:"version"`
  CardTypeID *uuid.UUID `json:"card_type_id,omitempty"`
  CardTypeVersion *int `json:"card_type_version,omitempty"`
  CreatedAt time.Time `json:"created_at"`
  UpdatedAt time.Time `json:"updated_at"`
}

type createInput struct {
  Name string `json:"name"`
  SourceFile string `json:"source_file"`
  Sheet string `json:"sheet"`
  Mapping map[string]any `json:"mapping"`
  State map[string]any `json:"state"`
  CardTypeID *uuid.UUID `json:"card_type_id"`
  CardTypeVersion *int `json:"card_type_version"`
}

type updateInput struct {
  Version int64 `json:"version"`
  SourceFile *string `json:"source_file,omitempty"`
  Sheet *string `json:"sheet,omitempty"`
  Mapping map[string]any `json:"mapping,omitempty"`
  State map[string]any `json:"state,omitempty"`
  CardTypeID *uuid.UUID `json:"card_type_id,omitempty"`
  CardTypeVersion *int `json:"card_type_version,omitempty"`
}

type Handler struct{ db *sql.DB }

func NewHandler(db *sql.DB) (*Handler, error) {
  if db == nil { return nil, errors.New("card batch handler requires database") }
  return &Handler{db: db}, nil
}

func (h *Handler) Register(mux *http.ServeMux) {
  mux.HandleFunc("GET /api/v1/projects/{project_id}/card-batches", h.list)
  mux.HandleFunc("POST /api/v1/projects/{project_id}/card-batches", h.create)
  mux.HandleFunc("GET /api/v1/projects/{project_id}/card-batches/{batch_id}", h.get)
  mux.HandleFunc("PATCH /api/v1/projects/{project_id}/card-batches/{batch_id}", h.update)
  mux.HandleFunc("DELETE /api/v1/projects/{project_id}/card-batches/{batch_id}", h.remove)
}

func (h *Handler) authProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
  principal, ok := auth.PrincipalFromContext(r.Context())
  if !ok { writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required"); return uuid.Nil, false }
  projectID, err := uuid.Parse(r.PathValue("project_id"))
  if err != nil { writeError(w, http.StatusBadRequest, "invalid_project_id", "invalid project id"); return uuid.Nil, false }
  var exists bool
  if err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')", projectID, principal.UserID).Scan(&exists); err != nil || !exists {
    writeError(w, http.StatusNotFound, "project_not_found", "project not found"); return uuid.Nil, false
  }
  return projectID, true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
  projectID, ok := h.authProject(w, r); if !ok { return }
  rows, err := h.db.QueryContext(r.Context(), `SELECT id,project_id,user_id,name,source_file,sheet,mapping,state,version,card_type_id,card_type_version,created_at,updated_at FROM card_batches WHERE project_id=$1 ORDER BY updated_at DESC,id DESC`, projectID)
  if err != nil { writeError(w, 500, "card_batch_list_failed", "could not list card batches"); return }
  defer func() { _ = rows.Close() }()
  out := []Batch{}
  for rows.Next() {
    item, err := scan(rows); if err != nil { writeError(w, 500, "card_batch_list_failed", "could not read card batch"); return }
    out = append(out, item)
  }
  if err := rows.Err(); err != nil { writeError(w, 500, "card_batch_list_failed", "could not read card batches"); return }
  writeJSON(w, 200, map[string]any{"batches": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
  projectID, ok := h.authProject(w, r); if !ok { return }
  id, err := uuid.Parse(r.PathValue("batch_id")); if err != nil { writeError(w, 400, "invalid_batch_id", "invalid batch id"); return }
  item, err := h.load(r.Context(), projectID, id)
  if errors.Is(err, sql.ErrNoRows) { writeError(w, 404, "card_batch_not_found", "card batch not found"); return }
  if err != nil { writeError(w, 500, "card_batch_get_failed", "could not load card batch"); return }
  writeJSON(w, 200, item)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
  projectID, ok := h.authProject(w, r); if !ok { return }
  var in createInput
  if err := decode(r, &in); err != nil { writeError(w, 400, "invalid_request", "invalid card batch payload"); return }
  in.Name = strings.TrimSpace(in.Name)
  if in.Name == "" || len([]rune(in.Name)) > 200 { writeError(w, 400, "invalid_card_batch", "name is required and must be <= 200 characters"); return }
  if len(in.SourceFile) > 1000 || len(in.Sheet) > 200 { writeError(w, 400, "invalid_card_batch", "source metadata is too long"); return }
  if in.Mapping == nil { in.Mapping = map[string]any{} }
  if in.State == nil { in.State = map[string]any{} }
  if !validJSON(in.Mapping) || !validJSON(in.State) { writeError(w, 400, "invalid_card_batch", "mapping or state is not valid JSON"); return }

  principal, _ := auth.PrincipalFromContext(r.Context())
  var item Batch
  var mappingBytes, stateBytes []byte
  err := h.db.QueryRowContext(r.Context(), `INSERT INTO card_batches(project_id,user_id,name,source_file,sheet,mapping,state,card_type_id,card_type_version) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8,$9) RETURNING id,project_id,user_id,name,source_file,sheet,mapping,state,version,card_type_id,card_type_version,created_at,updated_at`, projectID, principal.UserID, in.Name, in.SourceFile, in.Sheet, mustJSON(in.Mapping), mustJSON(in.State), in.CardTypeID, in.CardTypeVersion).Scan(scanArgs(&item, &mappingBytes, &stateBytes)...)
  if err == nil { _ = json.Unmarshal(mappingBytes, &item.Mapping); _ = json.Unmarshal(stateBytes, &item.State) }
  if err != nil {
    if strings.Contains(err.Error(), "card_batches_project_name_uq") { writeError(w, 409, "card_batch_name_conflict", "a card batch with this name already exists"); return }
    writeError(w, 500, "card_batch_create_failed", "could not create card batch"); return
  }
  writeJSON(w, 201, item)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
  projectID, ok := h.authProject(w, r); if !ok { return }
  id, err := uuid.Parse(r.PathValue("batch_id")); if err != nil { writeError(w, 400, "invalid_batch_id", "invalid batch id"); return }
  var in updateInput
  if err := decode(r, &in); err != nil { writeError(w, 400, "invalid_request", "invalid card batch update"); return }
  if in.Version < 1 { writeError(w, 400, "invalid_version", "version must be positive"); return }
  if in.State != nil && !validJSON(in.State) { writeError(w, 400, "invalid_state", "state is not valid JSON"); return }
  if in.Mapping != nil && !validJSON(in.Mapping) { writeError(w, 400, "invalid_mapping", "mapping is not valid JSON"); return }

  tx, err := h.db.BeginTx(r.Context(), nil); if err != nil { writeError(w, 500, "card_batch_update_failed", "could not start transaction"); return }
  defer func() { _ = tx.Rollback() }()
  var item Batch
  query := `UPDATE card_batches SET source_file=COALESCE($4,source_file),sheet=COALESCE($5,sheet),mapping=COALESCE($6::jsonb,mapping),state=COALESCE($7::jsonb,state),card_type_id=COALESCE($8,card_type_id),card_type_version=COALESCE($9,card_type_version),version=version+1,updated_at=now() WHERE id=$1 AND project_id=$2 AND version=$3 RETURNING id,project_id,user_id,name,source_file,sheet,mapping,state,version,card_type_id,card_type_version,created_at,updated_at`
  var mapping, state []byte
  if in.Mapping != nil { mapping = mustJSON(in.Mapping) }
  if in.State != nil { state = mustJSON(in.State) }
  var mappingBytes, stateBytes []byte
  err = tx.QueryRowContext(r.Context(), query, id, projectID, in.Version, nullableString(in.SourceFile), nullableString(in.Sheet), nullableJSON(mapping), nullableJSON(state), in.CardTypeID, in.CardTypeVersion).Scan(scanArgs(&item, &mappingBytes, &stateBytes)...)
  if err == nil { _ = json.Unmarshal(mappingBytes, &item.Mapping); _ = json.Unmarshal(stateBytes, &item.State) }
  if errors.Is(err, sql.ErrNoRows) {
    var current int64
    e := tx.QueryRowContext(r.Context(), "SELECT version FROM card_batches WHERE id=$1 AND project_id=$2", id, projectID).Scan(&current)
    if errors.Is(e, sql.ErrNoRows) { writeError(w, 404, "card_batch_not_found", "card batch not found") } else { writeError(w, 409, "card_batch_version_conflict", "card batch was changed by another client") }
    return
  }
  if err != nil { writeError(w, 500, "card_batch_update_failed", "could not update card batch"); return }
  if err := tx.Commit(); err != nil { writeError(w, 500, "card_batch_update_failed", "could not commit card batch"); return }
  writeJSON(w, 200, item)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
  projectID, ok := h.authProject(w, r); if !ok { return }
  id, err := uuid.Parse(r.PathValue("batch_id")); if err != nil { writeError(w, 400, "invalid_batch_id", "invalid batch id"); return }
  result, err := h.db.ExecContext(r.Context(), "DELETE FROM card_batches WHERE id=$1 AND project_id=$2", id, projectID)
  if err != nil { writeError(w, 500, "card_batch_delete_failed", "could not delete card batch"); return }
  if n, _ := result.RowsAffected(); n == 0 { writeError(w, 404, "card_batch_not_found", "card batch not found"); return }
  w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) load(ctx context.Context, projectID, id uuid.UUID) (Batch, error) {
  var item Batch
  var mapping, state []byte
  err := h.db.QueryRowContext(ctx, "SELECT id,project_id,user_id,name,source_file,sheet,mapping,state,version,card_type_id,card_type_version,created_at,updated_at FROM card_batches WHERE id=$1 AND project_id=$2", id, projectID).Scan(scanArgs(&item, &mapping, &state)...)
  if err != nil { return Batch{}, err }
  if err := json.Unmarshal(mapping, &item.Mapping); err != nil { return Batch{}, err }
  if err := json.Unmarshal(state, &item.State); err != nil { return Batch{}, err }
  return item, nil
}

type scanner interface{ Scan(...any) error }

func scan(row scanner) (Batch, error) {
  var item Batch
  var mapping, state []byte
  err := row.Scan(&item.ID,&item.ProjectID,&item.UserID,&item.Name,&item.SourceFile,&item.Sheet,&mapping,&state,&item.Version,&item.CardTypeID,&item.CardTypeVersion,&item.CreatedAt,&item.UpdatedAt)
  if err != nil { return Batch{}, err }
  if err := json.Unmarshal(mapping,&item.Mapping); err != nil { return Batch{}, err }
  if err := json.Unmarshal(state,&item.State); err != nil { return Batch{}, err }
  return item,nil
}
func scanArgs(item *Batch, mapping, state *[]byte) []any {
  return []any{&item.ID,&item.ProjectID,&item.UserID,&item.Name,&item.SourceFile,&item.Sheet,mapping,state,&item.Version,&item.CardTypeID,&item.CardTypeVersion,&item.CreatedAt,&item.UpdatedAt}
}
func mustJSON(v any) []byte { b,_:=json.Marshal(v); return b }
func validJSON(v any) bool { b,err:=json.Marshal(v); return err==nil && len(b)<=maxJSONBytes }
func nullableJSON(v []byte) any { if v==nil { return nil }; return string(v) }
func nullableString(v *string) any { if v==nil { return nil }; return *v }
func decode(r *http.Request, target any) error {
  decoder:=json.NewDecoder(io.LimitReader(r.Body,maxJSONBytes)); decoder.DisallowUnknownFields()
  if err:=decoder.Decode(target); err!=nil{return err}
  var extra any; if err:=decoder.Decode(&extra); err!=io.EOF{return errors.New("multiple JSON values")}; return nil
}
func writeJSON(w http.ResponseWriter,status int,value any){w.Header().Set("Content-Type","application/json");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(value)}
func writeError(w http.ResponseWriter,status int,code,message string){writeJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message,"request_id":uuid.NewString()}})}