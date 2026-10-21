package ssh

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsConsoleCancellation(t *testing.T) {
	if os.Getenv("DEVSY_TEST_PRIVATE_CONSOLE") == "true" {
		runPrivateConsoleChecks(t)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// #nosec G204,G702 -- Re-execute this test binary in an isolated console helper.
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsConsoleCancellation$")
	cmd.Env = append(os.Environ(), "DEVSY_TEST_PRIVATE_CONSOLE=true")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("private console helper: %v\n%s", err, output)
	}
}

func runPrivateConsoleChecks(t *testing.T) {
	t.Helper()
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	result, _, err := kernel.NewProc("AllocConsole").Call()
	if result == 0 {
		t.Fatalf("allocate isolated console: %v", err)
	}
	defer func() { _, _, _ = kernel.NewProc("FreeConsole").Call() }()
	file, err := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	requireNoError(t, err)
	defer func() { _ = file.Close() }()
	for range 32 {
		cancelConsoleRead(t, file)
	}
	reader, err := newConsoleInput(file)
	requireNoError(t, err)
	requireNoError(t, reader.Close())
	_, err = reader.Read(make([]byte, 1))
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("read after console close: %v", err)
	}
	var mode uint32
	requireNoError(t, windows.GetConsoleMode(windows.Handle(file.Fd()), &mode))
}

func cancelConsoleRead(t *testing.T, file *os.File) {
	t.Helper()
	reader, err := newConsoleInput(file)
	requireNoError(t, err)
	input := reader.(*consoleInput)
	result := make(chan error, 1)
	go func() { _, err := input.Read(make([]byte, 8)); result <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		input.state.Lock()
		started := input.thread != 0
		input.state.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("console read did not start")
		}
		time.Sleep(time.Millisecond)
	}
	requireNoError(t, input.Close())
	select {
	case err := <-result:
		if !errors.Is(err, os.ErrClosed) {
			t.Fatalf("console cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("console read did not join")
	}
	requireNoError(t, input.Close())
}
