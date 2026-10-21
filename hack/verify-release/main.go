package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const projectName = "devsy-provider-ssh"

var binaryNames = []string{
	projectName + "-darwin-amd64",
	projectName + "-darwin-arm64",
	projectName + "-linux-amd64",
	projectName + "-linux-arm64",
	projectName + "-windows-amd64.exe",
}

type releaseAsset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("expected release tag and distribution directory")
	}
	tag, dist := args[0], args[1]
	output, err := github("release", "view", tag, "--json", "assets")
	if err != nil {
		return err
	}
	var release struct {
		Assets []releaseAsset `json:"assets"`
	}
	if err := json.Unmarshal(output, &release); err != nil {
		return fmt.Errorf("decode release assets: %w", err)
	}
	if err := validateAssets(release.Assets); err != nil {
		return err
	}
	if err := downloadAndVerify(tag, dist); err != nil {
		return err
	}
	_, err = fmt.Fprintf(os.Stdout, "Verified all seven release assets for %s\n", tag)
	return err
}

func github(args ...string) ([]byte, error) {
	// #nosec G204 G702 -- Fixed gh subcommands receive argument values directly without a shell.
	command := exec.Command("gh", args...)
	command.Stderr = os.Stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func assetNames() []string {
	return append(append([]string{}, binaryNames...), "checksums.txt", "provider.yaml")
}

func validateAssets(assets []releaseAsset) error {
	for _, name := range assetNames() {
		count := 0
		for _, asset := range assets {
			if asset.Name == name && asset.Size > 0 {
				count++
			}
		}
		if count != 1 {
			return fmt.Errorf("missing, empty, or duplicate release asset: %s", name)
		}
	}
	return nil
}

func downloadAndVerify(tag, dist string) error {
	directory, err := os.MkdirTemp("", "verify-release-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(directory) }()
	for _, name := range assetNames() {
		if _, err := github(
			"release",
			"download",
			tag,
			"--pattern",
			name,
			"--dir",
			directory,
		); err != nil {
			return err
		}
	}
	return verifyDownloads(dist, directory)
}

func verifyDownloads(dist, directory string) error {
	distRoot, err := os.OpenRoot(dist)
	if err != nil {
		return err
	}
	defer func() { _ = distRoot.Close() }()
	downloadRoot, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { _ = downloadRoot.Close() }()
	if err := compareMetadata(distRoot, downloadRoot); err != nil {
		return err
	}
	contents, err := downloadRoot.ReadFile("checksums.txt")
	if err != nil {
		return err
	}
	checksums, err := parseChecksums(string(contents))
	if err != nil {
		return err
	}
	for _, name := range binaryNames {
		if err := verifyChecksum(downloadRoot, name, checksums[name]); err != nil {
			return err
		}
	}
	return nil
}

func compareMetadata(dist, download *os.Root) error {
	for _, name := range []string{"checksums.txt", "provider.yaml"} {
		expected, err := dist.ReadFile(name)
		if err != nil {
			return err
		}
		actual, err := download.ReadFile(name)
		if err != nil {
			return err
		}
		if !bytes.Equal(expected, actual) {
			return fmt.Errorf("published %s differs from build output", name)
		}
	}
	return nil
}

func parseChecksums(contents string) (map[string]string, error) {
	checksums := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid checksum line: %q", line)
		}
		if _, exists := checksums[fields[1]]; exists {
			return nil, fmt.Errorf("duplicate checksum: %s", fields[1])
		}
		checksums[fields[1]] = fields[0]
	}
	if len(checksums) != len(binaryNames) {
		return nil, fmt.Errorf(
			"expected %d binary checksums, got %d",
			len(binaryNames),
			len(checksums),
		)
	}
	for _, name := range binaryNames {
		digest, err := hex.DecodeString(checksums[name])
		if err != nil || len(digest) != sha256.Size {
			return nil, fmt.Errorf("missing or invalid checksum: %s", name)
		}
	}
	return checksums, nil
}

func verifyChecksum(root *os.Root, name, expected string) error {
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return fmt.Errorf("checksum mismatch: %s", name)
	}
	return nil
}
