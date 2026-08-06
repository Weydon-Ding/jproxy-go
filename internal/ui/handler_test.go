package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_ServeIndex(t *testing.T) {
	handler := NewHandler()

	tests := []struct {
		path           string
		method         string
		expectedStatus int
		expectedType   string
	}{
		{"/", http.MethodGet, http.StatusOK, "text/html"},
		{"/", http.MethodHead, http.StatusOK, "text/html"},
		{"/index.html", http.MethodGet, http.StatusOK, "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, tt.expectedType) {
				t.Errorf("expected content-type %s, got %s", tt.expectedType, contentType)
			}
		})
	}
}

func TestHandler_IndexMethodNotAllowed(t *testing.T) {
	handler := NewHandler()

	tests := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range tests {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", w.Code)
			}

			allow := w.Header().Get("Allow")
			if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") {
				t.Errorf("expected Allow header with GET, HEAD, got %s", allow)
			}
		})
	}
}

func TestHandler_ServeAssets(t *testing.T) {
	handler := NewHandler()

	assets := []struct {
		path           string
		expectedType   string
	}{
		{"/assets/app.css", "text/css"},
		{"/assets/app.js", "application/javascript"},
		{"/assets/core.js", "application/javascript"},
		{"/assets/auth-config.js", "application/javascript"},
		{"/assets/rules.js", "application/javascript"},
		{"/assets/examples.js", "application/javascript"},
		{"/assets/titles.js", "application/javascript"},
	}

	for _, asset := range assets {
		t.Run(asset.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, asset.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d", w.Code)
			}

			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, asset.expectedType) {
				t.Errorf("expected content-type %s, got %s", asset.expectedType, contentType)
			}

			cacheControl := w.Header().Get("Cache-Control")
			if !strings.Contains(cacheControl, "max-age=3600") {
				t.Errorf("expected cache header with max-age=3600, got %s", cacheControl)
			}
		})
	}
}

func TestHandler_AssetMethodNotAllowed(t *testing.T) {
	handler := NewHandler()

	req := httptest.NewRequest(http.MethodPost, "/assets/app.js", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}

	allow := w.Header().Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "HEAD") {
		t.Errorf("expected Allow header with GET, HEAD, got %s", allow)
	}
}

func TestHandler_UnknownAsset404(t *testing.T) {
	handler := NewHandler()

	unknownAssets := []string{
		"/assets/nonexistent.js",
		"/assets/old-app.js",
		"/assets/vendor.js",
		"/assets/style.css",
	}

	for _, path := range unknownAssets {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("expected 404 for %s, got %d", path, w.Code)
			}
		})
	}
}

func TestHandler_UnknownPath404(t *testing.T) {
	handler := NewHandler()

	tests := []struct {
		path string
	}{
		{"/unknown"},
		{"/foo/bar"},
		{"/api/system/config"},
		{"/sonarr/jackett/"},
		{"/radarr/prowlarr/"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("expected status 404 for %s, got %d", tt.path, w.Code)
			}

			body := w.Body.String()
			if strings.Contains(body, "<!DOCTYPE html>") {
				t.Error("unknown paths should not return HTML fallback")
			}
		})
	}
}

func TestHandler_NoHTMLFallback(t *testing.T) {
	handler := NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	body := w.Body.String()
	if strings.Contains(body, "<!DOCTYPE html>") || strings.Contains(body, "<html") {
		t.Error("unknown paths should not return HTML fallback")
	}
}

func TestHandler_CacheHeaders(t *testing.T) {
	handler := NewHandler()

	t.Run("index.html no-cache", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/index.html", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		cacheControl := w.Header().Get("Cache-Control")
		if !strings.Contains(cacheControl, "no-cache") || !strings.Contains(cacheControl, "no-store") {
			t.Errorf("expected no-cache headers for index.html, got %s", cacheControl)
		}
	})

	t.Run("assets cacheable", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		cacheControl := w.Header().Get("Cache-Control")
		if !strings.Contains(cacheControl, "public") || !strings.Contains(cacheControl, "max-age=3600") {
			t.Errorf("expected cacheable headers for assets, got %s", cacheControl)
		}
	})
}

func TestHandler_ProxyPathsNotServed(t *testing.T) {
	handler := NewHandler()

	proxyPaths := []string{
		"/sonarr/jackett/",
		"/radarr/prowlarr/",
		"/sonarr/prowlarr/1/",
		"/radarr/jackett/api/v2.0/",
	}

	for _, path := range proxyPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("expected 404 for proxy path %s, got %d", path, w.Code)
			}

			body := w.Body.String()
			if strings.Contains(body, "<!DOCTYPE html>") {
				t.Error("proxy paths should not return HTML from UI handler")
			}
		})
	}
}

