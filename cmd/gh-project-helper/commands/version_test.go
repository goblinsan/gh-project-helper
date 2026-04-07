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

	Version = "test-version"
	Commit = "test-commit-sha"
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
	}()

	output := captureStdout(t, func() {
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("execute root command: %v", err)
		}
	})

	if !strings.Contains(output, "gh-project-helper version test-version+g.test-co, commit test-commit-sha, built at test-date") {
		t.Fatalf("unexpected version output: %q", output)
	}
}

func TestFormatVersionString(t *testing.T) {
	cases := []struct {
		name string
		base string
		meta versionMetadata
		want string
	}{
		{
			name: "base only",
			base: "1.0.42",
			meta: versionMetadata{},
			want: "1.0.42",
		},
		{
			name: "with commit",
			base: "1.0.42",
			meta: versionMetadata{commit: "abcdef123456"},
			want: "1.0.42+g.abcdef1",
		},
		{
			name: "with dirty flag",
			base: "1.0.42",
			meta: versionMetadata{commit: "abcdef123456", dirty: true},
			want: "1.0.42+g.abcdef1.dirty",
		},
		{
			name: "empty base falls back to dev",
			base: "",
			meta: versionMetadata{},
			want: "dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatVersionString(tc.base, tc.meta)
			if got != tc.want {
				t.Fatalf("formatVersionString() = %q, want %q", got, tc.want)
			}
		})
	}
}
