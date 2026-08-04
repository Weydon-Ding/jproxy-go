package sqlite

import (
	"errors"
	"testing"
)

func TestSnapshotFromTransaction_joinsLoadAndRollbackErrors(t *testing.T) {
	// Given
	loadErr := errors.New("load")
	rollbackErr := errors.New("rollback")
	tx := &fakeSnapshotTransaction{rollbackErr: rollbackErr}

	// When
	_, err := snapshotFromTransaction(tx, func() (Snapshot, error) { return Snapshot{}, loadErr })

	// Then
	if !errors.Is(err, loadErr) || !errors.Is(err, rollbackErr) || !tx.rolledBack {
		t.Fatalf("snapshotFromTransaction() error = %v, rollback = %t", err, tx.rolledBack)
	}
}

func TestSnapshotFromTransaction_joinsCommitAndRollbackErrors(t *testing.T) {
	// Given
	commitErr := errors.New("commit")
	rollbackErr := errors.New("rollback")
	tx := &fakeSnapshotTransaction{commitErr: commitErr, rollbackErr: rollbackErr}

	// When
	_, err := snapshotFromTransaction(tx, func() (Snapshot, error) { return Snapshot{}, nil })

	// Then
	if !errors.Is(err, commitErr) || !errors.Is(err, rollbackErr) || !tx.rolledBack {
		t.Fatalf("snapshotFromTransaction() error = %v, rollback = %t", err, tx.rolledBack)
	}
}

type fakeSnapshotTransaction struct {
	commitErr   error
	rollbackErr error
	rolledBack  bool
}

func (tx *fakeSnapshotTransaction) Commit() error {
	return tx.commitErr
}

func (tx *fakeSnapshotTransaction) Rollback() error {
	tx.rolledBack = true
	return tx.rollbackErr
}
