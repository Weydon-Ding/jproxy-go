package sqlite

import (
	"fmt"
	"os"
)

type writerLock struct{ file *os.File }

func acquireWriterLock(databasePath string) (*writerLock, error) {
	file, err := os.OpenFile(databasePath+".jproxy-writer.lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open SQLite writer lock: %w", err)
	}
	if err := lockFile(file); err != nil {
		closeErr := file.Close()
		return nil, fmt.Errorf("acquire SQLite writer lock: %w", errorsJoin(err, closeErr))
	}
	return &writerLock{file: file}, nil
}

func (l *writerLock) Close() error {
	unlockErr := unlockFile(l.file)
	closeErr := l.file.Close()
	return errorsJoin(unlockErr, closeErr)
}

func errorsJoin(first, second error) error {
	if first == nil {
		return second
	}
	if second == nil {
		return first
	}
	return fmt.Errorf("%w; cleanup: %v", first, second)
}
