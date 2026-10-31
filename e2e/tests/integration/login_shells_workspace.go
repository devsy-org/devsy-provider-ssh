package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devsy-org/devsy/e2e/framework"
	"github.com/kballard/go-shellquote"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	"github.com/onsi/gomega"
)

const (
	workspaceFixture   = "tests/integration/testdata/login-shell-workspace"
	remoteAgentDirRoot = "/tmp/devsy-e2e-"
	literalAgentSuffix = " space'\\$-\u03bb"

	upTimeout     = 15 * time.Minute
	cliTimeout    = 3 * time.Minute
	remoteTimeout = time.Minute
	pollTimeout   = 2 * time.Minute
	pollInterval  = 5 * time.Second

	// Debug lines the core CLI logs for the agent transport it selected.
	logStdinTransfer        = "inject binary"
	logRemoteDownload       = "prefer downloading agent from URL"
	logHostSSHServerStarted = "starting ssh server: stdio=true"
	staleAgentVersion       = "v0.0.1-stale"
	agentBinaryName         = "agent"
)

var prepareQualificationOnce sync.Once

func prepareQualification() {
	prepareQualificationOnce.Do(func() {
		requireQualifiedPair()
		setupProvider()
		setupSSHKeys()
	})
}

// shellCell is one login shell and SSH implementation pair with its own Devsy home,
// provider, workspace, and remote agent directory.
type shellCell struct {
	pair         qualifiedPair
	shell        string
	user         string
	builtin      bool
	id           string
	devsyHome    string
	agentDir     string
	agentPath    string
	rootAgentDir bool
	loginShell   string
	report       *cellReport
}

func newShellCell(shell string, builtin bool) *shellCell {
	suffix := make([]byte, 4)
	_, err := rand.Read(suffix)
	framework.ExpectNoError(err)
	transport := "external"
	if builtin {
		transport = "builtin"
	}
	id := fmt.Sprintf("e2e-1621-%s-%s-%s", shell, transport, hex.EncodeToString(suffix))
	agentDir := remoteAgentDirRoot + id + literalAgentSuffix
	return &shellCell{
		shell:     shell,
		user:      "devsy-" + shell,
		builtin:   builtin,
		id:        id,
		agentDir:  agentDir,
		agentPath: agentDir + "/" + agentBinaryName,
	}
}

