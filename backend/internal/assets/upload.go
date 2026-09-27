package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

const MaxUploadPartSize int64 = 2 << 20

type UploadPart struct {
	AssetID uuid.UUID `json:"asset_id"`
	PartNumber int `json:"part_number"`
	StorageKey string `json:"storage_key"`
	Size int64 `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) CreatePendingUpload(ctx context.Context, userID, projectID uuid.UUID, mime string, size int64) (Asset, error) {
	if userID == uuid.Nil || projectID == uuid.Nil || size <= 0 || size > MaxAssetSize {
		return Asset{}, fmt.Errorf("invalid upload metadata")
	}
	if mime != "image/jpeg" && mime != "image/png" {
		return Asset{}, fmt.Errorf("unsupported upload MIME type %q", mime)
	}
	var ok bool
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted')", projectID, userID).Scan(&ok); err != nil {
		return Asset{}, err
	}
	if !ok {
		return Asset{}, ErrAssetNotFound
	}
	id := uuid.New()
	original, preview, thumbnail := storageKeys(userID, projectID, id)
	var item Asset
	var generationID sql.NullString
	var previewKey, thumbnailKey sql.NullString
	var exifData []byte
	var expiresAt *time.Time
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO assets(id,user_id,project_id,generation_id,type,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,sha256,checksum,exif,lifecycle_status,expires_at) VALUES($1,$2,$3,NULL,'uploaded',$4,$5,$6,$7,$8,NULL,NULL,NULL,NULL,'{}'::jsonb,'pending',now()+interval '24 hours') RETURNING id,project_id,generation_id,type,user_id,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,checksum,exif,lifecycle_status,expires_at,deleted_at,created_at",
		id,userID,projectID,original,preview,thumbnail,mime,size,
	).Scan(&item.ID,&item.ProjectID,&generationID,&item.Type,&item.UserID,&item.StorageKey,&previewKey,&thumbnailKey,&item.MIMEType,&item.Size,&item.Width,&item.Height,&item.Checksum,&exifData,&item.LifecycleStatus,&expiresAt,&item.DeletedAt,&item.CreatedAt)
	if err != nil {
		return Asset{}, fmt.Errorf("create pending upload: %w", err)
	}
	if generationID.Valid {
		item.GenerationID,_ = uuid.Parse(generationID.String)
	}
	if previewKey.Valid { v:=previewKey.String; item.PreviewKey=&v }
	if thumbnailKey.Valid { v:=thumbnailKey.String; item.ThumbnailKey=&v }
	item.ExpiresAt=expiresAt
	return item,nil
}

func (s *Store) GetOwned(ctx context.Context,userID,projectID,assetID uuid.UUID)(Asset,error){
	var item Asset
	var generationID sql.NullString
	var previewKey,thumbnailKey sql.NullString
	var checksum sql.NullString
	var exifData []byte
	err:=s.db.QueryRowContext(ctx,"SELECT id,project_id,generation_id,type,user_id,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,checksum,exif,lifecycle_status,expires_at,deleted_at,created_at FROM assets WHERE id=$1 AND project_id=$2 AND user_id=$3 AND lifecycle_status <> 'deleted'",assetID,projectID,userID).Scan(&item.ID,&item.ProjectID,&generationID,&item.Type,&item.UserID,&item.StorageKey,&previewKey,&thumbnailKey,&item.MIMEType,&item.Size,&item.Width,&item.Height,&checksum,&exifData,&item.LifecycleStatus,&item.ExpiresAt,&item.DeletedAt,&item.CreatedAt)
	if err==sql.ErrNoRows{return Asset{},ErrAssetNotFound};if err!=nil{return Asset{},err}
	if generationID.Valid{item.GenerationID,_=uuid.Parse(generationID.String)}
	if previewKey.Valid{v:=previewKey.String;item.PreviewKey=&v};if thumbnailKey.Valid{v:=thumbnailKey.String;item.ThumbnailKey=&v}
	if checksum.Valid{item.Checksum=checksum.String};if len(exifData)>0{_ = json.Unmarshal(exifData,&item.EXIF)}
	return item,nil
}

func (s *Store) AddUploadPart(ctx context.Context,userID,projectID,assetID uuid.UUID,partNumber int,key string,size int64)error{
	if partNumber<0||size<=0||size>MaxUploadPartSize{return fmt.Errorf("invalid upload part")}
	var ok bool
	if err:=s.db.QueryRowContext(ctx,"SELECT EXISTS (SELECT 1 FROM assets WHERE id=$1 AND project_id=$2 AND user_id=$3 AND lifecycle_status='pending')",assetID,projectID,userID).Scan(&ok);err!=nil{return err}
	if !ok{return ErrAssetNotFound}
	_,err:=s.db.ExecContext(ctx,"INSERT INTO asset_upload_parts(asset_id,part_number,storage_key,size) VALUES($1,$2,$3,$4) ON CONFLICT(asset_id,part_number) DO UPDATE SET storage_key=EXCLUDED.storage_key,size=EXCLUDED.size,created_at=now()",assetID,partNumber,key,size)
	return err
}

func (s *Store) ListUploadParts(ctx context.Context,userID,projectID,assetID uuid.UUID)([]UploadPart,error){
	var ok bool
	if err:=s.db.QueryRowContext(ctx,"SELECT EXISTS (SELECT 1 FROM assets WHERE id=$1 AND project_id=$2 AND user_id=$3 AND lifecycle_status='pending')",assetID,projectID,userID).Scan(&ok);err!=nil{return nil,err}
	if !ok{return nil,ErrAssetNotFound}
	rows,err:=s.db.QueryContext(ctx,"SELECT p.asset_id,p.part_number,p.storage_key,p.size,p.created_at FROM asset_upload_parts p JOIN assets a ON a.id=p.asset_id WHERE p.asset_id=$1 AND a.project_id=$2 AND a.user_id=$3 ORDER BY p.part_number",assetID,projectID,userID)
	if err!=nil{return nil,err};defer rows.Close()
	var out []UploadPart
	for rows.Next(){var p UploadPart;if err:=rows.Scan(&p.AssetID,&p.PartNumber,&p.StorageKey,&p.Size,&p.CreatedAt);err!=nil{return nil,err};out=append(out,p)}
	return out,rows.Err()
}

func (s *Store) DeleteUploadParts(ctx context.Context,assetID uuid.UUID)error{_,err:=s.db.ExecContext(ctx,"DELETE FROM asset_upload_parts WHERE asset_id=$1",assetID);return err}

func (s *Store) FinalizePendingUpload(ctx context.Context,userID,projectID,assetID uuid.UUID,size int64,width,height int,checksum string,exifData map[string]any)(Asset,bool,error){
	raw,err:=json.Marshal(exifData);if err!=nil{return Asset{},false,err}
	tx,err:=s.db.BeginTx(ctx,nil);if err!=nil{return Asset{},false,err};defer tx.Rollback()
	var current Asset
	var gen,preview,thumb sql.NullString
	var existingChecksum sql.NullString
	var storedExif []byte
	err=tx.QueryRowContext(ctx,"SELECT id,project_id,generation_id,type,user_id,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,checksum,exif,lifecycle_status,expires_at,deleted_at,created_at FROM assets WHERE id=$1 AND project_id=$2 AND user_id=$3 AND lifecycle_status='pending' FOR UPDATE",assetID,projectID,userID).Scan(&current.ID,&current.ProjectID,&gen,&current.Type,&current.UserID,&current.StorageKey,&preview,&thumb,&current.MIMEType,&current.Size,&current.Width,&current.Height,&existingChecksum,&storedExif,&current.LifecycleStatus,&current.ExpiresAt,&current.DeletedAt,&current.CreatedAt)
	if err==sql.ErrNoRows{return Asset{},false,ErrAssetNotFound};if err!=nil{return Asset{},false,err}
	var dup Asset
	var dupGen,dupPreview,dupThumb sql.NullString
	var dupChecksum sql.NullString
	var dupExif []byte
	err=tx.QueryRowContext(ctx,"SELECT id,project_id,generation_id,type,user_id,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,checksum,exif,lifecycle_status,expires_at,deleted_at,created_at FROM assets WHERE user_id=$1 AND checksum=$2 AND lifecycle_status='active' AND id<>$3 ORDER BY created_at LIMIT 1",userID,checksum,assetID).Scan(&dup.ID,&dup.ProjectID,&dupGen,&dup.Type,&dup.UserID,&dup.StorageKey,&dupPreview,&dupThumb,&dup.MIMEType,&dup.Size,&dup.Width,&dup.Height,&dupChecksum,&dupExif,&dup.LifecycleStatus,&dup.ExpiresAt,&dup.DeletedAt,&dup.CreatedAt)
	if err==nil{
		if dupGen.Valid{dup.GenerationID,_=uuid.Parse(dupGen.String)};if dupPreview.Valid{v:=dupPreview.String;dup.PreviewKey=&v};if dupThumb.Valid{v:=dupThumb.String;dup.ThumbnailKey=&v};if dupChecksum.Valid{dup.Checksum=dupChecksum.String};if len(dupExif)>0{_ = json.Unmarshal(dupExif,&dup.EXIF)}
		_,_=tx.ExecContext(ctx,"DELETE FROM assets WHERE id=$1",assetID);if err=tx.Commit();err!=nil{return Asset{},false,err};return dup,true,nil
	}
	if err!=sql.ErrNoRows{return Asset{},false,err}
	if _,err=tx.ExecContext(ctx,"UPDATE assets SET size=$1,width=$2,height=$3,sha256=$4,checksum=$4,exif=$5,lifecycle_status='active',expires_at=NULL,deleted_at=NULL WHERE id=$6 AND project_id=$7 AND user_id=$8",size,width,height,checksum,raw,assetID,projectID,userID);err!=nil{return Asset{},false,err}
	if err=tx.Commit();err!=nil{return Asset{},false,err}
	current.GenerationID=parseGeneration(gen);if preview.Valid{v:=preview.String;current.PreviewKey=&v};if thumb.Valid{v:=thumb.String;current.ThumbnailKey=&v};current.Size=size;current.Width=width;current.Height=height;current.Checksum=checksum;current.EXIF=exifData;current.LifecycleStatus="active";current.ExpiresAt=nil
	return current,false,nil
}

func (p *Processor) ProcessUpload(ctx context.Context,userID,projectID uuid.UUID,data []byte,mime string)(Asset,error){
	if len(data)==0||int64(len(data))>MaxAssetSize{return Asset{},fmt.Errorf("asset exceeds %d bytes",MaxAssetSize)}
	detected:=http.DetectContentType(data);if detected!="image/jpeg"&&detected!="image/png"{return Asset{},fmt.Errorf("unsupported upload MIME type %q",detected)}
	if mime!=""&&mime!=detected{return Asset{},fmt.Errorf("MIME mismatch: declared=%q detected=%q",mime,detected)}
	if err:=p.scanner().Scan(ctx,data,detected);err!=nil{return Asset{},fmt.Errorf("security scan failed: %w",err)}
	cfg,format,err:=image.DecodeConfig(bytes.NewReader(data));if err!=nil{return Asset{},fmt.Errorf("decode image config: %w",err)}
	if cfg.Width<=0||cfg.Height<=0||cfg.Width>MaxImageDimension||cfg.Height>MaxImageDimension||(format!="jpeg"&&format!="png"){return Asset{},fmt.Errorf("invalid image dimensions or format")}
	exifData:=readEXIF(data)
	normalized,err:=normalizeImage(data,detected);if err!=nil{return Asset{},fmt.Errorf("normalize image: %w",err)}
	sum:=sha256.Sum256(normalized);checksum:=hex.EncodeToString(sum[:])
	item,created,err:=p.Store.ReserveUploadedAsset(ctx,userID,projectID,detected,int64(len(normalized)),cfg.Width,cfg.Height,checksum,exifData);if err!=nil{return Asset{},err}
	if !created {
		if item.LifecycleStatus=="active"{return item,nil}
		active,waitErr:=p.Store.waitForActive(ctx,userID,projectID,checksum);if waitErr==nil{return active,nil}
		return Asset{},fmt.Errorf("asset with checksum %s is already being processed",checksum)
	}
	preview,err:=makePreview(normalized,1600);if err!=nil{_ = p.Store.MarkOrphaned(ctx,userID,item.ID);return Asset{},fmt.Errorf("create preview: %w",err)}
	thumbnail,err:=makePreview(normalized,320);if err!=nil{_ = p.Store.MarkOrphaned(ctx,userID,item.ID);return Asset{},fmt.Errorf("create thumbnail: %w",err)}
	uploaded:=[]string{}
	put:=func(key string,body []byte,ct string)error{if err:=p.Storage.Put(ctx,key,bytes.NewReader(body),int64(len(body)),storage.PutOptions{ContentType:ct,Metadata:map[string]string{"sha256":checksum}});err!=nil{return err};uploaded=append(uploaded,key);return nil}
	cleanup:=func(){for _,key:=range uploaded{_=p.Storage.Delete(context.Background(),key)};_=p.Store.MarkOrphaned(context.Background(),userID,item.ID)}
	if err:=put(item.StorageKey,normalized,detected);err!=nil{cleanup();return Asset{},fmt.Errorf("upload original: %w",err)}
	if item.PreviewKey!=nil{if err:=put(*item.PreviewKey,preview,"image/jpeg");err!=nil{cleanup();return Asset{},fmt.Errorf("upload preview: %w",err)}}
	if item.ThumbnailKey!=nil{if err:=put(*item.ThumbnailKey,thumbnail,"image/jpeg");err!=nil{cleanup();return Asset{},fmt.Errorf("upload thumbnail: %w",err)}}
	if err:=p.Store.Finalize(ctx,userID,item.ID);err!=nil{cleanup();return Asset{},fmt.Errorf("finalize asset: %w",err)}
	item.LifecycleStatus="active";item.ExpiresAt=nil
	return item,nil
}

func (p *Processor) ProcessPendingUpload(ctx context.Context,userID,projectID,assetID uuid.UUID)(Asset,error){
	item,err:=p.Store.GetOwned(ctx,userID,projectID,assetID);if err!=nil{return Asset{},err}
	obj,_,err:=p.Storage.Get(ctx,item.StorageKey);if err!=nil{_ = p.Store.MarkOrphaned(ctx,userID,assetID);return Asset{},err};defer obj.Close()
	data,err:=io.ReadAll(io.LimitReader(obj,MaxAssetSize+1));if err!=nil{return Asset{},err};if int64(len(data))>MaxAssetSize{return Asset{},fmt.Errorf("asset exceeds %d bytes",MaxAssetSize)}
	detected:=http.DetectContentType(data);if detected!=item.MIMEType{return Asset{},fmt.Errorf("MIME mismatch: declared=%q detected=%q",item.MIMEType,detected)}
	if err:=p.scanner().Scan(ctx,data,detected);err!=nil{return Asset{},fmt.Errorf("security scan failed: %w",err)}
	cfg,format,err:=image.DecodeConfig(bytes.NewReader(data));if err!=nil{return Asset{},err};if cfg.Width<=0||cfg.Height<=0||cfg.Width>MaxImageDimension||cfg.Height>MaxImageDimension||(format!="jpeg"&&format!="png"){return Asset{},fmt.Errorf("invalid image dimensions or format")}
	exifData:=readEXIF(data);normalized,err:=normalizeImage(data,detected);if err!=nil{return Asset{},err}
	sum:=sha256.Sum256(normalized);checksum:=hex.EncodeToString(sum[:])
	preview,err:=makePreview(normalized,1600);if err!=nil{return Asset{},err};thumbnail,err:=makePreview(normalized,320);if err!=nil{return Asset{},err}
	uploaded:=[]string{}
	put:=func(key string,body []byte,ct string)error{if err:=p.Storage.Put(ctx,key,bytes.NewReader(body),int64(len(body)),storage.PutOptions{ContentType:ct,Metadata:map[string]string{"sha256":checksum}});err!=nil{return err};uploaded=append(uploaded,key);return nil}
	if err:=put(item.StorageKey,normalized,detected);err!=nil{_ = p.Store.MarkOrphaned(ctx,userID,assetID);return Asset{},err}
	if item.PreviewKey!=nil{if err:=put(*item.PreviewKey,preview,"image/jpeg");err!=nil{_ = p.Store.MarkOrphaned(ctx,userID,assetID);return Asset{},err}}
	if item.ThumbnailKey!=nil{if err:=put(*item.ThumbnailKey,thumbnail,"image/jpeg");err!=nil{_ = p.Store.MarkOrphaned(ctx,userID,assetID);return Asset{},err}}
	final,duplicate,err:=p.Store.FinalizePendingUpload(ctx,userID,projectID,assetID,int64(len(normalized)),cfg.Width,cfg.Height,checksum,exifData);if err!=nil{for _,key:=range uploaded{_ = p.Storage.Delete(context.Background(),key)};return Asset{},err};if duplicate{for _,key:=range uploaded{_ = p.Storage.Delete(context.Background(),key)}}
	return final,nil
}

func parseGeneration(v sql.NullString)uuid.UUID{if !v.Valid{return uuid.Nil};id,_:=uuid.Parse(v.String);return id}
