//go:build windows

package cmd

import "os"

// exitBySignal exits with a failure code: Windows cannot re-raise a signal.
func exitBySignal(_ os.Signal) {
	os.Exit(1)
}
