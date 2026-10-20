//go:build !windows

package ssh

import (
	"io"
	"os"

	"github.com/muesli/cancelreader"
)

func cancellableFile(reader *os.File) (cancelreader.CancelReader, error) {
	info, err := reader.Stat()
	if err != nil {
		return nil, err
	}
	nullInfo, _ := os.Stat(os.DevNull)
	if info.Mode().IsRegular() || (nullInfo != nil && os.SameFile(info, nullInfo)) {
		return cancelreader.NewReader(struct{ io.Reader }{reader})
	}
	return cancelreader.NewReader(reader)
}
