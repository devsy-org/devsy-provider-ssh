package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/devsy-org/devsy/pkg/log"
	"github.com/spf13/cobra"
)

const (
	testErrorLevel = "error"
	testInfoLevel  = "info"
	testTraceLevel = "trace"
	testProbe      = "probe"
)

func TestProviderLogging(t *testing.T) {
	t.Cleanup(func() { log.Init(log.Config{}) })
	for _, tc := range []struct {
		level string
		want  string
		debug bool
	}{
		{"", testInfoLevel, false},
		{testErrorLevel, testErrorLevel, false},
		{"warn", "warn", false},
		{testInfoLevel, testInfoLevel, false},
		{"debug", "debug", true},
		{testTraceLevel, testTraceLevel, true},
		{"TRACE", testTraceLevel, true},
	} {
		t.Run(tc.want, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", tc.level)
			t.Setenv("DEVSY_LOG_FORMAT", "auto")
			t.Setenv("PROVIDER_SECRET", "known-secret-846297")
			var output bytes.Buffer
			remove := log.AddSink(&output)
			defer remove()
			root := NewRootCmd()
			root.AddCommand(&cobra.Command{Use: testProbe, Run: func(*cobra.Command, []string) {
				log.Errorf("message known-secret-846297")
				log.Debug("debug message")
			}})
			root.SetArgs([]string{testProbe})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if log.LevelString() != tc.want {
				t.Fatalf("level = %q, want %q", log.LevelString(), tc.want)
			}
			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			if (len(lines) == 2) != tc.debug {
				t.Fatalf("unexpected records: %s", output.String())
			}
			assertErrorDiagnostic(t, lines[0], output.String())
		})
	}
}

func TestInvalidLoggingEnvironmentStopsCommand(t *testing.T) {
	t.Cleanup(func() { log.Init(log.Config{}) })
	for _, key := range []string{"DEVSY_LOG_LEVEL", "DEVSY_LOG_FORMAT"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", "")
			t.Setenv("DEVSY_LOG_FORMAT", "")
			canary := t.Name() + "-private-value"
			t.Setenv(key, canary)
			ran := false
			root := NewRootCmd()
			root.AddCommand(
				&cobra.Command{Use: testProbe, Run: func(*cobra.Command, []string) { ran = true }},
			)
			root.SetArgs([]string{testProbe})
			err := root.Execute()
			if err == nil || ran || strings.Contains(err.Error(), canary) {
				t.Fatalf("error = %v, ran = %v", err, ran)
			}
		})
	}
}

func TestLoggingEnvironmentRejectsWhitespace(t *testing.T) {
	t.Cleanup(func() { log.Init(log.Config{}) })
	for _, key := range []string{"DEVSY_LOG_LEVEL", "DEVSY_LOG_FORMAT"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", "")
			t.Setenv("DEVSY_LOG_FORMAT", "")
			t.Setenv(key, " info ")
			root := NewRootCmd()
			root.AddCommand(
				&cobra.Command{
					Use: testProbe,
					Run: func(*cobra.Command, []string) { t.Fatal("command ran") },
				},
			)
			root.SetArgs([]string{testProbe})
			if err := root.Execute(); err == nil {
				t.Fatal("accepted whitespace")
			}
		})
	}
}

func TestProviderLoggingFormats(t *testing.T) {
	t.Cleanup(func() { log.Init(log.Config{}) })
	for _, format := range []string{"json", "text", "plain", "logfmt", "JSON", "PLAIN"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("DEVSY_LOG_LEVEL", testInfoLevel)
			t.Setenv("DEVSY_LOG_FORMAT", format)
			var output bytes.Buffer
			remove := log.AddSink(&output)
			defer remove()
			root := NewRootCmd()
			root.AddCommand(
				&cobra.Command{
					Use: testProbe,
					Run: func(*cobra.Command, []string) { log.Info("format probe") },
				},
			)
			root.SetArgs([]string{testProbe})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if json.Valid(bytes.TrimSpace(output.Bytes())) != (strings.ToLower(format) == "json") ||
				!strings.Contains(output.String(), "format probe") {
				t.Fatalf("unexpected output: %s", output.String())
			}
		})
	}
}

func assertErrorDiagnostic(t *testing.T, line, output string) {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatal(err)
	}
	if record["level"] != testErrorLevel || record["ts"] == nil ||
		!strings.Contains(record["msg"].(string), "message") ||
		strings.Contains(output, "known-secret-846297") {
		t.Fatalf("unexpected record: %s", output)
	}
}
