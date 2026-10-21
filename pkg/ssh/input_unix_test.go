//go:build !windows

package ssh

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func fileFlags(t *testing.T, file *os.File) int {
	t.Helper()
	conn, err := file.SyscallConn()
	requireNoError(t, err)
	var flags int
	requireNoError(
		t,
		conn.Control(func(fd uintptr) { flags, err = unix.FcntlInt(fd, unix.F_GETFL, 0) }),
	)
	requireNoError(t, err)
	return flags
}

func TestUnixFileCancellationPreservesCaller(t *testing.T) {
	for range 32 {
		reader, writer, err := os.Pipe()
		requireNoError(t, err)
		t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
		flags := fileFlags(t, reader)
		input, err := cancellableFile(reader)
		requireNoError(t, err)
		if fileFlags(t, reader) != flags {
			t.Fatal("input adapter changed caller descriptor flags")
		}
		result := make(chan error, 1)
		go func() { _, err := input.Read(make([]byte, 1)); result <- err }()
		waitFileRead(t, input.(*polledInput))
		closeFileInput(t, input, writer)
		select {
		case err := <-result:
			if !errors.Is(err, os.ErrClosed) {
				t.Fatalf("idle file cancellation: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("idle file read did not join")
		}
		requireNoError(t, input.Close())
		if fileFlags(t, reader) != flags {
			t.Fatal("input cleanup changed caller descriptor flags")
		}
		_, err = writer.Write([]byte{42})
		requireNoError(t, err)
		data := make([]byte, 1)
		_, err = io.ReadFull(reader, data)
		requireNoError(t, err)
		if data[0] != 42 {
			t.Fatal("caller file no longer usable")
		}
		requireNoError(t, reader.Close())
		requireNoError(t, writer.Close())
	}
}

func closeFileInput(t *testing.T, input io.ReadCloser, writer *os.File) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- input.Close() }()
	select {
	case err := <-done:
		requireNoError(t, err)
	case <-time.After(3 * time.Second):
		_, _ = writer.Write([]byte{1})
		select {
		case err := <-done:
			t.Logf("file cleanup after timeout: %v", err)
		case <-time.After(3 * time.Second):
		}
		t.Fatal("file cancellation did not wake and join its reader")
	}
}

func waitFileRead(t *testing.T, input *polledInput) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for input.reading.TryLock() {
		input.reading.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("idle file read did not start")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestUnixFileCloseBeforeRead(t *testing.T) {
	reader, writer, err := os.Pipe()
	requireNoError(t, err)
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	input, err := cancellableFile(reader)
	requireNoError(t, err)
	requireNoError(t, input.Close())
	_, err = input.Read(make([]byte, 1))
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("read after cancellation: %v", err)
	}
}

func TestUnixFilePayloadAndEOF(t *testing.T) {
	reader, writer, err := os.Pipe()
	requireNoError(t, err)
	defer func() { _ = reader.Close() }()
	input, err := cancellableFile(reader)
	requireNoError(t, err)
	defer func() { _ = input.Close() }()
	payload := make([]byte, 256)
	for i := range payload {
		payload[i] = byte(i)
	}
	_, err = writer.Write(payload)
	requireNoError(t, err)
	requireNoError(t, writer.Close())
	output, err := io.ReadAll(input)
	requireNoError(t, err)
	if !bytes.Equal(output, payload) {
		t.Fatalf("pipe bytes changed: %x", output)
	}
}
