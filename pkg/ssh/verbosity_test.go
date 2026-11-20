package ssh

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-provider-ssh/pkg/options"
	"github.com/devsy-org/devsy/pkg/log"
)

func TestExternalSSHTraceArgumentsAndStreams(t *testing.T) {
	for _, level := range []string{"warn", "debug", "trace"} {
		t.Run(level, func(t *testing.T) {
			log.Init(log.Config{Level: level, Format: "json"})
			root := prepareSSHFixture(t)
			provider := &SSHProvider{Config: &options.Options{Host: "example", Port: "22"}}
			var stdout, stderr bytes.Buffer
			if err := Command(
				context.Background(),
				provider,
				"true",
				Streams{Stdout: &stdout, Stderr: &stderr},
			); err != nil {
				t.Fatal(err)
			}
			data, err := root.ReadFile("args")
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSpace(string(data)), "\n")
			if (args[0] == "-vv") != (level == "trace") || args[len(args)-2] != "example" {
				t.Fatalf("unexpected arguments: %q", args)
			}
			if !bytes.Equal(stdout.Bytes(), []byte{1, 2}) || stderr.String() != "remote stderr" {
				t.Fatalf("streams changed: stdout %q stderr %q", stdout.Bytes(), stderr.String())
			}
		})
	}
	t.Cleanup(func() { log.Init(log.Config{}) })
}

func prepareSSHFixture(t *testing.T) *os.Root {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$CAPTURE\"\nprintf '\\001\\002'\nprintf 'remote stderr' >&2\n"
	file, err := root.OpenFile("ssh", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.WriteString(script); err != nil {
		t.Fatal(err)
	}
	if err := file.Chmod(0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CAPTURE", filepath.Join(dir, "args"))
	return root
}
