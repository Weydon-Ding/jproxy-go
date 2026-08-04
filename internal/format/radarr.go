package format

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	separatorRE    = regexp.MustCompile(`[\[\]【】]`)
	specialCharRE  = regexp.MustCompile(`[\$\(\)\*\+\.\?\^\{\}\|\\]`)
	articleRE      = regexp.MustCompile(`(?i)(\b|\s)(a|an|the)\s`)
	separatorRunRE = regexp.MustCompile(`/+`)
	spaceRunRE     = regexp.MustCompile(`\s+`)
	tokenRE        = regexp.MustCompile(`\{([^}]+)\}`)
	numberRE       = regexp.MustCompile(`\d+`)
)

// Rule is the Phase 1 subset of a static Radarr formatting rule.
type Rule struct {
	Token       string `json:"token"`
	Regex       string `json:"regex"`
	Replacement string `json:"replacement"`
	Offset      int    `json:"offset"`
}

// Title is a static Radarr title entry used only for {cleanTitle} matching.
type Title struct {
	MainTitle  string `json:"mainTitle"`
	Title      string `json:"title"`
	CleanTitle string `json:"cleanTitle"`
	Year       int    `json:"year"`
}

// Config contains all data needed to format a Radarr XML result without I/O.
type Config struct {
	Format          string
	CleanTitleRegex string
	Rules           []Rule
	Titles          []Title
}

func CleanTitle(title, cleanRegex string) string {
	original := separatorRE.ReplaceAllString(title, "/")
	original = specialCharRE.ReplaceAllString(original, " ")
	clean := original
	if cleanRegex != "" {
		if re, err := regexp.Compile(cleanRegex); err == nil {
			clean = re.ReplaceAllString(clean, " ")
		}
	}
	clean = articleRE.ReplaceAllString(clean, " ")
	if strings.TrimSpace(clean) == "" {
		clean = original
	}
	clean = separatorRunRE.ReplaceAllString(clean, "/")
	clean = spaceRunRE.ReplaceAllString(clean, " ")
	return strings.ToLower(strings.TrimSpace(clean))
}

func ReplaceToken(token, value, text string) string {
	return strings.ReplaceAll(text, "{"+token+"}", value)
}

func RemoveAllTokens(text string) string { return tokenRE.ReplaceAllString(text, "") }

// ExecuteOffset intentionally follows Java String.replace semantics: processing a
// matched number replaces every equal number currently in the value.
func ExecuteOffset(value string, offset int) string {
	if offset == 0 {
		return value
	}
	for _, numberText := range numberRE.FindAllString(value, -1) {
		number, err := strconv.Atoi(numberText)
		if err != nil {
			continue
		}
		value = strings.ReplaceAll(value, numberText, strconv.Itoa(number+offset))
	}
	return value
}

// ValidateConfig ensures static Phase 1 rules can be compiled at startup.
func ValidateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Format) == "" {
		return fmt.Errorf("format template is required")
	}
	if !strings.Contains(cfg.Format, "{title}") {
		return fmt.Errorf("format template must contain {title}")
	}
	if cfg.CleanTitleRegex != "" {
		if _, err := regexp.Compile(cfg.CleanTitleRegex); err != nil {
			return fmt.Errorf("compile clean title regex: %w", err)
		}
	}
	for _, rule := range cfg.Rules {
		if rule.Token == "" {
			return fmt.Errorf("format rule token is required")
		}
		if rule.Regex == "" {
			return fmt.Errorf("format rule regex is required for token %q", rule.Token)
		}
		expression := strings.ReplaceAll(rule.Regex, "{cleanTitle}", "placeholder")
		if _, err := regexp.Compile(expression); err != nil {
			return fmt.Errorf("compile format rule for token %q: %w", rule.Token, err)
		}
	}
	return nil
}

func formatItemTitle(title, description string, cfg Config) (string, bool) {
	title = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(title)
	rules := rulesByToken(cfg.Rules)
	format, matched := formatTitle(title, cfg, rules)
	if !matched || strings.Contains(format, "{title}") {
		return "", false
	}
	text := title
	if strings.TrimSpace(description) != "" {
		text += " / " + description
	}
	for _, token := range tokenRE.FindAllStringSubmatch(format, -1) {
		for _, rule := range rules[token[1]] {
			re := regexp.MustCompile(rule.Regex)
			if re.MatchString(text) {
				value := ExecuteOffset(re.ReplaceAllString(text, rule.Replacement), rule.Offset)
				format = ReplaceToken(rule.Token, value, format)
				break
			}
		}
	}
	return strings.TrimSpace(RemoveAllTokens(format)), true
}

func formatTitle(text string, cfg Config, rules map[string][]Rule) (string, bool) {
	for _, rule := range rules["title"] {
		if strings.Contains(rule.Regex, "{cleanTitle}") {
			for _, title := range cfg.Titles {
				cleanTitle := title.CleanTitle
				if cleanTitle == "" {
					cleanTitle = CleanTitle(title.Title, cfg.CleanTitleRegex)
				}
				cleanTitlePattern := strings.ReplaceAll(regexp.QuoteMeta(cleanTitle), " ", ".?")
				cleanText := CleanTitle(text, cfg.CleanTitleRegex)
				pattern := strings.ReplaceAll(rule.Regex, "{cleanTitle}", cleanTitlePattern)
				if matchesEntire(cleanText, pattern) {
					if hasAdjacentEnglishWord(cleanText, rule.Regex, cleanTitlePattern) || !matchesYear(text, title.Year, rules["year"]) {
						continue
					}
					format := ReplaceToken("title", title.MainTitle, cfg.Format)
					return ReplaceToken("year", strconv.Itoa(title.Year), format), true
				}
			}
			continue
		}
		re := regexp.MustCompile(rule.Regex)
		if re.MatchString(text) {
			return ReplaceToken("title", ExecuteOffset(re.ReplaceAllString(text, rule.Replacement), rule.Offset), cfg.Format), true
		}
	}
	return cfg.Format, false
}

func rulesByToken(rules []Rule) map[string][]Rule {
	result := make(map[string][]Rule)
	for _, rule := range rules {
		result[rule.Token] = append(result[rule.Token], rule)
	}
	return result
}

func matchesEntire(text, expression string) bool {
	re, err := regexp.Compile("^(?:" + expression + ")$")
	return err == nil && re.MatchString(text)
}

func hasAdjacentEnglishWord(cleanText, ruleRegex, cleanTitlePattern string) bool {
	if !regexp.MustCompile(`^[ .?a-zA-Z]+$`).MatchString(cleanTitlePattern) {
		return false
	}
	text := strings.ReplaceAll(cleanText, " aka ", "/")
	prefix := strings.ReplaceAll(ruleRegex, "{cleanTitle}", `[a-zA-Z]+ `+cleanTitlePattern)
	suffix := strings.ReplaceAll(ruleRegex, "{cleanTitle}", cleanTitlePattern+` [a-zA-Z]+`)
	return matchesEntire(text, prefix) || matchesEntire(text, suffix)
}

func matchesYear(text string, titleYear int, rules []Rule) bool {
	for _, rule := range rules {
		re := regexp.MustCompile(rule.Regex)
		if re.MatchString(text) {
			value := ExecuteOffset(re.ReplaceAllString(text, rule.Replacement), rule.Offset)
			return value == strconv.Itoa(titleYear)
		}
	}
	return true
}
