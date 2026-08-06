package proxy

import (
	"strconv"
	"strings"

	"jproxy-go/internal/runtime"
	"jproxy-go/internal/search"
)

func expandedSearchTitles(kind, searchKey string, snapshot runtime.Snapshot) []string {
	basic := searchTitles(kind, searchKey)
	if kind == "radarr" {
		return radarrSearchTitles(basic, snapshot.RadarrCandidates)
	}
	return sonarrSearchTitles(basic, snapshot.SonarrCandidates)
}

func sonarrSearchTitles(basic []string, candidates []search.Candidate) []string {
	if len(basic) == 0 || len(candidates) == 0 {
		return basic
	}
	anchor, matched := exactCandidate(basic[0], candidates)
	if !matched || strings.TrimSpace(anchor.MainTitle) == "" {
		return basic
	}
	titles := append([]string(nil), basic...)
	for _, candidate := range candidates {
		if candidate.MainTitle != anchor.MainTitle {
			continue
		}
		if anchor.SeasonNumber > 0 && candidate.SeasonNumber != anchor.SeasonNumber && candidate.SeasonNumber != -1 {
			continue
		}
		titles = append(titles, candidate.Query)
	}
	return uniqueNonEmpty(titles)
}

func radarrSearchTitles(basic []string, candidates []search.Candidate) []string {
	if len(basic) == 0 || len(candidates) == 0 {
		return basic
	}
	anchor, matched := radarrAnchor(basic, candidates)
	if !matched || strings.TrimSpace(anchor.MainTitle) == "" {
		return basic
	}
	titles := append([]string(nil), basic...)
	for _, candidate := range candidates {
		if candidate.MainTitle != anchor.MainTitle {
			continue
		}
		if anchor.Year > 0 && candidate.Year > 0 && candidate.Year != anchor.Year {
			continue
		}
		titles = append(titles, candidate.Query)
	}
	return uniqueNonEmpty(titles)
}

func exactCandidate(query string, candidates []search.Candidate) (search.Candidate, bool) {
	for _, candidate := range candidates {
		if candidate.Query == query {
			return candidate, true
		}
	}
	return search.Candidate{}, false
}

func radarrAnchor(basic []string, candidates []search.Candidate) (search.Candidate, bool) {
	year := radarrSearchYear(basic[0])
	for _, query := range basic {
		for _, candidate := range candidates {
			if candidate.Query == query && (year == 0 || candidate.Year == 0 || candidate.Year == year) {
				return candidate, true
			}
		}
	}
	base := removeYear(basic[0])
	for _, candidate := range candidates {
		if removeYear(candidate.Query) == base && (year == 0 || candidate.Year == 0 || candidate.Year == year) {
			return candidate, true
		}
	}
	return search.Candidate{}, false
}

func radarrSearchYear(searchKey string) int64 {
	suffix := strings.TrimSpace(yearRE.FindString(strings.TrimSpace(searchKey)))
	year, _ := strconv.ParseInt(suffix, 10, 64)
	return year
}