func (c *shellCell) devsy(timeout time.Duration, env []string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// #nosec G204 -- The test runner explicitly selects the candidate core binary.
	cmd := exec.CommandContext(ctx, c.pair.CoreBinary, args...)
	cmd.Env = append(os.Environ(), append([]string{"DEVSY_HOME=" + c.devsyHome}, env...)...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (c *shellCell) mustDevsy(timeout time.Duration, env []string, args ...string) string {
	output, err := c.devsy(timeout, env, args...)
	framework.ExpectNoError(err, "devsy "+strings.Join(args, " ")+"\n"+output)
	return output
}

// remote runs a POSIX script on the account through the provider under test.
// External SSH keeps helper commands independent of the transport being qualified.
func (c *shellCell) remote(script string) (string, error) {
	home, err := os.UserHomeDir()
	framework.ExpectNoError(err)
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	// #nosec G204 -- The provider binary is the artifact under test.
	cmd := exec.CommandContext(ctx, providerPath, "command")
	cmd.Env = append(
		os.Environ(),
		"HOST="+c.user+"@127.0.0.1",
		"PORT=22",
		"USE_BUILTIN_SSH=false",
		"EXTRA_FLAGS="+shellquote.Join("-i", filepath.Join(home, ".ssh", "id_rsa")),
		"AGENT_PATH="+c.agentPath,
		"DOCKER_PATH=docker",
		"COMMAND="+script,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *shellCell) mustRemote(script string) string {
	output, err := c.remote(script)
	framework.ExpectNoError(err, script)
	return output
}

func (c *shellCell) up(env ...string) string {
	fixture, err := filepath.Abs(workspaceFixture)
	framework.ExpectNoError(err)
	output := c.mustDevsy(
		upTimeout, env,
		"workspace", "up", fixture,
		"--id", c.id, "--provider", c.id, "--ide=none", "--debug",
	)
	ginkgo.GinkgoWriter.Println(output)
	return output
}

func (c *shellCell) expectWorkspaceSSH() {
	marker := "ssh-ok-" + c.id
	gomega.Eventually(func() (string, error) {
		output, err := c.devsy(
			cliTimeout, nil, "workspace", "ssh", c.id, "--command", "echo "+marker,
		)
		return strings.TrimSpace(output), err
	}).WithTimeout(pollTimeout).WithPolling(pollInterval).Should(gomega.ContainSubstring(marker))
}

// hostBootstrapOutput excludes the separate injection that delivers the agent
// from the remote SSH host into the devcontainer.
func hostBootstrapOutput(output string) string {
	end := strings.Index(output, logHostSSHServerStarted)
	gomega.Expect(end).To(gomega.BeNumerically(">", 0), "remote SSH server did not start")
	return output[:end]
}

func (c *shellCell) agentVersion() string {
	return c.mustRemote(shellquote.Join(c.agentPath, "--version"))
}

func (c *shellCell) agentSHA256() string {
	return strings.Fields(c.mustRemote(shellquote.Join("sha256sum", "--", c.agentPath)))[0]
}

// agentInode changes on every atomic replacement and survives a reused agent.
func (c *shellCell) agentInode() string {
	return c.mustRemote(shellquote.Join("stat", "-c", "%i", "--", c.agentPath))
}

func (c *shellCell) expectCandidateAgentInstalled() {
	gomega.Expect(c.agentVersion()).To(gomega.Equal(c.pair.CoreVersion))
	gomega.Expect(c.agentSHA256()).To(gomega.Equal(c.pair.CoreBinarySHA256))
	gomega.Expect(c.mustRemote(shellquote.Join("ls", "-A", "--", c.agentDir))).To(
		gomega.Equal(agentBinaryName), "installer must leave no temporary files",
	)
}

func (c *shellCell) installStaleAgent() {
	// A prior agent process can still hold the installed executable open.
	staged := shellquote.Join(c.agentPath + ".stale")
	c.mustRemote(fmt.Sprintf(
		"mkdir -p -- %s && printf '#!/bin/sh\\necho %s\\n' > %s "+
			"&& chmod 0755 -- %s && mv -f -- %s %s",
		shellquote.Join(
			c.agentDir,
		),
		staleAgentVersion,
		staged,
		staged,
		staged,
		shellquote.Join(c.agentPath),
	))
	gomega.Expect(c.agentVersion()).To(gomega.Equal(staleAgentVersion))
}

func (c *shellCell) passwdEntry() string {
	// #nosec G204 -- Fixed command with a controlled fixture account name.
	output, err := exec.Command("getent", "passwd", c.user).Output()
	framework.ExpectNoError(err)
	return strings.TrimSpace(string(output))
}

func (c *shellCell) requireDockerAccess() {
	output, err := c.remote("docker version --format '{{.Server.Version}}'")
	framework.ExpectNoError(err, fmt.Sprintf(
		"account %s needs docker access for the full workspace lifecycle", c.user,
	))
	gomega.Expect(output).NotTo(gomega.BeEmpty())
}

func (c *shellCell) cleanup() {
	var failures []string
	if c.devsyHome != "" {
		if output, err := c.devsy(
			cliTimeout, nil, "workspace", "delete", "--force", "--debug", c.id,
		); err != nil {
			failures = append(failures, "workspace: "+err.Error()+": "+output)
		}
		if output, err := c.devsy(cliTimeout, nil, "provider", "delete", c.id); err != nil {
			failures = append(failures, "provider: "+err.Error()+": "+output)
		}
		if err := os.RemoveAll(c.devsyHome); err != nil {
			failures = append(failures, "devsy home: "+err.Error())
		}
	}
	if err := c.removeAgentDirectory(); err != nil {
		failures = append(failures, "agent dir: "+err.Error())
	}
	c.report.CleanupStatus = "ok"
	if len(failures) > 0 {
		c.report.CleanupStatus = strings.Join(failures, "; ")
	}
	gomega.Expect(failures).To(gomega.BeEmpty())
}

func (c *shellCell) removeAgentDirectory() error {
	// Only the per-cell directory is removed, never a shared agent path.
	if !strings.HasPrefix(c.agentDir, remoteAgentDirRoot) ||
		len(c.agentDir) == len(remoteAgentDirRoot) {
		return nil
	}
	if c.rootAgentDir {
		return runFixtureSudo("-n", "rm", "-rf", "--", c.agentDir)
	}
	_, err := c.remote(shellquote.Join("rm", "-rf", "--", c.agentDir))
	return err
}

var _ = ginkgo.Describe(
	"SSH login shell full workspace lifecycle",
	ginkgo.Label("release-qualification"),
	func() {
		for _, cell := range shellTransports {
			describeShellCell(cell.shell, cell.builtin)
		}
	},
)

func describeShellCell(shell string, builtin bool) {
	ginkgo.Describe(shell+" builtin="+strconv.FormatBool(builtin), ginkgo.Ordered, func() {
		cell := newShellCell(shell, builtin)
		ginkgo.BeforeAll(cell.setup)
		ginkgo.ReportAfterEach(cell.recordScenario)
		ginkgo.It("installs a cold agent by remote download", cell.expectColdDownload)
		ginkgo.It(
			"reuses the current agent when the workspace is opened again",
			cell.expectCurrentAgentReuse,
		)
		ginkgo.It("replaces a deliberately old agent", cell.expectStaleAgentReplacement)
		ginkgo.It(
			"installs a cold agent by stdin transfer and keeps stdin for the live stream",
			cell.expectStdinTransfer,
		)
		ginkgo.It("leaves the account login shell unchanged", func() {
			gomega.Expect(cell.passwdEntry()).To(gomega.Equal(cell.loginShell))
		})
	})
}

func (c *shellCell) setup() {
	if !qualificationEnabled() {
		ginkgo.Skip(envQualification + " is not set to true")
	}
	prepareQualification()
	c.pair = requireQualifiedPair()
	c.report = &cellReport{
		Shell:       c.shell,
		User:        c.user,
		BuiltinSSH:  c.builtin,
		Workspace:   c.id,
		AgentDir:    c.agentDir,
		AgentSHA256: map[string]string{},
	}
	if !c.rootAgentDir {
		registerCellReport(c.report)
	}
	ginkgo.DeferCleanup(c.cleanup)

	installLoginAccountKey(c.user)
	c.requireDockerAccess()
	c.loginShell = c.passwdEntry()
	c.report.LoginShell = c.loginShell
	gomega.Expect(c.loginShell).To(gomega.HaveSuffix("/" + c.shell))

	var err error
	c.devsyHome, err = os.MkdirTemp("", "devsy-e2e-home-")
	framework.ExpectNoError(err)

	c.mustDevsy(
		cliTimeout, nil,
		"provider", "add", "../dist/provider.yaml", "--name", c.id,
		"-o", "HOST="+c.user+"@127.0.0.1",
		"-o", "USE_BUILTIN_SSH="+strconv.FormatBool(c.builtin),
		"-o", "AGENT_PATH="+c.agentPath,
		"-o", "EXTRA_FLAGS="+shellquote.Join("-i", sshPrivateKeyPath()),
		"-o", "INJECT_GIT_CREDENTIALS=false",
		"-o", "INJECT_DOCKER_CREDENTIALS=false",
	)
	output, err := c.remote(shellquote.Join("test", "!", "-e", c.agentPath))
	framework.ExpectNoError(err, "agent binary must start absent after provider init")
	gomega.Expect(output).To(gomega.BeEmpty())
}

func (c *shellCell) recordScenario(report ginkgo.SpecReport) {
	if c.report == nil ||
		report.State.Is(types.SpecStateSkipped|types.SpecStatePending) {
		return
	}
	c.report.Scenarios = append(c.report.Scenarios, scenarioResult{
		Name:    report.LeafNodeText,
		Outcome: report.State.String(),
	})
}

func (c *shellCell) expectColdDownload() {
	output := c.up()
	hostOutput := hostBootstrapOutput(output)
	gomega.Expect(hostOutput).To(gomega.ContainSubstring(logRemoteDownload))
	gomega.Expect(hostOutput).NotTo(gomega.ContainSubstring(logStdinTransfer))
	c.expectCandidateAgentInstalled()
	c.report.AgentSHA256["download"] = c.agentSHA256()
	c.expectWorkspaceSSH()
}

func (c *shellCell) expectCurrentAgentReuse() {
	inode := c.agentInode()
	output := c.up()
	hostOutput := hostBootstrapOutput(output)
	gomega.Expect(hostOutput).NotTo(gomega.ContainSubstring(logStdinTransfer))
	gomega.Expect(c.agentInode()).To(gomega.Equal(inode))
	c.expectCandidateAgentInstalled()
	c.expectWorkspaceSSH()
}

func (c *shellCell) expectStaleAgentReplacement() {
	c.installStaleAgent()
	staleInode := c.agentInode()
	output := c.up()
	hostOutput := hostBootstrapOutput(output)
	gomega.Expect(hostOutput).To(gomega.ContainSubstring(logRemoteDownload))
	gomega.Expect(hostOutput).NotTo(gomega.ContainSubstring(logStdinTransfer))
	gomega.Expect(c.agentInode()).NotTo(gomega.Equal(staleInode))
	c.expectCandidateAgentInstalled()
	c.expectWorkspaceSSH()
}

func (c *shellCell) expectStdinTransfer() {
	c.mustRemote(shellquote.Join("rm", "-rf", "--", c.agentDir))
	stdinMode := []string{envAgentPreferDownload + "=false"}
	output := c.up(stdinMode...)
	hostOutput := hostBootstrapOutput(output)
	gomega.Expect(hostOutput).To(gomega.ContainSubstring(logStdinTransfer))
	c.expectCandidateAgentInstalled()
	c.report.AgentSHA256["stdin"] = c.agentSHA256()
	c.expectWorkspaceSSH()

	inode := c.agentInode()
	output = c.up(stdinMode...)
	hostOutput = hostBootstrapOutput(output)
	gomega.Expect(hostOutput).NotTo(gomega.ContainSubstring(logStdinTransfer))
	gomega.Expect(c.agentInode()).To(gomega.Equal(inode))
	c.expectWorkspaceSSH()
}

func sshPrivateKeyPath() string {
	home, err := os.UserHomeDir()
	framework.ExpectNoError(err)
	return filepath.Join(home, ".ssh", "id_rsa")
}
