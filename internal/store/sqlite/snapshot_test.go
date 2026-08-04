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
