package ssh

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devsy-org/devsy-provider-ssh/pkg/options"
)

func TestRemoteExecShellRoundTrip(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			executable, err := exec.LookPath(shell)
			if err != nil {
				if os.Getenv("CI") != "" {
					t.Fatalf("required shell %s unavailable", shell)
				}
				t.Skipf("%s unavailable", shell)
			}
			values := []string{
				"", "\n", "a'b", `a\b\\c`,
				"$HOME `touch sentinel` (touch sentinel) $(touch sentinel)",
				"UTF8: café 世界\ntrailing\n", "'\\'\n",
			}
			for _, value := range values {
				scriptArg, err := quotePOSIXArgument(value)
				requireNoError(t, err)
				script := "printf '%s' " + scriptArg
				command, err := buildPOSIXRemoteExec(script)
				requireNoError(t, err)
				// #nosec G204 -- Exercise the selected installed shell with controlled test data.
				cmd := exec.Command(executable, "-c", command)
				cmd.Dir = t.TempDir()
				output, err := cmd.Output()
				if err != nil {
					t.Fatalf("%q: %v", value, err)
				}
				if string(output) != value {
					t.Fatalf("got %q, want %q", output, value)
				}
				if _, err := os.Stat(
					filepath.Join(cmd.Dir, "sentinel"),
				); !errors.Is(
					err,
					os.ErrNotExist,
				) {
					t.Fatalf("substitution executed: %v", err)
				}
			}
		})
	}
}

func TestRemoteExecRejectsNUL(t *testing.T) {
	if _, err := buildPOSIXRemoteExec("hello\x00world"); err == nil {
		t.Fatal("accepted NUL")
	}
	if _, err := quotePOSIXArgument("hello\x00world"); err == nil {
		t.Fatal("accepted NUL argument")
	}
}

func fakeExternalSSH(t *testing.T) *SSHProvider {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nwhile [ \"$#\" -gt 1 ]; do shift; done\nexec /bin/sh -c \"$1\"\n"
	// #nosec G306 -- Test SSH stub must be executable.
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &SSHProvider{Config: &options.Options{Host: "unused", Port: "22"}}
}

func TestExternalStreamsAndExit(t *testing.T) {
	provider := fakeExternalSSH(t)
	payload := []byte{0, 1, 2, 255, '\n'}
	var stdout, stderr bytes.Buffer
	err := Command(
		context.Background(),
		provider,
		"cat; printf diagnostic >&2; exit 37",
		Streams{Stdin: bytes.NewReader(payload), Stdout: &stdout, Stderr: &stderr},
	)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 37 {
		t.Fatalf("exit: %v", err)
	}
	if !bytes.Equal(stdout.Bytes(), payload) {
		t.Fatalf("stdout changed: %q", stdout.Bytes())
	}
	if stderr.String() != "diagnostic" {
		t.Fatalf("stderr: %q", stderr.String())
	}
}

func TestExternalCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "canceled"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			provider := fakeExternalSSH(t)
			ctx, cancel := context.WithCancel(context.Background())
			expected := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
				expected = context.DeadlineExceeded
			} else {
				timer := time.AfterFunc(100*time.Millisecond, cancel)
				defer timer.Stop()
			}
			defer cancel()
			start := time.Now()
			err := Command(ctx, provider, "exec sleep 30", Streams{})
			if !errors.Is(err, expected) {
				t.Fatalf("expected context identity %v: %v", expected, err)
			}
			if time.Since(start) > 5*time.Second {
				t.Fatal("cancellation did not terminate SSH")
			}
		})
	}
}

func TestSSHArguments(t *testing.T) {
	provider := &SSHProvider{
		Config: &options.Options{
			Host:       "alias",
			Port:       "2222",
			ExtraFlags: `-i '/key with spaces' -F config`,
		},
	}
	args, err := getSSHCommand(provider)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(
		args,
		"|",
	) != "-oStrictHostKeyChecking=no|-oBatchMode=yes|-p|2222|-i|/key with spaces|-F|config|alias" {
		t.Fatalf("args: %q", args)
	}
}

func TestExactBootstrapArgument(t *testing.T) {
	script := "\n'\\$HOME `touch sentinel` (touch sentinel) $(touch sentinel)\nUTF8: 世界\n"
	launcher, err := buildPOSIXRemoteExec(script)
	requireNoError(t, err)
	dir := t.TempDir()
	capture := filepath.Join(dir, "capture")
	// #nosec G306 -- Argument capture fixture must be executable.
	requireNoError(t, os.WriteFile(capture, []byte("#!/bin/sh\nprintf '%s\\000' \"$@\"\n"), 0o700))
	for _, shell := range []string{"bash", "zsh", "fish"} {
		t.Run(shell, func(t *testing.T) {
			executable, err := exec.LookPath(shell)
			if err != nil {
				if os.Getenv("CI") != "" {
					t.Fatalf("required shell %s unavailable", shell)
				}
				t.Skipf("%s unavailable", shell)
			}
			command := capture + strings.TrimPrefix(launcher, "/bin/sh")
			// #nosec G204 -- Capture shell argument boundaries with controlled test fixtures.
			cmd := exec.Command(executable, "-c", command)
			cmd.Dir = dir
			out, err := cmd.Output()
			requireNoError(t, err)
			if string(out) != "-c\x00"+script+"\x00" {
				t.Fatalf("argument boundaries changed: %q", out)
			}
			if _, err := os.Stat(filepath.Join(dir, "sentinel")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("substitution executed: %v", err)
			}
		})
	}
}

func TestInitQuotesRemotePaths(t *testing.T) {
	provider := fakeExternalSSH(t)
	dir := t.TempDir()
	for name, contents := range map[string]string{
		"uname": "#!/bin/sh\nprintf 'Linux\\n'\n",
		"id":    "#!/bin/sh\nprintf '1000\\n'\n",
	} {
		// #nosec G306 -- Controlled platform probes must be executable.
		requireNoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o700))
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	provider.Config.AgentPath = filepath.Join(dir, "agent 'quoted' $HOME `false` \\ path", "agent")
	provider.Config.DockerPath = filepath.Join(dir, "docker 'quoted' $HOME \\ path")
	// #nosec G306 -- Controlled Docker executable tests literal path semantics.
	requireNoError(
		t,
		os.WriteFile(provider.Config.DockerPath, []byte("#!/bin/sh\ntest \"$1\" = ps\n"), 0o700),
	)
	var stdout, stderr bytes.Buffer
	requireNoError(
		t,
		Init(context.Background(), provider, Streams{Stdout: &stdout, Stderr: &stderr}),
	)
	_, err := os.Stat(filepath.Dir(provider.Config.AgentPath))
	requireNoError(t, err)
	if stdout.Len() != 0 {
		t.Fatalf("init polluted stdout: %q", stdout.Bytes())
	}
}

func TestInitRejectsNULPaths(t *testing.T) {
	provider := &SSHProvider{
		Config: &options.Options{AgentPath: "/tmp/agent\x00suffix", DockerPath: "docker"},
	}
	if err := Init(context.Background(), provider, Streams{}); err == nil {
		t.Fatal("accepted NUL path")
	}
}

func TestRemoteUIDRejectsStartupOutput(t *testing.T) {
	for _, output := range []string{"1000", "welcome\n1000\n", "1000\nextra\n", "1000\n\n"} {
		_, err := remoteUID(
			func(_ string, out io.Writer) error { _, err := io.WriteString(out, output); return err },
		)
		if err == nil {
			t.Fatalf("accepted polluted UID stdout: %q", output)
		}
	}
}
