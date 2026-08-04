package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"jproxy-go/internal/cache"
	"jproxy-go/internal/config"
	"jproxy-go/internal/format"
)

type Server struct {
	cfg         config.Config
	client      *http.Client
	resultCache *cache.TTLCache[string]
	offsetCache *cache.TTLCache[[]int]
}

func NewServer(cfg config.Config) *Server {
	return &Server{
		cfg:         cfg,
		client:      &http.Client{Timeout: cfg.HTTPTimeout},
		resultCache: cache.NewTTLCache[string](cfg.IndexerResultCacheTTL, cfg.ResultCacheMaxEntries),
		offsetCache: cache.NewTTLCache[[]int](cfg.OffsetCacheTTL, cfg.OffsetCacheMaxEntries),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/sonarr/jackett/", s.handleIndexer("sonarr", "jackett"))
	mux.HandleFunc("/sonarr/prowlarr/", s.handleIndexer("sonarr", "prowlarr"))
	mux.HandleFunc("/radarr/jackett/", s.handleIndexer("radarr", "jackett"))
	mux.HandleFunc("/radarr/prowlarr/", s.handleIndexer("radarr", "prowlarr"))
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	return mux
}

func (s *Server) handleIndexer(kind, backend string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cacheKey := makeCacheKey(r)
		if xml, ok := s.resultCache.Get(cacheKey); ok {
			writeXML(w, xml)
			return
		}

		q := cloneQuery(r.URL.Query())
		searchKey := strings.TrimSpace(q.Get("q"))
		var xml string
		var err error

		if searchKey == "" {
			xml, err = s.executeRequest(r, backend, q)
		} else {
			xml, err = s.executeExpandedSearch(r, kind, backend, q, searchKey)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if kind == "radarr" && s.cfg.RadarrFormatting.Enabled {
			xml = format.RadarrXML(xml, s.cfg.RadarrFormatting.Config)
		}
		if kind == "sonarr" && s.cfg.SonarrFormatting.Enabled {
			xml = format.SonarrXML(xml, s.cfg.SonarrFormatting.Config)
		}
		if xml != "" && hasChannel(xml) {
			s.resultCache.Set(cacheKey, xml)
		}
		writeXML(w, xml)
	}
}

func (s *Server) executeExpandedSearch(r *http.Request, kind, backend string, q url.Values, searchKey string) (string, error) {
	searchKey = strings.TrimSuffix(searchKey, " 00")
	q.Set("q", searchKey)
	titles := searchTitles(kind, searchKey)
	if len(titles) == 0 {
		return s.executeRequest(r, backend, q)
	}

	offset := intParam(q, "offset", 0)
	limit := intParam(q, "limit", 100)
	offsetKey := makeOffsetKey(r)
	offsets, ok := s.offsetCache.Get(offsetKey)
	if !ok || len(offsets) != len(titles) {
		offsets = make([]int, len(titles))
	}

	idx := calculateCurrentIndex(offset, offsets)
	var merged string
	for ; idx < len(titles) && limit > 0; idx++ {
		localQ := cloneQuery(q)
		if idx > 0 {
			localQ.Set("q", strings.Replace(q.Get("q"), titles[0], titles[idx], 1))
			localQ.Set("offset", strconv.Itoa(offset-offsets[idx-1]))
		}
		if len(titles) > 2 && idx == len(titles)-1 && s.cfg.MinCount > 0 && idx > 0 && offsets[idx-1] < s.cfg.MinCount {
			localQ.Set("season", "")
			localQ.Set("ep", "")
			localQ.Set("q", removeSeasonEpisode(localQ.Get("q")))
		}

		newXML, err := s.executeRequest(r, backend, localQ)
		if err != nil {
			return merged, err
		}
		count := countItems(newXML)
		if count > limit {
			count = limit
			newXML = trimXMLItems(newXML, count)
		}
		if count > 0 || merged == "" {
			merged = mergeXML(merged, newXML)
		}
		offset += count
		offsets[idx] = offset
		s.offsetCache.Set(offsetKey, offsets)
		limit -= count
	}
	return merged, nil
}

func (s *Server) executeRequest(r *http.Request, backend string, q url.Values) (string, error) {
	upstream := s.upstreamURL(backend, r.URL.Path)
	u, err := url.Parse(upstream)
	if err != nil {
		return "", err
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if (resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices) && resp.StatusCode != http.StatusConflict {
		return "", fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Server) upstreamURL(backend, path string) string {
	base := s.cfg.JackettURL
	prefixRE := regexp.MustCompile(`^/(sonarr|radarr)/jackett`)
	if backend == "prowlarr" {
		base = s.cfg.ProwlarrURL
		prefixRE = regexp.MustCompile(`^/(sonarr|radarr)/prowlarr`)
	}
	return strings.TrimRight(base, "/") + prefixRE.ReplaceAllString(path, "")
}

func calculateCurrentIndex(offset int, offsets []int) int {
	if offset == 0 {
		return 0
	}
	for i, v := range offsets {
		if v >= offset {
			return i
		}
	}
	return len(offsets) - 1
}

func makeCacheKey(r *http.Request) string {
	return r.URL.Path + "?" + regexp.MustCompile(`apikey=[^&]*`).ReplaceAllString(r.URL.RawQuery, "")
}

func makeOffsetKey(r *http.Request) string {
	s := regexp.MustCompile(`(offset=\d+|apikey=[^&]*)`).ReplaceAllString(r.URL.RawQuery, "")
	return r.URL.Path + "?" + s
}

func cloneQuery(q url.Values) url.Values {
	out := make(url.Values, len(q))
	for k, vs := range q {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func intParam(q url.Values, key string, def int) int {
	v, err := strconv.Atoi(q.Get(key))
	if err != nil {
		return def
	}
	return v
}

func writeXML(w http.ResponseWriter, xml string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, err := w.Write([]byte(xml))
	if err != nil {
		log.Printf("write response: %v", err)
	}
}
