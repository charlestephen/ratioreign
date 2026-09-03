// Package qbittorrent talks to the qBittorrent WebUI API so ratioreign can
// mirror whatever's already added to a real qBittorrent instance: any
// torrent that shows up there gets its .torrent file pulled and dropped into
// ratioreign's watched folder to be seeded (faked) independently.
package qbittorrent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
)

func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

func NewClient(baseURL, username, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		http:     &http.Client{Jar: jar},
	}, nil
}

// Login authenticates and stores the SID session cookie in the client's
// cookie jar for subsequent requests.
func (c *Client) Login(ctx context.Context) error {
	form := url.Values{"username": {c.username}, "password": {c.password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// qBittorrent's login handler validates the Referer header matches its
	// own origin; without this every login is rejected with HTTP 403.
	req.Header.Set("Referer", c.baseURL)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != "Ok." {
		return fmt.Errorf("qbittorrent: login failed (HTTP %d): %s", resp.StatusCode, body)
	}
	return nil
}

// TorrentInfo is the subset of qBittorrent's torrents/info response
// ratioreign needs.
type TorrentInfo struct {
	Hash     string `json:"hash"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// ListTorrents returns every torrent qBittorrent knows about, optionally
// filtered to a single category (empty string = all).
func (c *Client) ListTorrents(ctx context.Context, category string) ([]TorrentInfo, error) {
	u := c.baseURL + "/api/v2/torrents/info"
	if category != "" {
		u += "?category=" + url.QueryEscape(category)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return nil, errNotAuthenticated
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qbittorrent: torrents/info returned HTTP %d", resp.StatusCode)
	}
	var torrents []TorrentInfo
	if err := decodeJSON(resp.Body, &torrents); err != nil {
		return nil, err
	}
	return torrents, nil
}

// ExportTorrent downloads the raw .torrent file for hash, using the
// torrents/export endpoint (qBittorrent >= 4.5 / WebAPI >= 2.8.19).
func (c *Client) ExportTorrent(ctx context.Context, hash string) ([]byte, error) {
	u := c.baseURL + "/api/v2/torrents/export?hash=" + url.QueryEscape(hash)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		return nil, errNotAuthenticated
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("qbittorrent: torrents/export not supported by this qBittorrent version (need >= 4.5)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qbittorrent: torrents/export returned HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 20<<20))
}

var errNotAuthenticated = fmt.Errorf("qbittorrent: not authenticated (session expired)")

// IsNotAuthenticated reports whether err indicates the session cookie
// expired and Login should be retried.
func IsNotAuthenticated(err error) bool { return err == errNotAuthenticated }
