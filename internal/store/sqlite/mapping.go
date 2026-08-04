package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"jproxy-go/internal/format"
)

func loadSystemConfig(ctx context.Context, tx *sql.Tx, key string) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT value FROM system_config WHERE key = ? AND valid_status = 1`, key)
	if err != nil {
		return "", fmt.Errorf("query active system config %q: %w", key, err)
	}
	defer rows.Close()
	var value string
	count := 0
	for rows.Next() {
		var nullableValue sql.NullString
		if err := rows.Scan(&nullableValue); err != nil {
			return "", fmt.Errorf("scan active system config %q: %w", key, err)
		}
		if !nullableValue.Valid {
			return "", fmt.Errorf("active system config %q must not be NULL", key)
		}
		value = nullableValue.String
		count++
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("read active system config %q: %w", key, err)
	}
	if count != 1 {
		return "", fmt.Errorf("active system config %q count = %d, want 1", key, count)
	}
	return value, nil
}

func loadRules(ctx context.Context, tx *sql.Tx, table string) ([]format.Rule, error) {
	query := `SELECT token, COALESCE(priority, 1000), regex, COALESCE(replacement, ''), COALESCE(offset, 0) FROM ` + table + ` WHERE valid_status = 1 ORDER BY COALESCE(priority, 1000) ASC`
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query active rules from %s: %w", table, err)
	}
	defer rows.Close()
	var rules []format.Rule
	for rows.Next() {
		var rule format.Rule
		if err := rows.Scan(&rule.Token, &rule.Priority, &rule.Regex, &rule.Replacement, &rule.Offset); err != nil {
			return nil, fmt.Errorf("scan active rule from %s: %w", table, err)
		}
		validStatus := 1
		rule.ValidStatus = &validStatus
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active rules from %s: %w", table, err)
	}
	return rules, nil
}

func loadRadarrTitles(ctx context.Context, tx *sql.Tx) ([]format.Title, error) {
	rows, err := tx.QueryContext(ctx, `SELECT main_title, COALESCE(title, ''), COALESCE(clean_title, ''), year FROM radarr_title WHERE valid_status = 1`)
	if err != nil {
		return nil, fmt.Errorf("query active Radarr titles: %w", err)
	}
	defer rows.Close()
	var titles []format.Title
	for rows.Next() {
		var title format.Title
		if err := rows.Scan(&title.MainTitle, &title.Title, &title.CleanTitle, &title.Year); err != nil {
			return nil, fmt.Errorf("scan active Radarr title: %w", err)
		}
		titles = append(titles, title)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active Radarr titles: %w", err)
	}
	return titles, nil
}

func loadSonarrTitles(ctx context.Context, tx *sql.Tx) ([]format.SonarrTitle, error) {
	rows, err := tx.QueryContext(ctx, `SELECT main_title, COALESCE(title, ''), COALESCE(clean_title, ''), COALESCE(season_number, 1) FROM sonarr_title WHERE valid_status = 1`)
	if err != nil {
		return nil, fmt.Errorf("query active Sonarr titles: %w", err)
	}
	defer rows.Close()
	var titles []format.SonarrTitle
	for rows.Next() {
		var title format.SonarrTitle
		var seasonNumber int
		if err := rows.Scan(&title.MainTitle, &title.Title, &title.CleanTitle, &seasonNumber); err != nil {
			return nil, fmt.Errorf("scan active Sonarr title: %w", err)
		}
		title.SeasonNumber = &seasonNumber
		titles = append(titles, title)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active Sonarr titles: %w", err)
	}
	return titles, nil
}
