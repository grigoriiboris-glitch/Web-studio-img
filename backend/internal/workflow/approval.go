package workflow

import (
    "database/sql"
    "encoding/json"
    "errors"
    "net/http"
    "sort"
    "strings"
    "time"

    "github.com/google/uuid"

    "github.com/oleg3190/Web-studio-img/backend/internal/humanactions"
    "github.com/oleg3190/Web-studio-img/backend/internal/provenance"
)

type workflowState string

const (
    workflowDraft workflowState = "draft"
    workflowExplore workflowState = "explore"
    workflowDevelop workflowState = "develop"
    workflowReady workflowState = "ready_for_review"
    workflowApproved workflowState = "approved"
    workflowFinal workflowState = "final"
)

type approvalCheck struct {
    Key string `json:"key"`
    Label string `json:"label"`
    Status string `json:"status"`
    Critical bool `json:"critical"`
    Message string `json:"message"`
}

type approvalGate struct {
    WorkflowState workflowState `json:"workflow_state"`
    FinalIterationID *uuid.UUID `json:"final_iteration_id,omitempty"`
    FinalAssetID *uuid.UUID `json:"final_asset_id,omitempty"`
    Checks []approvalCheck `json:"checks"`
    Warnings []string `json:"warnings"`
    Blocked []string `json:"blocked"`
    CanRequestReview bool `json:"can_request_review"`
    CanApprove bool `json:"can_approve"`
    CanFinalize bool `json:"can_finalize"`
    OverrideRequired bool `json:"override_required"`
    ApprovedAt *time.Time `json:"approved_at,omitempty"`
    FinalizedAt *time.Time `json:"finalized_at,omitempty"`
}

type finalizeInput struct {
    OverrideReason string `json:"override_reason,omitempty"`
    OverrideChecks []string `json:"override_checks,omitempty"`
}

func (h *Handler) registerApprovalRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /api/v1/projects/{project_id}/approval-gate", h.getApprovalGate)
    mux.HandleFunc("POST /api/v1/projects/{project_id}/approval-gate/review", h.requestReview)
    mux.HandleFunc("POST /api/v1/projects/{project_id}/approval-gate/approve", h.approveProject)
    mux.HandleFunc("POST /api/v1/projects/{project_id}/approval-gate/composition/approve", h.approveComposition)
    mux.HandleFunc("POST /api/v1/projects/{project_id}/approval-gate/finalize", h.finalizeProject)
    mux.HandleFunc("POST /api/v1/projects/{project_id}/approval-gate/revision", h.createRevision)
}

func (h *Handler) getApprovalGate(w http.ResponseWriter, r *http.Request) {
    userID, projectID, ok := approvalProject(w, r)
    if !ok { return }
    gate, err := h.evaluateApprovalGate(r.Context(), userID, projectID)
    if err != nil { workflowError(w, 500, "approval_gate_failed", "could not evaluate approval checklist"); return }
    jsonOut(w, 200, gate)
}

func (h *Handler) requestReview(w http.ResponseWriter, r *http.Request) {
    userID, projectID, ok := h.authProject(w, r)
    if !ok { return }
    gate, err := h.evaluateApprovalGate(r.Context(), userID, projectID)
    if err != nil { workflowError(w, 500, "approval_gate_failed", "could not evaluate approval checklist"); return }
    if len(gate.Blocked) > 0 { workflowError(w, 409, "approval_blocked", "critical checklist items are not complete"); return }
    if gate.WorkflowState == workflowFinal { workflowError(w, 409, "already_final", "final project must create a revision before review"); return }
    raw, _ := json.Marshal(gate.Checks)
    _, err = h.db.ExecContext(r.Context(),
        "INSERT INTO project_approval_gates(project_id,user_id,checklist,review_requested_at,updated_at) VALUES($1,$2,$3,now(),now()) ON CONFLICT(project_id) DO UPDATE SET user_id=EXCLUDED.user_id,checklist=EXCLUDED.checklist,review_requested_at=now(),updated_at=now()",
        projectID, userID, raw,
    )
    if err != nil { workflowError(w, 500, "approval_review_failed", "could not request review"); return }
    oldState := gate.WorkflowState
    if _, err = h.db.ExecContext(r.Context(), "UPDATE projects SET workflow_state='ready_for_review',updated_at=now() WHERE id=$1 AND user_id=$2", projectID, userID); err != nil {
        workflowError(w, 500, "approval_review_failed", "could not update workflow state"); return
    }
    h.recordApprovalAction(r, userID, projectID, projectID, "APPROVAL_REQUESTED",
        map[string]any{"checks": gate.Checks, "old_state": oldState, "new_state": workflowReady},
        map[string]any{"workflow_state": oldState}, map[string]any{"workflow_state": workflowReady})
    gate.WorkflowState = workflowReady
    gate.CanRequestReview = false
    gate.CanApprove = len(gate.Blocked) == 0
    gate.CanFinalize = false
    jsonOut(w, 200, gate)
}

