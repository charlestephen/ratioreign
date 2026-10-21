package api

import (
	"embed"
	"io/fs"
	"net/http"
)

// The web UI is a Vue SPA built from webui/ (see webui/package.json); the
// production build is committed under dist/ and embedded here so the Go
// binary stays self-contained. Build it with `npm run build` in webui/
// after touching the frontend.
//
//go:embed all:webui/dist
var distFS embed.FS

var indexHTML = mustReadIndex()

func mustReadIndex() []byte {
	data, err := fs.ReadFile(distFS, "webui/dist/index.html")
	if err != nil {
		panic("webui: embedded dist/index.html missing — run `npm run build` in webui/: " + err.Error())
	}
	return data
}

func registerUI(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", handleIndex)
	assets, err := fs.Sub(distFS, "webui/dist")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /assets/", http.FileServerFS(assets))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}
