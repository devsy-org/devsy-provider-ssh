package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

// NewRootCmd returns a new root command.
func NewRootCmd() *cobra.Command {
	sshCmd := &cobra.Command{
		Use:           "devsy-provider-ssh",
		Short:         "ssh Provider commands",
		SilenceErrors: true,
		SilenceUsage:  true,

		PersistentPreRunE: func(cobraCmd *cobra.Command, args []string) error {
			cfg := log.Config{Verbosity: 1}
			if os.Getenv("DEVSY_DEBUG") == "true" {
				cfg.Debug = true
			}
			log.Init(cfg)

			return nil
		},
	}

	return sshCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// build the root command
	rootCmd := BuildRoot()

	// execute command
	ctx, received, stop := notifyContext(context.Background())
	err := rootCmd.ExecuteContext(ctx)
	stop()
	if err == nil {
		return
	}
	// Devsy ends the tunnel by signalling this process once the remote work
	// is done. Cleanup has already run, so report the signal as an expected
	// end instead of a fatal error.
	if sig := received(); sig != nil && errors.Is(err, context.Canceled) {
		log.Debugf("stopped by %s", sig)
		exitBySignal(sig)
	}
	exitWithError(err)
}

func exitWithError(err error) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		log.Fatal(err)
	}
	if sshExit, ok := errors.AsType[*ssh.ExitError](err); ok {
		os.Exit(sshExit.ExitStatus())
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		if len(exitErr.Stderr) > 0 {
			log.Error(string(exitErr.Stderr))
		}
		os.Exit(exitErr.ExitCode())
	}
	log.Fatal(err)
}

// notifyContext is signal.NotifyContext that also reports which signal fired.
func notifyContext(parent context.Context) (context.Context, func() os.Signal, func()) {
	ctx, cancel := context.WithCancel(parent)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	var first atomic.Value
	go func() {
		select {
		case sig := <-signals:
			first.Store(sig)
			cancel()
		case <-ctx.Done():
		}
	}()
	received := func() os.Signal {
		sig, _ := first.Load().(os.Signal)
		return sig
	}
	return ctx, received, func() {
		signal.Stop(signals)
		cancel()
	}
}

// BuildRoot creates the SSH provider command tree.
func BuildRoot() *cobra.Command {
	rootCmd := NewRootCmd()

	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewCommandCmd())
	return rootCmd
}
