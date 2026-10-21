package ssh

import (
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

var cancelSynchronousIO = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")

type consoleInput struct {
	file     *os.File
	reading  sync.Mutex
	state    sync.Mutex
	closed   bool
	thread   windows.Handle
	done     chan struct{}
	once     sync.Once
	closeErr error
}

func newConsoleInput(file *os.File) (io.ReadCloser, error) {
	owned, err := duplicateInputFile(file)
	if err != nil {
		return nil, err
	}
	return &consoleInput{file: owned}, nil
}

func (input *consoleInput) Read(data []byte) (int, error) {
	input.reading.Lock()
	defer input.reading.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	thread, err := windows.OpenThread(windows.THREAD_TERMINATE, false, windows.GetCurrentThreadId())
	if err != nil {
		return 0, err
	}
	defer func() { _ = windows.CloseHandle(thread) }()
	input.state.Lock()
	if input.closed {
		input.state.Unlock()
		return 0, os.ErrClosed
	}
	input.thread = thread
	input.done = make(chan struct{})
	input.state.Unlock()
	n, err := input.file.Read(data)
	input.state.Lock()
	if input.closed && errors.Is(err, windows.ERROR_OPERATION_ABORTED) {
		err = os.ErrClosed
	}
	input.thread = 0
	close(input.done)
	input.state.Unlock()
	return n, err
}

func (input *consoleInput) Close() error {
	input.once.Do(func() {
		input.state.Lock()
		input.closed = true
		done := input.done
		input.state.Unlock()
		if done != nil {
			input.cancelRead(done)
		}
		input.closeErr = input.file.Close()
	})
	return input.closeErr
}

func (input *consoleInput) cancelRead(done <-chan struct{}) {
	// Retry covers cancellation between publishing the thread and entering ReadConsole.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		input.state.Lock()
		if input.thread != 0 {
			_, _, _ = cancelSynchronousIO.Call(uintptr(input.thread))
		}
		input.state.Unlock()
		select {
		case <-done:
			return
		case <-ticker.C:
		}
	}
}
