package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Purger invalidates CDN copies of the given absolute URLs.
type Purger interface {
	Purge(ctx context.Context, urls []string) error
}

// NoopPurger does nothing; used in dev and training.
type NoopPurger struct{}

// Purge implements Purger.
func (NoopPurger) Purge(context.Context, []string) error { return nil }

// cloudflareMaxFiles is the API limit of URLs per purge request.
const cloudflareMaxFiles = 30

// CloudflarePurger purges URLs through the Cloudflare zone API.
type CloudflarePurger struct {
	ZoneID string
	Token  string
	// Endpoint overrides the API base (tests only).
	Endpoint string
	Client   *http.Client
}

// NewCloudflarePurger builds a purger with a 10 s HTTP timeout.
func NewCloudflarePurger(zoneID, token string) *CloudflarePurger {
	return &CloudflarePurger{ZoneID: zoneID, Token: token, Client: &http.Client{Timeout: 10 * time.Second}}
}

// Purge sends the URLs in batches of 30 (the API maximum).
func (p *CloudflarePurger) Purge(ctx context.Context, urls []string) error {
	for len(urls) > 0 {
		n := min(len(urls), cloudflareMaxFiles)
		if err := p.purgeBatch(ctx, urls[:n]); err != nil {
			return err
		}
		urls = urls[n:]
	}
	return nil
}

func (p *CloudflarePurger) purgeBatch(ctx context.Context, urls []string) error {
	base := p.Endpoint
	if base == "" {
		base = "https://api.cloudflare.com/client/v4"
	}
	body, err := json.Marshal(map[string][]string{"files": urls})
	if err != nil {
		return fmt.Errorf("encode purge request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/zones/"+p.ZoneID+"/purge_cache", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build purge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare purge: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("cloudflare purge: status %d: %s", res.StatusCode, bytes.TrimSpace(data))
	}
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(data, &out); err != nil || !out.Success {
		return errors.New("cloudflare purge: response not successful: " + string(bytes.TrimSpace(data)))
	}
	return nil
}
