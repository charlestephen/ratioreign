// Package announce sends HTTP BitTorrent tracker announces and parses the
// bencoded response. Like Joal, ratioreign only speaks the HTTP tracker
// protocol (no UDP, no scrape) — HTTP trackers are the overwhelming majority
// on private trackers, which is what ratio-farming tools target.
package announce

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/bencode"
	"git.lan.cst.wtf/charlestephen/ratioreign/internal/profile"
)

type Response struct {
	Interval       int // seconds until next regular announce
	Complete       int // seeders
	Incomplete     int // leechers
	FailureReason  string
	WarningMessage string
}

// Client sends announce requests using a specific client profile's session
// identity.
type Client struct {
	HTTP    *http.Client
	Profile *profile.Profile
}

func NewClient(p *profile.Profile) *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}, Profile: p}
}

// Announce sends one announce to trackerURL and returns the parsed response.
func (c *Client) Announce(ctx context.Context, session *profile.Session, trackerURL string, params profile.QueryParams) (*Response, error) {
	if err := session.Refresh(); err != nil {
		return nil, err
	}
	query, err := session.BuildQuery(params)
	if err != nil {
		return nil, err
	}

	full := trackerURL
	if strings.Contains(trackerURL, "?") {
		full += "&" + query
	} else {
		full += "?" + query
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	for _, h := range c.Profile.BuildHeaders() {
		req.Header.Set(h.Name, h.Value)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("announce: tracker %s returned HTTP %d", host(trackerURL), resp.StatusCode)
	}
	return parseResponse(body)
}

func parseResponse(body []byte) (*Response, error) {
	raw, _, err := bencode.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("announce: invalid tracker response: %w", err)
	}
	dict, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("announce: tracker response is not a dict")
	}
	r := &Response{}
	if v, ok := dict["failure reason"].([]byte); ok {
		r.FailureReason = string(v)
		return r, nil
	}
	if v, ok := dict["warning message"].([]byte); ok {
		r.WarningMessage = string(v)
	}
	if v, ok := dict["interval"].(int64); ok {
		r.Interval = int(v)
	}
	if v, ok := dict["complete"].(int64); ok {
		r.Complete = int(v)
	}
	if v, ok := dict["incomplete"].(int64); ok {
		r.Incomplete = int(v)
	}
	return r, nil
}

func host(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
