package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jproxy-go/internal/ui"
)

// TestUIHandler_Isolation verifies UI handler serves only intended paths
func TestUIHandler_Isolation(t *testing.T) {
	handler := ui.NewHandler()

	t.Run("Root serves HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for root path, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("expected text/html, got %s", contentType)
		}
	})

	t.Run("API path returns 404 from UI handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/system/config/query", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for API path from UI handler, got %d", w.Code)
		}

		body := w.Body.String()
		if strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("API paths should not return HTML fallback")
		}
	})

	t.Run("Proxy path returns 404 from UI handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sonarr/jackett/", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for proxy path from UI handler, got %d", w.Code)
		}

		body := w.Body.String()
		if strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("proxy paths should not return HTML fallback")
		}
	})

	t.Run("Unknown path returns 404 without HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unknown path, got %d", w.Code)
		}

		body := w.Body.String()
		if strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("unknown paths should not return HTML fallback")
		}
	})
}
