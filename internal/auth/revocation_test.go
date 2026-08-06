package auth

import "testing"

func TestRevocations_evictsEarliestExpiry_whenAtCapacity(t *testing.T) {
	// Given
	revoked := newRevocations(2)
	revoked.add("first", 200, 100)
	revoked.add("second", 300, 100)

	// When
	revoked.add("latest", 400, 100)

	// Then
	if revoked.contains("latest", 100) == false || revoked.contains("first", 100) || len(revoked.entries) != 2 {
		t.Fatalf("entries=%#v", revoked.entries)
	}
}
