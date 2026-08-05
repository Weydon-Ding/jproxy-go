package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func requireFailureWithoutListener(t *testing.T, process *runtimeProcess, address string) processExit {
	t.Helper()
	result := make(chan processExit, 1)
	go func() { result <- process.wait() }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(processTimeout)
	defer timeout.Stop()
	for {
		assertConnectionRefused(t, address)
		select {
		case exit := <-result:
			assertConnectionRefused(t, address)
			if exit.err == nil || exit.contains("application.started") {
				t.Fatalf("startup unexpectedly succeeded: %s", exit.output)
			}
			return exit
		case <-ticker.C:
		case <-timeout.C:
			if err := process.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Fatalf("terminate startup process: %v", err)
			}
			t.Fatalf("startup process did not exit before deadline")
			return processExit{}
		}
	}
}

func assertConnectionRefused(t *testing.T, address string) {
	t.Helper()
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		_ = connection.Close()
		t.Fatalf("startup failure unexpectedly accepted a connection at %s", address)
	}
}

func getHealth(t *testing.T, address string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), processTimeout)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/health", nil)
	if err != nil {
		t.Fatalf("create health request: %v", err)
	}
	response, err := (&http.Client{Timeout: processTimeout}).Do(request)
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read health response: %v", err)
	}
	return response.StatusCode, string(body)
}

func reserveTestAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve test address: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved address: %v", err)
	}
	return address
}

func runtimeEnvironment(extra ...string) []string {
	keys := map[string]bool{"ADDR": true, "JPROXY_DB_ENABLED": true, "JPROXY_DB_PATH": true, "JPROXY_RADARR_FORMAT_ENABLED": true, "JPROXY_SONARR_FORMAT_ENABLED": true, "JPROXY_RADARR_FORMAT": true, "JPROXY_SONARR_FORMAT": true, "JPROXY_RADARR_FORMAT_RULES": true, "JPROXY_SONARR_FORMAT_RULES": true, "JPROXY_RADARR_TITLES": true, "JPROXY_SONARR_TITLES": true, "JPROXY_RADARR_TITLE_CLEAN_REGEX": true, "JPROXY_SONARR_TITLE_CLEAN_REGEX": true, "JPROXY_TEST_CONTROL": true}
	environment := make([]string, 0, len(os.Environ())+5+len(extra))
	for _, entry := range os.Environ() {
		key, _, found := strings.Cut(entry, "=")
		if !found || !keys[key] {
			environment = append(environment, entry)
		}
	}
	return append(environment, append([]string{"ADDR=127.0.0.1:0", "JPROXY_DB_ENABLED=false", "JPROXY_RADARR_FORMAT_ENABLED=false", "JPROXY_SONARR_FORMAT_ENABLED=false", "JPROXY_TEST_CONTROL=stdin"}, extra...)...)
}
