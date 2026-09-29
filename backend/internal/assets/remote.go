package assets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const remoteImportTimeout = 20 * time.Second

func (h *UploadHandler) importURL(w http.ResponseWriter, r *http.Request) {
	userID, ok := uploadUserID(r)
	if !ok {
		uploadErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	projectID, err := uuid.Parse(r.PathValue("project_id"))
	if err != nil {
		uploadErr(w, http.StatusBadRequest, "invalid_project_id", "invalid project id")
		return
	}

	var input struct {
		URL string `json:"url"`
	}
	if err := decodeUploadJSON(r, &input); err != nil {
		uploadErr(w, http.StatusBadRequest, "invalid_request", "invalid URL import payload")
		return
	}

	target, err := validateRemoteImportURL(r.Context(), input.URL)
	if err != nil {
		uploadErr(w, http.StatusBadRequest, "invalid_image_url", err.Error())
		return
	}

	client := &http.Client{
		Timeout: remoteImportTimeout,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if _, err := validateRemoteImportURL(req.Context(), req.URL.String()); err != nil {
				return err
			}
			if req.URL.User != nil {
				return errors.New("image URL must not contain credentials")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		uploadErr(w, http.StatusBadRequest, "invalid_image_url", "invalid image URL")
		return
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/jpeg;q=0.9,*/*;q=0.1")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("User-Agent", "WebStudioIMG/1.0")

	resp, err := client.Do(req)
	if err != nil {
		uploadErr(w, http.StatusBadGateway, "remote_fetch_failed", "could not fetch image from URL")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		uploadErr(w, http.StatusBadGateway, "remote_fetch_failed", fmt.Sprintf("image source returned HTTP %d", resp.StatusCode))
		return
	}
	if resp.ContentLength > MaxAssetSize {
		uploadErr(w, http.StatusRequestEntityTooLarge, "file_too_large", "remote image exceeds 10 MB")
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxAssetSize+1))
	if err != nil {
		uploadErr(w, http.StatusBadGateway, "remote_read_failed", "could not read remote image")
		return
	}
	if int64(len(data)) > MaxAssetSize {
		uploadErr(w, http.StatusRequestEntityTooLarge, "file_too_large", "remote image exceeds 10 MB")
		return
	}

	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	item, err := h.processor.ProcessUpload(r.Context(), userID, projectID, data, mime)
	if err != nil {
		uploadErr(w, http.StatusBadRequest, "asset_upload_failed", err.Error())
		return
	}

	h.emit(r, userID, projectID, item.ID, "asset_imported_from_url", map[string]any{
		"type":       item.Type,
		"storage_key": item.StorageKey,
		"checksum":   item.Checksum,
		"source_url": target.String(),
	})
	uploadJSON(w, http.StatusCreated, item)
}

func validateRemoteImportURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, errors.New("invalid image URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("image URL must use http or https")
	}
	if parsed.User != nil || parsed.Hostname() == "" {
		return nil, errors.New("image URL must not contain credentials and must include a host")
	}

	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return nil, errors.New("private image hosts are not allowed")
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if isBlockedRemoteIP(ip) {
			return nil, errors.New("private image hosts are not allowed")
		}
		return parsed, nil
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("image host could not be resolved")
	}
	for _, resolvedIP := range ips {
		if isBlockedRemoteIP(resolvedIP) {
			return nil, errors.New("private image hosts are not allowed")
		}
	}
	return parsed, nil
}

func isBlockedRemoteIP(ip net.IP) bool {
	return ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}
