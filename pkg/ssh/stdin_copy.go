package ssh

import (
	"errors"
	"io"
	"net"
	"os"
	"sync/atomic"
)

type stdinCopy struct {
	reader         io.ReadCloser
	stopping       atomic.Bool
	done           chan struct{}
	sourceErr      error
	sourceStopped  bool
	copyErr        error
	destinationErr error
}

func startStdinCopy(reader io.ReadCloser, writer io.WriteCloser) *stdinCopy {
	copy := &stdinCopy{reader: reader, done: make(chan struct{})}
	go func() {
		defer close(copy.done)
		_, copy.copyErr = io.Copy(
			trackedStdinWriter{Writer: writer, copy: copy},
			trackedStdin{copy: copy},
		)
		_ = writer.Close()
	}()
	return copy
}

type trackedStdin struct{ copy *stdinCopy }

func (input trackedStdin) Read(data []byte) (int, error) {
	n, err := input.copy.reader.Read(data)
	if err != nil && !errors.Is(err, io.EOF) {
		input.copy.sourceErr = err
		input.copy.sourceStopped = input.copy.stopping.Load()
	}
	return n, err
}

func (copy *stdinCopy) finish() error {
	copy.stopping.Store(true)
	_ = copy.reader.Close()
	<-copy.done
	// Source errors remain meaningful when a successful remote command closes its input.
	if copy.sourceErr != nil && (!copy.sourceStopped || !isStdinShutdown(copy.sourceErr)) {
		return copy.sourceErr
	}
	if copy.destinationErr != nil && isStdinDestinationClosed(copy.destinationErr) {
		return nil
	}
	if copy.sourceStopped && isStdinShutdown(copy.copyErr) {
		return nil
	}
	return copy.copyErr
}

func isStdinShutdown(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, os.ErrClosed) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF)
}

type trackedStdinWriter struct {
	io.Writer
	copy *stdinCopy
}

func (writer trackedStdinWriter) Write(data []byte) (int, error) {
	n, err := writer.Writer.Write(data)
	writer.copy.destinationErr = err
	return n, err
}

func isStdinDestinationClosed(err error) bool {
	return isStdinShutdown(err) || isPipeWriteClosed(err)
}
