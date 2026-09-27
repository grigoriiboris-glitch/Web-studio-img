package references

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

func (s *Store) GetOwned(ctx context.Context, userID, projectID, referenceID uuid.UUID) (Reference, error) {
	var r Reference
	var raw []byte
	err := s.db.QueryRowContext(ctx, "SELECT r.id,r.project_id,r.asset_id,r.source_url,r.source_type,r.license,r.license_verified,r.user_owned,r.sha256,r.notes,r.influence,r.created_at,r.updated_at FROM references r JOIN projects p ON p.id=r.project_id WHERE r.id=$1 AND r.project_id=$2 AND p.user_id=$3 AND p.status <> 'deleted'", referenceID, projectID, userID).
		Scan(&r.ID,&r.ProjectID,&r.AssetID,&r.SourceURL,&r.SourceType,&r.License,&r.LicenseVerified,&r.UserOwned,&r.SHA256,&r.Notes,&raw,&r.CreatedAt,&r.UpdatedAt)
	if err == sql.ErrNoRows { return Reference{}, ErrReferenceNotFound }
	if err != nil { return Reference{}, err }
	if len(raw)>0 { _ = json.Unmarshal(raw,&r.Influence) }
	return r,nil
}

func (s *Store) UpdateInfluence(ctx context.Context,userID,projectID,referenceID uuid.UUID,influence *Influence)(Reference,error){
	raw,err:=json.Marshal(influence);if err!=nil{return Reference{},err}
	var r Reference;var stored []byte
	err=s.db.QueryRowContext(ctx,"UPDATE references SET influence=$1,updated_at=now() WHERE id=$2 AND project_id=$3 AND project_id IN (SELECT id FROM projects WHERE user_id=$4 AND status <> 'deleted') RETURNING id,project_id,asset_id,source_url,source_type,license,license_verified,user_owned,sha256,notes,influence,created_at,updated_at",raw,referenceID,projectID,userID).
		Scan(&r.ID,&r.ProjectID,&r.AssetID,&r.SourceURL,&r.SourceType,&r.License,&r.LicenseVerified,&r.UserOwned,&r.SHA256,&r.Notes,&stored,&r.CreatedAt,&r.UpdatedAt)
	if err == sql.ErrNoRows{return Reference{},ErrReferenceNotFound}
	if err != nil{return Reference{},err}
	if len(stored)>0{_=json.Unmarshal(stored,&r.Influence)}
	return r,nil
}

func (s *Store) GetOwnedByID(ctx context.Context, userID, referenceID uuid.UUID) (Reference, error) {
	var r Reference
	var raw []byte
	err := s.db.QueryRowContext(ctx, "SELECT r.id,r.project_id,r.asset_id,r.source_url,r.source_type,r.license,r.license_verified,r.user_owned,r.sha256,r.notes,r.influence,r.created_at,r.updated_at FROM references r JOIN projects p ON p.id=r.project_id WHERE r.id=$1 AND p.user_id=$2 AND p.status <> 'deleted'", referenceID, userID).
		Scan(&r.ID,&r.ProjectID,&r.AssetID,&r.SourceURL,&r.SourceType,&r.License,&r.LicenseVerified,&r.UserOwned,&r.SHA256,&r.Notes,&raw,&r.CreatedAt,&r.UpdatedAt)
	if err == sql.ErrNoRows { return Reference{}, ErrReferenceNotFound }
	if err != nil { return Reference{}, err }
	if len(raw)>0 { _ = json.Unmarshal(raw,&r.Influence) }
	return r,nil
}