func (h *Handler) approveProject(w http.ResponseWriter, r *http.Request) {
    userID, projectID, ok := h.authProject(w, r)
    if !ok { return }
    gate, err := h.evaluateApprovalGate(r.Context(), userID, projectID)
    if err != nil { workflowError(w, 500, "approval_gate_failed", "could not evaluate approval checklist"); return }
    if gate.WorkflowState != workflowReady { workflowError(w, 409, "review_required", "project must be in ready_for_review before approval"); return }
    if len(gate.Blocked) > 0 { workflowError(w, 409, "approval_blocked", "critical checklist items are not complete"); return }
    oldState := gate.WorkflowState
    if _, err = h.db.ExecContext(r.Context(), "UPDATE projects SET workflow_state='approved',updated_at=now() WHERE id=$1 AND user_id=$2", projectID, userID); err != nil {
        workflowError(w, 500, "approval_failed", "could not approve project"); return
    }
    now := time.Now().UTC()
    raw, _ := json.Marshal(gate.Checks)
    _, err = h.db.ExecContext(r.Context(),
        "INSERT INTO project_approval_gates(project_id,user_id,checklist,review_requested_at,approved_at,approved_by,updated_at) VALUES($1,$2,$3,COALESCE((SELECT review_requested_at FROM project_approval_gates WHERE project_id=$1),now()),$4,$2,now()) ON CONFLICT(project_id) DO UPDATE SET checklist=EXCLUDED.checklist,approved_at=EXCLUDED.approved_at,approved_by=EXCLUDED.approved_by,updated_at=now()",
        projectID, userID, raw, now,
    )
    if err != nil { workflowError(w, 500, "approval_failed", "could not persist approval state"); return }
    h.recordApprovalAction(r, userID, projectID, projectID, "PROJECT_APPROVED",
        map[string]any{"checks": gate.Checks, "old_state": oldState, "new_state": workflowApproved},
        map[string]any{"workflow_state": oldState}, map[string]any{"workflow_state": workflowApproved})
    gate.WorkflowState = workflowApproved
    gate.ApprovedAt = &now
    gate.CanRequestReview = false
    gate.CanApprove = false
    gate.CanFinalize = len(gate.Blocked) == 0
    jsonOut(w, 200, gate)
}

func (h *Handler) approveComposition(w http.ResponseWriter, r *http.Request) {
    userID, projectID, ok := h.authProject(w, r)
    if !ok { return }
    var exists bool
    err := h.db.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM composition_specs WHERE project_id=$1 AND user_id=$2)", projectID, userID).Scan(&exists)
    if err != nil { workflowError(w, 500, "composition_approval_failed", "could not inspect composition"); return }
    if !exists { workflowError(w, 409, "composition_not_found", "create a composition before approving it"); return }
    oldAction := "COMPOSITION_CHANGED"
    _ = h.db.QueryRowContext(r.Context(), "SELECT action_type FROM human_actions WHERE project_id=$1 AND action_type IN ('COMPOSITION_CHANGED','COMPOSITION_APPROVED') ORDER BY version DESC LIMIT 1", projectID).Scan(&oldAction)
    payload := map[string]any{"check": "composition", "old_action": oldAction, "new_action": "COMPOSITION_APPROVED"}
    h.recordApprovalAction(r, userID, projectID, projectID, "COMPOSITION_APPROVED", payload,
        map[string]any{"composition_action": oldAction}, map[string]any{"composition_action": "COMPOSITION_APPROVED"})
    gate, err := h.evaluateApprovalGate(r.Context(), userID, projectID)
    if err != nil { workflowError(w, 500, "approval_gate_failed", "could not re-evaluate checklist"); return }
    jsonOut(w, 200, gate)
}

