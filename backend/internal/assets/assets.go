package assets

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/draw"

	"github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

const (
	MaxAssetSize       int64         = 10 << 20
	MaxImageDimension               = 8192
	AssetPendingTTL    time.Duration = 24 * time.Hour
)

var ErrAssetNotFound = errors.New("asset not found")

type Asset struct {
	ID              uuid.UUID      `json:"id"`
	ProjectID       uuid.UUID      `json:"project_id"`
	GenerationID    uuid.UUID      `json:"generation_id"`
	Type            string         `json:"type"`
	UserID          uuid.UUID      `json:"user_id"`
	StorageKey      string         `json:"storage_key"`
	PreviewKey      *string        `json:"preview_key,omitempty"`
	ThumbnailKey    *string        `json:"thumbnail_key,omitempty"`
	MIMEType        string         `json:"mime_type"`
	Size            int64          `json:"size"`
	Width           int            `json:"width"`
	Height          int            `json:"height"`
	Checksum        string         `json:"checksum"`
	EXIF            map[string]any `json:"exif,omitempty"`
	LifecycleStatus string         `json:"lifecycle_status"`
	ExpiresAt       *time.Time     `json:"expires_at,omitempty"`
	DeletedAt       *time.Time     `json:"deleted_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("asset store requires database")
	}
	return &Store{db: db}, nil
}

func (s *Store) Reserve(
	ctx context.Context,
	userID, projectID, generationID, assetID uuid.UUID,
	storageKey, previewKey, thumbnailKey, mime string,
	size int64,
	width, height int,
	checksum string,
	exifData map[string]any,
) (Asset, bool, error) {
	raw, err := json.Marshal(exifData)
	if err != nil {
		return Asset{}, false, err
	}
	var item Asset
	var storedRaw []byte
	var expiresAt *time.Time
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO assets
			(id, user_id, project_id, generation_id, type, storage_key, preview_key, thumbnail_key, mime_type, size, width, height, sha256, checksum, exif, lifecycle_status, expires_at)
		VALUES ($1,$2,$3,$4,'generated',$5,$6,$7,$8,$9,$10,$11,$12,$12,$13,'pending',now()+$14::interval)
		ON CONFLICT (user_id, checksum) DO NOTHING
		RETURNING id, project_id, generation_id, user_id, type, storage_key, preview_key, thumbnail_key, mime_type, size, width, height, checksum, exif, lifecycle_status, expires_at, deleted_at, created_at
	`, assetID, userID, projectID, generationID, storageKey, previewKey, thumbnailKey, mime, size, width, height, checksum, raw, "24 hours").Scan(
		&item.ID, &item.ProjectID, &item.GenerationID, &item.UserID, &item.Type, &item.StorageKey, &item.PreviewKey, &item.ThumbnailKey,
		&item.MIMEType, &item.Size, &item.Width, &item.Height, &item.Checksum, &storedRaw, &item.LifecycleStatus, &expiresAt, &item.DeletedAt, &item.CreatedAt,
	)
	if err == nil {
		item.ExpiresAt = expiresAt
		_ = json.Unmarshal(storedRaw, &item.EXIF)
		return item, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Asset{}, false, fmt.Errorf("reserve asset: %w", err)
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT id, project_id, generation_id, user_id, type, storage_key, preview_key, thumbnail_key, mime_type, size, width, height, checksum, exif, lifecycle_status, expires_at, deleted_at, created_at
		FROM assets
		WHERE user_id=$1 AND checksum=$2
	`, userID, checksum).Scan(
		&item.ID, &item.ProjectID, &item.GenerationID, &item.UserID, &item.Type, &item.StorageKey, &item.PreviewKey, &item.ThumbnailKey,
		&item.MIMEType, &item.Size, &item.Width, &item.Height, &item.Checksum, &storedRaw, &item.LifecycleStatus, &expiresAt, &item.DeletedAt, &item.CreatedAt,
	)
	if err != nil {
		return Asset{}, false, fmt.Errorf("load duplicate asset: %w", err)
	}
	item.ExpiresAt = expiresAt
	_ = json.Unmarshal(storedRaw, &item.EXIF)
	return item, false, nil
}


func (s *Store) ReserveUploadedAsset(ctx context.Context, userID, projectID uuid.UUID, mime string, size int64, width, height int, checksum string, exifData map[string]any) (Asset, bool, error) {
	if userID == uuid.Nil || projectID == uuid.Nil || size <= 0 || size > MaxAssetSize || width <= 0 || height <= 0 || checksum == "" {
		return Asset{}, false, fmt.Errorf("invalid uploaded asset metadata")
	}
	raw, err := json.Marshal(exifData)
	if err != nil {
		return Asset{}, false, err
	}
	id := uuid.New()
	original, preview, thumbnail := storageKeys(userID, projectID, id)
	var item Asset
	var generationID sql.NullString
	var previewKey, thumbnailKey sql.NullString
	var storedExif []byte
	var expiresAt *time.Time
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO assets(id,user_id,project_id,generation_id,type,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,sha256,checksum,exif,lifecycle_status,expires_at)
		VALUES($1,$2,$3,NULL,'uploaded',$4,$5,$6,$7,$8,$9,$10,$11,$11,$12,'pending',now()+interval '24 hours')
		ON CONFLICT (user_id, checksum) DO NOTHING
		RETURNING id,project_id,generation_id,type,user_id,storage_key,preview_key,thumbnail_key,mime_type,size,width,height,checksum,exif,lifecycle_status,expires_at,deleted_at,created_at
	`, id,userID,projectID,original,preview,thumbnail,mime,size,width,height,checksum,raw,
	).Scan(&item.ID,&item.ProjectID,&generationID,&item.Type,&item.UserID,&item.StorageKey,&previewKey,&thumbnailKey,&item.MIMEType,&item.Size,&item.Width,&item.Height,&item.Checksum,&storedExif,&item.LifecycleStatus,&expiresAt,&item.DeletedAt,&item.CreatedAt)
	if err == nil {
		item.ExpiresAt = expiresAt
		if previewKey.Valid { v := previewKey.String; item.PreviewKey = &v }
		if thumbnailKey.Valid { v := thumbnailKey.String; item.ThumbnailKey = &v }
		if generationID.Valid { item.GenerationID,_ = uuid.Parse(generationID.String) }
		_ = json.Unmarshal(storedExif, &item.EXIF)
		return item, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Asset{}, false, fmt.Errorf("reserve uploaded asset: %w", err)
	}
	item, err = s.lookupByChecksum(ctx, userID, projectID, checksum)
	return item, false, err
}

