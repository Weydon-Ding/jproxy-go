package proxy

import (
	"regexp"
	"strings"
)

var (
	yearRE          = regexp.MustCompile(` (19|20)\d{2}$`)
	episodeRE       = regexp.MustCompile(` 0*(\d{1,3}|1[0-8]\d{2})$`)
	seasonEpisodeRE = regexp.MustCompile(` (S\d+ |)\d+$`)
)

func removeYear(s string) string          { return yearRE.ReplaceAllString(strings.TrimSpace(s), "") }
func removeEpisode(s string) string       { return episodeRE.ReplaceAllString(strings.TrimSpace(s), "") }
func removeSeasonEpisode(s string) string { return seasonEpisodeRE.ReplaceAllString(strings.TrimSpace(s), "") }

func uniqueNonEmpty(xs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x == "" || seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}

func searchTitles(kind, searchKey string) []string {
	// 初始版：不接 SQLite 标题库，先复刻最关键的查询扩展。
	// Sonarr：去掉尾部集数；Radarr：保留原标题并尝试去年份。
	if kind == "radarr" {
		return uniqueNonEmpty([]string{searchKey, removeYear(searchKey)})
	}
	base := removeEpisode(searchKey)
	return uniqueNonEmpty([]string{base})
}