func (h *Handler) finalizeProject(w http.ResponseWriter, r *http.Request) {
    userID, projectID, ok := h.authProject(w, r)
    if !ok { return }
    var in finalizeInput
    if err := decodeWorkflowJSON(r, &in); err != nil { workflowError(w, 400, "invalid_request", "invalid finalize payload"); return }
    gate, err := h.evaluateApprovalGate(r.Context(), userID, projectID)
    if err != nil { workflowError(w, 500, "approval_gate_failed", "could not evaluate approval checklist"); return }
    if gate.WorkflowState != workflowApproved { workflowError(w, 409, "approval_required", "project must be approved before finalize"); return }
    if len(gate.Blocked) > 0 { workflowError(w, 409, "finalize_blocked", "critical checklist items are not complete"); return }
    if err := validateOverrides(gate.Warnings, in.OverrideChecks, in.OverrideReason); err != nil { workflowError(w, 409, "override_required", err.Error()); return }

    tx, err := h.db.BeginTx(r.Context(), nil)
    if err != nil { workflowError(w, 500, "finalize_failed", "could not start finalization"); return }
    defer func() { _ = tx.Rollback() }()
    var state string
    var oldFinal *uuid.UUID
    if err = tx.QueryRowContext(r.Context(), "SELECT workflow_state,final_iteration_id FROM projects WHERE id=$1 AND user_id=$2 FOR UPDATE", projectID, userID).Scan(&state, &oldFinal); err != nil { workflowError(w, 500, "finalize_failed", "could not lock project"); return }
    if state != string(workflowApproved) { workflowError(w, 409, "approval_required", "project must be approved before finalize"); return }

    var selectedID uuid.UUID
    var assetID uuid.UUID
    var generationIterationID uuid.UUID
    var selectedCount int
    if err = tx.QueryRowContext(r.Context(), "SELECT count(*) FROM variants WHERE project_id=$1 AND user_id=$2 AND decision='selected'", projectID, userID).Scan(&selectedCount); err != nil { workflowError(w, 500, "finalize_failed", "could not load selected variant"); return }
    if selectedCount != 1 { workflowError(w, 409, "final_variant_required", "exactly one final variant must be selected"); return }
    if err = tx.QueryRowContext(r.Context(), "SELECT v.id,v.asset_id,g.iteration_id FROM variants v JOIN generations g ON g.id=v.generation_id WHERE v.project_id=$1 AND v.user_id=$2 AND v.decision='selected' ORDER BY v.updated_at DESC LIMIT 1", projectID, userID).Scan(&selectedID, &assetID, &generationIterationID); err != nil { workflowError(w, 409, "final_variant_required", "selected variant has no usable generation"); return }
    if assetID == uuid.Nil { workflowError(w, 409, "final_variant_asset_required", "selected variant must have an asset"); return }

    var finalIterationID uuid.UUID
    if err = tx.QueryRowContext(r.Context(), "INSERT INTO iterations(project_id,parent_iteration_id,type,title,description) VALUES($1,$2,'final',$3,$4) RETURNING id", projectID, generationIterationID, "Final revision", "Finalized production result").Scan(&finalIterationID); err != nil { workflowError(w, 500, "finalize_failed", "could not create final iteration"); return }
    checklistRaw, _ := json.Marshal(gate.Checks)
    overrideRaw, _ := json.Marshal(in.OverrideChecks)
    now := time.Now().UTC()
    if _, err = tx.ExecContext(r.Context(), "UPDATE projects SET workflow_state='final',mode='finalize',final_iteration_id=$1,updated_at=now() WHERE id=$2 AND user_id=$3", finalIterationID, projectID, userID); err != nil { workflowError(w, 500, "finalize_failed", "could not set final project state"); return }
    _, err = tx.ExecContext(r.Context(), "INSERT INTO project_approval_gates(project_id,user_id,checklist,override_checks,override_reason,final_asset_id,review_requested_at,approved_at,approved_by,finalized_at,finalized_by,updated_at) VALUES($1,$2,$3,$4,$5,$6,COALESCE((SELECT review_requested_at FROM project_approval_gates WHERE project_id=$1),now()),COALESCE((SELECT approved_at FROM project_approval_gates WHERE project_id=$1),now()),COALESCE((SELECT approved_by FROM project_approval_gates WHERE project_id=$1),$2),$7,$2,now()) ON CONFLICT(project_id) DO UPDATE SET checklist=EXCLUDED.checklist,override_checks=EXCLUDED.override_checks,override_reason=EXCLUDED.override_reason,final_asset_id=EXCLUDED.final_asset_id,finalized_at=EXCLUDED.finalized_at,finalized_by=EXCLUDED.finalized_by,updated_at=now()", projectID, userID, checklistRaw, overrideRaw, strings.TrimSpace(in.OverrideReason), assetID, now)
    if err != nil { workflowError(w, 500, "finalize_failed", "could not persist final gate"); return }
    if err = tx.Commit(); err != nil { workflowError(w, 500, "finalize_failed", "could not commit finalization"); return }

    h.recordApprovalAction(r, userID, projectID, finalIterationID, "PROJECT_FINALIZED",
        map[string]any{"final_iteration_id": finalIterationID, "final_asset_id": assetID, "selected_variant_id": selectedID, "old_state": workflowApproved, "new_state": workflowFinal},
        map[string]any{"workflow_state": workflowApproved, "final_iteration_id": oldFinal},
        map[string]any{"workflow_state": workflowFinal, "final_iteration_id": finalIterationID})
    if len(in.OverrideChecks) > 0 {
        h.recordApprovalAction(r, userID, projectID, finalIterationID, "FINALIZE_OVERRIDE",
            map[string]any{"checks": in.OverrideChecks, "reason": strings.TrimSpace(in.OverrideReason)},
            map[string]any{"workflow_state": workflowApproved}, map[string]any{"workflow_state": workflowFinal})
    }
    gate, err = h.evaluateApprovalGate(r.Context(), userID, projectID)
    if err != nil { workflowError(w, 500, "approval_gate_failed", "final state was created but checklist reload failed"); return }
    gate.FinalIterationID = &finalIterationID
    gate.FinalAssetID = &assetID
    jsonOut(w, 201, gate)
}

