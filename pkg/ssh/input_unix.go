//go:build !windows

package ssh

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/unix"
)

type polledInput struct {
	file                *os.File
	fd                  int32
	cancelFD            int32
	wakeRead, wakeWrite *os.File
	reading             sync.Mutex
	closed              atomic.Bool
	once                sync.Once
	closeErr            error
}

func cancellableFile(file *os.File) (io.ReadCloser, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		return io.NopCloser(file), nil
	}
	fd, err := pollDescriptor(file)
	if err != nil {
		return nil, err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	cancelFD, err := pollDescriptor(reader)
	if err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return nil, err
	}
	return &polledInput{
		file:      file,
		fd:        fd,
		cancelFD:  cancelFD,
		wakeRead:  reader,
		wakeWrite: writer,
	}, nil
}

func (input *polledInput) Read(data []byte) (int, error) {
	defer runtime.KeepAlive(input.file)
	input.reading.Lock()
	defer input.reading.Unlock()
	if len(data) == 0 {
		return 0, nil
	}
	for !input.closed.Load() {
		if err := input.waitReady(); err != nil {
			return 0, err
		}
		n, err := unix.Read(int(input.fd), data)
		if errors.Is(err, syscall.EINTR) || errors.Is(err, syscall.EAGAIN) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		return n, err
	}
	return 0, os.ErrClosed
}

func pollDescriptor(file *os.File) (int32, error) {
	conn, err := file.SyscallConn()
	if err != nil {
		return 0, err
	}
	var fd int32
	var rangeErr error
	err = conn.Control(func(value uintptr) {
		if value > math.MaxInt32 {
			rangeErr = fmt.Errorf("SSH input descriptor exceeds poll range")
			return
		}
		fd = int32(value)
	})
	return fd, errors.Join(err, rangeErr)
}

func (input *polledInput) Close() error {
	input.once.Do(func() {
		input.closed.Store(true)
		_, signalErr := input.wakeWrite.Write([]byte{1})
		input.reading.Lock()
		defer input.reading.Unlock()
		input.closeErr = errors.Join(signalErr, input.wakeRead.Close(), input.wakeWrite.Close())
	})
	return input.closeErr
}

func (input *polledInput) waitReady() error {
	for {
		fds := []unix.PollFd{
			{Fd: input.fd, Events: unix.POLLIN},
			{Fd: input.cancelFD, Events: unix.POLLIN},
		}
		if _, err := unix.Poll(fds, -1); err != nil {
			if errors.Is(err, syscall.EINTR) {
				continue
			}
			return err
		}
		if input.closed.Load() || fds[1].Revents != 0 || fds[0].Revents&unix.POLLNVAL != 0 {
			return os.ErrClosed
		}
		return nil
	}
}

func isPipeWriteClosed(err error) bool { return errors.Is(err, syscall.EPIPE) }
