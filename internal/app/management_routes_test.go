package app

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"jproxy-go/internal/config"
	"jproxy-go/internal/runtime"
	"jproxy-go/internal/store/sqlite"
)

func TestRootHandler_exposesTodo7AndTodo8RoutesOnlyInDatabaseMode(t *testing.T) {
	// Given
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "root-routes.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	seedRootConfigs(t, store)
	snapshot, err := store.FormatterSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg := rootRouteConfig(true)
	databaseHandler := rootHandler(cfg, runtime.NewProvider(snapshot, store), store)
	environmentHandler := rootHandler(rootRouteConfig(false), runtime.NewStaticProvider(sqlite.Snapshot{}), nil)

	// When / Then
	routes := managementRouteContracts()
	for _, route := range routes {
		t.Run(route.method+route.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			databaseHandler.ServeHTTP(response, httptest.NewRequest(route.method, route.path, bytes.NewReader(route.body)))
			if response.Code != route.status || response.Header().Get("Content-Type") != route.contentType {
				t.Fatalf("database status=%d type=%q body=%q", response.Code, response.Header().Get("Content-Type"), response.Body.String())
			}

			environment := httptest.NewRecorder()
			environmentHandler.ServeHTTP(environment, httptest.NewRequest(route.method, route.path, bytes.NewReader(route.body)))
			if environment.Code != http.StatusNotFound {
				t.Fatalf("environment route status=%d", environment.Code)
			}
		})
	}
	if len(routes) != 35 {
		t.Fatalf("route_count=%d", len(routes))
	}

	wrongMethod := httptest.NewRecorder()
	databaseHandler.ServeHTTP(wrongMethod, httptest.NewRequest(http.MethodGet, "/api/sonarr/rule/save", nil))
	if wrongMethod.Code != http.StatusMethodNotAllowed || wrongMethod.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("wrong method=%d allow=%q", wrongMethod.Code, wrongMethod.Header().Get("Allow"))
	}
	unknown := httptest.NewRecorder()
	databaseHandler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/api/sonarr/title/unknown", nil))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown=%d", unknown.Code)
	}
	t.Logf("root_management_route_count=%d todo7_route_count=%d todo8_route_count=%d", len(routes), 25, 10)
}

func rootRouteConfig(database bool) config.Config {
	return config.Config{
		JackettURL: "http://127.0.0.1:1", ProwlarrURL: "http://127.0.0.1:1", HTTPTimeout: time.Second,
		Database: config.DatabaseConfig{Enabled: database}, IndexerResultCacheTTL: time.Minute, OffsetCacheTTL: time.Minute,
		ResultCacheMaxEntries: 8, OffsetCacheMaxEntries: 8,
	}
}

type managementRoute struct {
	method      string
	path        string
	body        []byte
	status      int
	contentType string
}

func managementRouteContracts() []managementRoute {
	ruleBody := []byte(`{"token":"title","regex":".*","replacement":"Movie","example":"Movie"}`)
	routes := make([]managementRoute, 0, 35)
	for _, domain := range []string{"sonarr", "radarr"} {
		prefix := "/api/" + domain + "/rule/"
		routes = append(routes,
			managementRoute{http.MethodPost, prefix + "sync", nil, http.StatusServiceUnavailable, "application/json"}, managementRoute{http.MethodGet, prefix + "query", nil, http.StatusOK, "application/json; charset=utf-8"},
			managementRoute{http.MethodPost, prefix + "save", ruleBody, http.StatusOK, ""}, managementRoute{http.MethodPost, prefix + "remove", []byte(`[]`), http.StatusOK, ""},
			managementRoute{http.MethodPost, prefix + "enable", []byte(`[]`), http.StatusOK, ""}, managementRoute{http.MethodPost, prefix + "disable", []byte(`[]`), http.StatusOK, ""},
			managementRoute{http.MethodPost, prefix + "export", []byte(`[]`), http.StatusOK, "application/json; charset=utf-8"}, managementRoute{http.MethodPost, prefix + "import", []byte(`bad`), http.StatusBadRequest, "application/json"},
			managementRoute{http.MethodGet, prefix + "token/list", nil, http.StatusOK, "application/json; charset=utf-8"},
		)
	}
	routes = append(routes, managementRoute{http.MethodGet, "/api/rule/test?regex=.&replacement=x&example=x", nil, http.StatusOK, "text/plain; charset=utf-8"})
	for _, domain := range []string{"sonarr", "radarr"} {
		prefix := "/api/" + domain + "/example/"
		routes = append(routes, managementRoute{http.MethodPost, prefix + "save", []byte(`{"originalText":"example"}`), http.StatusOK, ""}, managementRoute{http.MethodGet, prefix + "query", nil, http.StatusOK, "application/json; charset=utf-8"}, managementRoute{http.MethodPost, prefix + "remove", []byte(`[]`), http.StatusOK, ""})
	}
	for _, domain := range []string{"sonarr", "radarr", "tmdb"} {
		prefix := "/api/" + domain + "/title/"
		routes = append(routes, managementRoute{http.MethodGet, prefix + "query", nil, http.StatusOK, "application/json; charset=utf-8"}, managementRoute{http.MethodPost, prefix + "remove", []byte(`[]`), http.StatusOK, ""}, managementRoute{http.MethodPost, prefix + "sync", nil, http.StatusServiceUnavailable, "application/json"})
	}
	routes = append(routes, managementRoute{http.MethodPost, "/api/tmdb/title/save", []byte(`{"tvdbId":1,"language":"en","title":"Movie","validStatus":1}`), http.StatusOK, ""})
	return routes
}
