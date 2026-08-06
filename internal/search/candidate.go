// Package search builds stable database-title search candidates without I/O.
package search

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// Candidate is one upstream query variant and its source title metadata.
type Candidate struct {
	Query        string
	MainTitle    string
	SeasonNumber int64
	Year         int64
}

// SonarrRow is the search-relevant projection of a Sonarr and TMDB title join.
type SonarrRow struct {
	MainTitle    string
	Title        string
	CleanTitle   string
	SeasonNumber int64
	Monitored    bool
}

// RadarrRow is the search-relevant projection of a Radarr title row.
type RadarrRow struct {
	TMDBID     int64
	SNO        int64
	MainTitle  string
	Title      string
	CleanTitle string
	Year       int64
	Monitored  bool
}

// SonarrCandidates mirrors Java's joined-title ordering before removing duplicate
// query text. The final lexical keys make SQL ties deterministic for offset caching.
func SonarrCandidates(rows []SonarrRow) []Candidate {
	ordered := slices.Clone(rows)
	slices.SortFunc(ordered, compareSonarrRows)
	result := make([]Candidate, 0, len(ordered))
	seen := make(map[string]struct{}, len(ordered))
	for _, row := range ordered {
		query := strings.TrimSpace(row.Title)
		if query == "" {
			continue
		}
		if _, exists := seen[query]; exists {
			continue
		}
		seen[query] = struct{}{}
		result = append(result, Candidate{Query: query, MainTitle: row.MainTitle, SeasonNumber: row.SeasonNumber})
	}
	return result
}

// RadarrCandidates orders source title rows deterministically, then emits a
// year-qualified query before its base-title fallback when that year is absent.
func RadarrCandidates(rows []RadarrRow) []Candidate {
	ordered := slices.Clone(rows)
	slices.SortFunc(ordered, compareRadarrRows)
	result := make([]Candidate, 0, len(ordered)*2)
	seen := make(map[string]struct{}, len(ordered)*2)
	for _, row := range ordered {
		query := strings.TrimSpace(row.Title)
		if query == "" {
			continue
		}
		candidate := Candidate{Query: query, MainTitle: row.MainTitle, Year: row.Year}
		if row.Year > 0 && !hasYearSuffix(query, row.Year) {
			appendCandidate(&result, seen, Candidate{Query: query + " " + strconv.FormatInt(row.Year, 10), MainTitle: row.MainTitle, Year: row.Year})
		}
		appendCandidate(&result, seen, candidate)
	}
	return result
}

func appendCandidate(result *[]Candidate, seen map[string]struct{}, candidate Candidate) {
	if _, exists := seen[candidate.Query]; exists {
		return
	}
	seen[candidate.Query] = struct{}{}
	*result = append(*result, candidate)
}

func compareSonarrRows(left, right SonarrRow) int {
	if left.Monitored != right.Monitored {
		if left.Monitored {
			return -1
		}
		return 1
	}
	if result := cmp.Compare(len(right.Title), len(left.Title)); result != 0 {
		return result
	}
	if result := cmp.Compare(left.Title, right.Title); result != 0 {
		return result
	}
	if result := cmp.Compare(left.MainTitle, right.MainTitle); result != 0 {
		return result
	}
	if result := cmp.Compare(left.CleanTitle, right.CleanTitle); result != 0 {
		return result
	}
	return cmp.Compare(left.SeasonNumber, right.SeasonNumber)
}

func compareRadarrRows(left, right RadarrRow) int {
	if left.Monitored != right.Monitored {
		if left.Monitored {
			return -1
		}
		return 1
	}
	if result := cmp.Compare(left.TMDBID, right.TMDBID); result != 0 {
		return result
	}
	if result := cmp.Compare(left.SNO, right.SNO); result != 0 {
		return result
	}
	if result := cmp.Compare(left.Title, right.Title); result != 0 {
		return result
	}
	if result := cmp.Compare(left.MainTitle, right.MainTitle); result != 0 {
		return result
	}
	if result := cmp.Compare(left.Year, right.Year); result != 0 {
		return result
	}
	return cmp.Compare(left.CleanTitle, right.CleanTitle)
}

func hasYearSuffix(title string, year int64) bool {
	return strings.HasSuffix(title, " "+strconv.FormatInt(year, 10))
}
