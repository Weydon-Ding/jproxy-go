//go:build !windows

package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProcess_exitsCleanlyAfterSIGTERM(t *testing.T) {
	// Given
	root := filepath.Clean(filepath.Join("..", ".."))
	binary := filepath.Join(t.TempDir(), "jproxy")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/jproxy")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build process driver: %v: %s", err, output)
	}
	command := exec.Command(binary)
	command.Env = runtimeEnvironment()
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("open process stdout: %v", err)
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	ready := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), `"msg":"application.started"`) {
				ready <- nil
				return
			}
		}
		ready <- scanner.Err()
	}()
	select {
	case readyErr := <-ready:
		if readyErr != nil {
			_ = command.Process.Kill()
			t.Fatalf("wait for startup log: %v", readyErr)
		}
	case <-time.After(20 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("process startup timed out")
	}

	// When
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()

	// Then
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("process exit error = %v", err)
		}
	case <-time.After(20 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("process did not exit after SIGTERM")
	}
}

func runtimeEnvironment() []string {
	keys := map[string]bool{
		"ADDR":                            true,
		"JPROXY_DB_ENABLED":               true,
		"JPROXY_DB_PATH":                  true,
		"JPROXY_RADARR_FORMAT_ENABLED":    true,
		"JPROXY_SONARR_FORMAT_ENABLED":    true,
		"JPROXY_RADARR_FORMAT":            true,
		"JPROXY_SONARR_FORMAT":            true,
		"JPROXY_RADARR_FORMAT_RULES":      true,
		"JPROXY_SONARR_FORMAT_RULES":      true,
		"JPROXY_RADARR_TITLES":            true,
		"JPROXY_SONARR_TITLES":            true,
		"JPROXY_RADARR_TITLE_CLEAN_REGEX": true,
		"JPROXY_SONARR_TITLE_CLEAN_REGEX": true,
	}
	environment := make([]string, 0, len(os.Environ())+4)
	for _, entry := range os.Environ() {
		key, _, found := strings.Cut(entry, "=")
		if !found || !keys[key] {
			environment = append(environment, entry)
		}
	}
	return append(environment,
		"ADDR=127.0.0.1:0",
		"JPROXY_DB_ENABLED=false",
		"JPROXY_RADARR_FORMAT_ENABLED=false",
		"JPROXY_SONARR_FORMAT_ENABLED=false",
	)
}
