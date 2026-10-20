package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	devsyssh "github.com/devsy-org/devsy/pkg/ssh"
	"github.com/muesli/cancelreader"
	cryptossh "golang.org/x/crypto/ssh"
)

// Streams keeps protocol bytes separate from diagnostics. Non-file Stdin closers
// transfer ownership to the transport and must unblock concurrent Read on Close.
// Other non-file readers must be finite buffers; caller-owned files remain open.
type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func buildPOSIXRemoteExec(script string) (string, error) {
	if strings.ContainsRune(script, 0) {
		return "", fmt.Errorf("remote script contains NUL")
	}
	// Fish interprets backslashes inside single quotes differently from POSIX shells.
	encoded := strings.NewReplacer("\\", "'\\\\'", "'", "'\\''").Replace(script)
	return "/bin/sh -c '" + encoded + "'", nil
}

func quotePOSIXArgument(value string) (string, error) {
	if strings.ContainsRune(value, 0) {
		return "", fmt.Errorf("remote argument contains NUL")
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'", nil
}

func (provider *SSHProvider) execRemoteScript(
	ctx context.Context,
	script string,
	streams Streams,
) error {
	command, err := buildPOSIXRemoteExec(script)
	if err != nil {
		return err
	}
	if provider.Config.UseBuiltinSSH {
		return provider.execBuiltin(ctx, command, streams)
	}
	args, err := getSSHCommand(provider)
	if err != nil {
		return err
	}
	// #nosec G204 -- SSH arguments are explicit provider options; script is one encoded argument.
	cmd := exec.CommandContext(ctx, "ssh", append(args, command)...)
	cmd.Stdout, cmd.Stderr = streams.Stdout, streams.Stderr
	if err := runExternal(cmd, streams.Stdin); err != nil {
		return fmt.Errorf("execute remote /bin/sh script: %w", errors.Join(ctx.Err(), err))
	}
	return nil
}

func runExternal(cmd *exec.Cmd, input io.Reader) error {
	if input == nil {
		return cmd.Run()
	}
	if file, ok := input.(*os.File); ok {
		cmd.Stdin = file
		return cmd.Run()
	}
	reader, err := cancellableInput(input)
	if err != nil {
		return fmt.Errorf("prepare SSH stdin: %w", err)
	}
	defer func() { _ = reader.Close() }()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open SSH stdin: %w", err)
	}
	defer func() { _ = stdin.Close() }()
	if err := cmd.Start(); err != nil {
		return err
	}
	copy := startStdinCopy(reader, stdin)
	err = cmd.Wait()
	_ = stdin.Close()
	return errors.Join(err, copy.finish())
}

func (provider *SSHProvider) builtinConfig(
	ctx context.Context,
	stderr io.Writer,
) (*cryptossh.ClientConfig, string, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	args, err := getSSHCommand(provider)
	if err != nil {
		return nil, "", err
	}
	// #nosec G204 -- Resolve explicit SSH options without executing a remote command.
	lookup := exec.CommandContext(lookupCtx, "ssh", append([]string{"-G"}, args...)...)
	lookup.Stderr = stderr
	config, err := lookup.Output()
	if err != nil {
		return nil, "", fmt.Errorf("read SSH config: %w", err)
	}
	resolved := parseConfig(string(config))
	if resolved.hostname == "" || resolved.user == "" || resolved.port == "" ||
		resolved.identityfile == "" {
		return nil, "", fmt.Errorf("SSH config requires hostname, user, port and identityfile")
	}
	clientConfig, err := identityConfig(resolved.identityfile)
	if err != nil {
		return nil, "", err
	}
	clientConfig.User = resolved.user
	return clientConfig, net.JoinHostPort(resolved.hostname, resolved.port), nil
}

func identityConfig(identityfile string) (*cryptossh.ClientConfig, error) {
	if strings.HasPrefix(identityfile, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("resolve identity home: %w", err)
		}
		identityfile = filepath.Join(home, identityfile[2:])
	}
	// #nosec G304 -- Identity file is intentionally selected by the user SSH configuration.
	key, err := os.ReadFile(identityfile)
	if err != nil {
		return nil, fmt.Errorf("read SSH identity file: %w", err)
	}
	clientConfig, err := devsyssh.ConfigFromKeyBytes(key)
	if err != nil {
		return nil, fmt.Errorf("parse SSH identity: %w", err)
	}
	return clientConfig, nil
}

func (provider *SSHProvider) execBuiltin(
	ctx context.Context,
	command string,
	streams Streams,
) error {
	config, addr, err := provider.builtinConfig(ctx, streams.Stderr)
	if err != nil {
		return err
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial SSH: %w", err)
	}
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	sshConn, channels, requests, err := cryptossh.NewClientConn(conn, addr, config)
	if err != nil {
		return fmt.Errorf("SSH handshake: %w", err)
	}
	client := cryptossh.NewClient(sshConn, channels, requests)
	defer func() { _ = client.Close() }()
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer func() { _ = session.Close() }()
	session.Stdout, session.Stderr = streams.Stdout, streams.Stderr
	if err := runSession(session, command, streams.Stdin); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("execute remote /bin/sh script: %w", ctx.Err())
		}
		return fmt.Errorf("execute remote /bin/sh script: %w", err)
	}
	return nil
}

func runSession(session *cryptossh.Session, command string, input io.Reader) error {
	if input == nil {
		return session.Run(command)
	}
	reader, err := cancellableInput(input)
	if err != nil {
		return fmt.Errorf("prepare SSH stdin: %w", err)
	}
	defer func() { _ = reader.Close() }()
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("open SSH stdin: %w", err)
	}
	if err := session.Start(command); err != nil {
		return err
	}
	copy := startStdinCopy(reader, stdin)
	err = session.Wait()
	_ = stdin.Close()
	return errors.Join(err, copy.finish())
}

func cancellableInput(input io.Reader) (cancelreader.CancelReader, error) {
	switch reader := input.(type) {
	case *os.File:
		return cancellableFile(reader)
	case *bytes.Buffer, *bytes.Reader, *strings.Reader:
		return cancelreader.NewReader(input)
	case io.ReadCloser:
		return &closingInput{ReadCloser: reader}, nil
	default:
		return nil, fmt.Errorf(
			"SSH stdin must be a file, finite buffer, or interruptible ReadCloser",
		)
	}
}

type closingInput struct {
	io.ReadCloser
	once     sync.Once
	closeErr error
}

func (input *closingInput) Cancel() bool { return input.Close() == nil }

func (input *closingInput) Close() error {
	input.once.Do(func() { input.closeErr = input.ReadCloser.Close() })
	return input.closeErr
}
