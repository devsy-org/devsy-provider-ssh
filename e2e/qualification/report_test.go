package qualification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadRoundTrip(t *testing.T) {
	want := validReport(testSHA)
	data, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("load report: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("loaded report differs from the written report")
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	data := `{"schemaVersion": 1, "pair": {}, "cells": [], "privilege": null, "extra": true}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write report: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load accepted a report with an unknown field")
	}
}

func TestLoadMissingFileNamesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load accepted a missing file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error %q does not name the path %q", err, path)
	}
}
