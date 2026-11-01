package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	providerAssetName = "devsy-provider-ssh-linux-amd64"
	providerBinaryDir = "dist/build_linux_amd64_v1"
	providerDistDir   = "dist"
	providerChecksums = "checksums.txt"
)

type providerRelease struct {
	tag              string
	repo             string
	qualificationDir string
}

func verifyProvider() error {
	env, err := requireEnv("PROVIDER_TAG", "QUALIFICATION_DIR", "GITHUB_REPOSITORY")
	if err != nil {
		return err
	}
	release := providerRelease{
		tag:              env["PROVIDER_TAG"],
		repo:             env["GITHUB_REPOSITORY"],
		qualificationDir: env["QUALIFICATION_DIR"],
	}
	return release.verify()
}

func (r providerRelease) verify() error {
	binary := filepath.Join(providerBinaryDir, providerAssetName)
	if err := r.download(); err != nil {
		return err
	}
	digest, err := providerDigest(binary)
	if err != nil {
		return err
	}
	if err := r.requirePublishedDigest(digest); err != nil {
		return err
	}
	if err := os.Chmod(binary, 0o700); err != nil {
		return err
	}
	evidence := []string{
		"provider binary sha256: " + digest + " (matches published release digest)",
	}
	pairPath := filepath.Join(r.qualificationDir, "pair-verification.txt")
	if err := appendLines(pairPath, evidence); err != nil {
		return err
	}
	return appendLines(os.Getenv("GITHUB_STEP_SUMMARY"), evidence)
}

func (r providerRelease) download() error {
	if err := os.MkdirAll(providerBinaryDir, 0o750); err != nil {
		return err
	}
	if _, err := command(
		"gh",
		"release",
		"download",
		r.tag,
		"--repo",
		r.repo,
		"--pattern",
		providerAssetName,
		"--dir",
		providerBinaryDir,
		"--clobber",
	); err != nil {
		return fmt.Errorf("download provider binary: %w", err)
	}
	if _, err := command(
		"gh",
		"release",
		"download",
		r.tag,
		"--repo",
		r.repo,
		"--pattern",
		providerChecksums,
		"--dir",
		providerDistDir,
		"--clobber",
	); err != nil {
		return fmt.Errorf("download provider checksums: %w", err)
	}
	return nil
}

func (r providerRelease) requirePublishedDigest(digest string) error {
	published, err := command(
		"gh",
		"api",
		"repos/"+r.repo+"/releases/tags/"+r.tag,
		"--jq",
		".assets[] | select(.name == \""+providerAssetName+"\") | .digest",
	)
	if err != nil {
		return fmt.Errorf("read published provider digest: %w", err)
	}
	return validateAssetDigest(strings.TrimSpace(published), digest)
}

func providerDigest(binary string) (string, error) {
	output, err := command("sha256sum", binary)
	if err != nil {
		return "", err
	}
	return parseSHA256Output(output)
}
