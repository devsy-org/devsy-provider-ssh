package ssh

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/devsy-org/devsy-provider-ssh/pkg/options"
	cryptossh "golang.org/x/crypto/ssh"
)

type phaseReadyWriter struct {
	ready chan struct{}
	once  sync.Once
}

func (writer *phaseReadyWriter) Write(data []byte) (int, error) {
	writer.once.Do(func() { close(writer.ready) })
	return len(data), nil
}

func configTestProvider(t *testing.T, address, script string) *SSHProvider {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	requireNoError(t, err)
	dir, identity := writeTestIdentity(t, key)
	host, port, err := net.SplitHostPort(address)
	requireNoError(t, err)
	if script == "" {
		script = "printf '%s' 'hostname " + host + "\nuser fixture\nport " + port + "\nidentityfile " + identity + "\n'\n"
	}
	// #nosec G306 -- Controlled executable models SSH configuration lookup.
	requireNoError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte("#!/bin/sh\n"+script), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &SSHProvider{
		Config: &options.Options{Host: "context-fixture", Port: port, UseBuiltinSSH: true},
	}
}

func cancelAfterReady(t *testing.T, ready <-chan struct{}, run func(context.Context) error) error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- run(ctx) }()
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("SSH phase did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cancellation identity: %v", err)
		}
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("SSH phase did not stop on cancellation")
		return nil
	}
}

func TestBuiltinLookupCancellation(t *testing.T) {
	provider := configTestProvider(t, "127.0.0.1:22", "printf ready >&2\nexec sleep 30\n")
	output := &phaseReadyWriter{ready: make(chan struct{})}
	err := cancelAfterReady(t, output.ready, func(ctx context.Context) error {
		return provider.execBuiltin(ctx, "unused", Streams{Stderr: output})
	})
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		t.Fatalf("lost started lookup process error: %v", err)
	}
}

func TestBuiltinLookupOwnDeadline(t *testing.T) {
	provider := configTestProvider(t, "127.0.0.1:22", "exec sleep 30\n")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	err := provider.execBuiltin(ctx, "unused", Streams{})
	if !errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		t.Fatalf("lost independent lookup deadline: %v, parent=%v", err, ctx.Err())
	}
}

func TestBuiltinDialCancellation(t *testing.T) {
	provider := configTestProvider(t, "127.0.0.1:22", "")
	ready := make(chan struct{})
	dialErr := errors.New("controlled dial failure")
	err := cancelAfterReady(t, ready, func(ctx context.Context) error {
		return provider.execBuiltinWithDial(ctx, "unused", Streams{},
			func(ctx context.Context, _, _ string) (net.Conn, error) {
				close(ready)
				<-ctx.Done()
				return nil, dialErr
			})
	})
	if !errors.Is(err, dialErr) {
		t.Fatalf("lost dial cause: %v", err)
	}
}

func pendingBuiltinPhase(t *testing.T, session bool) (*SSHProvider, <-chan struct{}) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	requireNoError(t, err)
	signer, err := cryptossh.NewSignerFromKey(key)
	requireNoError(t, err)
	config := &cryptossh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	requireNoError(t, err)
	ready, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		if !session {
			reader := bufio.NewReader(conn)
			if _, err := reader.ReadString('\n'); err != nil {
				return
			}
			close(ready)
			_, _ = io.Copy(io.Discard, reader)
			return
		}
		server, channels, requests, err := cryptossh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		defer func() { _ = server.Close() }()
		go cryptossh.DiscardRequests(requests)
		if _, ok := <-channels; !ok {
			return
		}
		close(ready)
		_ = server.Wait()
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("pending SSH phase did not release its connection")
		}
	})
	return configTestProvider(t, listener.Addr().String(), ""), ready
}

func TestBuiltinHandshakeCancellation(t *testing.T) {
	provider, ready := pendingBuiltinPhase(t, false)
	err := cancelAfterReady(t, ready, func(ctx context.Context) error {
		return provider.execBuiltin(ctx, "unused", Streams{})
	})
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("lost closed handshake cause: %v", err)
	}
}

func TestBuiltinSessionOpeningCancellation(t *testing.T) {
	provider, ready := pendingBuiltinPhase(t, true)
	_ = cancelAfterReady(t, ready, func(ctx context.Context) error {
		return provider.execBuiltin(ctx, "unused", Streams{})
	})
}

func TestBuiltinCancellationPreservesSourceError(t *testing.T) {
	provider := builtinTestProvider(
		t,
		func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
			defer func() { _ = channel.Close() }()
			request := <-requests
			_ = request.Reply(true, nil)
			_, _ = io.Copy(io.Discard, channel)
		},
	)
	input := newLateErrorInput()
	defer func() { _ = input.Close() }()
	err := cancelAfterReady(t, input.started, func(ctx context.Context) error {
		return provider.execBuiltin(ctx, "unused", Streams{Stdin: input})
	})
	if !errors.Is(err, errInputRead) {
		t.Fatalf("lost canceled session source error: %v", err)
	}
}
