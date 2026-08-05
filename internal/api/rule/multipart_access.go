package rule

import (
	"errors"
	"io"
)

var ErrMultipartFilesystemDisabled = errors.New("multipart filesystem access disabled")

// MultipartAccess keeps the import boundary stream-only. Filesystem operations
// remain explicit capabilities so an import cannot introduce filename I/O by accident.
type MultipartAccess interface {
	Stream(string, io.Reader) io.Reader
	Open(string) (io.ReadCloser, error)
	Create(string) (io.WriteCloser, error)
	Remove(string) error
}

type streamOnlyMultipartAccess struct{}

func (streamOnlyMultipartAccess) Stream(_ string, reader io.Reader) io.Reader { return reader }

func (streamOnlyMultipartAccess) Open(string) (io.ReadCloser, error) {
	return nil, ErrMultipartFilesystemDisabled
}

func (streamOnlyMultipartAccess) Create(string) (io.WriteCloser, error) {
	return nil, ErrMultipartFilesystemDisabled
}

func (streamOnlyMultipartAccess) Remove(string) error { return ErrMultipartFilesystemDisabled }
