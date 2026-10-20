package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/devsy-org/devsy-provider-ssh/pkg/ssh"
	"github.com/spf13/cobra"
)

// CommandCmd holds the cmd flags.
type CommandCmd struct{}

// NewCommandCmd defines a command.
func NewCommandCmd() *cobra.Command {
	cmd := &CommandCmd{}
	commandCmd := &cobra.Command{
		Use:   "command",
		Short: "Command an instance",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			sshProvider, err := ssh.NewProvider()
			if err != nil {
				return err
			}

			return cmd.Run(
				cobraCmd.Context(),
				sshProvider,
			)
		},
	}

	return commandCmd
}

// Run runs the command logic.
func (cmd *CommandCmd) Run(
	ctx context.Context,
	providerSSH *ssh.SSHProvider,
) error {
	command := os.Getenv("COMMAND")
	if command == "" {
		return fmt.Errorf("command environment variable is missing")
	}

	return ssh.Command(
		ctx,
		providerSSH,
		command,
		ssh.Streams{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr},
	)
}
