package transmission

import (
	"context"
	"errors"
	"strings"
	"testing"

	"jproxy-go/internal/runtime"
)

func TestClient_rejectsInvalidOperationInputsAndOversizedPayload_beforeTransport(t *testing.T) {
	// Given
	client := testClient(t, &mutableProvider{snapshot: runtime.Snapshot{TransmissionURL: "http://example.test", TransmissionUsername: "user", TransmissionPassword: "password"}})
	largeName := strings.Repeat("a", maxRequestBytes)

	// When / Then
	for _, hash := range []string{"", "bad\x00hash"} {
		if _, err := client.Files(context.Background(), hash); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("invalid hash category=%v", err)
		}
	}
	for _, name := range []string{"", ".", "..", "dir/name", `dir\name`, "bad\x00name"} {
		if err := client.Rename(context.Background(), "hash", name); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("invalid name category=%v", err)
		}
	}
	if _, err := marshalRequest("torrent-rename-path", renamePathArguments{IDs: []int64{1}, Path: "old", Name: largeName}); !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("oversized request category=%v", err)
	}
}
