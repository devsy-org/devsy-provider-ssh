package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

func TestWindowsRedirectedStdin(t *testing.T) {
	if os.Getenv("DEVSY_TEST_REDIRECTED_STDIN") == "true" {
		reader, err := cancellableFile(os.Stdin)
		requireNoError(t, err)
		defer func() { _ = reader.Close() }()
		data := make([]byte, 5)
		_, err = io.ReadFull(reader, data)
		requireNoError(t, err)
		_, err = os.Stdout.Write(data)
		requireNoError(t, err)
		timer := time.AfterFunc(100*time.Millisecond, func() { reader.Cancel() })
		defer timer.Stop()
		_, err = reader.Read(make([]byte, 1))
		if err == nil {
			t.Fatal("idle redirected stdin read survived cancellation")
		}
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// #nosec G204,G702 -- Launch this test executable to exercise redirected process stdin without a console.
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsRedirectedStdin$")
	cmd.Env = append(os.Environ(), "DEVSY_TEST_REDIRECTED_STDIN=true")
	reader, writer, err := os.Pipe()
	requireNoError(t, err)
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	cmd.Stdin = reader
	payload := []byte{0, 1, 2, 255, '\n'}
	_, err = writer.Write(payload)
	requireNoError(t, err)
	output, err := cmd.Output()
	requireNoError(t, err)
	// The Go test process appends its PASS marker after the raw protocol bytes.
	if !bytes.HasPrefix(output, payload) {
		t.Fatalf("redirected stdin changed: %q", output)
	}
}

func windowsTestSession(
	t *testing.T,
	handle func(cryptossh.Channel),
) (*cryptossh.Client, *cryptossh.Session) {
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
		incoming := <-channels
		channel, reqs, err := incoming.Accept()
		if err != nil {
			return
		}
		request := <-reqs
		_ = request.Reply(true, nil)
		handle(channel)
	}()
	// #nosec G106 -- Isolated loopback SSH test server has an ephemeral host key.
	client, err := cryptossh.Dial(
		"tcp",
		listener.Addr().String(),
		&cryptossh.ClientConfig{
			User:            "fixture",
			HostKeyCallback: cryptossh.InsecureIgnoreHostKey(),
		},
	)
	requireNoError(t, err)
	session, err := client.NewSession()
	requireNoError(t, err)
	t.Cleanup(func() {
		_ = session.Close()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Windows SSH fixture did not stop")
		}
	})
	return client, session
}

func TestWindowsSessionPipeBytes(t *testing.T) {
	_, session := windowsTestSession(t, func(channel cryptossh.Channel) {
		defer func() { _ = channel.Close() }()
		_, _ = io.Copy(channel, channel)
		_, _ = channel.SendRequest(
			"exit-status",
			false,
			cryptossh.Marshal(struct{ Status uint32 }{0}),
		)
	})
	reader, writer, err := os.Pipe()
	requireNoError(t, err)
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	payload := []byte{0, 1, 2, 255, '\n', '\'', '\\'}
	_, err = writer.Write(payload)
	requireNoError(t, err)
	requireNoError(t, writer.Close())
	var stdout bytes.Buffer
	session.Stdout = &stdout
	requireNoError(t, runSession(session, "cat", reader))
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Fatalf("SSH pipe bytes changed: %q", stdout.Bytes())
	}
	_, err = reader.Stat()
	requireNoError(t, err)
}

func TestWindowsSessionIdlePipeCancellation(t *testing.T) {
	client, session := windowsTestSession(t, func(channel cryptossh.Channel) {
		_, _ = channel.Write([]byte("ready"))
		_, _ = io.Copy(io.Discard, channel)
		_ = channel.Close()
	})
	reader, writer, err := os.Pipe()
	requireNoError(t, err)
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	ready := make(chan struct{}, 1)
	session.Stdout = readyWriter{ready: ready}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ready:
			_ = client.Close()
		case <-done:
		}
	}()
	if err := runSession(session, "cat", reader); err == nil {
		t.Fatal("expected closed SSH session")
	}
	_, err = reader.Stat()
	requireNoError(t, err)
}

type readyWriter struct{ ready chan struct{} }

func (writer readyWriter) Write(data []byte) (int, error) {
	select {
	case writer.ready <- struct{}{}:
	default:
	}
	return len(data), nil
}

func TestWindowsExternalInputLifecycle(t *testing.T) {
	if mode := os.Getenv("DEVSY_TEST_EXTERNAL_INPUT_CHILD"); mode != "" {
		if mode == contextCancelScenario {
			_, err := io.Copy(io.Discard, os.Stdin)
			requireNoError(t, err)
		}
		return
	}
	for _, scenario := range []string{"remote-exit", contextCancelScenario, "large-unread"} {
		t.Run(scenario, func(t *testing.T) { runWindowsExternalInput(t, scenario) })
	}
}

func runWindowsExternalInput(t *testing.T, scenario string) {
	t.Helper()
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	input := &observedInput{ReadCloser: reader, started: make(chan struct{})}
	if scenario == "large-unread" {
		input.ReadCloser = io.NopCloser(bytes.NewReader(make([]byte, 1<<20)))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// #nosec G204,G702 -- Launch the controlled test executable as the external child.
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsExternalInputLifecycle$")
	cmd.Env = append(os.Environ(), "DEVSY_TEST_EXTERNAL_INPUT_CHILD="+scenario)
	result := make(chan error, 1)
	go func() { result <- runExternal(cmd, input) }()
	select {
	case <-input.started:
	case <-time.After(3 * time.Second):
		t.Fatal("stdin copy did not start")
	}
	cancelCommand := scenario == contextCancelScenario
	if cancelCommand {
		cancel()
	}
	err := waitInputResult(t, result, input)
	if (err != nil) != cancelCommand {
		t.Fatalf("unexpected external completion: %v", err)
	}
}
