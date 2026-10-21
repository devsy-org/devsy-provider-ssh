//go:build !windows

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestExecuteProcess(t *testing.T) {
	if os.Getenv("DEVSY_TEST_EXECUTE") != "1" {
		return
	}
	os.Args = []string{"devsy-provider-ssh", "command"}
	Execute()
}

func executeTestProcess(t *testing.T, script string) *exec.Cmd {
	t.Helper()
	dir := t.TempDir()
	// #nosec G306 -- Controlled executable models the SSH child at the CLI boundary.
	if err := os.WriteFile(
		filepath.Join(dir, "ssh"),
		[]byte("#!/bin/sh\n"+script),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"DEVSY_TEST_EXECUTE": "1", "DOCKER_PATH": "docker", "AGENT_PATH": "/tmp/agent",
		"HOST": "fixture", "PORT": "22", "USE_BUILTIN_SSH": "false", "COMMAND": "fixture",
		"EXTRA_FLAGS": "", "PATH": dir + string(os.PathListSeparator) + os.Getenv("PATH"),
	} {
		t.Setenv(key, value)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	// #nosec G204 -- Re-execute only this test binary with a controlled helper selector.
	return exec.CommandContext(ctx, executable, "-test.run=^TestExecuteProcess$")
}

func TestExecuteCancellation(t *testing.T) {
	cmd := executeTestProcess(t, "printf 'ready\\n'\nexec sleep 30\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ready, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || ready != "ready\n" {
		t.Fatalf("SSH child did not start: %q, %v", ready, err)
	}
	if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	assertProcessExit(t, cmd.Wait(), 1)
	if !strings.Contains(stderr.String(), "context canceled") {
		t.Fatalf("missing cancellation diagnostic: %q", stderr.String())
	}
}

func TestExecuteRemoteExit(t *testing.T) {
	cmd := executeTestProcess(t, "printf 'remote diagnostic' >&2\nexit 37\n")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	assertProcessExit(t, cmd.Run(), 37)
	if stderr.String() != "remote diagnostic" {
		t.Fatalf("remote diagnostic changed: %q", stderr.String())
	}
}

func assertProcessExit(t *testing.T, err error, code int) {
	t.Helper()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != code {
		t.Fatalf("expected exit %d, got %v", code, err)
	}
}