func TestHandler_EmbedBuild(t *testing.T) {
	handler := NewHandler()

	if handler == nil {
		t.Fatal("handler should not be nil")
	}

	if handler.fs == nil {
		t.Fatal("handler.fs should not be nil")
	}
}

func TestHandler_AllowedAssets(t *testing.T) {
	handler := NewHandler()

	for filename := range allowedAssets {
		path := "/assets/" + filename
		if filename == "index.html" {
			path = "/"
		}

		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("allowed asset %s should return 200, got %d", path, w.Code)
		}
	}
}

func TestJSFiles_NoXSSSinks(t *testing.T) {
	xssPatterns := []string{".innerHTML", ".outerHTML", "insertAdjacentHTML", "document.write"}
	jsFiles := []string{"core.js", "auth-config.js", "rules.js", "examples.js", "titles.js", "app.js"}

	for _, filename := range jsFiles {
		t.Run(filename, func(t *testing.T) {
			content, err := staticFiles.ReadFile("static/" + filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", filename, err)
			}

			for _, pattern := range xssPatterns {
				if strings.Contains(string(content), pattern) {
					t.Errorf("%s contains XSS sink: %s", filename, pattern)
				}
			}
		})
	}
}

func TestJSFiles_NoImplicitEvent(t *testing.T) {
	jsFiles := []string{"core.js", "auth-config.js", "rules.js", "examples.js", "titles.js", "app.js"}

	for _, filename := range jsFiles {
		t.Run(filename, func(t *testing.T) {
			content, err := staticFiles.ReadFile("static/" + filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", filename, err)
			}

			if strings.Contains(string(content), "event.target") || strings.Contains(string(content), "event.currentTarget") {
				t.Errorf("%s uses implicit global event", filename)
			}
		})
	}
}

func TestJSFiles_NoPrompt(t *testing.T) {
	jsFiles := []string{"core.js", "auth-config.js", "rules.js", "examples.js", "titles.js", "app.js"}

	for _, filename := range jsFiles {
		t.Run(filename, func(t *testing.T) {
			content, err := staticFiles.ReadFile("static/" + filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", filename, err)
			}

			if strings.Contains(string(content), "prompt(") {
				t.Errorf("%s uses prompt()", filename)
			}
		})
	}
}

func TestJSFiles_MaxPureLOC(t *testing.T) {
	jsFiles := []string{"core.js", "auth-config.js", "rules.js", "examples.js", "titles.js", "app.js"}

	for _, filename := range jsFiles {
		t.Run(filename, func(t *testing.T) {
			content, err := staticFiles.ReadFile("static/" + filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", filename, err)
			}

			lines := strings.Split(string(content), "\n")
			pureLOC := 0
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
					pureLOC++
				}
			}

			if pureLOC > 250 {
				t.Errorf("%s has %d pure LOC, exceeds 250 limit", filename, pureLOC)
			}
		})
	}
}

func TestIndexHTML_ReferencesAllJSFiles(t *testing.T) {
	indexContent, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	expectedScripts := []string{"core.js", "auth-config.js", "rules.js", "examples.js", "titles.js", "app.js"}
	html := string(indexContent)

	for _, script := range expectedScripts {
		if !strings.Contains(html, script) {
			t.Errorf("index.html does not reference %s", script)
		}
	}
}

func TestIndexHTML_NoFaviconRequest(t *testing.T) {
	indexContent, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	html := string(indexContent)
	if !strings.Contains(html, `rel="icon"`) && !strings.Contains(html, `rel='icon'`) {
		t.Error("index.html lacks favicon declaration, will cause 404 request")
	}
}

func TestIndexHTML_AccountTabHasStableId(t *testing.T) {
	indexContent, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	html := string(indexContent)
	if !strings.Contains(html, `id="tab-account"`) {
		t.Error("account tab button lacks stable id='tab-account'")
	}
	if !strings.Contains(html, `id="account-section"`) {
		t.Error("account panel lacks stable id='account-section'")
	}
}

func TestAppJS_TabActivation_ScrollsIntoView(t *testing.T) {
	appContent, err := staticFiles.ReadFile("static/app.js")
	if err != nil {
		t.Fatalf("failed to read app.js: %v", err)
	}

	js := string(appContent)
	if !strings.Contains(js, "scrollIntoView") {
		t.Error("app.js tab activation lacks scrollIntoView for mobile visibility")
	}
}

func TestCSS_NavScrollAffordance(t *testing.T) {
	cssContent, err := staticFiles.ReadFile("static/app.css")
	if err != nil {
		t.Fatalf("failed to read app.css: %v", err)
	}

	css := string(cssContent)
	if !strings.Contains(css, "mask") && !strings.Contains(css, "gradient") {
		t.Error("app.css lacks visual affordance for horizontal nav scroll")
	}
}
