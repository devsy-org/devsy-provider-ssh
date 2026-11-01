package integration

import (
	"strconv"

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
	"github.com/kballard/go-shellquote"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe(
	"SSH login shell full workspace lifecycle",
	ginkgo.Label(labelReleaseQualification),
	func() {
		for _, cell := range qualification.Transports {
			describeShellCell(cell.Shell, cell.Builtin)
		}
	},
)

func describeShellCell(shell string, builtin bool) {
	ginkgo.Describe(shell+" builtin="+strconv.FormatBool(builtin), ginkgo.Ordered, func() {
		cell := newShellCell(shell, builtin)
		ginkgo.BeforeAll(cell.setup)
		ginkgo.ReportAfterEach(cell.recordScenario)
		ginkgo.It(qualification.ScenarioColdDownload, cell.expectColdDownload)
		ginkgo.It(qualification.ScenarioCurrentReuse, cell.expectCurrentAgentReuse)
		ginkgo.It(qualification.ScenarioStaleReplacement, cell.expectStaleAgentReplacement)
		ginkgo.It(qualification.ScenarioStdinTransfer, cell.expectStdinTransfer)
		ginkgo.It(qualification.ScenarioLoginShellUnchanged, cell.expectLoginShellUnchanged)
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

func (c *shellCell) expectLoginShellUnchanged() {
	gomega.Expect(c.passwdEntry()).To(gomega.Equal(c.loginShell))
}
