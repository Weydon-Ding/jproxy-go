package runtime

import (
	"errors"
	"testing"
)

func TestTransmissionMutationAdmission_startsCurrentRevisionExactlyOnce(t *testing.T) {
	// Given
	provider := NewStaticProvider(testSnapshot("current"))
	admission, available := provider.(TransmissionMutationAdmission)
	if !available {
		t.Fatal("provider does not expose TransmissionMutationAdmission")
	}
	revision := provider.Snapshot().TransmissionRevision
	starts := 0

	// When
	err := admission.AdmitTransmissionMutation(revision, func() { starts++ })

	// Then
	if err != nil || starts != 1 {
		t.Fatalf("err=%v starts=%d", err, starts)
	}
}

func TestTransmissionMutationAdmission_rejectsRevisionAfterPreparedPublication(t *testing.T) {
	// Given
	initial := testSnapshot("old")
	provider := NewStaticProvider(initial)
	admission, available := provider.(TransmissionMutationAdmission)
	if !available {
		t.Fatal("provider does not expose TransmissionMutationAdmission")
	}
	staleRevision := provider.Snapshot().TransmissionRevision
	next := testSnapshot("new")

	// When
	if !PublishPrepared(provider, next) {
		t.Fatal("PublishPrepared() = false")
	}
	starts := 0
	err := admission.AdmitTransmissionMutation(staleRevision, func() { starts++ })

	// Then
	if !errors.Is(err, ErrTransmissionRevisionStale) || starts != 0 {
		t.Fatalf("err=%v starts=%d", err, starts)
	}
}
