package qualification

import (
	"errors"
	"fmt"
	"strings"
)

const (
	outcomePassed = "passed"
	cleanupOK     = "ok"
)

var agentDirLiterals = []string{" ", "'", `\`, "$", "\u03bb"}

type Expectation struct {
	CoreBinarySHA256 string
}

func (r Report) Validate(want Expectation) error {
	if err := r.validateHeader(want); err != nil {
		return err
	}
	if err := r.validateCells(want); err != nil {
		return err
	}
	return r.validatePrivilege(want)
}

func (r Report) validateHeader(want Expectation) error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema version %d, want %d", r.SchemaVersion, SchemaVersion)
	}
	if want.CoreBinarySHA256 == "" {
		return errors.New("expected core binary sha256 is empty")
	}
	if r.Pair.CoreBinarySHA256 != want.CoreBinarySHA256 {
		return fmt.Errorf(
			"pair core binary sha256 %q, want %q",
			r.Pair.CoreBinarySHA256, want.CoreBinarySHA256,
		)
	}
	return nil
}

func (r Report) validateCells(want Expectation) error {
	if len(r.Cells) != len(Transports) {
		return fmt.Errorf("%d cells, want %d", len(r.Cells), len(Transports))
	}
	if err := validateTransports(r.Cells); err != nil {
		return err
	}
	for _, cell := range r.Cells {
		if err := validateCell(cell, want); err != nil {
			return err
		}
	}
	return nil
}

func validateTransports(cells []*Cell) error {
	seen := make(map[Transport]bool, len(cells))
	for _, cell := range cells {
		if cell == nil {
			return errors.New("report has a null cell")
		}
		transport := cell.transport()
		if seen[transport] {
			return fmt.Errorf("duplicate cell for %s", describe(transport))
		}
		seen[transport] = true
	}
	for _, transport := range Transports {
		if !seen[transport] {
			return fmt.Errorf("missing cell for %s", describe(transport))
		}
	}
	return nil
}

func validateCell(cell *Cell, want Expectation) error {
	label := describe(cell.transport())
	if cell.CleanupStatus != cleanupOK {
		return fmt.Errorf("%s: cleanup %q, want %q", label, cell.CleanupStatus, cleanupOK)
	}
	if err := validateScenarios(cell.Scenarios); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if err := validateAgentDir(cell.AgentDir); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if err := validateAgentDigests(cell.AgentSHA256, want.CoreBinarySHA256); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

func validateScenarios(scenarios []Scenario) error {
	if len(scenarios) != len(LifecycleScenarios) {
		return fmt.Errorf("%d scenarios, want %d", len(scenarios), len(LifecycleScenarios))
	}
	for i, scenario := range scenarios {
		if scenario.Name != LifecycleScenarios[i] {
			return fmt.Errorf("scenario %d is %q, want %q", i, scenario.Name, LifecycleScenarios[i])
		}
		if scenario.Outcome != outcomePassed {
			return fmt.Errorf("scenario %q outcome %q, want %q",
				scenario.Name, scenario.Outcome, outcomePassed)
		}
	}
	return nil
}

func validateAgentDir(agentDir string) error {
	for _, literal := range agentDirLiterals {
		if !strings.Contains(agentDir, literal) {
			return fmt.Errorf("agent dir %q lacks %q", agentDir, literal)
		}
	}
	return nil
}

func validateAgentDigests(digests map[string]string, want string) error {
	for _, source := range []string{"download", "stdin"} {
		if digests[source] != want {
			return fmt.Errorf("agent sha256 for %s is %q, want %q", source, digests[source], want)
		}
	}
	return nil
}

func (r Report) validatePrivilege(want Expectation) error {
	privilege := r.Privilege
	if privilege == nil {
		return errors.New("report has no privilege section")
	}
	if err := validatePrivilegeUser(privilege); err != nil {
		return err
	}
	return validatePrivilegeAgent(privilege, r.Pair.CoreVersion, want.CoreBinarySHA256)
}

func validatePrivilegeUser(privilege *Privilege) error {
	if privilege.User != PrivilegeUser {
		return fmt.Errorf("privilege user %q, want %q", privilege.User, PrivilegeUser)
	}
	if privilege.UserUID == 0 {
		return errors.New("privilege user has uid 0")
	}
	if privilege.DirectoryState != "0:755" {
		return fmt.Errorf("privilege directory state %q, want \"0:755\"", privilege.DirectoryState)
	}
	if !privilege.WriteDenied {
		return errors.New("privilege user write to the agent directory was not denied")
	}
	return nil
}

func validatePrivilegeAgent(privilege *Privilege, version, sha string) error {
	if privilege.AgentUID != 0 {
		return fmt.Errorf("privilege agent uid %d, want 0", privilege.AgentUID)
	}
	if privilege.AgentVersion != version {
		return fmt.Errorf("privilege agent version %q, want %q", privilege.AgentVersion, version)
	}
	if privilege.AgentSHA256 != sha {
		return fmt.Errorf("privilege agent sha256 %q, want %q", privilege.AgentSHA256, sha)
	}
	if !privilege.LiveCommand {
		return errors.New("privilege live command did not run")
	}
	if privilege.Outcome != outcomePassed {
		return fmt.Errorf("privilege outcome %q, want %q", privilege.Outcome, outcomePassed)
	}
	if privilege.CleanupStatus != cleanupOK {
		return fmt.Errorf("privilege cleanup %q, want %q", privilege.CleanupStatus, cleanupOK)
	}
	return nil
}

func (c *Cell) transport() Transport {
	return Transport{Shell: c.Shell, Builtin: c.BuiltinSSH}
}

func describe(transport Transport) string {
	return fmt.Sprintf("shell %s builtinSsh %t", transport.Shell, transport.Builtin)
}
