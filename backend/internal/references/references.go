package references

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SourceType string

const (
	SourceInspiration SourceType = "inspiration"
	SourceReference SourceType = "reference"
	SourceDirect SourceType = "direct_source"
	SourceUserCreated SourceType = "user_created"
	SourcePublicDomain SourceType = "public_domain"
	SourceUnknown SourceType = "unknown"
)

var (
	ErrInvalidReference = errors.New("invalid reference")
	ErrReferenceNotFound = errors.New("reference not found")
)

var sha256Pattern = regexp.MustCompile("^[0-9a-fA-F]{64}$")

type Influence struct {
	Composition float64 `json:"composition"`
	Semantic    float64 `json:"semantic"`
	Color       float64 `json:"color"`
	Style       float64 `json:"style"`
	Material    float64 `json:"material"`
	Geometry    float64 `json:"geometry"`
	Warning     string  `json:"warning,omitempty"`
}

type Reference struct {
	ID              uuid.UUID  `json:"id"`
	ProjectID       uuid.UUID  `json:"project_id"`
	AssetID         *uuid.UUID `json:"asset_id,omitempty"`
	SourceURL       *string    `json:"source_url,omitempty"`
	SourceType      SourceType `json:"source_type"`
	License         string     `json:"license"`
	LicenseVerified bool       `json:"license_verified"`
	UserOwned       bool       `json:"user_owned"`
	SHA256          *string    `json:"sha256,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
	Influence       *Influence `json:"influence,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Request struct {
	AssetID         *uuid.UUID
	SourceURL       *string
	SourceType      SourceType
	License         string
	LicenseVerified bool
	UserOwned       bool
	SHA256          *string
	Notes           *string
	Influence       *Influence
}

func NormalizeInfluence(input *Influence) *Influence {
	if input == nil { return nil }
	value := *input
	value.Warning = ""
	if value.Composition >= 0.8 { value.Warning = "High composition similarity; review before use." }
	return &value
}

func (r Request) Validate() error {
	switch r.SourceType {
	case SourceInspiration, SourceReference, SourceDirect, SourceUserCreated, SourcePublicDomain, SourceUnknown:
	default:
		return ErrInvalidReference
	}
	r.License = strings.TrimSpace(r.License)
	if r.License == "" || len([]rune(r.License)) > 500 {
		return ErrInvalidReference
	}
	if r.SourceURL == nil && r.AssetID == nil {
		return ErrInvalidReference
	}
	if r.SourceURL != nil && (len([]rune(*r.SourceURL)) > 4000 || strings.TrimSpace(*r.SourceURL) == "") {
		return ErrInvalidReference
	}
	if r.Notes != nil && len([]rune(*r.Notes)) > 10000 {
		return ErrInvalidReference
	}
	if r.SHA256 != nil && !sha256Pattern.MatchString(*r.SHA256) {
		return ErrInvalidReference
	}
	if r.Influence != nil {
		for _, score := range []float64{r.Influence.Composition, r.Influence.Semantic, r.Influence.Color, r.Influence.Style, r.Influence.Material, r.Influence.Geometry} {
			if score < 0 || score > 1 { return ErrInvalidReference }
		}
	}
	return nil
}
