package qualification

import (
	"strings"
	"testing"
)

const testSHA = "5b1c9e0d"

const (
	testAgentDir  = "/tmp/devsy agent'\\$\u03bb"
	staleSHA      = "other"
	outcomeFailed = "failed"
)

func validReport(sha string) Report {
	cells := make([]*Cell, 0, len(Transports))
	for _, transport := range Transports {
		cells = append(cells, &Cell{
			Shell:         transport.Shell,
			User:          "devsy-user",
			BuiltinSSH:    transport.Builtin,
			Workspace:     "workspace",
			AgentDir:      testAgentDir,
			LoginShell:    "/bin/" + transport.Shell,
			Scenarios:     passedScenarios(),
			AgentSHA256:   map[string]string{"download": sha, "stdin": sha},
			CleanupStatus: cleanupOK,
		})
	}
	return Report{
		SchemaVersion: SchemaVersion,
		Pair:          Pair{CoreVersion: "1.2.3", CoreBinarySHA256: sha},
		Cells:         cells,
		Privilege: &Privilege{
			User:           PrivilegeUser,
			UserUID:        1001,
			AgentDir:       "/opt/devsy-agent",
			DirectoryState: "0:755",
			WriteDenied:    true,
			WriteError:     "permission denied",
			AgentUID:       0,
			AgentVersion:   "1.2.3",
			AgentSHA256:    sha,
			LiveCommand:    true,
			Outcome:        outcomePassed,
			CleanupStatus:  cleanupOK,
		},
	}
}

func passedScenarios() []Scenario {
	scenarios := make([]Scenario, 0, len(LifecycleScenarios))
	for _, name := range LifecycleScenarios {
		scenarios = append(scenarios, Scenario{Name: name, Outcome: outcomePassed})
	}
	return scenarios
}

func TestValidReportPasses(t *testing.T) {
	if err := validReport(testSHA).Validate(Expectation{CoreBinarySHA256: testSHA}); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
}

type violation struct {
	name   string
	mutate func(r *Report, want *Expectation)
}

func agentDirWithout(literal string) func(*Report, *Expectation) {
	return func(r *Report, _ *Expectation) {
		r.Cells[0].AgentDir = strings.ReplaceAll(r.Cells[0].AgentDir, literal, "")
	}
}

func privilegeViolation(mutate func(*Privilege)) func(*Report, *Expectation) {
	return func(r *Report, _ *Expectation) { mutate(r.Privilege) }
}

func cellViolation(mutate func(*Cell)) func(*Report, *Expectation) {
	return func(r *Report, _ *Expectation) { mutate(r.Cells[0]) }
}

var headerViolations = []violation{
	{"wrong schema version", func(r *Report, _ *Expectation) {
		r.SchemaVersion = SchemaVersion + 1
	}},
	{"empty expectation sha", func(r *Report, want *Expectation) {
		r.Pair.CoreBinarySHA256 = ""
		want.CoreBinarySHA256 = ""
	}},
	{"pair sha differs from expectation", func(r *Report, _ *Expectation) {
		r.Pair.CoreBinarySHA256 = staleSHA
	}},
	{"missing cell", func(r *Report, _ *Expectation) {
		r.Cells = r.Cells[:len(r.Cells)-1]
	}},
	{"duplicate transport", func(r *Report, _ *Expectation) {
		r.Cells[1].Shell = r.Cells[0].Shell
		r.Cells[1].BuiltinSSH = r.Cells[0].BuiltinSSH
	}},
}

var cellViolations = []violation{
	{"wrong cleanup", cellViolation(func(c *Cell) { c.CleanupStatus = outcomeFailed })},
	{"scenario failed", cellViolation(func(c *Cell) { c.Scenarios[0].Outcome = outcomeFailed })},
	{"scenario order wrong", cellViolation(func(c *Cell) {
		c.Scenarios[0], c.Scenarios[1] = c.Scenarios[1], c.Scenarios[0]
	})},
	{"agent dir missing space", agentDirWithout(" ")},
	{"agent dir missing apostrophe", agentDirWithout("'")},
	{"agent dir missing backslash", agentDirWithout(`\`)},
	{"agent dir missing dollar", agentDirWithout("$")},
	{"agent dir missing lambda", agentDirWithout("\u03bb")},
	{
		"download sha mismatch",
		cellViolation(func(c *Cell) { c.AgentSHA256["download"] = staleSHA }),
	},
	{"stdin sha mismatch", cellViolation(func(c *Cell) { c.AgentSHA256["stdin"] = staleSHA })},
}

var privilegeViolations = []violation{
	{"nil privilege", func(r *Report, _ *Expectation) { r.Privilege = nil }},
	{"privilege user wrong", privilegeViolation(func(p *Privilege) { p.User = "root" })},
	{"root uid user", privilegeViolation(func(p *Privilege) { p.UserUID = 0 })},
	{
		"wrong directory state",
		privilegeViolation(func(p *Privilege) { p.DirectoryState = "0:777" }),
	},
	{"write not denied", privilegeViolation(func(p *Privilege) { p.WriteDenied = false })},
	{"agent uid not 0", privilegeViolation(func(p *Privilege) { p.AgentUID = 1000 })},
	{"agent version mismatch", privilegeViolation(func(p *Privilege) { p.AgentVersion = "0.0.1" })},
	{
		"privilege agent sha mismatch",
		privilegeViolation(func(p *Privilege) { p.AgentSHA256 = staleSHA }),
	},
	{"live command false", privilegeViolation(func(p *Privilege) { p.LiveCommand = false })},
	{
		"privilege outcome failed",
		privilegeViolation(func(p *Privilege) { p.Outcome = outcomeFailed }),
	},
	{
		"privilege cleanup not ok",
		privilegeViolation(func(p *Privilege) { p.CleanupStatus = outcomeFailed }),
	},
}

func TestValidateRejectsViolations(t *testing.T) {
	var all []violation
	all = append(all, headerViolations...)
	all = append(all, cellViolations...)
	all = append(all, privilegeViolations...)
	for _, tt := range all {
		t.Run(tt.name, func(t *testing.T) {
			report := validReport(testSHA)
			want := Expectation{CoreBinarySHA256: testSHA}
			tt.mutate(&report, &want)
			if err := report.Validate(want); err == nil {
				t.Fatal("Validate accepted the report")
			}
		})
	}
}
