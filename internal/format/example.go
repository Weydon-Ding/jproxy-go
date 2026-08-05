package format

import "strings"

const exampleMatchTitleFail = "Match Title Fail"

// RadarrExample projects a stored example through the Radarr item formatter.
func RadarrExample(originalText string, cfg Config) string {
	if ValidateConfig(cfg) != nil {
		return originalText
	}
	formatted, matched := formatItemTitle(originalText, "", cfg)
	if matched {
		return formatted
	}
	return strings.TrimSpace(RemoveAllTokens(ReplaceToken("title", exampleMatchTitleFail, cfg.Format)))
}

// SonarrExample projects a stored example through the Sonarr item formatter.
func SonarrExample(originalText string, cfg SonarrConfig) string {
	if ValidateSonarrConfig(cfg) != nil {
		return originalText
	}
	formatted, matched := formatSonarrItemTitle(originalText, "", cfg)
	if matched {
		return formatted
	}
	return strings.TrimSpace(RemoveAllTokens(ReplaceToken("title", exampleMatchTitleFail, cfg.Format)))
}
