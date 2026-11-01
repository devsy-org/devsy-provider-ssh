package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func withCommands(t *testing.T, replacement map[string]func() error) {
	t.Helper()
	original := commands
	commands = replacement
	t.Cleanup(func() { commands = original })
}

func TestRunRejectsMissingOrExtraArguments(t *testing.T) {
	withCommands(t, map[string]func() error{"only": func() error { return nil }})
	for _, args := range [][]string{nil, {}, {"only", "extra"}} {
		var stderr bytes.Buffer
		if code := run(args, &stderr); code != 2 {
			t.Fatalf("run(%v) = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "usage: go run ./hack/ci <only>") {
			t.Fatalf("usage message missing command list: %q", stderr.String())
		}
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	withCommands(t, map[string]func() error{
		"alpha": func() error { return nil },
		"beta":  func() error { return nil },
	})
	var stderr bytes.Buffer
	if code := run([]string{"gamma"}, &stderr); code != 2 {
		t.Fatalf("run unknown = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown command "gamma"`) ||
		!strings.Contains(stderr.String(), "alpha|beta") {
		t.Fatalf("unexpected message: %q", stderr.String())
	}
}

func TestRunMapsHandlerResultToExitCode(t *testing.T) {
	withCommands(t, map[string]func() error{
		"passes": func() error { return nil },
		"fails":  func() error { return errors.New("boom") },
	})
	var stderr bytes.Buffer
	if code := run([]string{"passes"}, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("passes = %d with stderr %q", code, stderr.String())
	}
	if code := run([]string{"fails"}, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), "boom") {
		t.Fatalf("fails = %d with stderr %q", code, stderr.String())
	}
}

func TestEveryCommandIsRegisteredWithAHandler(t *testing.T) {
	for name, handler := range commands {
		if handler == nil {
			t.Fatalf("command %q has no handler", name)
		}
	}
}
