package variants

import (
  "context"
  "database/sql"
  "encoding/json"
  "errors"
  "net/http"
  "strings"

  "github.com/google/uuid"

  "github.com/oleg3190/Web-studio-img/backend/internal/generation"
)

func (h *Handler) regenerateVariant(w http.ResponseWriter, r *http.Request) {
  userID, projectID, ok := h.authProject(w, r)
  if !ok { return }
  key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
  if key == "" || len(key) > 200 {
    writeError(w, 400, "idempotency_key_required", "Idempotency-Key is required and must be at most 200 characters")
    return
  }
  if h.generationCreator == nil {
    writeError(w, 503, "generation_unavailable", "generation service is not configured")
    return
  }

  setID, err := uuid.Parse(r.PathValue("set_id"))
  if err != nil { writeError(w, 400, "invalid_variant_set_id", "invalid variant set id"); return }
  variantID, err := uuid.Parse(r.PathValue("variant_id"))
  if err != nil { writeError(w, 400, "invalid_variant_id", "invalid variant id"); return }

  var source generation.Request
  var sourceGenerationID, sourceProjectID, sourceUserID uuid.UUID
  var iterationRaw, recipeRaw sql.NullString
  var seed sql.NullInt64
  var recipeVersion int
  var aspectRatio string
  var prompt, negativePrompt string
  var parametersRaw, refsRaw, workflowRaw, influenceRaw, finalRaw []byte
  var sourceStatus generation.Status

  err = h.db.QueryRowContext(r.Context(), `+
    "SELECT g.id,g.project_id,g.user_id,g.status,g.iteration_id::text,"+
    "g.prompt,g.negative_prompt,g.seed,g.aspect_ratio,g.parameters,"+
    "g.reference_ids,g.recipe_id::text,g.recipe_version,"+
    "g.resolved_workflow,g.resolved_workflow_hash,g.resolved_reference_influence,g.final_parameters "+
    "FROM variants v JOIN variant_sets vs ON vs.id=v.variant_set_id JOIN generations g ON g.id=v.generation_id "+
    "WHERE v.id=$1 AND v.variant_set_id=$2 AND vs.project_id=$3 AND vs.user_id=$4 AND g.project_id=$3 AND g.user_id=$4" + `,
    variantID, setID, projectID, userID).Scan(
    &sourceGenerationID,&sourceProjectID,&sourceUserID,&sourceStatus,&iterationRaw,
    &prompt,&negativePrompt,&seed,&aspectRatio,&parametersRaw,&refsRaw,
    &recipeRaw,&recipeVersion,&workflowRaw,&source.ResolvedWorkflowHash,&influenceRaw,&finalRaw,
  )
  if errors.Is(err, sql.ErrNoRows) { writeError(w, 404, "variant_not_found", "variant not found"); return }
  if err != nil { writeError(w, 500, "variant_regenerate_failed", "could not load variant generation context"); return }
  if sourceProjectID != projectID || sourceUserID != userID || sourceGenerationID == uuid.Nil {
    writeError(w, 404, "variant_not_found", "variant not found"); return
  }
  if sourceStatus == generation.StatusFailed || sourceStatus == generation.StatusCancelled {
    writeError(w, 409, "generation_not_regenerable", "the source generation is failed or cancelled"); return
  }

  source.ProjectID = projectID
  source.Prompt = prompt
  source.NegativePrompt = negativePrompt
  source.AspectRatio = aspectRatio
  source.Parameters = map[string]any{}
  source.FinalParameters = map[string]any{}
  source.ResolvedWorkflow = map[string]any{}
  source.ResolvedReferenceInfluence = map[string]any{}
  _ = json.Unmarshal(parametersRaw, &source.Parameters)
  _ = json.Unmarshal(finalRaw, &source.FinalParameters)
  _ = json.Unmarshal(workflowRaw, &source.ResolvedWorkflow)
  _ = json.Unmarshal(influenceRaw, &source.ResolvedReferenceInfluence)

  if seed.Valid { v := seed.Int64; source.Seed = &v }
  if iterationRaw.Valid {
    id, e := uuid.Parse(iterationRaw.String)
    if e != nil { writeError(w, 500, "variant_regenerate_failed", "source iteration id is invalid"); return }
    source.IterationID = &id
  }
  if recipeRaw.Valid {
    id, e := uuid.Parse(recipeRaw.String)
    if e != nil { writeError(w, 500, "variant_regenerate_failed", "source recipe id is invalid"); return }
    source.RecipeID = &id
    source.RecipeVersion = recipeVersion
  }
  if err := json.Unmarshal(refsRaw, &source.ReferenceIDs); err != nil {
    writeError(w, 500, "variant_regenerate_failed", "source reference ids are invalid"); return
  }

  generationKey := "variant-regenerate-" + setID.String() + "-" + variantID.String() + "-" + key
  source.IdempotencyKey = generationKey
  generated, _, err := h.generationCreator.Create(r.Context(), userID, source)
  if errors.Is(err, generation.ErrIdempotencyConflict) {
    writeError(w, 409, "idempotency_conflict", "generation idempotency key belongs to another request"); return
  }
  if errors.Is(err, generation.ErrInvalidGeneration) {
    writeError(w, 400, "invalid_generation", "source generation context is invalid"); return
  }
  if err != nil { writeError(w, 503, "generation_queue_failed", "could not queue regeneration"); return }

  requestHash := mutationRequestHash("regenerate_variant", map[string]any{
    "project_id": projectID, "variant_set_id": setID, "variant_id": variantID, "source_generation_id": sourceGenerationID,
  })
  operation := "regenerate_variant:" + setID.String() + ":" + variantID.String()
  tx, err := h.db.BeginTx(r.Context(), nil)
  if err != nil { writeError(w, 500, "variant_regenerate_failed", "could not start transaction"); return }
  defer func() { _ = tx.Rollback() }()

  replay, response, err := claimVariantMutation(r.Context(), tx, userID, projectID, operation, key, requestHash)
  if err != nil {
    if errors.Is(err, ErrIdempotencyConflict) { writeError(w, 409, "idempotency_conflict", err.Error()) } else { writeError(w, 500, "variant_idempotency_failed", "could not claim idempotency key") }
    return
  }
  if replay {
    w.Header().Set("Content-Type", "application/json"); w.WriteHeader(200); _, _ = w.Write(response); return
  }

  var lockedSet uuid.UUID
  if err := tx.QueryRowContext(r.Context(), "SELECT id FROM variant_sets WHERE id=$1 AND project_id=$2 AND user_id=$3 FOR UPDATE", setID, projectID, userID).Scan(&lockedSet); err != nil {
    if errors.Is(err, sql.ErrNoRows) { writeError(w, 404, "variant_set_not_found", "variant set not found") } else { writeError(w, 500, "variant_regenerate_failed", "could not lock variant set") }
    return
  }
  var ordinal int
  if err := tx.QueryRowContext(r.Context(), "SELECT COALESCE(MAX(ordinal),0)+1 FROM variants WHERE variant_set_id=$1", setID).Scan(&ordinal); err != nil {
    writeError(w, 500, "variant_regenerate_failed", "could not allocate variant ordinal"); return
  }

  var assetID sql.NullString
  var v Variant
  v.ID = uuid.New(); v.VariantSetID = setID; v.GenerationID = generated.ID; v.Ordinal = ordinal; v.Decision = "candidate"; v.RejectReason = []string{}
  if err := tx.QueryRowContext(r.Context(), "SELECT a.id FROM assets a WHERE a.generation_id=$1 AND a.user_id=$2 AND a.lifecycle_status='active' ORDER BY a.created_at DESC LIMIT 1", generated.ID, userID).Scan(&assetID); err == nil && assetID.Valid {
    if id, e := uuid.Parse(assetID.String); e == nil { v.AssetID = &id }
  } else if err != nil && !errors.Is(err, sql.ErrNoRows) {
    writeError(w, 500, "variant_regenerate_failed", "could not load regenerated asset"); return
  }

  if _, err := tx.ExecContext(r.Context(), "INSERT INTO variants(id,variant_set_id,project_id,user_id,generation_id,asset_id,ordinal,reject_reason) VALUES($1,$2,$3,$4,$5,$6,$7,'[]'::jsonb)", v.ID,setID,projectID,userID,v.GenerationID,v.AssetID,v.Ordinal); err != nil {
    writeError(w, 500, "variant_regenerate_failed", "could not add regenerated variant"); return
  }

  response, _ = json.Marshal(map[string]any{"generation": generated, "variant": v})
  if err := storeVariantMutationResponse(r.Context(), tx, userID, operation, key, v.ID, response); err != nil {
    writeError(w, 500, "variant_idempotency_failed", "could not persist idempotency response"); return
  }
  if err := tx.Commit(); err != nil { writeError(w, 500, "variant_regenerate_failed", "could not commit regenerated variant"); return }

  w.Header().Set("Content-Type", "application/json"); w.WriteHeader(202); _, _ = w.Write(response)
}
