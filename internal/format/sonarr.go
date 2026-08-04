package format

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SonarrTitle is a static Sonarr title entry used only for {cleanTitle} matching.
type SonarrTitle struct {
	MainTitle    string `json:"mainTitle"`
	Title        string `json:"title"`
	CleanTitle   string `json:"cleanTitle"`
	SeasonNumber *int   `json:"seasonNumber"`
}

// SonarrConfig contains all data needed to format a Sonarr XML result without I/O.
type SonarrConfig struct {
	Format          string
	CleanTitleRegex string
	Rules           []Rule
	Titles          []SonarrTitle
}

// ValidateSonarrConfig ensures static Sonarr rules and title data can be compiled at startup.
func ValidateSonarrConfig(cfg SonarrConfig) error {
	if err := ValidateConfig(Config{Format: cfg.Format, CleanTitleRegex: cfg.CleanTitleRegex, Rules: cfg.Rules}); err != nil {
		return err
	}
	for _, title := range cfg.Titles {
		if strings.TrimSpace(title.MainTitle) == "" {
			return fmt.Errorf("Sonarr title mainTitle is required")
		}
		if strings.TrimSpace(title.Title) == "" && strings.TrimSpace(title.CleanTitle) == "" {
			return fmt.Errorf("Sonarr title must contain title or cleanTitle for %q", title.MainTitle)
		}
		if title.SeasonNumber == nil {
			return fmt.Errorf("Sonarr title seasonNumber is required for %q", title.MainTitle)
		}
	}
	return nil
}

func formatSonarrItemTitle(title, description string, cfg SonarrConfig) (string, bool) {
	title = normalizeTitleNewlines(title)
	rules := rulesByToken(cfg.Rules)
	if len(rules["title"]) == 0 {
		return "", false
	}
	formatted, matched := formatSonarrTitle(title, cfg, rules)
	if !matched || strings.Contains(formatted, "{title}") {
		return "", false
	}
	text := title
	if strings.TrimSpace(description) != "" {
		text += " / " + description
	}
	for _, token := range tokenRE.FindAllStringSubmatch(formatted, -1) {
		for _, rule := range rules[token[1]] {
			re := regexp.MustCompile(rule.Regex)
			if re.MatchString(text) {
				value := ExecuteOffset(re.ReplaceAllString(text, rule.Replacement), rule.Offset)
				formatted = ReplaceToken(rule.Token, value, formatted)
				break
			}
		}
	}
	if strings.Contains(formatted, "{episode}") {
		return text, true
	}
	return strings.TrimSpace(RemoveAllTokens(formatted)), true
}

func formatSonarrTitle(text string, cfg SonarrConfig, rules map[string][]Rule) (string, bool) {
	for _, rule := range rules["title"] {
		if !strings.Contains(rule.Regex, "{cleanTitle}") {
			re := regexp.MustCompile(rule.Regex)
			if re.MatchString(text) {
				return ReplaceToken("title", ExecuteOffset(re.ReplaceAllString(text, rule.Replacement), rule.Offset), cfg.Format), true
			}
			continue
		}
		cleanText := CleanTitle(text, cfg.CleanTitleRegex)
		for _, title := range cfg.Titles {
			cleanTitle := title.CleanTitle
			if cleanTitle == "" {
				cleanTitle = CleanTitle(title.Title, cfg.CleanTitleRegex)
			}
			cleanTitlePattern := strings.ReplaceAll(regexp.QuoteMeta(cleanTitle), " ", ".?")
			pattern := strings.ReplaceAll(rule.Regex, "{cleanTitle}", cleanTitlePattern)
			if !matchesEntire(cleanText, pattern) || sonarrHasAdjacentEnglishWord(cleanText, cleanTitlePattern) {
				continue
			}
			formatted := ReplaceToken("title", title.MainTitle, cfg.Format)
			if *title.SeasonNumber != -1 && *title.SeasonNumber != 1 {
				formatted = ReplaceToken("season", "S"+strconv.Itoa(*title.SeasonNumber), formatted)
			}
			return formatted, true
		}
	}
	return cfg.Format, false
}

func normalizeTitleNewlines(title string) string {
	return strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(title)
}

func sonarrHasAdjacentEnglishWord(cleanText, cleanTitlePattern string) bool {
	if !regexp.MustCompile(`^[ .?a-zA-Z]+$`).MatchString(cleanTitlePattern) {
		return false
	}
	pattern, err := regexp.Compile(cleanTitlePattern)
	if err != nil {
		return true
	}
	match := pattern.FindStringIndex(cleanText)
	if match == nil {
		return true
	}
	return sonarrAdjacentWord(cleanText[:match[0]], true) || sonarrAdjacentWord(cleanText[match[1]:], false)
}

func sonarrAdjacentWord(text string, before bool) bool {
	words := strings.Fields(text)
	if len(words) == 0 {
		return false
	}
	word := words[0]
	if before {
		word = words[len(words)-1]
	}
	if !regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(word) {
		return false
	}
	if strings.EqualFold(word, "aka") {
		return false
	}
	if !before && (strings.EqualFold(word, "season") || strings.EqualFold(word, "episode") || strings.EqualFold(word, "ep")) && len(words) > 1 && regexp.MustCompile(`^\d+$`).MatchString(words[1]) {
		return false
	}
	return true
}
