package sqlite

import (
	"errors"
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
		return nil, fmt.Errorf("acquire SQLite writer lock: %w", errors.Join(err, closeErr))
	}
	return &writerLock{file: file}, nil
}

func (l *writerLock) Close() error {
	unlockErr := unlockFile(l.file)
	closeErr := l.file.Close()
	return errors.Join(unlockErr, closeErr)
}
