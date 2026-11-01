package integration

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/devsy-org/devsy/e2e/framework"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe(
	"devsy provider ssh lifecycle suite (compatibility core "+compatCoreVersion+" unless overridden)",
	ginkgo.Label(labelIntegration),
	ginkgo.Ordered,
	func() {
		ginkgo.BeforeAll(func() {
			setupProvider()
			setupSSHKeys()
			setupDevsyCLI()
		})

		ginkgo.It("should generate valid provider.yaml", func() {
			data, err := os.ReadFile("../dist/provider.yaml")
			framework.ExpectNoError(err)
			framework.ExpectNotEqual(len(data), 0)
		})

		ginkgo.It("should have required provider options", func() {
			data, err := os.ReadFile("../dist/provider.yaml")
			framework.ExpectNoError(err)

			content := string(data)
			framework.ExpectEqual(strings.Contains(content, "HOST"), true)
			framework.ExpectEqual(strings.Contains(content, "AGENT_PATH"), true)
			framework.ExpectEqual(strings.Contains(content, "DOCKER_PATH"), true)
		})
		ginkgo.It("should install provider with devsy", func() {
			cmd := exec.Command(
				"bin/devsy",
				"provider",
				"add",
				"../dist/provider.yaml",
				"-o",
				"HOST=127.0.0.1",
			)
			err := cmd.Run()
			framework.ExpectNoError(err)
		})

		ginkgo.It("should fail the init", func() {
			cmd := exec.Command(providerPath, "init")
			cmd.Env = localProviderEnv("1234", "ls")
			err := cmd.Run()
			framework.ExpectError(err)
		})

		ginkgo.It("should run the init", func() {
			cmd := exec.Command(providerPath, "init")
			cmd.Env = localProviderEnv("22", "ls")
			err := cmd.Run()
			framework.ExpectNoError(err)
		})

		ginkgo.It("should run a command", func() {
			cmd := exec.Command(providerPath, "command")
			cmd.Env = localProviderEnv("22", "ls")
			err := cmd.Run()
			framework.ExpectNoError(err)
		})

		ginkgo.It("should run a command and verify the output", func() {
			cmd := exec.Command("ls", "/")
			controlOutput, err := cmd.Output()
			framework.ExpectNoError(err)

			cmd = exec.Command(providerPath, "command")
			cmd.Env = localProviderEnv("22", "ls /")
			output, err := cmd.Output()
			framework.ExpectNoError(err)
			gomega.Expect(output).To(gomega.Equal(controlOutput))
		})

		ginkgo.It("should run a multiline command and verify the output", func() {
			cmd := exec.Command("echo", `line1
line2
line3`)
			controlOutput, err := cmd.Output()
			framework.ExpectNoError(err)

			script := "echo line1\necho line2\necho line3"
			ginkgo.GinkgoT().Setenv("COMMAND", script)

			cmd = exec.Command(providerPath, "command")
			cmd.Env = localProviderEnv("22", script)
			output, err := cmd.CombinedOutput()
			framework.ExpectNoError(err)
			gomega.Expect(output).To(gomega.Equal(controlOutput))
		})

		ginkgo.It("should run a failing command and fail", func() {
			cmd := exec.Command(providerPath, "command")
			cmd.Env = localProviderEnv("22", "not-a-command")
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			framework.ExpectError(err)
			gomega.Expect(cmd.ProcessState).NotTo(gomega.BeNil())
			gomega.Expect(cmd.ProcessState.ExitCode()).To(gomega.Equal(127))
			gomega.Expect(stdout.Bytes()).To(gomega.BeEmpty())
			gomega.Expect(stderr.Bytes()).NotTo(gomega.BeEmpty())
			gomega.Expect(stderr.String()).To(gomega.ContainSubstring("not-a-command"))
		})

		ginkgo.It("should run devsy up", func() {
			cmd := exec.Command("bin/devsy", "workspace", "up", "--debug", "--ide=none", "../")
			err := cmd.Run()
			framework.ExpectNoError(err)
		})

		ginkgo.It("should run commands to workspace via ssh", func() {
			cmd := exec.Command("ssh", "devsy-provider-ssh.devsy", "echo", "test")
			output, err := cmd.Output()
			framework.ExpectNoError(err)
			gomega.Expect(output).To(gomega.Equal([]byte("test\n")))
		})

		ginkgo.It("should cleanup devsy workspace", func() {
			cmd := exec.Command(
				"bin/devsy",
				"workspace",
				"delete",
				"--debug",
				"--force",
				"devsy-provider-ssh",
			)
			err := cmd.Run()
			framework.ExpectNoError(err)
		})
	},
)

func localProviderEnv(port, command string) []string {
	return append(
		os.Environ(),
		"AGENT_PATH=/tmp/devsy/agent",
		"COMMAND="+command,
		"DOCKER_PATH=docker",
		"HOST=localhost",
		"PORT="+port,
		"USE_BUILTIN_SSH=false",
	)
}
