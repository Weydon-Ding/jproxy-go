package sqlite

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestWriterLock_crossProcessRejectsAndReleases(t *testing.T) {
	path := javaFinalFixture(t)
	command := startLockHelper(t, path)
	if _, err := Open(context.Background(), path); !errors.Is(err, ErrWriterOwned) {
		_ = command.Process.Kill()
		t.Fatalf("Open() error = %v, want ErrWriterOwned", err)
	}
	stopLockHelper(t, command)
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() after helper exit error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func assertSeparateProcessCanAcquireLock(t *testing.T, path string) {
	t.Helper()
	stopLockHelper(t, startLockHelper(t, path))
}

func startLockHelper(t *testing.T, path string) *exec.Cmd {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestWriterLockHelperProcess$")
	command.Env = append(os.Environ(), "JPROXY_LOCK_HELPER=1", "JPROXY_LOCK_PATH="+path)
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("open lock helper stdout: %v", err)
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start lock helper: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		line, readErr := bufio.NewReader(stdout).ReadString('\n')
		if readErr != nil || line != "ready\n" {
			result <- fmt.Errorf("lock helper readiness = %q, %w", line, readErr)
			return
		}
		result <- nil
	}()
	select {
	case err := <-result:
		if err != nil {
			stopLockHelper(t, command)
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		stopLockHelper(t, command)
		t.Fatal("lock helper readiness timed out")
	}
	return command
}

func stopLockHelper(t *testing.T, command *exec.Cmd) {
	t.Helper()
	if command.Process == nil {
		return
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("terminate lock helper: %v", err)
	}
	result := make(chan error, 1)
	go func() { result <- command.Wait() }()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("lock helper wait error = nil, want killed process")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("lock helper teardown timed out")
	}
}

func TestWriterLockHelperProcess(_ *testing.T) {
	if os.Getenv("JPROXY_LOCK_HELPER") != "1" {
		return
	}
	lock, err := acquireWriterLock(os.Getenv("JPROXY_LOCK_PATH"))
	if err != nil {
		os.Exit(2)
	}
	if _, err := fmt.Fprintln(os.Stdout, "ready"); err != nil {
		_ = lock.Close()
		os.Exit(3)
	}
	select {}
}