func (h *Handler) createRevision(w http.ResponseWriter, r *http.Request) {
    userID, projectID, ok := h.authProject(w, r)
    if !ok { return }
    tx, err := h.db.BeginTx(r.Context(), nil)
    if err != nil { workflowError(w, 500, "revision_failed", "could not start revision"); return }
    defer func() { _ = tx.Rollback() }()
    var state string
    var finalID *uuid.UUID
    if err = tx.QueryRowContext(r.Context(), "SELECT workflow_state,final_iteration_id FROM projects WHERE id=$1 AND user_id=$2 FOR UPDATE", projectID, userID).Scan(&state, &finalID); err != nil { workflowError(w, 500, "revision_failed", "could not load workflow state"); return }
    if state != string(workflowFinal) || finalID == nil { workflowError(w, 409, "revision_required", "a final project is required before creating a revision"); return }
    var iterationID uuid.UUID
    if err = tx.QueryRowContext(r.Context(), "INSERT INTO iterations(project_id,parent_iteration_id,type,title,description) VALUES($1,$2,'idea',$3,$4) RETURNING id", projectID, *finalID, "Revision from final", "New editable revision branched from the immutable final").Scan(&iterationID); err != nil { workflowError(w, 500, "revision_failed", "could not create revision"); return }
    if _, err = tx.ExecContext(r.Context(), "UPDATE projects SET workflow_state='develop',mode='develop',final_iteration_id=NULL,updated_at=now() WHERE id=$1 AND user_id=$2", projectID, userID); err != nil { workflowError(w, 500, "revision_failed", "could not reopen project"); return }
    if err = tx.Commit(); err != nil { workflowError(w, 500, "revision_failed", "could not commit revision"); return }
    h.recordApprovalAction(r, userID, projectID, iterationID, "PROJECT_REVISION_CREATED",
        map[string]any{"parent_final_iteration_id": *finalID, "revision_iteration_id": iterationID, "old_state": workflowFinal, "new_state": workflowDevelop},
        map[string]any{"workflow_state": workflowFinal, "final_iteration_id": finalID},
        map[string]any{"workflow_state": workflowDevelop, "final_iteration_id": nil})
    gate, err := h.evaluateApprovalGate(r.Context(), userID, projectID)
    if err != nil { workflowError(w, 500, "approval_gate_failed", "revision created but checklist reload failed"); return }
    jsonOut(w, 201, gate)
}

