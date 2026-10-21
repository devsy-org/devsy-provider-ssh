package ssh

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

func cancellableFile(file *os.File) (io.ReadCloser, error) {
	kind, err := windows.GetFileType(windows.Handle(file.Fd()))
	if err != nil {
		return nil, fmt.Errorf("inspect SSH input handle: %w", err)
	}
	if kind == windows.FILE_TYPE_DISK {
		return io.NopCloser(file), nil
	}
	if kind != windows.FILE_TYPE_PIPE {
		var mode uint32
		if windows.GetConsoleMode(windows.Handle(file.Fd()), &mode) != nil {
			return io.NopCloser(file), nil
		}
		return newConsoleInput(file)
	}
	// Own a duplicate so closing the session interrupts pipe I/O without closing caller stdin.
	owned, err := duplicateInputFile(file)
	if err != nil {
		return nil, err
	}
	return &closingInput{ReadCloser: owned}, nil
}

func duplicateInputFile(file *os.File) (*os.File, error) {
	var duplicate windows.Handle
	process := windows.CurrentProcess()
	err := windows.DuplicateHandle(
		process,
		windows.Handle(file.Fd()),
		process,
		&duplicate,
		0,
		false,
		windows.DUPLICATE_SAME_ACCESS,
	)
	if err != nil {
		return nil, fmt.Errorf("duplicate SSH input handle: %w", err)
	}
	owned := os.NewFile(uintptr(duplicate), file.Name())
	if owned == nil {
		_ = windows.CloseHandle(duplicate)
		return nil, fmt.Errorf("open duplicated SSH input handle")
	}
	return owned, nil
}

func isPipeWriteClosed(err error) bool {
	return errors.Is(err, windows.ERROR_BROKEN_PIPE) || errors.Is(err, windows.ERROR_NO_DATA)
}
