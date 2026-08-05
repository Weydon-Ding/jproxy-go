package main

import (
	"io"
	"net"
	"testing"
)

func TestProcess_servesHealthAndStopsThroughTestControl(t *testing.T) {
	// Given
	process := startRuntimeProcess(t, buildRuntimeBinary(t), runtimeEnvironment())

	// When
	address := requireReadiness(t, process)
	status, body := getHealth(t, address)
	stopRuntimeProcess(t, process)

	// Then
	if status != 200 || body != "ok" {
		t.Fatalf("health = %d %q, want 200 ok", status, body)
	}
	t.Logf("process_pid=%d listen_addr=%s health_status=%d health_body=%q stop_exit=0", process.pid(), address, status, body)
}

func TestProcess_stopsWithRepeatedStopAndEOF(t *testing.T) {
	// Given
	process := startRuntimeProcess(t, buildRuntimeBinary(t), runtimeEnvironment())
	_ = requireReadiness(t, process)

	// When
	if _, err := io.WriteString(process.stdin, "STOP\nSTOP\n"); err != nil {
		t.Fatalf("send repeated STOP: %v", err)
	}
	if err := process.stdin.Close(); err != nil {
		t.Fatalf("close repeated STOP input: %v", err)
	}
	exit := requireExit(t, process)

	// Then
	if exit.err != nil {
		t.Fatalf("repeated STOP exit: %v: %s", exit.err, exit.output)
	}
	t.Logf("process_pid=%d repeated_stop_eof_exit=0", exit.pid)
}

func TestProcess_rejectsInvalidConfigurationWithoutListener(t *testing.T) {
	// Given
	address := reserveTestAddress(t)
	process := startRuntimeProcess(t, buildRuntimeBinary(t), runtimeEnvironment("ADDR="+address, "JPROXY_DB_ENABLED=not-a-boolean", "JPROXY_CANARY_TOKEN=canary-secret"))

	// When
	exit := requireFailureWithoutListener(t, process, address)

	// Then
	if !exit.failedWith("invalid_configuration") || exit.contains("canary-secret") {
		t.Fatalf("startup output = %s", exit.output)
	}
	t.Logf("process_pid=%d startup_failure=invalid_configuration listener_refused=true", exit.pid)
}

func TestProcess_rejectsBindFailureWithoutServing(t *testing.T) {
	// Given
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	process := startRuntimeProcess(t, buildRuntimeBinary(t), runtimeEnvironment("ADDR="+listener.Addr().String()))

	// When
	exit := requireExit(t, process)

	// Then
	if !exit.failedWith("listen_failed") || exit.contains("application.started") {
		t.Fatalf("bind failure output = %s", exit.output)
	}
	t.Logf("process_pid=%d startup_failure=listen_failed parent_listener=%s", exit.pid, listener.Addr())
}