func (h *Handler) evaluateApprovalGate(ctx context.Context, userID, projectID uuid.UUID) (approvalGate, error) {
    gate := approvalGate{}
    var state workflowState
    var finalID *uuid.UUID
    if err := h.db.QueryRowContext(ctx, "SELECT workflow_state,final_iteration_id FROM projects WHERE id=$1 AND user_id=$2 AND status<>'deleted'", projectID, userID).Scan(&state, &finalID); err != nil { return gate, err }
    gate.WorkflowState = state
    gate.FinalIterationID = finalID
    var finalAssetID *uuid.UUID
    _ = h.db.QueryRowContext(ctx, "SELECT final_asset_id,approved_at,finalized_at FROM project_approval_gates WHERE project_id=$1 AND user_id=$2", projectID, userID).Scan(&finalAssetID, &gate.ApprovedAt, &gate.FinalizedAt)
    gate.FinalAssetID = finalAssetID

    checks := make([]approvalCheck, 0, 10)
    add := func(key, label, status, message string, critical bool) { checks = append(checks, approvalCheck{Key:key, Label:label, Status:status, Critical:critical, Message:message}) }

    var briefTotal, briefApproved int
    if err := h.db.QueryRowContext(ctx, "SELECT count(*),count(*) FILTER (WHERE status='approved') FROM creative_briefs WHERE project_id=$1 AND user_id=$2", projectID, userID).Scan(&briefTotal, &briefApproved); err != nil { return gate, err }
    if briefTotal == 0 { add("creative_brief","Creative Brief","passed","Not used for this project",false) } else if briefApproved > 0 { add("creative_brief","Creative Brief","passed","Approved brief exists",false) } else { add("creative_brief","Creative Brief","blocked","A Creative Brief exists but none is approved",true) }

    var promptAction string
    err := h.db.QueryRowContext(ctx, "SELECT action_type FROM human_actions WHERE project_id=$1 AND action_type IN ('PROMPT_EDITED','PROMPT_APPROVED') ORDER BY version DESC LIMIT 1", projectID).Scan(&promptAction)
    if errors.Is(err, sql.ErrNoRows) { add("prompt","Prompt approval","blocked","No approved prompt recorded",true) } else if err != nil { return gate, err } else if promptAction == "PROMPT_APPROVED" { add("prompt","Prompt approval","passed","Latest prompt decision is approved",true) } else { add("prompt","Prompt approval","blocked","Latest prompt decision is not approved",true) }

    var compositionCount int
    if err := h.db.QueryRowContext(ctx, "SELECT count(*) FROM composition_specs WHERE project_id=$1 AND user_id=$2", projectID, userID).Scan(&compositionCount); err != nil { return gate, err }
    if compositionCount == 0 { add("composition","Composition approval","warning","No composition spec is used",false) } else {
        var action string
        e := h.db.QueryRowContext(ctx, "SELECT action_type FROM human_actions WHERE project_id=$1 AND action_type IN ('COMPOSITION_CHANGED','COMPOSITION_APPROVED') ORDER BY version DESC LIMIT 1", projectID).Scan(&action)
        if errors.Is(e,sql.ErrNoRows) || action != "COMPOSITION_APPROVED" { add("composition","Composition approval","blocked","Composition must be explicitly approved after its latest change",true) } else { add("composition","Composition approval","passed","Latest composition decision is approved",true) }
    }

    var selectedCount int
    if err := h.db.QueryRowContext(ctx, "SELECT count(*) FROM variants WHERE project_id=$1 AND user_id=$2 AND decision='selected'", projectID, userID).Scan(&selectedCount); err != nil { return gate, err }
    var selectedAsset *uuid.UUID
    var generationIteration *uuid.UUID
    var provider, model string
    var params []byte
    if selectedCount != 1 {
        add("final_variant","Selected final variant","blocked","Exactly one variant must be selected",true)
    } else {
        err = h.db.QueryRowContext(ctx, "SELECT v.asset_id,g.iteration_id,g.provider,g.model,g.parameters FROM variants v JOIN generations g ON g.id=v.generation_id WHERE v.project_id=$1 AND v.user_id=$2 AND v.decision='selected' ORDER BY v.updated_at DESC LIMIT 1", projectID, userID).Scan(&selectedAsset,&generationIteration,&provider,&model,&params)
        if err != nil { add("final_variant","Selected final variant","blocked","Selected variant has no generation",true) } else if selectedAsset == nil || *selectedAsset == uuid.Nil { add("final_variant","Selected final variant","blocked","Selected variant has no asset",true) } else { add("final_variant","Selected final variant","passed","Exactly one final variant is selected",true) }
        if strings.TrimSpace(provider) == "" || strings.TrimSpace(model) == "" || len(params) == 0 { add("provider","Provider/model/workflow","blocked","Selected generation is missing provider, model or parameters",true) } else { add("provider","Provider/model/workflow","passed","Selected generation has reproducible provider parameters",true) }
    }

    var referenceCount, verifiedRights int
    if err := h.db.QueryRowContext(ctx, "SELECT count(*) FROM \"references\" WHERE project_id=$1 AND user_id=$2", projectID, userID).Scan(&referenceCount); err != nil { return gate, err }
    if referenceCount == 0 { add("references","References attached","warning","No references are attached",false); add("rights","Rights state","passed","Not applicable without references",false) } else {
        add("references","References attached","passed","References are attached",false)
        if err := h.db.QueryRowContext(ctx, "SELECT count(*) FROM \"references\" r WHERE r.project_id=$1 AND r.user_id=$2 AND (EXISTS(SELECT 1 FROM rights_registry rr WHERE rr.project_id=$1 AND rr.user_id=$2 AND rr.target_type='reference' AND rr.target_id=r.id AND rr.verification_state='verified') OR (r.asset_id IS NOT NULL AND EXISTS(SELECT 1 FROM rights_registry rr WHERE rr.project_id=$1 AND rr.user_id=$2 AND rr.target_type='asset' AND rr.target_id=r.asset_id AND rr.verification_state='verified')))", projectID, userID).Scan(&verifiedRights); err != nil { return gate, err }
        if verifiedRights == referenceCount { add("rights","Rights state","passed","All attached references have verified rights",true) } else { add("rights","Rights state","blocked","Every attached reference must have verified rights",true) }
    }

    if selectedAsset != nil && *selectedAsset != uuid.Nil {
        var n int
        if err := h.db.QueryRowContext(ctx, "SELECT count(*) FROM similarity_checks WHERE project_id=$1 AND user_id=$2 AND target_asset_id=$3", projectID, userID, *selectedAsset).Scan(&n); err != nil { return gate, err }
        if n > 0 { add("similarity","Similarity check","passed","Similarity check exists for the selected final asset",false) } else { add("similarity","Similarity check","warning","Similarity check is missing; explicit skip is required to finalize",false) }
    } else { add("similarity","Similarity check","warning","Similarity check cannot run until a final asset is selected",false) }

    var manualCount, draftManual int
    if err := h.db.QueryRowContext(ctx, "SELECT count(*),count(*) FILTER (WHERE status='draft') FROM manual_edits WHERE project_id=$1 AND user_id=$2", projectID, userID).Scan(&manualCount, &draftManual); err != nil { return gate, err }
    if draftManual > 0 { add("manual_edits","Manual edits","blocked","There are unfinished manual edits",true) } else if manualCount == 0 { add("manual_edits","Manual edits","warning","No manual edits recorded",false) } else { add("manual_edits","Manual edits","passed","All recorded manual edits are decided",false) }

    if h.provenance == nil { add("provenance","Provenance integrity","blocked","Provenance verifier is unavailable",true) } else {
        verification, e := h.provenance.Verify(ctx, userID, projectID)
        if e != nil || !verification.Valid { add("provenance","Provenance integrity","blocked","Provenance chain verification failed",true) } else { add("provenance","Provenance integrity","passed","Provenance chain verified",true) }
    }

    gate.Checks = checks
    for _, check := range checks {
        if check.Status == "blocked" { gate.Blocked = append(gate.Blocked, check.Key) }
        if check.Status == "warning" { gate.Warnings = append(gate.Warnings, check.Key) }
    }
    sort.Strings(gate.Blocked)
    sort.Strings(gate.Warnings)
    gate.CanRequestReview = len(gate.Blocked) == 0 && state != workflowFinal
    gate.CanApprove = state == workflowReady && len(gate.Blocked) == 0
    gate.CanFinalize = state == workflowApproved && len(gate.Blocked) == 0
    gate.OverrideRequired = len(gate.Warnings) > 0
    _ = generationIteration
    return gate, nil
}

