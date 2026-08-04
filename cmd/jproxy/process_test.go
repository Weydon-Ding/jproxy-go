package main

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProcess_servesHealthAndStopsThroughTestControl(t *testing.T) {
	// Given
	binary := buildBinary(t)
	process := startProcess(t, binary, runtimeEnvironment())
	address := awaitAddress(t, process.address)

	// When
	status, body := health(t, address)
	stopProcess(t, process)

	// Then
	if status != http.StatusOK || body != "ok" {
		t.Fatalf("GET /health = %d %q, want 200 ok", status, body)
	}
}

func TestProcess_rejectsInvalidConfigurationWithoutReadiness(t *testing.T) {
	// Given
	binary := buildBinary(t)
	environment := append(runtimeEnvironment(), "JPROXY_DB_ENABLED=not-a-boolean", "JPROXY_CANARY_TOKEN=canary-secret")
	command := exec.Command(binary)
	command.Env = environment

	// When
	output, err := command.CombinedOutput()

	// Then
	if err == nil {
		t.Fatalf("invalid configuration exited successfully: %s", output)
	}
	if strings.Contains(string(output), "application.started") || strings.Contains(string(output), "canary-secret") {
		t.Fatalf("invalid startup output = %s", output)
	}
}

type processHandle struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	address <-chan string
	output  *processOutput
}

type processOutput struct {
	mu   sync.Mutex
	text strings.Builder
}

func (output *processOutput) append(line string) {
	output.mu.Lock()
	defer output.mu.Unlock()
	output.text.WriteString(line)
	output.text.WriteByte('\n')
}

func (output *processOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.text.String()
}

func buildBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "jproxy")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	command := exec.Command("go", "build", "-o", binary, "./cmd/jproxy")
	command.Dir = filepath.Clean(filepath.Join("..", ".."))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build driver: %v: %s", err, output)
	}
	return binary
}

func startProcess(t *testing.T, binary string, environment []string) processHandle {
	t.Helper()
	command := exec.Command(binary)
	command.Env = environment
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("open stdin: %v", err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		t.Fatalf("open stderr: %v", err)
	}
	if err := command.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	output := &processOutput{}
	address := make(chan string, 1)
	go collectProcessOutput(stderr, output, address)
	return processHandle{command: command, stdin: stdin, address: address, output: output}
}

func collectProcessOutput(stderr io.Reader, output *processOutput, address chan<- string) {
	defer close(address)
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		output.append(line)
		if strings.Contains(line, `"msg":"application.started"`) {
			const prefix = `"listen_addr":"`
			start := strings.Index(line, prefix)
			if start >= 0 {
				value := line[start+len(prefix):]
				select {
				case address <- strings.Split(value, `"`)[0]:
				default:
				}
			}
		}
	}
}

func awaitAddress(t *testing.T, address <-chan string) string {
	t.Helper()
	select {
	case value, ok := <-address:
		if !ok || value == "" {
			t.Fatal("process did not publish listener address")
		}
		return value
	case <-time.After(20 * time.Second):
		t.Fatal("process startup timed out")
		return ""
	}
}

func health(t *testing.T, address string) (int, string) {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+address+"/health", nil)
	if err != nil {
		t.Fatalf("create health request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read health body: %v", err)
	}
	return response.StatusCode, string(body)
}

func stopProcess(t *testing.T, process processHandle) {
	t.Helper()
	if _, err := io.WriteString(process.stdin, "STOP\n"); err != nil {
		t.Fatalf("send STOP: %v", err)
	}
	if err := process.stdin.Close(); err != nil {
		t.Fatalf("close STOP input: %v", err)
	}
	finished := make(chan error, 1)
	go func() { finished <- process.command.Wait() }()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("process exit: %v", err)
		}
	case <-time.After(20 * time.Second):
		_ = process.command.Process.Kill()
		t.Fatal("hung process terminated")
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
		"JPROXY_TEST_CONTROL":             true,
	}
	environment := make([]string, 0, len(os.Environ())+5)
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
		"JPROXY_TEST_CONTROL=stdin",
	)
}
