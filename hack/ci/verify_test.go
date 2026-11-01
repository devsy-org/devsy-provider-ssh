package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
)

const (
	testCoreSHA  = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	testAgentDir = "/tmp/devsy-e2e-verify space'\\$-\u03bb"
	testUserName = "devsy-user"
	testVersion  = "v1.0.0"
)

func TestEvaluateGuardResult(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		output   string
		wantErr  bool
	}{
		{
			name:     "exit 1 with message passes",
			exitCode: 1,
			output:   guardMessage,
			wantErr:  false,
		},
		{
			name:     "exit 0 with message fails",
			exitCode: 0,
			output:   guardMessage,
			wantErr:  true,
		},
		{
			name:     "exit 1 without message fails",
			exitCode: 1,
			output:   "--- FAIL: TestRelease",
			wantErr:  true,
		},
		{
			name:     "exit 2 with message fails",
			exitCode: 2,
			output:   guardMessage,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := evaluateGuardResult(tt.exitCode, tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("evaluateGuardResult(%d, %q) error = %v, want error %t",
					tt.exitCode, tt.output, err, tt.wantErr)
			}
		})
	}
}

func TestVerifyReportRequiresEnvironment(t *testing.T) {
	t.Setenv("QUALIFICATION_DIR", "")
	t.Setenv("CORE_SHA256", "")
	err := verifyReport()
	if err == nil || !strings.Contains(err.Error(), "missing required environment") {
		t.Fatalf("verifyReport() error = %v, want missing required environment", err)
	}
}

func TestVerifyReportAcceptsValidReport(t *testing.T) {
	t.Setenv("QUALIFICATION_DIR", writeTestReport(t, validTestReport()))
	t.Setenv("CORE_SHA256", testCoreSHA)
	if err := verifyReport(); err != nil {
		t.Fatalf("verifyReport() error = %v", err)
	}
}

func TestVerifyReportRejectsOtherCoreBinary(t *testing.T) {
	t.Setenv("QUALIFICATION_DIR", writeTestReport(t, validTestReport()))
	t.Setenv("CORE_SHA256", strings.Repeat("0", 64))
	if err := verifyReport(); err == nil {
		t.Fatal("verifyReport() accepted a report built for another core binary")
	}
}

func writeTestReport(t *testing.T, report qualification.Report) string {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, reportFileName), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func validTestReport() qualification.Report {
	report := qualification.Report{SchemaVersion: qualification.SchemaVersion}
	report.Pair = qualification.Pair{
		CoreTag:          testVersion,
		CoreVersion:      testVersion,
		CoreBinarySHA256: testCoreSHA,
	}
	for _, transport := range qualification.Transports {
		report.Cells = append(report.Cells, validTestCell(transport))
	}
	report.Privilege = validTestPrivilege()
	return report
}

func validTestCell(transport qualification.Transport) *qualification.Cell {
	scenarios := make([]qualification.Scenario, 0, len(qualification.LifecycleScenarios))
	for _, name := range qualification.LifecycleScenarios {
		scenarios = append(scenarios, qualification.Scenario{Name: name, Outcome: "passed"})
	}
	return &qualification.Cell{
		Shell:         transport.Shell,
		User:          testUserName,
		BuiltinSSH:    transport.Builtin,
		Workspace:     "workspace",
		AgentDir:      testAgentDir,
		LoginShell:    "/bin/" + transport.Shell,
		Scenarios:     scenarios,
		AgentSHA256:   map[string]string{"download": testCoreSHA, "stdin": testCoreSHA},
		CleanupStatus: "ok",
	}
}

func validTestPrivilege() *qualification.Privilege {
	return &qualification.Privilege{
		User:           qualification.PrivilegeUser,
		UserUID:        1001,
		AgentDir:       testAgentDir,
		DirectoryState: "0:755",
		WriteDenied:    true,
		AgentUID:       0,
		AgentVersion:   testVersion,
		AgentSHA256:    testCoreSHA,
		LiveCommand:    true,
		Outcome:        "passed",
		CleanupStatus:  "ok",
	}
}
