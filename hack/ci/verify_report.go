package main

import (
	"fmt"
	"path/filepath"

	"github.com/devsy-org/devsy-provider-ssh/e2e/qualification"
)

const reportFileName = "qualification-report.json"

func verifyReport() error {
	env, err := requireEnv("QUALIFICATION_DIR", "CORE_SHA256")
	if err != nil {
		return err
	}
	report, err := qualification.Load(filepath.Join(env["QUALIFICATION_DIR"], reportFileName))
	if err != nil {
		return fmt.Errorf("load qualification report: %w", err)
	}
	want := qualification.Expectation{CoreBinarySHA256: env["CORE_SHA256"]}
	if err := report.Validate(want); err != nil {
		return fmt.Errorf("validate qualification report: %w", err)
	}
	return nil
}
