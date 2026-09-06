// Package announce sends HTTP BitTorrent tracker announces and parses the
// bencoded response. Like Joal, ratioreign only speaks the HTTP tracker
// protocol (no UDP, no scrape) — HTTP trackers are the overwhelming majority
// on private trackers, which is what ratio-farming tools target.
package announce

import (
	"context"
	"fmt"
	"io"
	"math"
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
		r.Interval = clampInt(v, maxReasonableInterval)
	}
	if v, ok := dict["complete"].(int64); ok {
		r.Complete = clampInt(v, maxReasonablePeerCount)
	}
	if v, ok := dict["incomplete"].(int64); ok {
		r.Incomplete = clampInt(v, maxReasonablePeerCount)
	}
	return r, nil
}

const (
	// maxReasonableInterval caps a tracker-supplied announce interval at one
	// day; real trackers use minutes, never more than a few hours.
	maxReasonableInterval = 24 * 60 * 60
	// maxReasonablePeerCount is a generous upper bound for seeder/leecher
	// counts — real swarms never get remotely this large.
	maxReasonablePeerCount = 1 << 20
)

// clampInt converts a tracker-supplied int64 (interval/seeder/leecher counts
// from a bencoded announce response) to int, clamped to [0, max]. A
// malicious or broken tracker can put an arbitrary int64 in these fields;
// converting straight to int truncates on 32-bit platforms and can produce
// a negative or nonsensical value that then drives scheduling logic (e.g.
// the next-announce time), so out-of-range values are clamped rather than
// trusted as-is. v is clamped directly against math.MaxInt32 (not just
// against max) immediately before the conversion below, so the function is
// safe on its own terms regardless of what max the caller passes in — see
// TestClampInt / TestParseResponseClampsOutOfRangeValues.
func clampInt(v int64, max int64) int {
	if v < 0 {
		v = 0
	}
	if v > max {
		v = max
	}
	if v > math.MaxInt32 {
		v = math.MaxInt32
	}
	// codeql[go/incorrect-integer-conversion]: v is bounds-checked against
	// math.MaxInt32 immediately above; CodeQL's dataflow analysis doesn't
	// recognize this reassign-then-convert shape as a sufficient guard
	// (confirmed by reshaping this three different ways — a direct early
	// return, a single-variable clamp chain, this one — all still flagged),
	// but the bound is real and covered by unit tests.
	return int(v)
}

func host(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Host
}
