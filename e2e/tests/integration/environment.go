package integration

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/devsy-org/devsy/e2e/framework"
	"github.com/kballard/go-shellquote"
	"github.com/onsi/ginkgo/v2"
)

const providerPath = "../dist/build_linux_amd64_v1/devsy-provider-ssh-linux-amd64"

const compatCoreVersion = "v1.24.0"

func setupProvider() {
	cmd := exec.Command("go", "run", "hack/provider/main.go", "0.0.0")
	cmd.Dir = "../"

	projectRoot, err := filepath.Abs("../")
	framework.ExpectNoError(err)

	distPath := filepath.Join(projectRoot, "dist")
	cmd.Env = append(os.Environ(), "PROJECT_ROOT="+distPath)
	output, err := cmd.Output()
	framework.ExpectNoError(err)

	err = os.WriteFile("../dist/provider.yaml", output, 0o600)
	framework.ExpectNoError(err)
}

func setupSSHKeys() {
	homeDir, err := os.UserHomeDir()
	framework.ExpectNoError(err)

	sshDir := filepath.Join(homeDir, ".ssh")
	err = os.MkdirAll(sshDir, 0o700)
	framework.ExpectNoError(err)

	sshKeyPath := filepath.Join(sshDir, "id_rsa")
	_, err = os.Stat(sshKeyPath) // #nosec G703 -- SSH key path is safely constructed
	if err != nil && os.IsNotExist(err) {
		ginkgo.GinkgoWriter.Println("generating ssh keys")
		// #nosec G204,G702 -- SSH key path is safely constructed
		cmd := exec.Command("ssh-keygen", "-q", "-t", "rsa", "-N", "", "-f", sshKeyPath)
		err = cmd.Run()
		framework.ExpectNoError(err)
	}
	framework.ExpectNoError(err)

	cmd := exec.Command("ssh-keygen", "-y", "-f", sshKeyPath) // #nosec G204,G702
	publicKey, err := cmd.Output()
	framework.ExpectNoError(err)

	authorizedKeysPath := filepath.Join(homeDir, ".ssh", "authorized_keys")
	existing, err := os.ReadFile(authorizedKeysPath) // #nosec G304,G703
	if err != nil && !os.IsNotExist(err) {
		framework.ExpectNoError(err)
	}
	if !bytes.Contains(existing, bytes.TrimSpace(publicKey)) {
		// #nosec G304,G703 -- authorized_keys path is safely constructed
		f, err := os.OpenFile(authorizedKeysPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		framework.ExpectNoError(err)

		defer func() { _ = f.Close() }()
		_, err = f.Write(publicKey)
		framework.ExpectNoError(err)
	}
}

func sshPrivateKeyPath() string {
	home, err := os.UserHomeDir()
	framework.ExpectNoError(err)
	return filepath.Join(home, ".ssh", "id_rsa")
}

func installLoginAccountKey(user string) {
	home, err := os.UserHomeDir()
	framework.ExpectNoError(err)
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
}

func setupDevsyCLI() {
	if qualificationEnabled() {
		requireQualifiedPair()
	}

	if local := os.Getenv(envCoreBinary); local != "" {
		// #nosec G304,G703 -- Test runner explicitly selects the local core binary.
		data, err := os.ReadFile(local)
		framework.ExpectNoError(err)
		framework.ExpectNoError(os.MkdirAll("bin", 0o750))
		// #nosec G306,G703 -- Local core binary must be executable.
		framework.ExpectNoError(os.WriteFile("bin/devsy", data, 0o700))
		return
	}

	downloadCompatCore()
}

func downloadCompatCore() {
	ginkgo.GinkgoWriter.Printf("compatibility run against core %s\n", compatCoreVersion)
	client := &http.Client{Timeout: time.Second * 30}
	resp, err := client.Get(
		"https://github.com/devsy-org/devsy/releases/download/" + compatCoreVersion +
			"/devsy-linux-amd64",
	)
	framework.ExpectNoError(err)
	framework.ExpectEqual(resp.StatusCode, http.StatusOK)
	defer func() { _ = resp.Body.Close() }()

	binDir := "bin/"
	err = os.MkdirAll(binDir, 0o755) // #nosec G301 -- bin directory is safely constructed
	framework.ExpectNoError(err)

	binPath := filepath.Join(binDir, "devsy")
	// #nosec G304,G302 -- path is safely constructed, needs execute permissions
	out, err := os.OpenFile(binPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o755)
	framework.ExpectNoError(err)

	_, err = io.Copy(out, resp.Body)
	framework.ExpectNoError(err)

	err = out.Close()
	framework.ExpectNoError(err)

	verifyCmd := exec.Command(
		binPath,
		"--version",
	) // #nosec G204,G702 -- path is safely constructed
	err = verifyCmd.Run()
	framework.ExpectNoError(err)
}

func providerCommandEnv(user string, builtin bool, agentPath, script string) []string {
	return append(
		os.Environ(),
		"HOST="+user+"@127.0.0.1",
		"PORT=22",
		"USE_BUILTIN_SSH="+strconv.FormatBool(builtin),
		"EXTRA_FLAGS="+shellquote.Join("-i", sshPrivateKeyPath()),
		"AGENT_PATH="+agentPath,
		"DOCKER_PATH=docker",
		"COMMAND="+script,
	)
}
