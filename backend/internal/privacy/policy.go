package privacy

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

const (
	LocalOnly       = "local_only"
	ProviderAllowed = "provider_allowed"
	ProjectDefault  = "project_default"
)

var ErrExternalProviderBlocked = errors.New("external creative provider is blocked by project privacy mode")

type Policy struct{ db *sql.DB }

func NewPolicy(db *sql.DB) (*Policy, error) {
	if db == nil { return nil, errors.New("privacy policy requires database") }
	return &Policy{db: db}, nil
}

func (p *Policy) AllowsProvider(ctx context.Context, userID, projectID uuid.UUID, provider string) (bool, string, error) {
	var mode string
	if err := p.db.QueryRowContext(ctx, "SELECT privacy_mode FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted'", projectID, userID).Scan(&mode); err != nil {
		return false, "", err
	}
	if mode == LocalOnly && provider != "comfyui" {
		return false, mode, nil
	}
	return true, mode, nil
}


func (p *Policy) AllowsPlannerEndpoint(ctx context.Context, userID, projectID uuid.UUID, endpoint string) (bool, string, error) {
	var mode string
	if err := p.db.QueryRowContext(ctx, "SELECT privacy_mode FROM projects WHERE id=$1 AND user_id=$2 AND status <> 'deleted'", projectID, userID).Scan(&mode); err != nil {
		return false, "", err
	}
	if mode != LocalOnly {
		return true, mode, nil
	}
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || u.Hostname() == "" {
		return false, mode, nil
	}
	host := strings.ToLower(u.Hostname())
	if isLocalEndpointHost(host) {
		return true, mode, nil
	}
	return false, mode, nil
}


func isLocalEndpointHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
