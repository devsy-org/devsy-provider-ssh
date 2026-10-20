package ssh

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

const contextCancelScenario = "context-cancel"

type lateErrorInput struct {
	started  chan struct{}
	released chan struct{}
	start    sync.Once
	close    sync.Once
}

func newLateErrorInput() *lateErrorInput {
	return &lateErrorInput{started: make(chan struct{}), released: make(chan struct{})}
}

func (input *lateErrorInput) Read([]byte) (int, error) {
	input.start.Do(func() { close(input.started) })
	<-input.released
	return 0, errInputRead
}

func (input *lateErrorInput) Close() error {
	input.close.Do(func() { close(input.released) })
	return nil
}

func waitInputResult(t *testing.T, result <-chan error, input io.Closer) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(3 * time.Second):
		_ = input.Close()
		select {
		case err := <-result:
			t.Logf("stdin cleanup after timeout: %v", err)
		case <-time.After(3 * time.Second):
		}
		t.Fatal("SSH command did not join its idle stdin copier")
		return nil
	}
}

type observedInput struct {
	io.ReadCloser
	started chan struct{}
	once    sync.Once
	closed  atomic.Int32
}

func (input *observedInput) Read(data []byte) (int, error) {
	input.once.Do(func() { close(input.started) })
	return input.ReadCloser.Read(data)
}

func (input *observedInput) Close() error {
	input.closed.Add(1)
	return input.ReadCloser.Close()
}

func TestExternalIdlePipeCompletion(t *testing.T) {
	for _, test := range []struct {
		name, script string
		cancel       bool
	}{
		{"remote-exit", "exit 0", false},
		{contextCancelScenario, "exec cat", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			provider := fakeExternalSSH(t)
			reader, writer := io.Pipe()
			defer func() { _ = reader.Close(); _ = writer.Close() }()
			input := &observedInput{ReadCloser: reader, started: make(chan struct{})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- Command(ctx, provider, test.script, Streams{Stdin: input}) }()
			select {
			case <-input.started:
			case <-time.After(3 * time.Second):
				t.Fatal("stdin copy did not start")
			}
			if test.cancel {
				cancel()
			}
			err := waitInputResult(t, result, reader)
			if input.closed.Load() != 1 {
				t.Fatalf("owned input closed %d times", input.closed.Load())
			}
			if (err != nil) != test.cancel {
				t.Fatalf("unexpected completion: %v", err)
			}
		})
	}
}

func TestBuiltinLateInputError(t *testing.T) {
	input := newLateErrorInput()
	defer func() { _ = input.Close() }()
	provider := builtinTestProvider(
		t,
		func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
			request := <-requests
			_ = request.Reply(true, nil)
			<-input.started
			_, _ = channel.SendRequest(
				"exit-status",
				false,
				cryptossh.Marshal(struct{ Status uint32 }{0}),
			)
			_ = channel.Close()
		},
	)
	result := make(chan error, 1)
	go func() { result <- Command(context.Background(), provider, "exit 0", Streams{Stdin: input}) }()
	if err := waitInputResult(t, result, input); !errors.Is(err, errInputRead) {
		t.Fatalf("late genuine input error lost: %v", err)
	}
}

func TestExternalKeepsCallerFileOpen(t *testing.T) {
	provider := fakeExternalSSH(t)
	reader, writer, err := os.Pipe()
	requireNoError(t, err)
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	requireNoError(t, Command(context.Background(), provider, "exit 0", Streams{Stdin: reader}))
	_, err = reader.Stat()
	requireNoError(t, err)
}

func TestExternalLateInputError(t *testing.T) {
	provider := fakeExternalSSH(t)
	input := newLateErrorInput()
	defer func() { _ = input.Close() }()
	result := make(chan error, 1)
	go func() { result <- Command(context.Background(), provider, "exit 0", Streams{Stdin: input}) }()
	if err := waitInputResult(t, result, input); !errors.Is(err, errInputRead) {
		t.Fatalf("late genuine input error lost: %v", err)
	}
}

func TestBuiltinIdlePipeCompletion(t *testing.T) {
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	input := &observedInput{ReadCloser: reader, started: make(chan struct{})}
	provider := builtinTestProvider(
		t,
		func(channel cryptossh.Channel, requests <-chan *cryptossh.Request) {
			request := <-requests
			_ = request.Reply(true, nil)
			<-input.started
			_, _ = channel.SendRequest(
				"exit-status",
				false,
				cryptossh.Marshal(struct{ Status uint32 }{0}),
			)
			_ = channel.Close()
		},
	)
	result := make(chan error, 1)
	go func() { result <- Command(context.Background(), provider, "exit 0", Streams{Stdin: input}) }()
	requireNoError(t, waitInputResult(t, result, input))
}

func TestExternalEarlyExitWithLargeInput(t *testing.T) {
	provider := fakeExternalSSH(t)
	for range 5 {
		requireNoError(
			t,
			Command(
				context.Background(),
				provider,
				"exit 0",
				Streams{Stdin: bytes.NewReader(make([]byte, 1<<20))},
			),
		)
	}
}

type sourcePipeError struct{}

func (sourcePipeError) Read([]byte) (int, error) { return 0, syscall.EPIPE }
func (sourcePipeError) Close() error             { return nil }

func TestGenuineSourcePipeError(t *testing.T) {
	for _, builtin := range []bool{false, true} {
		t.Run(map[bool]string{false: "external", true: "builtin"}[builtin], func(t *testing.T) {
			provider := fakeExternalSSH(t)
			if builtin {
				provider = builtinTestProvider(
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
			}
			err := Command(context.Background(), provider, "cat", Streams{Stdin: sourcePipeError{}})
			if !errors.Is(err, syscall.EPIPE) {
				t.Fatalf("genuine source EPIPE lost: %v", err)
			}
		})
	}
}

func TestExternalStartFailureClosesOwnedInput(t *testing.T) {
	input := newLateErrorInput()
	// #nosec G204 -- Intentionally nonexistent controlled path exercises start failure.
	cmd := exec.CommandContext(context.Background(), filepath.Join(t.TempDir(), "missing"))
	if err := runExternal(cmd, input); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("start error: %v", err)
	}
	select {
	case <-input.released:
	default:
		t.Fatal("owned stdin not closed on start failure")
	}
	select {
	case <-input.started:
		t.Fatal("stdin copier started despite process start failure")
	default:
	}
}
