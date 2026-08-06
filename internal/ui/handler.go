// Package ui provides the embedded admin console static assets and HTTP handler.
package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static/*
var staticFiles embed.FS

type assetMeta struct {
	contentType string
	methods     []string
}

var allowedAssets = map[string]assetMeta{
	"index.html":     {contentType: "text/html; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
	"app.css":        {contentType: "text/css; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
	"core.js":        {contentType: "application/javascript; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
	"auth-config.js": {contentType: "application/javascript; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
	"rules.js":       {contentType: "application/javascript; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
	"examples.js":    {contentType: "application/javascript; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
	"titles.js":      {contentType: "application/javascript; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
	"app.js":         {contentType: "application/javascript; charset=utf-8", methods: []string{http.MethodGet, http.MethodHead}},
}

type Handler struct {
	fs http.FileSystem
}

func NewHandler() *Handler {
	subFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic("ui: failed to create static subdirectory fs: " + err.Error())
	}
	return &Handler{
		fs: http.FS(subFS),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	if path == "/" || path == "/index.html" {
		meta := allowedAssets["index.html"]
		if !allowedMethod(method, meta.methods) {
			w.Header().Set("Allow", strings.Join(meta.methods, ", "))
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.serveFile(w, r, "index.html", meta.contentType)
		return
	}

	if strings.HasPrefix(path, "/assets/") {
		filename := strings.TrimPrefix(path, "/assets/")
		meta, ok := allowedAssets[filename]
		if !ok {
			http.NotFound(w, r)
			return
		}

		if !allowedMethod(method, meta.methods) {
			w.Header().Set("Allow", strings.Join(meta.methods, ", "))
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		h.serveFile(w, r, filename, meta.contentType)
		return
	}

	http.NotFound(w, r)
}

func allowedMethod(method string, allowed []string) bool {
	for _, m := range allowed {
		if m == method {
			return true
		}
	}
	return false
}

func (h *Handler) serveFile(w http.ResponseWriter, r *http.Request, name, contentType string) {
	file, err := h.fs.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)

	if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Expires", "0")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), file)
}
