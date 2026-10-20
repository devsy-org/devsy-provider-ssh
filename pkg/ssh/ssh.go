package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/devsy-org/devsy-provider-ssh/pkg/options"
	"github.com/kballard/go-shellquote"
)

type SSHProvider struct {
	Config           *options.Options
	WorkingDirectory string
}

func NewProvider() (*SSHProvider, error) {
	config, err := options.FromEnv()
	if err != nil {
		return nil, err
	}

	// create provider
	provider := &SSHProvider{
		Config: config,
	}

	return provider, nil
}

func getSSHCommand(provider *SSHProvider) ([]string, error) {
	result := []string{"-oStrictHostKeyChecking=no", "-oBatchMode=yes"}

	if provider.Config.Port != "22" {
		result = append(result, []string{"-p", provider.Config.Port}...)
	}

	if provider.Config.ExtraFlags != "" {
		flags, err := shellquote.Split(provider.Config.ExtraFlags)
		if err != nil {
			return nil, fmt.Errorf("parse EXTRA_FLAGS: %w", err)
		}

		result = append(result, flags...)
	}

	result = append(result, provider.Config.Host)
	return result, nil
}

func Init(ctx context.Context, provider *SSHProvider, streams Streams) error {
	if strings.ContainsRune(provider.Config.AgentPath, 0) ||
		strings.ContainsRune(provider.Config.DockerPath, 0) {
		return fmt.Errorf("remote executable or agent path contains NUL")
	}
	run := func(script string, output io.Writer) error {
		return provider.execRemoteScript(
			ctx,
			script,
			Streams{Stdout: output, Stderr: streams.Stderr},
		)
	}
	if err := checkRemotePlatform(run); err != nil {
		return err
	}
	uid, err := remoteUID(run)
	if err != nil {
		return err
	}
	if uid == 0 {
		return nil
	}
	return provider.checkAccess(run)
}

func checkRemotePlatform(run func(string, io.Writer) error) error {
	for _, check := range []struct{ script, expected, stage string }{
		{"echo Devsy Test", "Devsy Test\n", "check remote /bin/sh and SSH connection"},
		{"uname", "Linux\n", "check Linux operating system"},
	} {
		var out bytes.Buffer
		if err := run(check.script, &out); err != nil {
			return fmt.Errorf("%s: %w", check.stage, err)
		}
		if out.String() != check.expected {
			return fmt.Errorf(
				"%s: unexpected SSH stdout; check login shell startup output",
				check.stage,
			)
		}
	}
	return nil
}

func remoteUID(run func(string, io.Writer) error) (uint64, error) {
	var out bytes.Buffer
	if err := run("id -ru", &out); err != nil {
		return 0, fmt.Errorf("detect remote user: %w", err)
	}
	uid, err := strconv.ParseUint(strings.TrimSuffix(out.String(), "\n"), 10, 32)
	if err != nil || !strings.HasSuffix(out.String(), "\n") {
		return 0, fmt.Errorf(
			"detect remote user: unexpected SSH stdout; check login shell startup output",
		)
	}
	return uid, nil
}

func (provider *SSHProvider) checkAccess(run func(string, io.Writer) error) error {
	agentDir, err := quotePOSIXArgument(path.Dir(provider.Config.AgentPath))
	if err != nil {
		return fmt.Errorf("agent path: %w", err)
	}
	dockerPath, err := quotePOSIXArgument(provider.Config.DockerPath)
	if err != nil {
		return fmt.Errorf("docker path: %w", err)
	}
	if err := checkAgentDirectory(run, agentDir); err != nil {
		return err
	}
	if err := run(dockerPath+" ps", io.Discard); err != nil {
		if sudoErr := run("sudo -nl", io.Discard); sudoErr != nil {
			return fmt.Errorf(
				"docker executable inaccessible; passwordless sudo or docker group required: %w",
				err,
			)
		}
	}
	return nil
}

func checkAgentDirectory(run func(string, io.Writer) error, agentDir string) error {
	mkdirErr := run("mkdir -p -- "+agentDir, io.Discard)
	writableErr := run("test -w "+agentDir, io.Discard)
	if mkdirErr == nil && writableErr == nil {
		return nil
	}
	if err := run("sudo -nl", io.Discard); err != nil {
		return fmt.Errorf(
			"agent directory inaccessible; passwordless sudo or root required: %w",
			errors.Join(mkdirErr, writableErr, err),
		)
	}
	return nil
}

func Command(ctx context.Context, provider *SSHProvider, command string, streams Streams) error {
	return provider.execRemoteScript(ctx, command, streams)
}

type resolvedConfig struct{ hostname, user, port, identityfile string }

func parseConfig(config string) resolvedConfig {
	var resolved resolvedConfig
	for line := range strings.SplitSeq(config, "\n") {
		key, value, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		if key == "hostname" {
			resolved.hostname = strings.TrimSpace(value)
			continue
		}
		if key == "user" {
			resolved.user = strings.TrimSpace(value)
			continue
		}
		if key == "port" {
			resolved.port = strings.TrimSpace(value)
			continue
		}
		// just take the first one
		if key == "identityfile" && resolved.identityfile == "" {
			resolved.identityfile = strings.TrimSpace(value)
			continue
		}
	}

	return resolved
}
