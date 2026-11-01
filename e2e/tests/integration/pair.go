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

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
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

	envAgentURL            = "DEVSY_AGENT_URL"
	envAgentBinary         = "DEVSY_AGENT_BINARY"
	envAgentPreferDownload = "DEVSY_AGENT_PREFER_DOWNLOAD"

	devVersion = "v0.0.0"
)

var (
	qualificationMu    sync.Mutex
	qualificationState *qualification.Pair
	currentReport      = &qualification.Report{SchemaVersion: qualification.SchemaVersion}
)

func qualificationEnabled() bool {
	return os.Getenv(envQualification) == "true"
}

func requireQualifiedPair() qualification.Pair {
	qualificationMu.Lock()
	defer qualificationMu.Unlock()
	if qualificationState != nil {
		return *qualificationState
	}

	requireQualificationEnvironment()

	pair := qualification.Pair{
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

func registerCellReport(report *qualification.Cell) {
	qualificationMu.Lock()
	defer qualificationMu.Unlock()
	currentReport.Cells = append(currentReport.Cells, report)
}

func registerPrivilegeReport(report *qualification.Privilege) {
	qualificationMu.Lock()
	defer qualificationMu.Unlock()
	currentReport.Privilege = report
}

var _ = ginkgo.ReportAfterSuite("release qualification report", func(ginkgo.Report) {
	qualificationMu.Lock()
	defer qualificationMu.Unlock()
	if len(currentReport.Cells) == 0 && currentReport.Privilege == nil {
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
