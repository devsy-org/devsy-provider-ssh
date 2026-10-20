package ssh

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devsy-org/devsy-provider-ssh/pkg/options"
	cryptossh "golang.org/x/crypto/ssh"
)

func TestInitLeavesCallerInputUntouched(t *testing.T) {
	for _, builtin := range []bool{false, true} {
		name := "external"
		if builtin {
			name = "builtin"
		}
		t.Run(name, func(t *testing.T) {
			provider := initProbeFailureProvider(t, builtin)
			payload := bytes.NewReader([]byte("caller protocol input"))
			input := &observedInput{ReadCloser: io.NopCloser(payload), started: make(chan struct{})}
			var stdout, stderr bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := Init(ctx, provider, Streams{Stdin: input, Stdout: &stdout, Stderr: &stderr})
			assertInitProbeFailure(t, err, &stdout, &stderr)
			select {
			case <-input.started:
				t.Error("initialization read caller input")
			default:
			}
			if input.closed.Load() != 0 || payload.Len() != len("caller protocol input") {
				t.Fatalf(
					"initialization consumed or closed caller input: remaining=%d closes=%d",
					payload.Len(),
					input.closed.Load(),
				)
			}
		})
	}
}

func initProbeFailureProvider(t *testing.T, builtin bool) *SSHProvider {
	t.Helper()
	if builtin {
		return builtinTestProvider(
			t,
			func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
				defer func() { _ = channel.Close() }()
				request := <-requests
				_ = request.Reply(true, nil)
				_, _ = io.Copy(io.Discard, channel)
				_, _ = io.WriteString(channel.Stderr(), "probe diagnostic")
				_, _ = channel.SendRequest(
					"exit-status",
					false,
					cryptossh.Marshal(struct{ Status uint32 }{37}),
				)
			},
		)
	}
	dir := t.TempDir()
	// #nosec G306 -- Controlled SSH stub drains input to expose accidental forwarding.
	requireNoError(t, os.WriteFile(filepath.Join(dir, "ssh"),
		[]byte("#!/bin/sh\ncat >/dev/null\nprintf 'probe diagnostic' >&2\nexit 37\n"), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &SSHProvider{Config: &options.Options{Host: "unused", Port: "22"}}
}

func assertInitProbeFailure(t *testing.T, err error, stdout, stderr *bytes.Buffer) {
	t.Helper()
	if err == nil || stderr.String() != "probe diagnostic" || stdout.Len() != 0 {
		t.Fatalf(
			"probe error/streams changed: %v, stdout=%q stderr=%q",
			err,
			stdout.Bytes(),
			stderr.Bytes(),
		)
	}
}
