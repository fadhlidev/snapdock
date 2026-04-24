package output

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

var (
	SuccessColor = color.New(color.FgGreen, color.Bold)
	ErrorColor   = color.New(color.FgRed, color.Bold)
	InfoColor    = color.New(color.FgCyan)
	WarningColor = color.New(color.FgYellow, color.Bold)
	DryRunColor  = color.New(color.FgMagenta, color.Bold, color.Italic)
)

const (
	IconSuccess = "✅"
	IconError   = "❌"
	IconInfo    = "ℹ️ "
	IconWarning = "⚠️ "
	IconDryRun  = "🔍"
)

// Success prints a success message with an icon.
func Success(msg string) {
	SuccessColor.Printf("%s %s\n", IconSuccess, msg)
}

// Successf prints a formatted success message with an icon.
func Successf(format string, a ...interface{}) {
	SuccessColor.Printf("%s %s\n", IconSuccess, fmt.Sprintf(format, a...))
}

// Error prints an error message with an icon.
func Error(msg string) {
	ErrorColor.Printf("%s %s\n", IconError, msg)
}

// Errorf prints a formatted error message with an icon.
func Errorf(format string, a ...interface{}) {
	ErrorColor.Printf("%s %s\n", IconError, fmt.Sprintf(format, a...))
}

// Info prints an info message with an icon.
func Info(msg string) {
	InfoColor.Printf("%s %s\n", IconInfo, msg)
}

// Infof prints a formatted info message with an icon.
func Infof(format string, a ...interface{}) {
	InfoColor.Printf("%s %s\n", IconInfo, fmt.Sprintf(format, a...))
}

// Warning prints a warning message with an icon.
func Warning(msg string) {
	WarningColor.Printf("%s %s\n", IconWarning, msg)
}

// Warningf prints a formatted warning message with an icon.
func Warningf(format string, a ...interface{}) {
	WarningColor.Printf("%s %s\n", IconWarning, fmt.Sprintf(format, a...))
}

// DryRun prints a dry-run message with an icon.
func DryRun(msg string) {
	DryRunColor.Printf("%s %s\n", IconDryRun, msg)
}

// DryRunf prints a formatted dry-run message with an icon.
func DryRunf(format string, a ...interface{}) {
	DryRunColor.Printf("%s %s\n", IconDryRun, fmt.Sprintf(format, a...))
}

// SimpleSpinner is a basic ASCII spinner.
type SimpleSpinner struct {
	suffix string
	stop   chan struct{}
	wg     sync.WaitGroup
}

// NewSpinner creates a new basic spinner.
func NewSpinner(text string) *SimpleSpinner {
	return &SimpleSpinner{
		suffix: text,
		stop:   make(chan struct{}),
	}
}

// Start starts the spinner in a background goroutine.
func (s *SimpleSpinner) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K") // Clear line
				return
			default:
				fmt.Printf("\r%s %s", color.CyanString(frames[i]), s.suffix)
				i = (i + 1) % len(frames)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// Stop stops the spinner and clears the line.
func (s *SimpleSpinner) Stop() {
	close(s.stop)
	s.wg.Wait()
}

// PrintTable prints a basic padded table.
func PrintTable(headers []string, data [][]string) {
	if len(headers) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range data {
		for i, col := range row {
			if i < len(widths) && len(col) > widths[i] {
				widths[i] = len(col)
			}
		}
	}

	// Print headers
	fmt.Println()
	sb := strings.Builder{}
	sb.WriteString("  ")
	for i, h := range headers {
		sb.WriteString(color.New(color.Bold, color.FgYellow).Sprintf("%-*s", widths[i], h))
		if i < len(headers)-1 {
			sb.WriteString("   ")
		}
	}
	fmt.Println(sb.String())

	// Print separator
	sb.Reset()
	sb.WriteString("  ")
	for i, w := range widths {
		sb.WriteString(strings.Repeat("-", w))
		if i < len(widths)-1 {
			sb.WriteString("   ")
		}
	}
	fmt.Println(sb.String())

	// Print data
	for _, row := range data {
		sb.Reset()
		sb.WriteString("  ")
		for i, col := range row {
			if i < len(widths) {
				sb.WriteString(fmt.Sprintf("%-*s", widths[i], col))
				if i < len(widths)-1 {
					sb.WriteString("   ")
				}
			}
		}
		fmt.Println(sb.String())
	}
}
