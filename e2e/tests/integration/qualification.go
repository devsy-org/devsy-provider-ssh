package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/devsy-org/devsy/e2e/framework"
	"github.com/onsi/ginkgo/v2"
)

const (
	envQualification = "DEVSY_E2E_RELEASE_QUALIFICATION"
	envCoreBinary    = "DEVSY_E2E_BINARY"
	envCoreVersion   = "DEVSY_E2E_CORE_VERSION"
	envCoreTag       = "DEVSY_E2E_CORE_TAG"
	envCoreSHA       = "DEVSY_E2E_CORE_SHA"
	envReportPath    = "DEVSY_E2E_REPORT"
	envProviderTag   = "DEVSY_E2E_PROVIDER_TAG"
	envProviderSHA   = "DEVSY_E2E_PROVIDER_SHA"

	defaultReportPath = "qualification-report.json"

	// Any of these changes how the core selects the agent transport or disables the
	// remote version check, so a qualification run must not inherit them.
	envAgentURL            = "DEVSY_AGENT_URL"
	envAgentBinary         = "DEVSY_AGENT_BINARY"
	envAgentPreferDownload = "DEVSY_AGENT_PREFER_DOWNLOAD"

	devVersion = "v0.0.0"
)

// qualifiedPair identifies the exact core artifact and provider build under test.
type qualifiedPair struct {
	CoreBinary           string `json:"coreBinary"`
	CoreTag              string `json:"coreTag"`
	CoreSHA              string `json:"coreSha"`
	CoreVersion          string `json:"coreVersion"`
	CoreBinarySHA256     string `json:"coreBinarySha256"`
	ProviderRef          string `json:"providerRef"`
	ProviderSHA          string `json:"providerSha"`
	ProviderBinary       string `json:"providerBinary"`
	ProviderBinarySHA256 string `json:"providerBinarySha256"`
	HarnessSHA           string `json:"harnessSha"`
}

type scenarioResult struct {
	Name    string `json:"name"`
	Outcome string `json:"outcome"`
}

type cellReport struct {
	Shell         string            `json:"shell"`
	User          string            `json:"user"`
	BuiltinSSH    bool              `json:"builtinSsh"`
	Workspace     string            `json:"workspace"`
	AgentDir      string            `json:"agentDir"`
	LoginShell    string            `json:"loginShell"`
	Scenarios     []scenarioResult  `json:"scenarios"`
	AgentSHA256   map[string]string `json:"agentSha256"`
	CleanupStatus string            `json:"cleanup"`
}

type qualificationReport struct {
	Pair  qualifiedPair `json:"pair"`
	Cells []*cellReport `json:"cells"`
}

var (
	qualificationMu    sync.Mutex
	qualificationState *qualifiedPair
	currentReport      = &qualificationReport{}
)

func qualificationEnabled() bool {
	return os.Getenv(envQualification) == "true"
}

// requireQualifiedPair fails instead of skipping: a release row that silently
// falls back to a downloaded core would qualify the wrong pair.
func requireQualifiedPair() qualifiedPair {
	qualificationMu.Lock()
	defer qualificationMu.Unlock()
	if qualificationState != nil {
		return *qualificationState
	}

	requireQualificationEnvironment()

	pair := qualifiedPair{
		CoreBinary:     requiredEnv(envCoreBinary, "the candidate core CLI binary"),
		CoreVersion:    requiredEnv(envCoreVersion, "the expected `devsy --version` output"),
		CoreTag:        requiredEnv(envCoreTag, "the core release tag under test"),
		CoreSHA:        requiredEnv(envCoreSHA, "the core commit the tag resolves to"),
		ProviderBinary: providerPath,
	}
	if pair.CoreVersion == devVersion {
		ginkgo.Fail(
			envCoreVersion + " must be a release version: dev builds skip the remote version check",
		)
	}

	pair.CoreBinarySHA256 = mustSHA256(pair.CoreBinary)
	pair.ProviderBinarySHA256 = mustSHA256(pair.ProviderBinary)
	pair.HarnessSHA = gitOutput("rev-parse", "HEAD")
	pair.ProviderRef = os.Getenv(envProviderTag)
	pair.ProviderSHA = os.Getenv(envProviderSHA)
	if (pair.ProviderRef == "") != (pair.ProviderSHA == "") {
		ginkgo.Fail(envProviderTag + " and " + envProviderSHA + " must be set together")
	}
	if pair.ProviderRef == "" {
		pair.ProviderRef = gitOutput("describe", "--tags", "--always", "--dirty")
		pair.ProviderSHA = pair.HarnessSHA
	}

	// #nosec G204 -- The test runner explicitly selects the candidate core binary.
	output, err := exec.Command(pair.CoreBinary, "--version").Output()
	framework.ExpectNoError(err, "run candidate core --version")
	if actual := strings.TrimSpace(string(output)); actual != pair.CoreVersion {
		ginkgo.Fail(fmt.Sprintf(
			"candidate core reports version %q, expected %q", actual, pair.CoreVersion,
		))
	}

	qualificationState = &pair
	currentReport.Pair = pair
	return pair
}

func requireQualificationEnvironment() {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		ginkgo.Fail(fmt.Sprintf(
			"release qualification requires linux/amd64, running on %s/%s",
			runtime.GOOS, runtime.GOARCH,
		))
	}
	for _, name := range []string{envAgentURL, envAgentBinary, envAgentPreferDownload} {
		if os.Getenv(name) != "" {
			ginkgo.Fail(
				name + " must be unset: it overrides agent transport or disables the version check",
			)
		}
	}
}

func requiredEnv(name, description string) string {
	value := os.Getenv(name)
	if value == "" {
		ginkgo.Fail(fmt.Sprintf("%s is required: set it to %s", name, description))
	}
	return value
}

func mustSHA256(path string) string {
	// #nosec G304 -- The test runner selects these artifact paths.
	file, err := os.Open(path)
	framework.ExpectNoError(err, "open "+path)
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	framework.ExpectNoError(err, "hash "+path)
	return hex.EncodeToString(hash.Sum(nil))
}

func gitOutput(args ...string) string {
	// #nosec G204 -- Fixed git subcommands inspect the provider checkout under test.
	output, err := exec.Command("git", append([]string{"-C", ".."}, args...)...).Output()
	framework.ExpectNoError(err, "git "+strings.Join(args, " "))
	return strings.TrimSpace(string(output))
}

func registerCellReport(report *cellReport) {
	qualificationMu.Lock()
	defer qualificationMu.Unlock()
	currentReport.Cells = append(currentReport.Cells, report)
}

var _ = ginkgo.ReportAfterSuite("release qualification report", func(ginkgo.Report) {
	qualificationMu.Lock()
	defer qualificationMu.Unlock()
	if len(currentReport.Cells) == 0 {
		return
	}
	path := os.Getenv(envReportPath)
	if path == "" {
		path = defaultReportPath
	}
	data, err := json.MarshalIndent(currentReport, "", "  ")
	framework.ExpectNoError(err)
	// #nosec G306,G703 -- The report holds no secrets and is uploaded as a CI artifact.
	framework.ExpectNoError(os.WriteFile(path, append(data, '\n'), 0o644))
})
