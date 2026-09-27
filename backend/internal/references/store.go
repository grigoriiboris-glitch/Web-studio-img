package references

import (
	"context"
	"database/sql"
	"errors"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil { return nil, errors.New("reference store requires database") }
	return &Store{db: db}, nil
}

func (s *Store) Create(ctx context.Context, userID, projectID uuid.UUID, req Request) (Reference, error) {
	if err := req.Validate(); err != nil { return Reference{}, err }
	if req.AssetID != nil {
		var assetProject uuid.UUID
		if err := s.db.QueryRowContext(ctx, "SELECT project_id FROM assets WHERE id=$1", *req.AssetID).Scan(&assetProject); errors.Is(err, sql.ErrNoRows) || assetProject != projectID {
			return Reference{}, ErrReferenceNotFound
		} else if err != nil { return Reference{}, err }
	}
	var owner uuid.UUID
	if err := s.db.QueryRowContext(ctx, "SELECT user_id FROM projects WHERE id=$1 AND status <> 'deleted'", projectID).Scan(&owner); errors.Is(err, sql.ErrNoRows) || owner != userID {
		return Reference{}, ErrReferenceNotFound
	} else if err != nil { return Reference{}, err }
	var r Reference
	influenceRaw, _ := json.Marshal(req.Influence)
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO "references"(project_id,asset_id,source_url,source_type,license,license_verified,user_owned,sha256,notes,influence)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING id,project_id,asset_id,source_url,source_type,license,license_verified,user_owned,sha256,notes,influence,created_at,updated_at
	`, projectID, req.AssetID, req.SourceURL, req.SourceType, strings.TrimSpace(req.License), req.LicenseVerified, req.UserOwned, req.SHA256, req.Notes, influenceRaw).Scan(
		&r.ID,&r.ProjectID,&r.AssetID,&r.SourceURL,&r.SourceType,&r.License,&r.LicenseVerified,&r.UserOwned,&r.SHA256,&r.Notes,&influenceRaw,&r.CreatedAt,&r.UpdatedAt)
	_ = json.Unmarshal(influenceRaw, &r.Influence)
	if err != nil { return Reference{}, fmt.Errorf("create reference: %w", err) }
	return r,nil
}

func (s *Store) List(ctx context.Context,userID,projectID uuid.UUID)([]Reference,error){
	rows,err:=s.db.QueryContext(ctx,`
		SELECT r.id,r.project_id,r.asset_id,r.source_url,r.source_type,r.license,r.license_verified,r.user_owned,r.sha256,r.notes,r.influence,r.created_at,r.updated_at
		FROM "references" r JOIN projects p ON p.id=r.project_id
		WHERE r.project_id=$1 AND p.user_id=$2 AND p.status <> 'deleted'
		ORDER BY r.created_at DESC
	`,projectID,userID);if err!=nil{return nil,err};defer rows.Close()
	var out []Reference
	for rows.Next(){var r Reference;var influenceRaw []byte;if err:=rows.Scan(&r.ID,&r.ProjectID,&r.AssetID,&r.SourceURL,&r.SourceType,&r.License,&r.LicenseVerified,&r.UserOwned,&r.SHA256,&r.Notes,&influenceRaw,&r.CreatedAt,&r.UpdatedAt);err!=nil{return nil,err};_ = json.Unmarshal(influenceRaw,&r.Influence);out=append(out,r)}
	return out,rows.Err()
}

func (s *Store) Update(ctx context.Context,userID,projectID,referenceID uuid.UUID,req Request)(Reference,error){
	if err:=req.Validate();err!=nil{return Reference{},err}
	if err:=s.validateAssetForProject(ctx,userID,projectID,req.AssetID);err!=nil{return Reference{},err}
	influenceRaw,err:=json.Marshal(req.Influence);if err!=nil{return Reference{},err}
	var r Reference
	err=s.db.QueryRowContext(ctx,`
		UPDATE "references"
		SET asset_id=$1,source_url=$2,source_type=$3,license=$4,license_verified=$5,user_owned=$6,sha256=$7,notes=$8,influence=$9,updated_at=now()
		WHERE id=$10 AND project_id=$11
		  AND project_id IN (SELECT id FROM projects WHERE user_id=$12 AND status <> 'deleted')
		RETURNING id,project_id,asset_id,source_url,source_type,license,license_verified,user_owned,sha256,notes,influence,created_at,updated_at
	`,req.AssetID,req.SourceURL,req.SourceType,strings.TrimSpace(req.License),req.LicenseVerified,req.UserOwned,req.SHA256,req.Notes,influenceRaw,referenceID,projectID,userID).Scan(
		&r.ID,&r.ProjectID,&r.AssetID,&r.SourceURL,&r.SourceType,&r.License,&r.LicenseVerified,&r.UserOwned,&r.SHA256,&r.Notes,&influenceRaw,&r.CreatedAt,&r.UpdatedAt)
	if errors.Is(err,sql.ErrNoRows){return Reference{},ErrReferenceNotFound}
	if err!=nil{return Reference{},err}
	_ = json.Unmarshal(influenceRaw,&r.Influence)
	return r,nil
}

func (s *Store) Delete(ctx context.Context,userID,projectID,referenceID uuid.UUID) error {
	res,err:=s.db.ExecContext(ctx,`DELETE FROM "references" WHERE id=$1 AND project_id=$2 AND project_id IN (SELECT id FROM projects WHERE user_id=$3)`,referenceID,projectID,userID)
	if err!=nil{return err}
	n,_:=res.RowsAffected();if n!=1{return ErrReferenceNotFound};return nil
}


func (s *Store) validateAssetForProject(ctx context.Context, userID, projectID uuid.UUID, assetID *uuid.UUID) error {
	if assetID == nil { return nil }
	var ok bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM assets a JOIN projects p ON p.id=a.project_id
		WHERE a.id=$1 AND a.project_id=$2 AND p.user_id=$3 AND p.status <> 'deleted'
	)`, *assetID, projectID, userID).Scan(&ok)
	if err != nil { return err }
	if !ok { return ErrReferenceNotFound }
	return nil
}
