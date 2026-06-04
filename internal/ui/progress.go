package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

const (
	defaultProgressBarWidth = 20
	minProgressBarWidth     = 4
)

var (
	progressOutput    = func() io.Writer { return os.Stdout }
	progressTermWidth = func() int {
		width, _, err := term.GetSize(os.Stdout.Fd())
		if err != nil || width <= 0 {
			return 0
		}
		return width
	}
)

// ProgressBar displays an aggregate progress bar that can be updated from multiple goroutines
type ProgressBar struct {
	total       int64
	current     atomic.Int64
	completed   atomic.Int32
	totalItems  int32
	description string
	mu          sync.Mutex
	showBytes   bool
	termWidth   int
}

// ProgressBarConfig configures the progress bar display
type ProgressBarConfig struct {
	Description string
	TotalBytes  int64
	TotalItems  int
	ShowBytes   bool // if true, shows bytes; if false, shows items only
	TermWidth   int  // optional terminal width override; 0 auto-detects
}

// NewProgressBar creates a new aggregate progress bar
func NewProgressBar(cfg ProgressBarConfig) *ProgressBar {
	pb := &ProgressBar{
		total:       cfg.TotalBytes,
		totalItems:  int32(cfg.TotalItems),
		description: cfg.Description,
		showBytes:   cfg.ShowBytes,
		termWidth:   cfg.TermWidth,
	}
	pb.render()
	return pb
}

// Add adds bytes to the progress (thread-safe)
func (p *ProgressBar) Add(n int64) {
	p.current.Add(n)
	p.render()
}

// CompleteItem marks one item as complete (thread-safe)
func (p *ProgressBar) CompleteItem() {
	p.completed.Add(1)
	p.render()
}

// Finish completes the progress bar and moves to next line
func (p *ProgressBar) Finish() {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprint(progressOutput(), "\r", ansi.EraseLineRight)
}

func (p *ProgressBar) render() {
	p.mu.Lock()
	defer p.mu.Unlock()

	current := p.current.Load()
	completed := p.completed.Load()

	line := p.buildLine(current, completed)
	fmt.Fprint(progressOutput(), "\r", ansi.EraseLineRight, line)
}

func (p *ProgressBar) buildLine(current int64, completed int32) string {
	width := p.maxLineWidth()
	prefix := infoPrefix()
	status := p.status(current, completed)
	line := p.formatLine(prefix, status, defaultProgressBarWidth, current)

	if width <= 0 || ansi.StringWidth(line) <= width {
		return line
	}

	fixedWidth := ansi.StringWidth(p.formatLineWithBar(prefix, "", status))
	barWidth := width - fixedWidth
	if barWidth >= minProgressBarWidth {
		line = p.formatLine(prefix, status, min(barWidth, defaultProgressBarWidth), current)
		if ansi.StringWidth(line) <= width {
			return line
		}
	}

	compactStatus := fmt.Sprintf("%d/%d", completed, p.totalItems)
	for _, candidateBarWidth := range []int{minProgressBarWidth, 0} {
		line = p.formatLine(prefix, compactStatus, candidateBarWidth, current)
		if ansi.StringWidth(line) <= width {
			return line
		}
	}

	return ansi.Truncate(line, width, "")
}

func (p *ProgressBar) maxLineWidth() int {
	width := p.termWidth
	if width <= 0 {
		width = progressTermWidth()
	}
	if width <= 1 {
		return width
	}
	return width - 1
}

func (p *ProgressBar) formatLine(prefix, status string, barWidth int, current int64) string {
	if barWidth <= 0 {
		return fmt.Sprintf("%s %s %s", prefix, p.description, status)
	}
	return p.formatLineWithBar(prefix, p.bar(current, barWidth), status)
}

func (p *ProgressBar) formatLineWithBar(prefix, bar, status string) string {
	return fmt.Sprintf("%s %s [%s] %s", prefix, p.description, bar, status)
}

func (p *ProgressBar) bar(current int64, barWidth int) string {
	// Calculate percentage
	var percent float64
	if p.total > 0 {
		percent = float64(current) / float64(p.total) * 100
	}
	if percent > 100 {
		percent = 100
	}

	filledWidth := min(int(percent/100*float64(barWidth)), barWidth)

	filled := strings.Repeat("█", filledWidth)
	empty := strings.Repeat("░", barWidth-filledWidth)
	return filled + empty
}

func (p *ProgressBar) status(current int64, completed int32) string {
	if p.showBytes && p.total > 0 {
		return fmt.Sprintf("%d/%d (%s / %s)",
			completed, p.totalItems,
			formatBytes(current), formatBytes(p.total))
	}
	return fmt.Sprintf("%d/%d", completed, p.totalItems)
}

// formatBytes formats bytes into human-readable format
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
