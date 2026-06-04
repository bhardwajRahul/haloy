package haloy

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestProgressDemoCommandIsHidden(t *testing.T) {
	cmd := NewRootCmd()

	progressCmd, _, err := cmd.Find([]string{"__progress-demo"})
	if err != nil {
		t.Fatalf("expected hidden progress command to be registered: %v", err)
	}
	if progressCmd.Name() != "__progress-demo" {
		t.Fatalf("Find() command = %q, want __progress-demo", progressCmd.Name())
	}
	if !progressCmd.Hidden {
		t.Fatal("expected progress demo command to be hidden")
	}
}

func TestProgressDemoCommandRunsWithoutConfig(t *testing.T) {
	t.Chdir(t.TempDir())

	output := captureStdout(t, func() {
		err := runRootCommand(
			t,
			"__progress-demo",
			"--total-bytes", "1",
			"--step-bytes", "1",
			"--delay", "0",
			"--width", "32",
		)
		if err != nil {
			t.Fatalf("expected progress demo to run without config, got: %v", err)
		}
	})

	if !strings.Contains(output, "Uploading layers") {
		t.Fatalf("expected progress demo output, got %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	output := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		output <- string(data)
	}()

	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close stdout writer: %v", err)
	}
	os.Stdout = originalStdout

	return <-output
}
