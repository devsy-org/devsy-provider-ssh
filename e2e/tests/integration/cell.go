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

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
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

type shellCell struct {
	pair         qualification.Pair
	shell        string
	user         string
	builtin      bool
	id           string
	devsyHome    string
	agentDir     string
	agentPath    string
	rootAgentDir bool
	loginShell   string
	report       *qualification.Cell
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

func (c *shellCell) remote(script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	// #nosec G204 -- The provider binary is the artifact under test.
	cmd := exec.CommandContext(ctx, providerPath, "command")
	cmd.Env = providerCommandEnv(c.user, false, c.agentPath, script)
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

func hostBootstrapOutput(output string) string {
	end := strings.Index(output, logHostSSHServerStarted)
	gomega.Expect(end).To(gomega.BeNumerically(">", 0), "remote SSH server did not start")
	return output[:end]
}

func (c *shellCell) agentVersion() string {
	return c.mustRemote(shellquote.Join(c.agentPath, "--version"))
}

func (c *shellCell) agentSHA256() string {
	return strings.Fields(c.mustRemote("sha256sum < " + shellquote.Join(c.agentPath)))[0]
}

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

func (c *shellCell) setup() {
	if !qualificationEnabled() {
		ginkgo.Skip(envQualification + " is not set to true")
	}
	prepareQualification()
	c.pair = requireQualifiedPair()
	c.report = &qualification.Cell{
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
	c.report.Scenarios = append(c.report.Scenarios, qualification.Scenario{
		Name:    report.LeafNodeText,
		Outcome: report.State.String(),
	})
}
