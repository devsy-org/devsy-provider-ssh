package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
)

const (
	guardModuleDir      = "e2e"
	guardLogName        = "missing-binary-guard.log"
	guardMessage        = "DEVSY_E2E_BINARY is required"
	envCoreBinary       = "DEVSY_E2E_BINARY"
	envReleaseQualified = "DEVSY_E2E_RELEASE_QUALIFICATION"
)

func requireExplicitCandidate() error {
	env, err := requireEnv("QUALIFICATION_DIR")
	if err != nil {
		return err
	}
	output, exitCode, err := runMissingBinaryGuard()
	if err != nil {
		return err
	}
	logPath := filepath.Join(env["QUALIFICATION_DIR"], guardLogName)
	if err := os.WriteFile(logPath, output, 0o600); err != nil {
		return err
	}
	if _, err := os.Stdout.Write(output); err != nil {
		return err
	}
	return evaluateGuardResult(exitCode, string(output))
}

func runMissingBinaryGuard() ([]byte, int, error) {
	cmd := exec.Command(
		"go",
		"test",
		"-v",
		"-timeout",
		"300s",
		"--ginkgo.v",
		"--ginkgo.label-filter=release-qualification",
		"--ginkgo.focus="+missingBinaryFocus(),
	)
	cmd.Dir = guardModuleDir
	cmd.Env = guardEnvironment()
	output, err := cmd.CombinedOutput()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return output, exitErr.ExitCode(), nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("run missing binary guard: %w", err)
	}
	return output, 0, nil
}

func missingBinaryFocus() string {
	transport := qualification.Transports[0]
	return fmt.Sprintf(
		"%s builtin=%t %s",
		transport.Shell,
		transport.Builtin,
		qualification.ScenarioColdDownload,
	)
}

func guardEnvironment() []string {
	environ := os.Environ()
	env := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if !strings.HasPrefix(entry, envCoreBinary+"=") {
			env = append(env, entry)
		}
	}
	return append(env, envReleaseQualified+"=true")
}

func evaluateGuardResult(exitCode int, output string) error {
	if exitCode != 1 {
		return fmt.Errorf("missing binary guard exited with %d, expected 1", exitCode)
	}
	if !strings.Contains(output, guardMessage) {
		return fmt.Errorf("missing binary guard output does not contain %q", guardMessage)
	}
	return nil
}
