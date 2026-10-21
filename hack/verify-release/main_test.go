package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateAssets(t *testing.T) {
	var assets []releaseAsset
	for _, name := range assetNames() {
		assets = append(assets, releaseAsset{Name: name, Size: 1})
	}
	if err := validateAssets(assets); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name   string
		assets []releaseAsset
	}{
		{name: "empty release"},
		{name: "missing asset", assets: assets[:len(assets)-1]},
		{name: "empty binary", assets: append([]releaseAsset{{Name: binaryNames[0]}}, assets[1:]...)},
		{name: "duplicate binary", assets: append(append([]releaseAsset{}, assets...), assets[0])},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if err := validateAssets(scenario.assets); err == nil {
				t.Fatal("accepted incomplete release")
			}
		})
	}
}

func TestVerifyDownloads(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		mutate func(*testing.T, string, string)
	}{
		{name: "complete release"},
		{name: "corrupt binary", mutate: func(t *testing.T, _, download string) {
			writeFile(t, download, binaryNames[0], "corrupt")
		}},
		{name: "stale manifest", mutate: func(t *testing.T, _, download string) {
			writeFile(t, download, "provider.yaml", "version: wrong")
		}},
		{name: "stale checksums", mutate: func(t *testing.T, _, download string) {
			writeFile(t, download, "checksums.txt", "wrong")
		}},
		{name: "missing binary", mutate: func(t *testing.T, _, download string) {
			if err := os.Remove(filepath.Join(download, binaryNames[0])); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			dist, download := t.TempDir(), t.TempDir()
			checksums := checksumFixture()
			for _, directory := range []string{dist, download} {
				writeFile(t, directory, "checksums.txt", checksums)
				writeFile(t, directory, "provider.yaml", "version: v1.1.46\n")
			}
			for _, name := range binaryNames {
				writeFile(t, download, name, name)
			}
			if scenario.mutate != nil {
				scenario.mutate(t, dist, download)
			}
			err := verifyDownloads(dist, download)
			if (err != nil) != (scenario.mutate != nil) {
				t.Fatalf("unexpected verification result: %v", err)
			}
		})
	}
}

func TestParseChecksums(t *testing.T) {
	valid := checksumFixture()
	for _, contents := range []string{
		"",
		"malformed",
		strings.Join(strings.Split(strings.TrimSpace(valid), "\n")[1:], "\n"),
		valid + strings.Split(valid, "\n")[0] + "\n",
		strings.Replace(valid, binaryNames[0], "unexpected", 1),
		strings.Replace(valid, fmt.Sprintf("%x", sha256.Sum256([]byte(binaryNames[0]))), "invalid", 1),
	} {
		if _, err := parseChecksums(contents); err == nil {
			t.Fatalf("accepted invalid checksums: %q", contents)
		}
	}
	if _, err := parseChecksums(valid); err != nil {
		t.Fatal(err)
	}
}

func checksumFixture() string {
	var contents strings.Builder
	for _, name := range binaryNames {
		fmt.Fprintf(&contents, "%x  %s\n", sha256.Sum256([]byte(name)), name)
	}
	return contents.String()
}

func writeFile(t *testing.T, directory, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
