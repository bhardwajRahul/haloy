package ui

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestProgressBarUsesConfiguredTerminalWidth(t *testing.T) {
	var output bytes.Buffer
	configureProgressTestDoubles(t, &output, 120)

	width := 34
	progress := NewProgressBar(ProgressBarConfig{
		Description: "Uploading layers",
		TotalBytes:  26_906_214,
		TotalItems:  1,
		ShowBytes:   true,
		TermWidth:   width,
	})
	progress.Add(32 * 1024)
	progress.CompleteItem()
	progress.Finish()

	for _, line := range renderedProgressLines(output.String()) {
		if got, wantMax := ansi.StringWidth(line), width-1; got > wantMax {
			t.Fatalf("rendered line width = %d, want <= %d: %q", got, wantMax, ansi.Strip(line))
		}
	}
}

func TestProgressBarAutoDetectsTerminalWidth(t *testing.T) {
	var output bytes.Buffer
	width := 42
	configureProgressTestDoubles(t, &output, width)

	progress := NewProgressBar(ProgressBarConfig{
		Description: "Uploading layers",
		TotalBytes:  26_906_214,
		TotalItems:  1,
		ShowBytes:   true,
	})
	progress.Add(32 * 1024)
	progress.Finish()

	for _, line := range renderedProgressLines(output.String()) {
		if got, wantMax := ansi.StringWidth(line), width-1; got > wantMax {
			t.Fatalf("rendered line width = %d, want <= %d: %q", got, wantMax, ansi.Strip(line))
		}
	}
}

func TestProgressBarClearsLineOnEveryRedraw(t *testing.T) {
	var output bytes.Buffer
	configureProgressTestDoubles(t, &output, 80)

	progress := NewProgressBar(ProgressBarConfig{
		Description: "Uploading layers",
		TotalBytes:  1024,
		TotalItems:  1,
		ShowBytes:   true,
	})
	progress.Add(512)
	progress.Finish()

	if got, want := strings.Count(output.String(), "\r"+ansi.EraseLineRight), 3; got != want {
		t.Fatalf("clear-line redraw count = %d, want %d; output: %q", got, want, output.String())
	}
}

func TestProgressBarFallsBackWhenTerminalWidthUnknown(t *testing.T) {
	var output bytes.Buffer
	configureProgressTestDoubles(t, &output, 0)

	progress := NewProgressBar(ProgressBarConfig{
		Description: "Uploading layers",
		TotalBytes:  26_906_214,
		TotalItems:  1,
		ShowBytes:   true,
	})
	progress.Add(32 * 1024)
	progress.Finish()

	lines := renderedProgressLines(output.String())
	if len(lines) == 0 {
		t.Fatal("expected progress output")
	}
	if !strings.Contains(ansi.Strip(lines[0]), "[░░░░░░░░░░░░░░░░░░░░]") {
		t.Fatalf("expected full progress bar when width is unknown, got %q", ansi.Strip(lines[0]))
	}
}

func configureProgressTestDoubles(t *testing.T, output *bytes.Buffer, width int) {
	t.Helper()

	originalOutput := progressOutput
	originalTermWidth := progressTermWidth

	progressOutput = func() io.Writer { return output }
	progressTermWidth = func() int { return width }

	t.Cleanup(func() {
		progressOutput = originalOutput
		progressTermWidth = originalTermWidth
	})
}

func renderedProgressLines(output string) []string {
	parts := strings.Split(output, "\r"+ansi.EraseLineRight)
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			lines = append(lines, part)
		}
	}
	return lines
}
