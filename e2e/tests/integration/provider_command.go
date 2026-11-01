package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe(
	"SSH login shells",
	ginkgo.Label(labelIntegration, labelProviderCommand),
	func() {
		ginkgo.BeforeEach(func() {
			if os.Getenv("DEVSY_E2E_SHELL_MATRIX") != "true" {
				ginkgo.Skip("requires provisioned Linux login shell accounts")
			}
			setupSSHKeys()
		})
		for _, transport := range qualification.Transports {
			ginkgo.It(
				fmt.Sprintf(
					"preserves bootstrap streams with %s builtin=%t",
					transport.Shell,
					transport.Builtin,
				),
				func() {
					expectBootstrapStreams(transport.Shell, transport.Builtin)
				},
			)
		}
	},
)

func expectBootstrapStreams(shell string, builtin bool) {
	user := "devsy-" + shell
	installLoginAccountKey(user)
	payload := bytes.Repeat([]byte{0, 1, 2, 255, '\n', '\'', '\\'}, 512)
	cmd := exec.Command(providerPath, "command")
	cmd.Env = providerCommandEnv(
		user,
		builtin,
		"/tmp/devsy/agent",
		"printf 'READY\\n'\ncat\nprintf 'diagnostic' >&2\nexit 37",
	)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	gomega.Expect(err).To(gomega.HaveOccurred())
	gomega.Expect(cmd.ProcessState.ExitCode()).To(gomega.Equal(37))
	gomega.Expect(stdout.Bytes()).
		To(gomega.Equal(append([]byte("READY\n"), payload...)))
	gomega.Expect(stderr.String()).To(gomega.ContainSubstring("diagnostic"))
}