func (s *Store) Finalize(ctx context.Context, userID, assetID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET lifecycle_status='active', expires_at=NULL, deleted_at=NULL
		WHERE id=$1 AND user_id=$2 AND lifecycle_status='pending'
	`, assetID, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrAssetNotFound
	}
	return nil
}

func (s *Store) MarkOrphaned(ctx context.Context, userID, assetID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE assets
		SET lifecycle_status='orphaned', expires_at=now()+$1::interval
		WHERE id=$2 AND user_id=$3 AND lifecycle_status='pending'
	`, "24 hours", assetID, userID)
	return err
}

func (s *Store) ListExpired(ctx context.Context, limit int) ([]Asset, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, generation_id, user_id, type, storage_key, preview_key, thumbnail_key, mime_type, size, width, height, checksum, exif, lifecycle_status, expires_at, deleted_at, created_at
		FROM assets
		WHERE lifecycle_status IN ('pending','orphaned') AND expires_at IS NOT NULL AND expires_at <= now()
		ORDER BY expires_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Asset
	for rows.Next() {
		var item Asset
		var raw []byte
		if err := rows.Scan(
			&item.ID, &item.ProjectID, &item.GenerationID, &item.UserID, &item.Type, &item.StorageKey, &item.PreviewKey, &item.ThumbnailKey,
			&item.MIMEType, &item.Size, &item.Width, &item.Height, &item.Checksum, &raw, &item.LifecycleStatus, &item.ExpiresAt, &item.DeletedAt, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &item.EXIF)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) DeleteExpired(ctx context.Context, assetID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM assets
		WHERE id=$1 AND lifecycle_status IN ('pending','orphaned') AND expires_at IS NOT NULL AND expires_at <= now()
	`, assetID)
	return err
}

type SecurityScanner interface {
	Scan(context.Context, []byte, string) error
}

type ImageSecurityScanner struct{}

func (ImageSecurityScanner) Scan(_ context.Context, data []byte, mime string) error {
	if mime != "image/jpeg" && mime != "image/png" {
		return fmt.Errorf("unsupported image MIME type %q", mime)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return fmt.Errorf("invalid image dimensions")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("decode image: %w", err)
	}
	if (mime == "image/jpeg" && format != "jpeg") || (mime == "image/png" && format != "png") {
		return fmt.Errorf("image content does not match declared MIME")
	}
	return nil
}

type ClamAVScanner struct {
	Address string
	Timeout time.Duration
}

func (c ClamAVScanner) Scan(ctx context.Context, data []byte, _ string) error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("ClamAV address is empty")
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.Address)
	if err != nil {
		return fmt.Errorf("connect ClamAV: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return fmt.Errorf("start ClamAV stream: %w", err)
	}
	for offset := 0; offset < len(data); {
		end := offset + 64*1024
		if end > len(data) {
			end = len(data)
		}
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(end-offset))
		if _, err := conn.Write(size[:]); err != nil {
			return fmt.Errorf("write ClamAV chunk: %w", err)
		}
		if _, err := conn.Write(data[offset:end]); err != nil {
			return fmt.Errorf("write ClamAV data: %w", err)
		}
		offset = end
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("finish ClamAV stream: %w", err)
	}
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read ClamAV response: %w", err)
	}
	response = strings.TrimSpace(response)
	if strings.HasSuffix(response, "FOUND") {
		return fmt.Errorf("malware detected by ClamAV: %s", response)
	}
	if !strings.HasSuffix(response, "OK") {
		return fmt.Errorf("ClamAV scan failed: %s", response)
	}
	return nil
}

type CompositeScanner []SecurityScanner

func (c CompositeScanner) Scan(ctx context.Context, data []byte, mime string) error {
	for _, scanner := range c {
		if scanner == nil {
			continue
		}
		if err := scanner.Scan(ctx, data, mime); err != nil {
			return err
		}
	}
	return nil
}

type Processor struct {
	Storage storage.StorageProvider
	Store   *Store
	Scanner SecurityScanner
}

func (p *Processor) Process(ctx context.Context, userID, projectID, generationID uuid.UUID, imageData []byte, mime string) (Asset, error) {
	if len(imageData) == 0 || int64(len(imageData)) > MaxAssetSize {
		return Asset{}, fmt.Errorf("asset exceeds %d bytes", MaxAssetSize)
	}
	detected := http.DetectContentType(imageData)
	if detected != "image/jpeg" && detected != "image/png" {
		return Asset{}, fmt.Errorf("unsupported image MIME type %q", detected)
	}
	if mime != "" && mime != detected {
		return Asset{}, fmt.Errorf("MIME mismatch: declared=%q detected=%q", mime, detected)
	}
	mime = detected

	if err := p.scanner().Scan(ctx, imageData, mime); err != nil {
		return Asset{}, fmt.Errorf("security scan failed: %w", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(imageData))
	if err != nil {
		return Asset{}, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > MaxImageDimension || cfg.Height > MaxImageDimension {
		return Asset{}, fmt.Errorf("invalid image dimensions")
	}
	if format != "jpeg" && format != "png" {
		return Asset{}, fmt.Errorf("unsupported image format %q", format)
	}

	exifData := readEXIF(imageData)
	normalized, err := normalizeImage(imageData, mime)
	if err != nil {
		return Asset{}, fmt.Errorf("normalize image: %w", err)
	}
	sum := sha256.Sum256(normalized)
	checksum := hex.EncodeToString(sum[:])

	assetID := uuid.New()
	originalKey, previewKey, thumbnailKey := storageKeys(userID, projectID, assetID)
	preview, err := makePreview(normalized, 1600)
	if err != nil {
		return Asset{}, fmt.Errorf("create preview: %w", err)
	}
	thumbnail, err := makePreview(normalized, 320)
	if err != nil {
		return Asset{}, fmt.Errorf("create thumbnail: %w", err)
	}

	item, created, err := p.Store.Reserve(ctx, userID, projectID, generationID, assetID, originalKey, previewKey, thumbnailKey, mime, int64(len(normalized)), cfg.Width, cfg.Height, checksum, exifData)
	if err != nil {
		return Asset{}, err
	}
	if !created {
		if item.LifecycleStatus == "active" {
			return item, nil
		}
		active, waitErr := p.Store.waitForActive(ctx, userID, projectID, checksum)
		if waitErr == nil {
			return active, nil
		}
		return Asset{}, fmt.Errorf("asset with checksum %s is already being processed", checksum)
	}

	uploaded := make([]string, 0, 3)
	upload := func(key string, data []byte, contentType string) error {
		if err := p.Storage.Put(ctx, key, bytes.NewReader(data), int64(len(data)), storage.PutOptions{
			ContentType: contentType,
			Metadata:    map[string]string{"sha256": checksum},
		}); err != nil {
			return err
		}
		uploaded = append(uploaded, key)
		return nil
	}
	cleanup := func() {
		for _, key := range uploaded {
			_ = p.Storage.Delete(context.Background(), key)
		}
		_ = p.Store.MarkOrphaned(context.Background(), userID, assetID)
	}

	if err := upload(originalKey, normalized, mime); err != nil {
		cleanup()
		return Asset{}, fmt.Errorf("upload original: %w", err)
	}
	if err := upload(previewKey, preview, "image/jpeg"); err != nil {
		cleanup()
		return Asset{}, fmt.Errorf("upload preview: %w", err)
	}
	if err := upload(thumbnailKey, thumbnail, "image/jpeg"); err != nil {
		cleanup()
		return Asset{}, fmt.Errorf("upload thumbnail: %w", err)
	}
	if err := p.Store.Finalize(ctx, userID, assetID); err != nil {
		return Asset{}, fmt.Errorf("finalize asset: %w", err)
	}
	item.LifecycleStatus = "active"
	item.ExpiresAt = nil
	return item, nil
}

func (p *Processor) CleanupExpired(ctx context.Context, limit int) error {
	items, err := p.Store.ListExpired(ctx, limit)
	if err != nil {
		return err
	}
	for _, item := range items {
		parts, partsErr := p.Store.ListUploadParts(ctx, item.UserID, item.ProjectID, item.ID)
		if partsErr == nil {
			for _, part := range parts {
				_ = deleteObjectIfPresent(ctx, p.Storage, part.StorageKey)
			}
		}
		_ = deleteObjectIfPresent(ctx, p.Storage, item.StorageKey)
		_ = deleteObjectIfPresent(ctx, p.Storage, stagingKey(item.StorageKey))
		if item.PreviewKey != nil {
			_ = deleteObjectIfPresent(ctx, p.Storage, *item.PreviewKey)
		}
		if item.ThumbnailKey != nil {
			_ = deleteObjectIfPresent(ctx, p.Storage, *item.ThumbnailKey)
		}
		if err := p.Store.DeleteExpired(ctx, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) scanner() SecurityScanner {
	if p.Scanner != nil {
		return p.Scanner
	}
	return ImageSecurityScanner{}
}

func deleteObjectIfPresent(ctx context.Context, provider storage.StorageProvider, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	return provider.Delete(ctx, key)
}

func stagingKey(storageKey string) string {
	return storageKey + "/staging"
}

func storageKeys(userID, projectID, assetID uuid.UUID) (string, string, string) {
	base := fmt.Sprintf("users/%s/projects/%s/assets/%s", userID, projectID, assetID)
	return base + "/original", base + "/preview", base + "/thumbnail"
}

func normalizeImage(data []byte, mime string) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	switch mime {
	case "image/jpeg":
		if err := jpeg.Encode(&out, src, &jpeg.Options{Quality: 95}); err != nil {
			return nil, err
		}
	case "image/png":
		if err := png.Encode(&out, src); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported image MIME type %q", mime)
	}
	return out.Bytes(), nil
}

func makePreview(data []byte, maxWidth int) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width > maxWidth {
		height = height * maxWidth / width
		width = maxWidth
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 88}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func readEXIF(data []byte) map[string]any {
	result := map[string]any{}
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return result
	}
	for _, tag := range []exif.FieldName{exif.Make, exif.Model, exif.DateTimeOriginal, exif.Software, exif.Orientation} {
		if value, err := x.Get(tag); err == nil {
			result[string(tag)] = strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return result
}

func (s *Store) waitForActive(ctx context.Context, userID, projectID uuid.UUID, checksum string) (Asset, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		item, err := s.lookupByChecksum(ctx, userID, projectID, checksum)
		if err == nil && item.LifecycleStatus == "active" {
			return item, nil
		}
		if err != nil && !errors.Is(err, ErrAssetNotFound) {
			return Asset{}, err
		}
		select {
		case <-ctx.Done():
			return Asset{}, ctx.Err()
		case <-deadline.C:
			return Asset{}, errors.New("timed out waiting for asset deduplication")
		case <-ticker.C:
		}
	}
}

func (s *Store) lookupByChecksum(ctx context.Context, userID, projectID uuid.UUID, checksum string) (Asset, error) {
	var item Asset
	var raw []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, generation_id, user_id, type, storage_key, preview_key, thumbnail_key, mime_type, size, width, height, checksum, exif, lifecycle_status, expires_at, deleted_at, created_at
		FROM assets WHERE user_id=$1 AND project_id=$2 AND checksum=$3
	`, userID, projectID, checksum).Scan(
		&item.ID, &item.ProjectID, &item.GenerationID, &item.UserID, &item.Type, &item.StorageKey, &item.PreviewKey, &item.ThumbnailKey,
		&item.MIMEType, &item.Size, &item.Width, &item.Height, &item.Checksum, &raw, &item.LifecycleStatus, &item.ExpiresAt, &item.DeletedAt, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Asset{}, ErrAssetNotFound
	}
	if err != nil {
		return Asset{}, err
	}
	_ = json.Unmarshal(raw, &item.EXIF)
	return item, nil
}
