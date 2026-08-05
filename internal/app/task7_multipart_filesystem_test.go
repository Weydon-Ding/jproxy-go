package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jproxy-go/internal/api/rule"
)

type task7MultipartAccess struct {
	filenames       []string
	targets         map[string]string
	partReads       int
	streamedTargets []string
	operations      []string
}

func newTask7MultipartAccess(t *testing.T) *task7MultipartAccess {
	t.Helper()
	root := t.TempDir()
	return &task7MultipartAccess{
		filenames: []string{
			"../task7-traversal-target",
			`C:\task7-drive-target`,
			`\\task7-host\share\target`,
			"task7-control\x00-target",
			strings.Repeat("x", 256),
		},
		targets: map[string]string{
			"../task7-traversal-target": filepath.Join(root, "traversal-target"),
			`C:\task7-drive-target`:     filepath.Join(root, "drive-target"),
			`\\task7-host\share\target`: filepath.Join(root, "unc-target"),
			"task7-control\x00-target":  filepath.Join(root, "control-target"),
			strings.Repeat("x", 256):    filepath.Join(root, "overlong-target"),
		},
	}
}

func (a *task7MultipartAccess) Stream(filename string, reader io.Reader) io.Reader {
	if target, ok := a.targets[filename]; ok {
		a.streamedTargets = append(a.streamedTargets, target)
	}
	a.partReads++
	return reader
}

func (a *task7MultipartAccess) Open(path string) (io.ReadCloser, error) {
	a.operations = append(a.operations, "open:"+path)
	return nil, rule.ErrMultipartFilesystemDisabled
}

func (a *task7MultipartAccess) Create(path string) (io.WriteCloser, error) {
	a.operations = append(a.operations, "create:"+path)
	return nil, rule.ErrMultipartFilesystemDisabled
}

func (a *task7MultipartAccess) Remove(path string) error {
	a.operations = append(a.operations, "remove:"+path)
	return rule.ErrMultipartFilesystemDisabled
}

func (a *task7MultipartAccess) assertUnchanged(t *testing.T) (int, int) {
	a.assertAbsent(t)
	if len(a.streamedTargets) != 0 {
		t.Fatalf("malicious filenames reached stream boundary targets=%d", len(a.streamedTargets))
	}
	return len(a.targets), len(a.operations)
}

func (a *task7MultipartAccess) assertAbsent(t *testing.T) {
	t.Helper()
	for _, target := range a.targets {
		_, err := os.Stat(target)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("filesystem target state=%v", err)
		}
	}
}
