package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"jproxy-go/internal/format"
	"jproxy-go/internal/search"
	"jproxy-go/internal/transmissionconfig"

	// Register the required pure-Go SQLite database/sql driver.
	_ "modernc.org/sqlite"
)

const busyTimeoutMilliseconds = 5000

// Snapshot is the immutable runtime configuration read once during startup.
type Snapshot struct {
	Radarr               format.Config
	Sonarr               format.SonarrConfig
	RadarrCandidates     []search.Candidate
	SonarrCandidates     []search.Candidate
	JackettURL           string
	ProwlarrURL          string
	QBittorrentURL       string
	QBittorrentUsername  string
	QBittorrentPassword  string
	TransmissionURL      string
	TransmissionUsername string
	TransmissionPassword string
}

// Load reads all active formatter records from an existing SQLite database.
func Load(ctx context.Context, path string) (Snapshot, error) {
	dsn, err := readOnlyDSN(path)
	if err != nil {
		return Snapshot{}, err
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return Snapshot{}, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return Snapshot{}, fmt.Errorf("ping SQLite database: %w", err)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin read-only SQLite transaction: %w", err)
	}
	snapshot, err := loadSnapshot(ctx, tx)
	if err != nil {
		_ = tx.Rollback()
		return Snapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit read-only SQLite transaction: %w", err)
	}
	return snapshot, nil
}

// FormatterSnapshot reads and validates formatter data through this already-open
// writable store. It is intentionally a startup-only snapshot, not a live provider.
func (s *Store) FormatterSnapshot(ctx context.Context) (snapshot Snapshot, err error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin formatter snapshot transaction: %w", err)
	}
	return snapshotFromTransaction(tx, func() (Snapshot, error) { return loadSnapshot(ctx, tx) })
}

// LoadFormatterSnapshot satisfies the runtime loader contract while keeping the
// Store open for the complete serving lifetime.
func (s *Store) LoadFormatterSnapshot(ctx context.Context) (Snapshot, error) {
	return s.FormatterSnapshot(ctx)
}

type snapshotTransaction interface {
	Commit() error
	Rollback() error
}

func snapshotFromTransaction(tx snapshotTransaction, load func() (Snapshot, error)) (snapshot Snapshot, err error) {
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback formatter snapshot transaction: %w", rollbackErr))
		}
	}()
	snapshot, err = load()
	if err != nil {
		return Snapshot{}, err
	}
	if err = tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("commit formatter snapshot transaction: %w", err)
	}
	return snapshot, nil
}

func readOnlyDSN(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve SQLite database path: %w", err)
	}
	query := url.Values{
		"mode":    []string{"ro"},
		"_pragma": []string{"query_only(1)", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMilliseconds)},
	}
	return (&url.URL{Scheme: "file", Opaque: filepath.ToSlash(absolutePath), RawQuery: query.Encode()}).String(), nil
}

func loadSnapshot(ctx context.Context, tx *sql.Tx) (Snapshot, error) {
	radarrFormat, err := loadSystemConfig(ctx, tx, "radarrIndexerFormat")
	if err != nil {
		return Snapshot{}, err
	}
	sonarrFormat, err := loadSystemConfig(ctx, tx, "sonarrIndexerFormat")
	if err != nil {
		return Snapshot{}, err
	}
	cleanTitleRegex, err := loadSystemConfig(ctx, tx, "cleanTitleRegex")
	if err != nil {
		return Snapshot{}, err
	}
	jackettURL, err := loadOptionalSystemConfig(ctx, tx, "jackettUrl")
	if err != nil {
		return Snapshot{}, err
	}
	prowlarrURL, err := loadOptionalSystemConfig(ctx, tx, "prowlarrUrl")
	if err != nil {
		return Snapshot{}, err
	}
	qbittorrentURL, err := loadOptionalSystemConfig(ctx, tx, "qbittorrentUrl")
	if err != nil {
		return Snapshot{}, err
	}
	qbittorrentUsername, err := loadOptionalSystemConfig(ctx, tx, "qbittorrentUsername")
	if err != nil {
		return Snapshot{}, err
	}
	qbittorrentPassword, err := loadOptionalSystemConfig(ctx, tx, "qbittorrentPassword")
	if err != nil {
		return Snapshot{}, err
	}
	transmissionURL, err := loadOptionalSystemConfig(ctx, tx, "transmissionUrl")
	if err != nil {
		return Snapshot{}, err
	}
	transmissionUsername, err := loadOptionalSystemConfig(ctx, tx, "transmissionUsername")
	if err != nil {
		return Snapshot{}, err
	}
	transmissionPassword, err := loadOptionalSystemConfig(ctx, tx, "transmissionPassword")
	if err != nil {
		return Snapshot{}, err
	}
	transmissionURL, err = transmissionconfig.NormalizeEndpoint(transmissionURL)
	if err != nil {
		return Snapshot{}, fmt.Errorf("normalize Transmission SQLite formatter snapshot: %w", err)
	}
	if err := transmissionconfig.ValidateCredentials(transmissionUsername, transmissionPassword); err != nil {
		return Snapshot{}, fmt.Errorf("validate Transmission SQLite formatter snapshot: %w", err)
	}
	radarrRules, err := loadRules(ctx, tx, "radarr_rule")
	if err != nil {
		return Snapshot{}, err
	}
	sonarrRules, err := loadRules(ctx, tx, "sonarr_rule")
	if err != nil {
		return Snapshot{}, err
	}
	radarrTitles, err := loadRadarrTitles(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	sonarrTitles, err := loadSonarrTitles(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	radarrCandidateRows, err := loadRadarrCandidateRows(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	sonarrCandidateRows, err := loadSonarrCandidateRows(ctx, tx)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		Radarr:               format.Config{Format: radarrFormat, CleanTitleRegex: cleanTitleRegex, Rules: radarrRules, Titles: radarrTitles},
		Sonarr:               format.SonarrConfig{Format: sonarrFormat, CleanTitleRegex: cleanTitleRegex, Rules: sonarrRules, Titles: sonarrTitles},
		RadarrCandidates:     search.RadarrCandidates(radarrCandidateRows),
		SonarrCandidates:     search.SonarrCandidates(sonarrCandidateRows),
		JackettURL:           jackettURL,
		ProwlarrURL:          prowlarrURL,
		QBittorrentURL:       qbittorrentURL,
		QBittorrentUsername:  qbittorrentUsername,
		QBittorrentPassword:  qbittorrentPassword,
		TransmissionURL:      transmissionURL,
		TransmissionUsername: transmissionUsername,
		TransmissionPassword: transmissionPassword,
	}
	if err := format.ValidateConfig(snapshot.Radarr); err != nil {
		return Snapshot{}, fmt.Errorf("validate Radarr SQLite formatter snapshot: %w", err)
	}
	if err := format.ValidateSonarrConfig(snapshot.Sonarr); err != nil {
		return Snapshot{}, fmt.Errorf("validate Sonarr SQLite formatter snapshot: %w", err)
	}
	return snapshot, nil
}
