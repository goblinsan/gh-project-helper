package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return buf.String()
}

func TestRootVersionFlag(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	originalArgs := os.Args

	Version = "test-version"
	Commit = "test-commit"
	Date = "test-date"
	showVersion = false
	rootCmd.SetArgs([]string{"--version"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	defer func() {
		Version = originalVersion
		Commit = originalCommit
		Date = originalDate
		showVersion = false
		rootCmd.SetArgs(nil)
		os.Args = originalArgs
	}()

	output := captureStdout(t, func() {
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute root command: %v", err)
		}
	})

	if !strings.Contains(output, "gh-project-helper version test-version, commit test-commit, built at test-date") {
		t.Fatalf("unexpected version output: %q", output)
	}
}
