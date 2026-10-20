package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/devsy-org/devsy/e2e/framework"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("SSH login shells", ginkgo.Label("integration"), func() {
	ginkgo.BeforeEach(func() {
		if os.Getenv("DEVSY_E2E_SHELL_MATRIX") != "true" {
			ginkgo.Skip("requires provisioned Linux login shell accounts")
		}
		setupSSHKeys()
	})
	for _, shell := range []string{"bash", "zsh", "fish"} {
		for _, builtin := range []bool{false, true} {
			ginkgo.It(
				fmt.Sprintf("preserves bootstrap streams with %s builtin=%t", shell, builtin),
				func() {
					home, err := os.UserHomeDir()
					framework.ExpectNoError(err)
					user := "devsy-" + shell
					sshDir := "/home/" + user + "/.ssh"
					for _, args := range [][]string{
						{"install", "-d", "-m", "700", "-o", user, "-g", user, sshDir},
						{
							"install", "-m", "600", "-o", user, "-g", user,
							filepath.Join(home, ".ssh", "id_rsa.pub"), sshDir + "/authorized_keys",
						},
					} {
						// #nosec G204 -- Install only the controlled fixture key into provisioned test accounts.
						output, err := exec.Command("sudo", args...).CombinedOutput()
						framework.ExpectNoError(err, string(output))
					}
					payload := bytes.Repeat([]byte{0, 1, 2, 255, '\n', '\'', '\\'}, 512)
					cmd := exec.Command(providerPath, "command")
					cmd.Env = append(
						os.Environ(),
						"HOST="+user+"@127.0.0.1",
						"PORT=22",
						"USE_BUILTIN_SSH="+strconv.FormatBool(builtin),
						"EXTRA_FLAGS=-i "+filepath.Join(home, ".ssh", "id_rsa"),
						"AGENT_PATH=/tmp/devsy/agent",
						"DOCKER_PATH=docker",
						"COMMAND=printf 'READY\\n'\ncat\nprintf 'diagnostic' >&2\nexit 37",
					)
					cmd.Stdin = bytes.NewReader(payload)
					var stdout, stderr bytes.Buffer
					cmd.Stdout, cmd.Stderr = &stdout, &stderr
					err = cmd.Run()
					gomega.Expect(err).To(gomega.HaveOccurred())
					gomega.Expect(cmd.ProcessState.ExitCode()).To(gomega.Equal(37))
					gomega.Expect(stdout.Bytes()).
						To(gomega.Equal(append([]byte("READY\n"), payload...)))
					gomega.Expect(stderr.String()).To(gomega.ContainSubstring("diagnostic"))
				},
			)
		}
	}
})
