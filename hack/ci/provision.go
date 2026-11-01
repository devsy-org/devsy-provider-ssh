package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
)

const (
	sudoersPolicyPath  = "/etc/sudoers.d/devsy-e2e-qualification"
	policyEnv          = "DEVSY_SUDO_FIXTURE_POLICY"
	qualificationGroup = "docker"
	privilegeShell     = "bash"
)

type provisioner struct {
	run       func(name string, args ...string) (string, error)
	lookPath  func(file string) (string, error)
	appendEnv func(lines []string) error
	tempDir   string
}

func newProvisioner() *provisioner {
	return &provisioner{
		run:       command,
		lookPath:  exec.LookPath,
		appendEnv: appendGitHubEnv,
		tempDir:   os.TempDir(),
	}
}

func appendGitHubEnv(lines []string) error {
	return appendLines(os.Getenv("GITHUB_ENV"), lines)
}

func provisionAccounts() error {
	return newProvisioner().provisionLogins()
}

func provisionQualificationAccounts() error {
	return newProvisioner().provisionQualification()
}

func cleanupQualificationAccounts() error {
	return newProvisioner().removeSudoersPolicy()
}

func (p *provisioner) provisionLogins() error {
	if err := p.startSSH(); err != nil {
		return err
	}
	return p.createLoginAccounts()
}

func (p *provisioner) provisionQualification() error {
	if err := p.startSSH(); err != nil {
		return err
	}
	if err := p.createLoginAccounts(qualificationGroup); err != nil {
		return err
	}
	if err := p.createSudoAccount(); err != nil {
		return err
	}
	return p.installSudoersPolicy()
}

func (p *provisioner) sudo(args ...string) error {
	_, err := p.run("sudo", args...)
	return err
}

func (p *provisioner) startSSH() error {
	if err := p.sudo("service", "ssh", "start"); err != nil {
		return fmt.Errorf("start ssh service: %w", err)
	}
	return nil
}

func (p *provisioner) createLoginAccounts(groups ...string) error {
	for _, shell := range qualification.Shells() {
		account := "devsy-" + shell
		if err := p.useradd(account, shell); err != nil {
			return err
		}
		if len(groups) == 0 {
			continue
		}
		if err := p.addGroups(account, groups); err != nil {
			return err
		}
	}
	return nil
}

func (p *provisioner) createSudoAccount() error {
	return p.useradd(qualification.PrivilegeUser, privilegeShell, "--groups", qualificationGroup)
}

func (p *provisioner) useradd(account, shell string, extra ...string) error {
	path, err := p.lookPath(shell)
	if err != nil {
		return fmt.Errorf("resolve shell %s: %w", shell, err)
	}
	args := append([]string{"useradd", "--create-home", "--shell", path}, extra...)
	if err := p.sudo(append(args, account)...); err != nil {
		return fmt.Errorf("create account %s: %w", account, err)
	}
	return nil
}

func (p *provisioner) addGroups(account string, groups []string) error {
	list := strings.Join(groups, ",")
	if err := p.sudo("usermod", "--append", "--groups", list, account); err != nil {
		return fmt.Errorf("add %s to groups %s: %w", account, list, err)
	}
	return nil
}

func (p *provisioner) installSudoersPolicy() error {
	if err := p.sudo("test", "!", "-e", sudoersPolicyPath); err != nil {
		return fmt.Errorf("sudoers policy %s must not exist: %w", sudoersPolicyPath, err)
	}
	ruleFile, err := p.writeSudoersRule()
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(ruleFile) }()
	if err := p.sudo("visudo", "-cf", ruleFile); err != nil {
		return fmt.Errorf("validate sudoers rule: %w", err)
	}
	if err := p.appendEnv([]string{policyEnv + "=" + sudoersPolicyPath}); err != nil {
		return fmt.Errorf("export %s: %w", policyEnv, err)
	}
	install := []string{"install", "-m", "0440", "-o", "root", "-g", "root"}
	if err := p.sudo(append(install, ruleFile, sudoersPolicyPath)...); err != nil {
		return fmt.Errorf("install sudoers policy: %w", err)
	}
	if err := p.sudo("visudo", "-cf", sudoersPolicyPath); err != nil {
		return fmt.Errorf("validate installed sudoers policy: %w", err)
	}
	return nil
}

func (p *provisioner) writeSudoersRule() (string, error) {
	file, err := os.CreateTemp(p.tempDir, "devsy-sudoers-")
	if err != nil {
		return "", fmt.Errorf("create sudoers temp file: %w", err)
	}
	path := file.Name()
	_, writeErr := file.WriteString(sudoersRule() + "\n")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("write sudoers rule: %w", err)
	}
	return path, nil
}

func (p *provisioner) removeSudoersPolicy() error {
	policy := os.Getenv(policyEnv)
	if policy == "" {
		return nil
	}
	if policy != sudoersPolicyPath {
		return fmt.Errorf("refusing to remove %s: only %s is allowed", policy, sudoersPolicyPath)
	}
	if err := p.sudo("rm", "-f", "--", policy); err != nil {
		return fmt.Errorf("remove sudoers policy: %w", err)
	}
	return nil
}

func sudoersRule() string {
	return qualification.PrivilegeUser +
		" ALL=(root) NOPASSWD: SETENV: /bin/sh -c * devsy-install *"
}
