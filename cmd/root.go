package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := rootCmd.ExecuteContext(ctx)
	stop()
	if err == nil {
		return
	}
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

// BuildRoot creates the SSH provider command tree.
func BuildRoot() *cobra.Command {
	rootCmd := NewRootCmd()

	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewCommandCmd())
	return rootCmd
}
