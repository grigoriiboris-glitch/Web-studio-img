package references

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Usage struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	ProjectID  uuid.UUID  `json:"project_id"`
	ReferenceID uuid.UUID `json:"reference_id"`
	IterationID *uuid.UUID `json:"iteration_id,omitempty"`
	GenerationID *uuid.UUID `json:"generation_id,omitempty"`
	UsageType  string     `json:"usage_type"`
	Role       string     `json:"role"`
	Influence  *Influence `json:"influence,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type UsageRequest struct {
	IterationID  *uuid.UUID
	GenerationID *uuid.UUID
	UsageType    string
	Role         string
	Influence    *Influence
}

var ErrInvalidUsage = errors.New("invalid reference usage")
var ErrUsageNotFound = errors.New("reference usage not found")

func (r UsageRequest) Validate() error {
	if r.IterationID == nil && r.GenerationID == nil {
		return ErrInvalidUsage
	}
	if strings.TrimSpace(r.UsageType) == "" || len([]rune(r.UsageType)) > 100 {
		return ErrInvalidUsage
	}
	roles := map[string]struct{}{"inspiration": {}, "composition": {}, "subject": {}, "color": {}, "material": {}, "mood": {}}
	if r.Role == "" { r.Role = "inspiration" }
	if _, ok := roles[strings.TrimSpace(r.Role)]; !ok { return ErrInvalidUsage }
	if r.Influence != nil {
		r.Influence = NormalizeInfluence(r.Influence)
		for _, score := range []float64{r.Influence.Composition, r.Influence.Semantic, r.Influence.Color, r.Influence.Style, r.Influence.Material, r.Influence.Geometry, r.Influence.Mood} {
			if score < 0 || score > 1 {
				return ErrInvalidUsage
			}
		}
	}
	return nil
}

func (s *Store) CreateUsage(ctx context.Context, userID, projectID, referenceID uuid.UUID, req UsageRequest) (Usage, error) {
	if err := req.Validate(); err != nil {
		return Usage{}, err
	}
	var influenceRaw []byte
	var err error
	if req.Influence != nil {
		influenceRaw, err = json.Marshal(NormalizeInfluence(req.Influence))
		if err != nil {
			return Usage{}, err
		}
	} else {
		influenceRaw = []byte("{}")
	}
	var owned bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM "references" r
		JOIN projects p ON p.id=r.project_id
		WHERE r.id=$1 AND r.project_id=$2 AND p.user_id=$3 AND p.status <> 'deleted'
	)`, referenceID, projectID, userID).Scan(&owned)
	if err != nil || !owned {
		if err != nil { return Usage{}, err }
		return Usage{}, ErrReferenceNotFound
	}
	if req.IterationID != nil {
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM iterations i WHERE i.id=$1 AND i.project_id=$2
		)`, *req.IterationID, projectID).Scan(&owned); err != nil {
			return Usage{}, err
		}
		if !owned { return Usage{}, ErrUsageNotFound }
	}
	if req.GenerationID != nil {
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS (
			SELECT 1 FROM generations g WHERE g.id=$1 AND g.project_id=$2 AND g.user_id=$3
		)`, *req.GenerationID, projectID, userID).Scan(&owned); err != nil {
			return Usage{}, err
		}
		if !owned { return Usage{}, ErrUsageNotFound }
	}
	var out Usage
	var stored []byte
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO reference_usages(user_id,project_id,reference_id,iteration_id,generation_id,usage_type,role,influence)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id,user_id,project_id,reference_id,iteration_id,generation_id,usage_type,role,influence,created_at
	`, userID, projectID, referenceID, req.IterationID, req.GenerationID, strings.TrimSpace(req.UsageType), strings.TrimSpace(req.Role), influenceRaw).Scan(
		&out.ID,&out.UserID,&out.ProjectID,&out.ReferenceID,&out.IterationID,&out.GenerationID,&out.UsageType,&out.Role,&stored,&out.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) { return Usage{}, ErrUsageNotFound }
	if err != nil { return Usage{}, fmt.Errorf("create reference usage: %w", err) }
	_ = json.Unmarshal(stored, &out.Influence)
	return out, nil
}

func (s *Store) ListUsage(ctx context.Context, userID, projectID, referenceID uuid.UUID) ([]Usage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id,u.user_id,u.project_id,u.reference_id,u.iteration_id,u.generation_id,u.usage_type,u.role,u.influence,u.created_at
		FROM reference_usages u
		JOIN projects p ON p.id=u.project_id
		WHERE u.reference_id=$1 AND u.project_id=$2 AND p.user_id=$3 AND p.status <> 'deleted'
		ORDER BY u.created_at ASC,u.id ASC
	`, referenceID, projectID, userID)
	if err != nil { return nil, err }
	defer func(){ _ = rows.Close() }()
	var out []Usage
	for rows.Next() {
		var item Usage
		var raw []byte
		if err := rows.Scan(&item.ID,&item.UserID,&item.ProjectID,&item.ReferenceID,&item.IterationID,&item.GenerationID,&item.UsageType,&item.Role,&raw,&item.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &item.Influence)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil { return nil, err }
	return out, nil
}
