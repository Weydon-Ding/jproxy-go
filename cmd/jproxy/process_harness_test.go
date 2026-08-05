package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const processTimeout = 20 * time.Second

type runtimeProcess struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	ready   chan string
	output  *lockedBuffer
	drained *sync.WaitGroup
	waited  sync.Once
	exit    processExit
}

type processExit struct {
	err    error
	output string
	pid    int
}

func (exit processExit) failedWith(kind string) bool {
	return exit.err != nil && strings.Contains(exit.output, `"error_kind":"`+kind+`"`)
}

func (exit processExit) contains(value string) bool { return strings.Contains(exit.output, value) }

type lockedBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (buffer *lockedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.data.Write(data)
}

func (buffer *lockedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.data.String()
}

func buildRuntimeBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "jproxy")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	t.Cleanup(cancel)
	command := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/jproxy")
	command.Dir = filepath.Clean(filepath.Join("..", ".."))
	command.WaitDelay = time.Second
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build runtime binary: %v: %s", err, output)
	}
	return binary
}

func startRuntimeProcess(t *testing.T, binary string, environment []string) *runtimeProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	command := exec.CommandContext(ctx, binary)
	command.Env = environment
	command.WaitDelay = time.Second
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatalf("open process stdin: %v", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		cancel()
		t.Fatalf("open process stderr: %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatalf("open process stdout: %v", err)
	}
	if err := command.Start(); err != nil {
		cancel()
		t.Fatalf("start process: %v", err)
	}
	process := &runtimeProcess{command: command, stdin: stdin, ready: make(chan string, 1), output: &lockedBuffer{}}
	var drained sync.WaitGroup
	drained.Add(2)
	process.drained = &drained
	go drainProcessOutput(stderr, process, &drained)
	go drainProcessOutput(stdout, process, &drained)
	t.Cleanup(func() {
		_ = process.stdin.Close()
		if process.command.ProcessState == nil {
			_ = process.command.Process.Kill()
		}
		process.wait()
		cancel()
	})
	return process
}

func drainProcessOutput(reader io.Reader, process *runtimeProcess, drained *sync.WaitGroup) {
	defer drained.Done()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		_, _ = process.output.Write(append([]byte(line), '\n'))
		if address, ok := startedAddress(line); ok {
			select {
			case process.ready <- address:
			default:
			}
		}
	}
}

func startedAddress(line string) (string, bool) {
	const prefix = `"listen_addr":"`
	if !strings.Contains(line, `"msg":"application.started"`) {
		return "", false
	}
	start := strings.Index(line, prefix)
	if start < 0 {
		return "", false
	}
	address := strings.Split(line[start+len(prefix):], `"`)[0]
	return address, address != ""
}

func (process *runtimeProcess) wait() processExit {
	process.waited.Do(func() {
		process.exit = processExit{err: process.command.Wait(), pid: process.command.Process.Pid}
		process.drained.Wait()
		process.exit.output = process.output.String()
	})
	return process.exit
}

func (process *runtimeProcess) pid() int { return process.command.Process.Pid }

func requireReadiness(t *testing.T, process *runtimeProcess) string {
	t.Helper()
	select {
	case address := <-process.ready:
		return address
	case <-time.After(processTimeout):
		exit := requireExit(t, process)
		t.Fatalf("process readiness timed out: %s", exit.output)
		return ""
	}
}

func requireExit(t *testing.T, process *runtimeProcess) processExit {
	t.Helper()
	result := make(chan processExit, 1)
	go func() { result <- process.wait() }()
	select {
	case exit := <-result:
		return exit
	case <-time.After(processTimeout):
		if err := process.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Fatalf("terminate timed out process: %v", err)
		}
		t.Fatalf("process did not exit before deadline")
		return processExit{}
	}
}

func stopRuntimeProcess(t *testing.T, process *runtimeProcess) {
	t.Helper()
	if _, err := io.WriteString(process.stdin, "STOP\n"); err != nil {
		t.Fatalf("send STOP: %v", err)
	}
	if err := process.stdin.Close(); err != nil {
		t.Fatalf("close process stdin: %v", err)
	}
	if exit := requireExit(t, process); exit.err != nil {
		t.Fatalf("graceful process exit: %v: %s", exit.err, exit.output)
	}
}
