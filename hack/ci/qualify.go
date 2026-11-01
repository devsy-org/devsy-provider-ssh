package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var coreFixes = []string{
	"e22be00149ace87d512de0ec953541e3f7642c69",
	"9c80b29c7672f4cc3bc54553f57c9a915cad7d19",
}

var coreTagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$`)

func qualifyCore() error {
	input, err := loadInputs()
	if err != nil {
		return err
	}
	core, err := verifyCoreLineage(input)
	if err != nil {
		return err
	}
	provider, err := verifyProviderLineage(input)
	if err != nil {
		return err
	}
	asset, err := fetchCoreArtifact(input)
	if err != nil {
		return err
	}
	binary, err := verifyCoreBinary(input, asset)
	if err != nil {
		return err
	}
	return writeEvidence(input, qualificationResult{core, provider, asset, binary})
}

type (
	qualificationInput struct{ tag, repo, artifactKind, directory, providerTag, providerFix string }
	coreLineage        struct {
		sha      string
		statuses []string
	}
	providerLineage     struct{ tag, fix, sha string }
	coreArtifact        struct{ name, path, digest string }
	coreBinary          struct{ path, digest, version string }
	qualificationResult struct {
		core     coreLineage
		provider providerLineage
		asset    coreArtifact
		binary   coreBinary
	}
)

const (
	statusAhead     = "ahead"
	statusIdentical = "identical"
)

func loadInputs() (qualificationInput, error) {
	i := qualificationInput{
		os.Getenv("CORE_TAG"),
		os.Getenv("CORE_REPO"),
		os.Getenv("CORE_ARTIFACT"),
		os.Getenv("QUALIFICATION_DIR"),
		os.Getenv("PROVIDER_TAG"),
		os.Getenv("PROVIDER_FIX_SHA"),
	}
	if i.tag == "" || i.repo == "" || i.directory == "" || i.providerTag == "" ||
		i.providerFix == "" {
		return i, fmt.Errorf("qualification environment is incomplete")
	}
	if !validTag(i.tag) {
		return i, fmt.Errorf("core_tag must look like v1.2.3 or v1.2.3-beta.1")
	}
	return i, nil
}

func verifyCoreLineage(i qualificationInput) (coreLineage, error) {
	sha, err := command("gh", "api", "repos/"+i.repo+"/commits/"+i.tag, "--jq", ".sha")
	if err != nil {
		return coreLineage{}, err
	}
	lineage := coreLineage{sha: strings.TrimSpace(sha), statuses: make([]string, len(coreFixes))}
	for index, fix := range coreFixes {
		status, err := command(
			"gh",
			"api",
			"repos/"+i.repo+"/compare/"+fix+"..."+lineage.sha,
			"--jq",
			".status",
		)
		if err != nil {
			return coreLineage{}, err
		}
		lineage.statuses[index] = strings.TrimSpace(status)
	}
	return lineage, validateAncestry(lineage.statuses)
}

func verifyProviderLineage(i qualificationInput) (providerLineage, error) {
	sha, err := command("git", "rev-parse", "--verify", "refs/tags/"+i.providerTag+"^{commit}")
	if err != nil {
		return providerLineage{}, err
	}
	lineage := providerLineage{i.providerTag, i.providerFix, strings.TrimSpace(sha)}
	if _, err := command(
		"git",
		"merge-base",
		"--is-ancestor",
		lineage.fix,
		lineage.sha,
	); err != nil {
		return providerLineage{}, fmt.Errorf(
			"provider %s does not contain %s",
			lineage.tag,
			lineage.fix,
		)
	}
	return lineage, nil
}

func fetchCoreArtifact(i qualificationInput) (coreArtifact, error) {
	name := map[string]string{"cli": "devsy-linux-amd64", "desktop": "Devsy_linux_amd64.deb"}[i.artifactKind]
	if name == "" {
		return coreArtifact{}, fmt.Errorf("unknown core artifact %s", i.artifactKind)
	}
	directory := i.directory + "/core"
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return coreArtifact{}, err
	}
	if _, err := command(
		"gh",
		"release",
		"download",
		i.tag,
		"--repo",
		i.repo,
		"--pattern",
		name,
		"--dir",
		directory,
		"--clobber",
	); err != nil {
		return coreArtifact{}, err
	}
	path := directory + "/" + name
	output, err := command("sha256sum", path)
	if err != nil {
		return coreArtifact{}, err
	}
	digest, err := parseSHA256Output(output)
	if err != nil {
		return coreArtifact{}, err
	}
	published, err := command(
		"gh",
		"api",
		"repos/"+i.repo+"/releases/tags/"+i.tag,
		"--jq",
		".assets[] | select(.name == \""+name+"\") | .digest",
	)
	if err != nil {
		return coreArtifact{}, err
	}
	if err := validateAssetDigest(strings.TrimSpace(published), digest); err != nil {
		return coreArtifact{}, err
	}
	return coreArtifact{name, path, digest}, nil
}

func verifyCoreBinary(i qualificationInput, asset coreArtifact) (coreBinary, error) {
	path := asset.path
	if i.artifactKind == "desktop" {
		destination := i.directory + "/core/desktop"
		if _, err := command("dpkg-deb", "--extract", asset.path, destination); err != nil {
			return coreBinary{}, err
		}
		path = destination + "/opt/Devsy/resources/bin/devsy"
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return coreBinary{}, err
	}
	output, err := command("sha256sum", path)
	if err != nil {
		return coreBinary{}, err
	}
	digest, err := parseSHA256Output(output)
	if err != nil {
		return coreBinary{}, err
	}
	version, err := runCoreBinary(path)
	if err != nil {
		return coreBinary{}, err
	}
	version = strings.TrimSpace(version)
	if version != i.tag {
		return coreBinary{}, fmt.Errorf("core binary reports %q, expected %q", version, i.tag)
	}
	return coreBinary{path, digest, version}, nil
}

func runCoreBinary(path string) (string, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return "", err
	}
	process, err := os.StartProcess(path, []string{path, "--version"}, &os.ProcAttr{
		Dir:   "",
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, writer, os.Stderr},
	})
	_ = writer.Close()
	if err != nil {
		_ = reader.Close()
		return "", err
	}
	output, readErr := io.ReadAll(reader)
	_ = reader.Close()
	state, err := process.Wait()
	if err != nil {
		return "", err
	}
	if readErr != nil {
		return "", readErr
	}
	if !state.Success() {
		return "", fmt.Errorf("core binary failed: %s", state)
	}
	return string(output), nil
}

func writeEvidence(i qualificationInput, result qualificationResult) error {
	harness, err := command("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	values := []string{
		"DEVSY_E2E_BINARY=" + result.binary.path,
		"DEVSY_E2E_CORE_TAG=" + i.tag,
		"DEVSY_E2E_CORE_SHA=" + result.core.sha,
		"DEVSY_E2E_CORE_VERSION=" + result.binary.version,
		"DEVSY_E2E_PROVIDER_TAG=" + result.provider.tag,
		"DEVSY_E2E_PROVIDER_SHA=" + result.provider.sha,
		"CORE_SHA256=" + result.binary.digest,
	}
	if err := appendLines(os.Getenv("GITHUB_ENV"), values); err != nil {
		return err
	}
	evidence := []string{
		"core tag: " + i.tag,
		"core sha: " + result.core.sha,
		"core artifact: " + result.asset.name,
		"core artifact source: https://github.com/" + i.repo + "/releases/download/" + i.tag + "/" + result.asset.name,
		"tested binary platform: linux/amd64",
		"core artifact sha256: " + result.asset.digest + " (matches published release digest)",
		"tested core binary sha256: " + result.binary.digest,
		"core version: " + result.binary.version,
		"core fix " + coreFixes[0] + " is an ancestor (compare status: " + result.core.statuses[0] + ")",
		"core fix " + coreFixes[1] + " is an ancestor (compare status: " + result.core.statuses[1] + ")",
		"provider tag: " + result.provider.tag,
		"provider sha: " + result.provider.sha,
		"qualification harness sha: " + strings.TrimSpace(harness),
		"provider fix " + result.provider.fix + " is an ancestor",
	}
	if err := appendLines(i.directory+"/pair-verification.txt", evidence); err != nil {
		return err
	}
	return appendLines(os.Getenv("GITHUB_STEP_SUMMARY"), evidence)
}

func validateAncestry(statuses []string) error {
	if len(statuses) != len(coreFixes) {
		return fmt.Errorf(
			"expected %d core ancestry results, got %d",
			len(coreFixes),
			len(statuses),
		)
	}
	for i, status := range statuses {
		if status != statusAhead && status != statusIdentical {
			return fmt.Errorf(
				"core fix %s is not an ancestor (compare status: %s)",
				coreFixes[i],
				status,
			)
		}
	}
	return nil
}

func validateAssetDigest(published, digest string) error {
	if published != "sha256:"+digest {
		return fmt.Errorf("sha256:%s differs from published digest %q", digest, published)
	}
	return nil
}

func validTag(tag string) bool {
	return coreTagPattern.MatchString(tag)
}
