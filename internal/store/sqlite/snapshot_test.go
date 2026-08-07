package sqlite

import (
	"context"
	"strings"
	"testing"
)

func TestStoreFormatterSnapshot_readsThroughWritableStore(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})

	// When
	snapshot, err := store.FormatterSnapshot(context.Background())

	// Then
	if err != nil {
		t.Fatalf("FormatterSnapshot() error = %v", err)
	}
	if snapshot.Radarr.Format != "{title}" || snapshot.Sonarr.Format != "{title}" {
		t.Fatalf("FormatterSnapshot() = %#v", snapshot)
	}
}

func TestStoreFormatterSnapshot_normalizesTransmissionValuesAfterReopen(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	if _, err := db.Exec(`INSERT INTO system_config (id, "key", value, valid_status) VALUES (13, 'transmissionUrl', 'https://transmission.test/prefix/transmission/web/', 1), (21, 'transmissionUsername', 'transmission-user', 1), (22, 'transmissionPassword', 'transmission-password', 1)`); err != nil {
		t.Fatalf("seed Transmission configuration: %v", err)
	}
	closeTestDB(t, db)

	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	store, err = Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen SQLite store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// When
	snapshot, err := store.FormatterSnapshot(context.Background())

	// Then
	if err != nil || snapshot.TransmissionURL != "https://transmission.test/prefix/transmission/rpc" || snapshot.TransmissionUsername != "transmission-user" || snapshot.TransmissionPassword != "transmission-password" {
		t.Fatalf("FormatterSnapshot() = %#v, %v", snapshot, err)
	}
	db = openTestDB(t, path)
	defer closeTestDB(t, db)
	var persistedURL string
	if err := db.QueryRow(`SELECT value FROM system_config WHERE id = 13`).Scan(&persistedURL); err != nil || persistedURL != "https://transmission.test/prefix/transmission/web/" {
		t.Fatalf("persisted Transmission URL = %q, %v", persistedURL, err)
	}
}

func TestStoreFormatterSnapshot_preservesJavaCompatibleTransmissionPasswordWithoutUsername(t *testing.T) {
	// Given
	path := javaFinalFixture(t)
	db := openTestDB(t, path)
	const credentialCanary = "transmission-password-canary"
	if _, err := db.Exec(`INSERT INTO system_config (id, "key", value, valid_status) VALUES (13, 'transmissionUrl', 'https://transmission.test', 1), (21, 'transmissionUsername', '', 1), (22, 'transmissionPassword', ?, 1)`, credentialCanary); err != nil {
		t.Fatalf("seed Java-compatible Transmission credentials: %v", err)
	}
	closeTestDB(t, db)
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// When
	snapshot, err := store.FormatterSnapshot(context.Background())

	// Then
	if err != nil {
		t.Fatalf("FormatterSnapshot() error = %v", err)
	}
	if snapshot.TransmissionURL != "https://transmission.test/transmission/rpc" || snapshot.TransmissionUsername != "" || snapshot.TransmissionPassword != credentialCanary {
		t.Fatalf("FormatterSnapshot() = %#v", snapshot)
	}
}

func TestStoreFormatterSnapshot_rejectsOversizedTransmissionValuesWithoutLeakingValue(t *testing.T) {
	for _, test := range []struct {
		name   string
		key    string
		value  string
		canary string
	}{
		{name: "URL", key: "transmissionUrl", value: "https://transmission.test/" + strings.Repeat("transmission-url-canary", 1024), canary: "transmission-url-canary"},
		{name: "username", key: "transmissionUsername", value: strings.Repeat("transmission-username-canary", 1024), canary: "transmission-username-canary"},
		{name: "password", key: "transmissionPassword", value: strings.Repeat("transmission-password-canary", 1024), canary: "transmission-password-canary"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			path := javaFinalFixture(t)
			db := openTestDB(t, path)
			if _, err := db.Exec(`INSERT INTO system_config (id, "key", value, valid_status) VALUES (13, 'transmissionUrl', 'https://transmission.test', 1), (21, 'transmissionUsername', 'username', 1), (22, 'transmissionPassword', 'password', 1)`); err != nil {
				t.Fatalf("seed Transmission configuration: %v", err)
			}
			if _, err := db.Exec(`UPDATE system_config SET value = ? WHERE "key" = ?`, test.value, test.key); err != nil {
				t.Fatalf("seed oversized Transmission value: %v", err)
			}
			closeTestDB(t, db)
			store, err := Open(context.Background(), path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			t.Cleanup(func() { _ = store.Close() })

			// When
			_, err = store.FormatterSnapshot(context.Background())

			// Then
			if err == nil || !strings.Contains(err.Error(), "Transmission SQLite formatter snapshot") || strings.Contains(err.Error(), test.canary) {
				t.Fatalf("FormatterSnapshot() error = %v", err)
			}
		})
	}
}

func TestStoreLoadFormatterSnapshot_readsDownloaderValuesIncludingEmpty(t *testing.T) {
	// Given / When / Then
	for _, test := range []struct {
		name                                                        string
		qbittorrentURL, qbittorrentUsername, qbittorrentPassword    string
		transmissionURL, transmissionUsername, transmissionPassword string
	}{
		{"values", "http://qbittorrent.test", "qbittorrent-user", "qbittorrent-password", "http://transmission.test/transmission/rpc", "transmission-user", "transmission-password"},
		{"empty values", "", "", "", "", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, err := Open(context.Background(), javaFinalFixture(t))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			values, err := store.Repositories().SystemConfigs.List(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			values = append(values,
				SystemConfig{ID: 12, Key: "qbittorrentUrl", Value: &test.qbittorrentURL, ValidStatus: Valid},
				SystemConfig{ID: 18, Key: "qbittorrentUsername", Value: &test.qbittorrentUsername, ValidStatus: Valid},
				SystemConfig{ID: 19, Key: "qbittorrentPassword", Value: &test.qbittorrentPassword, ValidStatus: Valid},
				SystemConfig{ID: 13, Key: "transmissionUrl", Value: &test.transmissionURL, ValidStatus: Valid},
				SystemConfig{ID: 21, Key: "transmissionUsername", Value: &test.transmissionUsername, ValidStatus: Valid},
				SystemConfig{ID: 22, Key: "transmissionPassword", Value: &test.transmissionPassword, ValidStatus: Valid},
			)
			for index := range values {
				if values[index].Key == "qbittorrentUrl" {
					values[index].Value = &test.qbittorrentURL
				}
				if values[index].Key == "qbittorrentUsername" {
					values[index].Value = &test.qbittorrentUsername
				}
				if values[index].Key == "qbittorrentPassword" {
					values[index].Value = &test.qbittorrentPassword
				}
				if values[index].Key == "transmissionUrl" {
					values[index].Value = &test.transmissionURL
				}
				if values[index].Key == "transmissionUsername" {
					values[index].Value = &test.transmissionUsername
				}
				if values[index].Key == "transmissionPassword" {
					values[index].Value = &test.transmissionPassword
				}
			}
			snapshot, err := store.UpdateSystemConfigs(context.Background(), values)
			if err != nil || snapshot.QBittorrentURL != test.qbittorrentURL || snapshot.QBittorrentUsername != test.qbittorrentUsername || snapshot.QBittorrentPassword != test.qbittorrentPassword || snapshot.TransmissionURL != test.transmissionURL || snapshot.TransmissionUsername != test.transmissionUsername || snapshot.TransmissionPassword != test.transmissionPassword {
				t.Fatal("downloader snapshot did not preserve the expected key mapping")
			}
		})
	}
}
