package sqlite

import (
	"context"
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
