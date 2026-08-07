package rename

import (
	"context"
	"time"
)

func fixedNow() time.Time { return time.Unix(100, 0) }

type fakeHistory struct{ events []Event }

func (f fakeHistory) Fetch(context.Context, string, string, time.Time) ([]Event, error) {
	return f.events, nil
}

type recordingHistory struct{ keys []string }

func (h *recordingHistory) Fetch(_ context.Context, _ string, key string, _ time.Time) ([]Event, error) {
	h.keys = append(h.keys, key)
	return nil, nil
}

type fakeConfig struct {
	values map[string]string
	calls  int
}

func (f *fakeConfig) ValueByKey(_ context.Context, key string) (string, error) {
	f.calls++
	return f.values[key], nil
}

type fakeTorrent struct {
	renames []string
	errors  map[string]error
}

func (f *fakeTorrent) Rename(_ context.Context, hash, name string) error {
	f.renames = append(f.renames, hash+":"+name)
	return f.errors[hash]
}

type fakeQB struct {
	fakeTorrent
	renameErrors     map[string]error
	files            map[string][]string
	fileRenames      []string
	fileRenameErrors map[string]error
	cancel           context.CancelFunc
}

func (f *fakeQB) Rename(_ context.Context, hash, name string) error {
	f.renames = append(f.renames, hash+":"+name)
	if f.cancel != nil {
		f.cancel()
	}
	return f.renameErrors[hash]
}

func (f *fakeQB) Files(_ context.Context, hash string) ([]string, error) { return f.files[hash], nil }

func (f *fakeQB) RenameFile(_ context.Context, hash, oldPath, newPath string) error {
	f.fileRenames = append(f.fileRenames, hash+":"+oldPath+":"+newPath)
	return f.fileRenameErrors[oldPath]
}

type fakeLogin struct {
	calls int
	err   error
}

func (f *fakeLogin) Login(context.Context) error {
	f.calls++
	return f.err
}
