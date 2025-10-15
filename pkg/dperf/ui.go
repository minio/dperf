// This file is part of MinIO dperf
// Copyright (c) 2021 MinIO, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package dperf

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

// DriveStatus represents the current status of a drive being tested
type DriveStatus struct {
	Path            string
	Phase           string // "write", "read", or "complete"
	WriteThroughput uint64
	ReadThroughput  uint64
	WriteProgress   float64 // 0.0 to 1.0
	ReadProgress    float64 // 0.0 to 1.0
	Error           error
	// Track individual I/O operations
	IOProgress map[int]IOStatus // key is IOIndex
}

// IOStatus represents the status of an individual I/O operation
type IOStatus struct {
	BytesProcessed uint64
	TotalBytes     uint64
	Throughput     uint64
}

// ProgressMsg is sent when progress is updated
type ProgressMsg ProgressUpdate

// CompleteMsg is sent when all tests are complete
type CompleteMsg struct {
	Results []*DrivePerfResult
}

// UIModel is the Bubble Tea model for the real-time UI
type UIModel struct {
	drives       map[string]*DriveStatus
	driveOrder   []string // to maintain consistent ordering
	progressBars map[string]progress.Model
	width        int
	height       int
	Complete     bool
	Results      []*DrivePerfResult
	startTime    time.Time
	writeOnly    bool
	verbose      bool
}

// NewUIModel creates a new UI model for the given paths
func NewUIModel(paths []string, writeOnly, verbose bool) *UIModel {
	m := &UIModel{
		drives:       make(map[string]*DriveStatus),
		driveOrder:   paths,
		progressBars: make(map[string]progress.Model),
		startTime:    time.Now(),
		writeOnly:    writeOnly,
		verbose:      verbose,
	}

	for _, path := range paths {
		m.drives[path] = &DriveStatus{
			Path:       path,
			Phase:      "write",
			IOProgress: make(map[int]IOStatus),
		}
		pb := progress.New(progress.WithDefaultGradient())
		pb.Width = 40
		m.progressBars[path] = pb
	}

	return m
}

func (m *UIModel) Init() tea.Cmd {
	return nil
}

