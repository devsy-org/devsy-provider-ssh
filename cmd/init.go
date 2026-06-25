package cmd

import (
	"context"

	"github.com/devsy-org/devsy-provider-ssh/pkg/ssh"
	"github.com/spf13/cobra"
)

// InitCmd holds the cmd flags
type InitCmd struct{}

// NewInitCmd defines a init
func NewInitCmd() *cobra.Command {
	cmd := &InitCmd{}
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Init account",
		RunE: func(_ *cobra.Command, args []string) error {
			sshProvider, err := ssh.NewProvider()
			if err != nil {
				return err
			}

			return cmd.Run(
				context.Background(),
				sshProvider,
			)
		},
	}

	return initCmd
}

// Run runs the init logic
func (cmd *InitCmd) Run(
	ctx context.Context,
	providerSSH *ssh.SSHProvider,
) error {
	return ssh.Init(providerSSH)
}
