package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var allowedTools = map[string]struct{}{
	"dpkg-deb":  {},
	"gh":        {},
	"git":       {},
	"install":   {},
	"rm":        {},
	"service":   {},
	"sha256sum": {},
	"sudo":      {},
	"test":      {},
	"useradd":   {},
	"usermod":   {},
	"visudo":    {},
}

func command(name string, args ...string) (string, error) {
	if _, ok := allowedTools[name]; !ok {
		return "", fmt.Errorf("unsupported command %q", name)
	}
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(output), nil
}

func requireEnv(names ...string) (map[string]string, error) {
	values := make(map[string]string, len(names))
	var missing []string
	for _, name := range names {
		value := os.Getenv(name)
		if value == "" {
			missing = append(missing, name)
		}
		values[name] = value
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment: %s", strings.Join(missing, ", "))
	}
	return values, nil
}

func appendLines(path string, lines []string) error {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	directory, name := filepath.Split(filepath.Clean(path))
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	file, err := root.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	for _, line := range lines {
		if _, err := fmt.Fprintln(file, line); err != nil {
			return err
		}
	}
	return nil
}

func parseSHA256Output(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) < 2 || len(fields[0]) != sha256.Size*2 {
		return "", fmt.Errorf("invalid sha256sum output %q", output)
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", fmt.Errorf("invalid sha256sum output %q", output)
	}
	return fields[0], nil
}