func (m *UIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update progress bar widths
		barWidth := min(40, m.width-50)
		if barWidth < 10 {
			barWidth = 10
		}
		for path := range m.progressBars {
			pb := m.progressBars[path]
			pb.Width = barWidth
			m.progressBars[path] = pb
		}
		return m, nil

	case ProgressMsg:
		update := ProgressUpdate(msg)
		drive, ok := m.drives[update.Path]
		if !ok {
			return m, nil
		}

		// Update IO status
		drive.IOProgress[update.IOIndex] = IOStatus{
			BytesProcessed: update.BytesProcessed,
			TotalBytes:     update.TotalBytes,
			Throughput:     update.Throughput,
		}

		// Calculate aggregate progress for this phase
		var totalProcessed, totalBytes uint64
		var totalThroughput uint64
		for _, io := range drive.IOProgress {
			totalProcessed += io.BytesProcessed
			totalBytes += io.TotalBytes
			totalThroughput += io.Throughput
		}

		if totalBytes > 0 {
			if update.Phase == "write" {
				drive.WriteProgress = float64(totalProcessed) / float64(totalBytes)
				drive.WriteThroughput = totalThroughput
				drive.Phase = "write"
			} else {
				drive.ReadProgress = float64(totalProcessed) / float64(totalBytes)
				drive.ReadThroughput = totalThroughput
				drive.Phase = "read"
			}
		}

		if update.Error != nil {
			drive.Error = update.Error
		}

		return m, nil

	case CompleteMsg:
		m.Complete = true
		m.Results = msg.Results
		// Mark all drives as complete
		for _, drive := range m.drives {
			drive.Phase = "complete"
		}
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *UIModel) View() string {
	if m.Complete {
		// Show final results
		return m.RenderFinalResults()
	}

	var b strings.Builder

	// Header - use cyan (works on both backgrounds)
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")). // Cyan
		MarginBottom(1)

	elapsed := time.Since(m.startTime).Round(time.Second)
	b.WriteString(headerStyle.Render(fmt.Sprintf("dperf - Drive Performance Test (elapsed: %s)", elapsed)))
	b.WriteString("\n\n")

	// Render each drive
	for _, path := range m.driveOrder {
		drive := m.drives[path]
		b.WriteString(m.renderDrive(path, drive))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Faint(true)
	b.WriteString(footerStyle.Render("Press Ctrl+C to cancel"))

	return b.String()
}

func (m *UIModel) renderDrive(path string, drive *DriveStatus) string {
	var b strings.Builder

	// Use bold blue for path (works on both backgrounds)
	pathStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")) // Blue

	// Use default color for phase
	phaseStyle := lipgloss.NewStyle()

	b.WriteString(pathStyle.Render(path))
	b.WriteString("  ")

	if drive.Error != nil {
		// Use red for errors
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")). // Red
			Bold(true)
		b.WriteString(errorStyle.Render("✗ " + drive.Error.Error()))
		return b.String()
	}

	// Show phase
	phase := drive.Phase
	if phase == "complete" {
		// Use green for success
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")). // Green
			Bold(true)
		b.WriteString(successStyle.Render("✓ Complete"))
	} else {
		b.WriteString(phaseStyle.Render("[" + strings.ToUpper(phase) + "]"))
	}
	b.WriteString("\n")

	// Write phase
	if drive.WriteProgress > 0 || drive.Phase == "write" {
		b.WriteString("  Write: ")
		pb := m.progressBars[path]
		b.WriteString(pb.ViewAs(drive.WriteProgress))
		b.WriteString(fmt.Sprintf(" %.0f%% ", drive.WriteProgress*100))
		if drive.WriteThroughput > 0 {
			// Use cyan for throughput
			throughputStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")) // Cyan
			b.WriteString(throughputStyle.Render(humanize.IBytes(drive.WriteThroughput) + "/s"))
		}
		b.WriteString("\n")
	}

	// Read phase
	if !m.writeOnly && (drive.ReadProgress > 0 || drive.Phase == "read" || drive.Phase == "complete") {
		b.WriteString("  Read:  ")
		pb := m.progressBars[path]
		b.WriteString(pb.ViewAs(drive.ReadProgress))
		b.WriteString(fmt.Sprintf(" %.0f%% ", drive.ReadProgress*100))
		if drive.ReadThroughput > 0 {
			// Use magenta for read throughput to differentiate from write
			throughputStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")) // Magenta
			b.WriteString(throughputStyle.Render(humanize.IBytes(drive.ReadThroughput) + "/s"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// RenderFinalResults renders the final results as a string (exported for printing to terminal)
func (m *UIModel) RenderFinalResults() string {
	var b strings.Builder

	// Header - use cyan (same as progress view)
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")). // Cyan
		MarginBottom(1)

	elapsed := time.Since(m.startTime).Round(time.Second)
	b.WriteString(headerStyle.Render(fmt.Sprintf("dperf - Drive Performance Test (elapsed: %s)", elapsed)))
	b.WriteString("\n\n")

	// Sort results by read throughput (fastest first) to match ordering during test
	sortedResults := make([]*DrivePerfResult, len(m.Results))
	copy(sortedResults, m.Results)
	for i := 0; i < len(sortedResults); i++ {
		for j := i + 1; j < len(sortedResults); j++ {
			if sortedResults[i].ReadThroughput < sortedResults[j].ReadThroughput {
				sortedResults[i], sortedResults[j] = sortedResults[j], sortedResults[i]
			}
		}
	}

	// Render each drive with 100% progress bars
	for _, result := range sortedResults {
		b.WriteString(m.renderDriveComplete(result))
		b.WriteString("\n")
	}

	// Calculate and show aggregate totals
	var totalWrite, totalRead uint64
	for _, result := range sortedResults {
		if result.Error == nil {
			totalWrite += result.WriteThroughput
			totalRead += result.ReadThroughput
		}
	}

	b.WriteString("\n")
	summaryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("2")) // Green

	b.WriteString(summaryStyle.Render("Total Write: "))
	b.WriteString(humanize.IBytes(totalWrite) + "/s")

	if !m.writeOnly {
		b.WriteString("  ")
		b.WriteString(summaryStyle.Render("Total Read: "))
		b.WriteString(humanize.IBytes(totalRead) + "/s")
	}
	b.WriteString("\n")

	return b.String()
}

// renderDriveComplete renders a drive with 100% complete progress bars
func (m *UIModel) renderDriveComplete(result *DrivePerfResult) string {
	var b strings.Builder

	// Use bold blue for path
	pathStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")) // Blue

	b.WriteString(pathStyle.Render(result.Path))
	b.WriteString("  ")

	if result.Error != nil {
		// Use red for errors
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")). // Red
			Bold(true)
		b.WriteString(errorStyle.Render("✗ " + result.Error.Error()))
		return b.String()
	}

	// Show complete status
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")). // Green
		Bold(true)
	b.WriteString(successStyle.Render("✓ Complete"))
	b.WriteString("\n")

	// Write phase with 100% progress bar
	b.WriteString("  Write: ")
	pb := m.progressBars[result.Path]
	b.WriteString(pb.ViewAs(1.0)) // 100% complete
	b.WriteString(" 100% ")
	if result.WriteThroughput > 0 {
		throughputStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")) // Cyan
		b.WriteString(throughputStyle.Render(humanize.IBytes(result.WriteThroughput) + "/s"))
	}
	b.WriteString("\n")

	// Read phase with 100% progress bar
	if !m.writeOnly {
		b.WriteString("  Read:  ")
		b.WriteString(pb.ViewAs(1.0)) // 100% complete
		b.WriteString(" 100% ")
		if result.ReadThroughput > 0 {
			throughputStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")) // Magenta
			b.WriteString(throughputStyle.Render(humanize.IBytes(result.ReadThroughput) + "/s"))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
