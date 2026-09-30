package assetlibrary

import (
  "database/sql"
  "errors"
  "strings"
  "time"

  "github.com/google/uuid"
  "github.com/oleg3190/Web-studio-img/backend/internal/storage"
)

var (
  errSourceAssetNotFound = errors.New("source asset not found")
)

var allowedTypes = map[string]struct{}{
  "character": {}, "object": {}, "product": {}, "logo": {}, "symbol": {},
  "background": {}, "texture": {}, "material": {}, "mask": {}, "image": {}, "other": {},
}
var allowedRights = map[string]struct{}{
  "inherited": {}, "verified": {}, "unverified": {}, "restricted": {}, "unknown": {},
}

type Handler struct {
  db      *sql.DB
  storage storage.StorageProvider
}

type Item struct {
  ID             uuid.UUID      `json:"id"`
  UserID         uuid.UUID      `json:"user_id"`
  Name           string         `json:"name"`
  Description    *string        `json:"description,omitempty"`
  AssetType      string         `json:"asset_type"`
  Tags           []string       `json:"tags"`
  Status         string         `json:"status"`
  CurrentVersion int            `json:"current_version"`
  CreatedAt      time.Time      `json:"created_at"`
  UpdatedAt      time.Time      `json:"updated_at"`
  Current        *Version       `json:"current,omitempty"`
}

type Version struct {
  ID              uuid.UUID      `json:"id"`
  LibraryItemID   uuid.UUID      `json:"library_item_id"`
  Version         int            `json:"version"`
  SourceAssetID   *uuid.UUID     `json:"source_asset_id,omitempty"`
  SourceProjectID *uuid.UUID     `json:"source_project_id,omitempty"`
  StorageKey      string         `json:"-"`
  PreviewKey      *string        `json:"-"`
  ThumbnailKey    *string        `json:"-"`
  MIMEType        string         `json:"mime_type"`
  Size            int64          `json:"size"`
  Width           int            `json:"width"`
  Height          int            `json:"height"`
  Checksum        string         `json:"checksum"`
  RightsSnapshot  map[string]any `json:"rights_snapshot"`
  Provenance      map[string]any `json:"provenance"`
  OriginalURL     string         `json:"original_url,omitempty"`
  PreviewURL      string         `json:"preview_url,omitempty"`
  ThumbnailURL    string         `json:"thumbnail_url,omitempty"`
  CreatedAt       time.Time      `json:"created_at"`
}

type Usage struct {
  ID               uuid.UUID `json:"id"`
  LibraryVersionID uuid.UUID `json:"library_version_id"`
  UserID           uuid.UUID `json:"user_id"`
  ProjectID        uuid.UUID `json:"project_id"`
  ProjectName      string    `json:"project_name,omitempty"`
  RightsStatus     string    `json:"rights_status"`
  RightsNotes      *string   `json:"rights_notes,omitempty"`
  CreatedAt        time.Time `json:"created_at"`
}

type SourceAsset struct {
  ID         uuid.UUID `json:"id"`
  ProjectID  uuid.UUID `json:"project_id"`
  Type       string    `json:"type"`
  MIMEType   string    `json:"mime_type"`
  Size       int64     `json:"size"`
  Width      int       `json:"width"`
  Height     int       `json:"height"`
  Checksum   string    `json:"checksum"`
  PreviewURL string    `json:"preview_url,omitempty"`
  CreatedAt  time.Time `json:"created_at"`
}

type SourceAssetDB struct {
  ID uuid.UUID
  ProjectID uuid.UUID
  StorageKey string
  PreviewKey *string
  ThumbnailKey *string
  MIMEType string
  Size int64
  Width int
  Height int
  Checksum string
  CreatedAt time.Time
}

func NewHandler(db *sql.DB, provider storage.StorageProvider) (*Handler, error) {
  if db == nil {
    return nil, errors.New("asset library requires database")
  }
  return &Handler{db: db, storage: provider}, nil
}

func validType(value string) bool {
  _, ok := allowedTypes[strings.TrimSpace(value)]
  return ok
}

func validRights(value string) bool {
  _, ok := allowedRights[strings.TrimSpace(value)]
  return ok
}

func normalizeTags(tags []string) []string {
  seen := make(map[string]struct{}, len(tags))
  out := make([]string, 0, len(tags))
  for _, tag := range tags {
    tag = strings.ToLower(strings.TrimSpace(tag))
    if tag == "" || len([]rune(tag)) > 100 {
      continue
    }
    if _, ok := seen[tag]; ok {
      continue
    }
    seen[tag] = struct{}{}
    out = append(out, tag)
  }
  if len(out) > 50 {
    out = out[:50]
  }
  return out
}
