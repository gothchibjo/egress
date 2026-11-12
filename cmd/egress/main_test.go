package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestIsTerminalRejectsPipe(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	if isTerminal(reader) {
		t.Fatal("pipe reported as a terminal")
	}
}

func TestRunRequiresTerminal(t *testing.T) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	defer stdoutReader.Close()
	defer stdoutWriter.Close()

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	defer stderrReader.Close()

	code := run(stdoutWriter, stderrWriter)
	stderrWriter.Close()

	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	message, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if !strings.Contains(string(message), "requires a terminal") {
		t.Fatalf("stderr = %q", message)
	}
}
