package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devsy-org/devsy-provider-ssh/pkg/options"
	cryptossh "golang.org/x/crypto/ssh"
)

func builtinTestProvider(
	t *testing.T,
	handle func(cryptossh.Channel, <-chan *cryptossh.Request),
) *SSHProvider {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	requireNoError(t, err)
	signer, err := cryptossh.NewSignerFromKey(key)
	requireNoError(t, err)
	config := &cryptossh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	requireNoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		server, channels, requests, err := cryptossh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		defer func() { _ = server.Close() }()
		go cryptossh.DiscardRequests(requests)
		for incoming := range channels {
			channel, requests, err := incoming.Accept()
			if err != nil {
				return
			}
			handle(channel, requests)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("SSH server did not terminate")
		}
	})
	dir, identity := writeTestIdentity(t, key)
	host, port, err := net.SplitHostPort(listener.Addr().String())
	requireNoError(t, err)
	output := "hostname " + host + "\nuser fixture\nport " + port + "\nidentityfile " + identity + "\n"
	// #nosec G306 -- Controlled SSH configuration stub must be executable.
	requireNoError(
		t,
		os.WriteFile(
			filepath.Join(dir, "ssh"),
			[]byte("#!/bin/sh\nprintf '%s' '"+output+"'\n"),
			0o700,
		),
	)

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &SSHProvider{Config: &options.Options{Host: "fixture", Port: port, UseBuiltinSSH: true}}
}

func TestBuiltinStreamsCommandAndExit(t *testing.T) {
	script := "echo 'bootstrap'\ncat"
	expected, err := buildPOSIXRemoteExec(script)
	requireNoError(t, err)
	received := make(chan string, 1)
	provider := builtinTestProvider(
		t,
		func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
			defer func() { _ = channel.Close() }()
			request := <-requests
			var command struct{ Command string }
			if err := cryptossh.Unmarshal(request.Payload, &command); err != nil {
				return
			}
			received <- command.Command
			_ = request.Reply(true, nil)
			data := make([]byte, 5)
			_, _ = io.ReadFull(channel, data)
			_, _ = channel.Write(data)
			_, _ = channel.Stderr().Write([]byte("diagnostic"))
			_, _ = channel.SendRequest(
				"exit-status",
				false,
				cryptossh.Marshal(struct{ Status uint32 }{37}),
			)
		},
	)
	var stdout, stderr bytes.Buffer
	payload := []byte{0, 1, 2, 255, '\n'}
	err = Command(
		context.Background(),
		provider,
		script,
		Streams{Stdin: bytes.NewReader(payload), Stdout: &stdout, Stderr: &stderr},
	)
	var exit *cryptossh.ExitError
	if !errors.As(err, &exit) || exit.ExitStatus() != 37 {
		t.Fatalf("exit: %v", err)
	}
	if got := <-received; got != expected {
		t.Fatalf("command: %q", got)
	}
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Fatalf("stdout: %q", stdout.Bytes())
	}
	if stderr.String() != "diagnostic" {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestBuiltinCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := builtinTestProvider(
		t,
		func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
			request := <-requests
			_ = request.Reply(true, nil)
			cancel()
			for range requests {
			}
			_ = channel.Close()
		},
	)
	stdin, writer, err := os.Pipe()
	requireNoError(t, err)
	defer func() { _ = stdin.Close(); _ = writer.Close() }()
	err = Command(ctx, provider, "sleep 30", Streams{Stdin: stdin})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func writeTestIdentity(t *testing.T, key ed25519.PrivateKey) (string, string) {
	t.Helper()
	raw, err := x509.MarshalPKCS8PrivateKey(key)
	requireNoError(t, err)
	dir := t.TempDir()
	identity := filepath.Join(dir, "key")
	if err := os.WriteFile(
		identity,
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	return dir, identity
}

func TestBuiltinCancellationWithPipeReader(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := builtinTestProvider(
		t,
		func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
			request := <-requests
			_ = request.Reply(true, nil)
			cancel()
			for range requests {
			}
			_ = channel.Close()
		},
	)
	reader, writer := io.Pipe()
	defer func() { _ = writer.Close() }()
	err := Command(ctx, provider, "sleep 30", Streams{Stdin: reader})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

var errInputRead = errors.New("fixture input read failure")

type failingInput struct{}

func (failingInput) Read([]byte) (int, error) { return 0, errInputRead }
func (failingInput) Close() error             { return nil }

func TestBuiltinPreservesStdinReadError(t *testing.T) {
	provider := builtinTestProvider(
		t,
		func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
			defer func() { _ = channel.Close() }()
			request := <-requests
			_ = request.Reply(true, nil)
			_, _ = io.Copy(io.Discard, channel)
			_, _ = channel.SendRequest(
				"exit-status",
				false,
				cryptossh.Marshal(struct{ Status uint32 }{0}),
			)
		},
	)
	err := Command(context.Background(), provider, "cat", Streams{Stdin: failingInput{}})
	if !errors.Is(err, errInputRead) {
		t.Fatalf("stdin failure lost: %v", err)
	}
}
