package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"jproxy-go/internal/search"
)

func loadSonarrCandidateRows(ctx context.Context, tx *sql.Tx) ([]search.SonarrRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT main_title,title,COALESCE(clean_title, ''),season_number,monitored FROM (SELECT st.main_title,st.title,st.clean_title,st.season_number,st.monitored FROM sonarr_title st GROUP BY st.clean_title UNION SELECT st.main_title,tt.title,NULL,-1,st.monitored FROM sonarr_title st LEFT JOIN tmdb_title tt ON st.tvdb_id=tt.tvdb_id WHERE st.sno=0 GROUP BY tt.title) WHERE title IS NOT NULL ORDER BY monitored DESC,LENGTH(title) DESC`)
	if err != nil {
		return nil, fmt.Errorf("query Sonarr search title candidates: %w", err)
	}
	defer rows.Close()
	var result []search.SonarrRow
	for rows.Next() {
		var row search.SonarrRow
		if err := rows.Scan(&row.MainTitle, &row.Title, &row.CleanTitle, &row.SeasonNumber, &row.Monitored); err != nil {
			return nil, fmt.Errorf("scan Sonarr search title candidate: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read Sonarr search title candidates: %w", err)
	}
	return result, nil
}

func loadRadarrCandidateRows(ctx context.Context, tx *sql.Tx) ([]search.RadarrRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT tmdb_id,sno,COALESCE(main_title, ''),COALESCE(title, ''),COALESCE(clean_title, ''),COALESCE(year, 0),monitored FROM radarr_title WHERE valid_status = 1`)
	if err != nil {
		return nil, fmt.Errorf("query active Radarr search title candidates: %w", err)
	}
	defer rows.Close()
	var result []search.RadarrRow
	for rows.Next() {
		var row search.RadarrRow
		if err := rows.Scan(&row.TMDBID, &row.SNO, &row.MainTitle, &row.Title, &row.CleanTitle, &row.Year, &row.Monitored); err != nil {
			return nil, fmt.Errorf("scan active Radarr search title candidate: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active Radarr search title candidates: %w", err)
	}
	return result, nil
}
