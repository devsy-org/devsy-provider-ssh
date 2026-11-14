//go:build !windows

package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// exitBySignal ends the process the way an uncaught signal would, so the
// caller sees a signal death instead of a failure exit code.
func exitBySignal(sig os.Signal) {
	if s, ok := sig.(syscall.Signal); ok {
		signal.Reset(sig)
		_ = syscall.Kill(os.Getpid(), s)
		// The signal is delivered asynchronously; the fallback keeps the
		// shell convention if it is somehow blocked.
		time.Sleep(time.Second)
		os.Exit(128 + int(s))
	}
	os.Exit(1)
}
