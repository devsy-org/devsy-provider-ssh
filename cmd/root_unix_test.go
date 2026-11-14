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
	"strconv"
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

func TestExecuteSignalEndsQuietlyAfterCleanup(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT} {
		t.Run(sig.String(), func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "ssh.pid")
			cmd := executeTestProcess(
				t,
				"printf '%s' \"$$\" > '"+pidFile+"'\nprintf 'ready\\n'\nexec sleep 30\n",
			)
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
			if err = cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			assertProcessSignaled(t, cmd.Wait(), sig)
			if stderr.Len() != 0 {
				t.Fatalf("expected shutdown logged below error level, got %q", stderr.String())
			}
			assertChildGone(t, pidFile)
		})
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

func assertProcessSignaled(t *testing.T, err error, want syscall.Signal) {
	t.Helper()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("expected signal death, got %v", err)
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != want {
		t.Fatalf("expected death by %s, got %v", want, err)
	}
}

func assertChildGone(t *testing.T, pidFile string) {
	t.Helper()
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("SSH child %d survived shutdown: %v", pid, err)
	}
}