func validateOverrides(warnings, selected []string, reason string) error {
    allowed := make(map[string]bool, len(warnings))
    for _, key := range warnings { allowed[key] = true }
    selectedMap := make(map[string]bool, len(selected))
    for _, key := range selected {
        if selectedMap[key] { return errors.New("duplicate override check") }
        if !allowed[key] { return errors.New("override contains a non-warning check") }
        selectedMap[key] = true
    }
    for _, key := range warnings {
        if !selectedMap[key] { return errors.New("all warnings must be explicitly overridden") }
    }
    if len(warnings) > 0 && strings.TrimSpace(reason) == "" { return errors.New("override reason is required when finalizing with warnings") }
    return nil
}

func (h *Handler) recordApprovalAction(r *http.Request, userID, projectID, entityID uuid.UUID, action string, payload, oldState, newState map[string]any) {
    if h.actions != nil {
        _, _ = h.actions.Create(r.Context(), userID, projectID, humanactions.Request{ActionType:action, Payload:payload, OldState:oldState, NewState:newState})
    }
    if h.events != nil { _, _ = h.events.Append(r.Context(), userID, projectID, strings.ToLower(action), "workflow", entityID, payload) }
    if h.provenance != nil {
        _, _ = h.provenance.Append(r.Context(), provenance.Event{UserID:userID, ProjectID:projectID, EntityType:"workflow", EntityID:entityID, Action:strings.ToLower(action), Payload:payload})
    }
}

func decodeWorkflowJSON(r *http.Request, v any) error {
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    return dec.Decode(v)
}

func approvalProject(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
    userID, ok := user(r)
    if !ok { workflowError(w, 401, "unauthorized", "authentication required"); return uuid.Nil, uuid.Nil, false }
    projectID, err := id(r, "project_id")
    if err != nil { workflowError(w, 400, "invalid_project_id", "invalid project id"); return uuid.Nil, uuid.Nil, false }
    return userID, projectID, true
}
func workflowError(w http.ResponseWriter, status int, code, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code":code, "message":message, "request_id":uuid.NewString()}})
}
