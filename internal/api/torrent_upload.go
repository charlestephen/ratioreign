package api

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"git.lan.cst.wtf/charlestephen/ratioreign/internal/torrentfile"
)

// maxTorrentUpload is generous — real .torrent files are almost always a
// few KB to a few hundred KB (they hold metadata, not payload data).
const maxTorrentUpload = 20 << 20

func (s *Server) handleUploadTorrent(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxTorrentUpload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload: "+err.Error())
		return
	}
	file, header, err := r.FormFile("torrent")
	if err != nil {
		writeError(w, http.StatusBadRequest, `missing "torrent" file field: `+err.Error())
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxTorrentUpload))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read upload: "+err.Error())
		return
	}

	if err := os.MkdirAll(s.Config.TorrentsDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// The uploaded filename isn't a trustworthy path component; derive a
	// safe one from the content hash instead. That also makes re-uploading
	// the same file idempotent, matching how the RSS and qBittorrent intake
	// paths already name their files.
	sum := sha1.Sum(data)
	name := sanitizeFilename(header.Filename)
	if name == "" {
		name = "upload"
	}
	path := filepath.Join(s.Config.TorrentsDir, fmt.Sprintf("%s-%x.torrent", name, sum[:8]))

	t, err := torrentfile.Parse(path, data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid torrent file: "+err.Error())
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Manager.Add(t)
	writeJSON(w, http.StatusCreated, map[string]string{"infoHash": t.InfoHashHex(), "name": t.Name})
}

func sanitizeFilename(name string) string {
	name = strings.TrimSuffix(filepath.Base(name), ".torrent")
	b := []byte(name)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			b[i] = '_'
		}
	}
	return string(b)
}
