package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const (
	startSSHCall   = "sudo service ssh start"
	testPolicyLine = "devsy-sudo ALL=(root) NOPASSWD: SETENV: /bin/sh -c * devsy-install *\n"
	testTempCall   = "<tmp>/devsy-sudoers-N"
)

var tempRuleName = regexp.MustCompile(`[^\s]*/devsy-sudoers-[^\s]+`)

type recorder struct {
	calls  []string
	failOn string
	rule   string
	env    []string
}

func (r *recorder) run(name string, args ...string) (string, error) {
	raw := strings.Join(append([]string{name}, args...), " ")
	call := tempRuleName.ReplaceAllString(raw, testTempCall)
	r.calls = append(r.calls, call)
	if isTempValidation(name, args) {
		content, err := os.ReadFile(args[2])
		if err != nil {
			return "", err
		}
		r.rule = string(content)
	}
	if call == r.failOn {
		return "", fmt.Errorf("%s: exit status 1", call)
	}
	return "", nil
}

func isTempValidation(name string, args []string) bool {
	return name == "sudo" && len(args) == 3 && args[0] == "visudo" && args[1] == "-cf" &&
		args[2] != sudoersPolicyPath
}

func (r *recorder) lookPath(name string) (string, error) {
	return "/bin/" + name, nil
}

func (r *recorder) appendEnv(lines []string) error {
	r.env = append(r.env, lines...)
	return nil
}

func newTestProvisioner(r *recorder, dir string) *provisioner {
	return &provisioner{
		run:       r.run,
		lookPath:  r.lookPath,
		appendEnv: r.appendEnv,
		tempDir:   dir,
	}
}

func assertCalls(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("calls:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

var (
	loginCalls = []string{
		startSSHCall,
		"sudo useradd --create-home --shell /bin/bash devsy-bash",
		"sudo useradd --create-home --shell /bin/zsh devsy-zsh",
		"sudo useradd --create-home --shell /bin/fish devsy-fish",
	}
	qualificationCalls = []string{
		startSSHCall,
		"sudo useradd --create-home --shell /bin/bash devsy-bash",
		"sudo usermod --append --groups docker devsy-bash",
		"sudo useradd --create-home --shell /bin/zsh devsy-zsh",
		"sudo usermod --append --groups docker devsy-zsh",
		"sudo useradd --create-home --shell /bin/fish devsy-fish",
		"sudo usermod --append --groups docker devsy-fish",
		"sudo useradd --create-home --shell /bin/bash --groups docker devsy-sudo",
		"sudo test ! -e /etc/sudoers.d/devsy-e2e-qualification",
		"sudo visudo -cf " + testTempCall,
		"sudo install -m 0440 -o root -g root " + testTempCall +
			" /etc/sudoers.d/devsy-e2e-qualification",
		"sudo visudo -cf /etc/sudoers.d/devsy-e2e-qualification",
	}
)

func TestProvisionLoginsRunsSSHStartThenOneAccountPerShell(t *testing.T) {
	r := &recorder{}
	p := newTestProvisioner(r, t.TempDir())

	if err := p.provisionLogins(); err != nil {
		t.Fatalf("provisionLogins() error = %v", err)
	}
	assertCalls(t, r.calls, loginCalls)
}

func TestProvisionQualificationRunsFullSequence(t *testing.T) {
	r := &recorder{}
	p := newTestProvisioner(r, t.TempDir())

	if err := p.provisionQualification(); err != nil {
		t.Fatalf("provisionQualification() error = %v", err)
	}
	assertCalls(t, r.calls, qualificationCalls)
	if r.rule != testPolicyLine {
		t.Fatalf("sudoers rule = %q, want %q", r.rule, testPolicyLine)
	}
	wantEnv := []string{"DEVSY_SUDO_FIXTURE_POLICY=/etc/sudoers.d/devsy-e2e-qualification"}
	if !slices.Equal(r.env, wantEnv) {
		t.Fatalf("env lines = %q, want %q", r.env, wantEnv)
	}
}

func TestProvisionQualificationStopsAtFailedUsermod(t *testing.T) {
	r := &recorder{failOn: "sudo usermod --append --groups docker devsy-bash"}
	p := newTestProvisioner(r, t.TempDir())

	err := p.provisionQualification()
	if err == nil {
		t.Fatal("provisionQualification() error = nil, want usermod failure")
	}
	if !strings.Contains(err.Error(), "usermod") {
		t.Fatalf("error %q does not mention usermod", err)
	}
	assertCalls(t, r.calls, qualificationCalls[:3])
}

func TestProvisionQualificationRejectsExistingPolicyBeforeVisudo(t *testing.T) {
	r := &recorder{failOn: "sudo test ! -e /etc/sudoers.d/devsy-e2e-qualification"}
	p := newTestProvisioner(r, t.TempDir())

	if err := p.provisionQualification(); err == nil {
		t.Fatal("provisionQualification() error = nil, want existing policy failure")
	}
	for _, call := range r.calls {
		if strings.Contains(call, "visudo") || strings.Contains(call, "install") {
			t.Fatalf("call %q ran after the policy existence check failed", call)
		}
	}
	if len(r.env) != 0 {
		t.Fatalf("env lines = %q, want none", r.env)
	}
}

func TestProvisionQualificationRemovesTempRule(t *testing.T) {
	dir := t.TempDir()
	p := newTestProvisioner(&recorder{}, dir)

	if err := p.provisionQualification(); err != nil {
		t.Fatalf("provisionQualification() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp dir holds %d leftover entries, want none", len(entries))
	}
}

func TestCleanupQualificationAccountsRemovesOnlyTheExactPolicy(t *testing.T) {
	const policy = "/etc/sudoers.d/devsy-e2e-qualification"
	tests := []struct {
		name    string
		env     string
		wantErr bool
		want    []string
	}{
		{name: "unset is a no-op", env: ""},
		{name: "foreign path is refused", env: "/etc/passwd", wantErr: true},
		{name: "exact path is removed", env: policy, want: []string{"sudo rm -f -- " + policy}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(policyEnv, tt.env)
			r := &recorder{}
			p := newTestProvisioner(r, t.TempDir())

			err := p.removeSudoersPolicy()
			if (err != nil) != tt.wantErr {
				t.Fatalf("removeSudoersPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
			assertCalls(t, r.calls, tt.want)
		})
	}
}

func TestProvisionLoginsStopsWhenSSHStartFails(t *testing.T) {
	r := &recorder{failOn: startSSHCall}
	p := newTestProvisioner(r, t.TempDir())

	err := p.provisionLogins()
	if err == nil || !strings.Contains(err.Error(), "service") {
		t.Fatalf("provisionLogins() error = %v, want ssh start failure", err)
	}
	assertCalls(t, r.calls, loginCalls[:1])
}
