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

func TestStoreLoadFormatterSnapshot_readsQBittorrentValuesIncludingEmpty(t *testing.T) {
	// Given / When / Then
	for _, test := range []struct{ name, url, username, password string }{{"values", "http://qbit.test", "user", "password"}, {"empty values", "", "", ""}} {
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
				SystemConfig{ID: 12, Key: "qbittorrentUrl", Value: &test.url, ValidStatus: Valid},
				SystemConfig{ID: 18, Key: "qbittorrentUsername", Value: &test.username, ValidStatus: Valid},
				SystemConfig{ID: 19, Key: "qbittorrentPassword", Value: &test.password, ValidStatus: Valid},
			)
			for index := range values {
				if values[index].Key == "qbittorrentUrl" {
					values[index].Value = &test.url
				}
				if values[index].Key == "qbittorrentUsername" {
					values[index].Value = &test.username
				}
				if values[index].Key == "qbittorrentPassword" {
					values[index].Value = &test.password
				}
			}
			if err := store.Repositories().SystemConfigs.UpsertBatch(context.Background(), values); err != nil {
				t.Fatal(err)
			}
			snapshot, err := store.LoadFormatterSnapshot(context.Background())
			if err != nil || snapshot.QBittorrentURL != test.url || snapshot.QBittorrentUsername != test.username || snapshot.QBittorrentPassword != test.password {
				t.Fatalf("snapshot=%+v err=%v", snapshot, err)
			}
		})
	}
}
