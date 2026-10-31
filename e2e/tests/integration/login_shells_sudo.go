package integration

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/devsy-org/devsy/e2e/framework"
	"github.com/kballard/go-shellquote"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe(
	"SSH installer privilege transition",
	ginkgo.Label("release-qualification"),
	func() {
		ginkgo.It(
			"installs an executable agent through real sudo for a non-root user",
			qualifySudoInstallation,
		)
	},
)

func qualifySudoInstallation() {
	if !qualificationEnabled() {
		ginkgo.Skip(envQualification + " is not set to true")
	}
	prepareQualification()
	cell := newShellCell("bash", false)
	cell.user = "devsy-sudo"
	cell.rootAgentDir = true
	result := &privilegeReport{User: cell.user, AgentDir: cell.agentDir, Outcome: "failed"}
	registerPrivilegeReport(result)
	ginkgo.DeferCleanup(func() { cell.finishPrivilegeReport(result) })
	ginkgo.DeferCleanup(func() {
		// Root directory creation can fail before setup registers its cleanup.
		if cell.report == nil {
			framework.ExpectNoError(cell.removeAgentDirectory())
		}
	})

	framework.ExpectNoError(runFixtureSudo(
		"-n", "install", "-d", "-m", "0755", "-o", "root", "-g", "root", cell.agentDir,
	))
	cell.setup()
	cell.requireSudoPreconditions(result)
	cell.up()
	cell.expectCandidateAgentInstalled()
	cell.expectWorkspaceSSH()
	result.LiveCommand = true
	result.AgentVersion = cell.agentVersion()
	result.AgentSHA256 = cell.agentSHA256()
	var err error
	result.AgentUID, err = strconv.ParseUint(
		cell.mustRemote(shellquote.Join("stat", "-c", "%u", "--", cell.agentPath)), 10, 32,
	)
	framework.ExpectNoError(err)
	gomega.Expect(result.AgentUID).To(gomega.BeZero(), "sudo must own the newly installed agent")
	gomega.Expect(cell.passwdEntry()).To(gomega.Equal(cell.loginShell))
	result.Outcome = "passed"
}

func (c *shellCell) requireSudoPreconditions(result *privilegeReport) {
	var err error
	result.UserUID, err = strconv.ParseUint(c.mustRemote("id -u"), 10, 32)
	framework.ExpectNoError(err)
	gomega.Expect(result.UserUID).To(gomega.BeNumerically(">", 0))
	result.DirectoryState = c.mustRemote(shellquote.Join("stat", "-c", "%u:%a", "--", c.agentDir))
	gomega.Expect(result.DirectoryState).To(gomega.Equal("0:755"))
	probe := c.agentDir + "/nonroot-write-probe"
	_, err = c.remote(shellquote.Join("touch", "--", probe))
	gomega.Expect(err).
		To(gomega.HaveOccurred(), "the non-root account must not write the agent directory")
	result.WriteError = err.Error()
	gomega.Expect(c.mustRemote(shellquote.Join("test", "!", "-e", probe))).To(gomega.BeEmpty())
	result.WriteDenied = true
}

func (c *shellCell) finishPrivilegeReport(result *privilegeReport) {
	if c.report != nil {
		result.CleanupStatus = c.report.CleanupStatus
	}
	if ginkgo.CurrentSpecReport().Failed() {
		result.Outcome = "failed"
	}
}

func runFixtureSudo(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	// #nosec G204 -- Only fixed commands manage this job's unique owned fixture directory.
	output, err := exec.CommandContext(ctx, "sudo", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("fixture sudo: %w: %s", err, output)
	}
	return nil
}
