package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors
var (
	Primary     = lipgloss.Color("#7C3AED") // Purple
	Success     = lipgloss.Color("#22C55E") // Green
	Warning     = lipgloss.Color("#F59E0B") // Orange/Amber
	Error       = lipgloss.Color("#EF4444") // Red
	Muted       = lipgloss.Color("#6B7280") // Gray
	Highlight   = lipgloss.Color("#3B82F6") // Blue
	Background  = lipgloss.Color("#1F2937") // Dark gray
	Foreground  = lipgloss.Color("#F9FAFB") // Light gray
	BorderColor = lipgloss.Color("#374151") // Medium gray
)

// Base styles
var (
	// Title bar style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Foreground).
			Background(Primary).
			Padding(0, 1)

	// Box/border style
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	// Header style (section headers)
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary)

	// Muted text
	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Success text (green)
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	// Error text (red)
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	// Warning text (orange) - used for anomalies
	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	// Highlighted/selected item
	SelectedStyle = lipgloss.NewStyle().
			Background(Highlight).
			Foreground(Foreground)

	// Progress bar styles
	ProgressFullStyle = lipgloss.NewStyle().
				Foreground(Success)

	ProgressEmptyStyle = lipgloss.NewStyle().
				Foreground(Muted)
)

// Tab styles
var (
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(Primary).
			Padding(0, 1)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(Muted).
				Padding(0, 1)

	TabGapStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

// Tree styles
var (
	TreeBranchStyle = lipgloss.NewStyle().
			Foreground(Muted)

	TreeNodeStyle = lipgloss.NewStyle().
			Foreground(Foreground)

	TreeExpandedStyle = lipgloss.NewStyle().
				Foreground(Success).
				Bold(true)

	TreeCollapsedStyle = lipgloss.NewStyle().
				Foreground(Muted)

	// Count badge style (for aggregated view)
	CountBadgeStyle = lipgloss.NewStyle().
			Foreground(Highlight).
			Bold(true)

	// Anomaly badge style
	AnomalyBadgeStyle = lipgloss.NewStyle().
				Foreground(Warning).
				Bold(true)
)

// Help/footer styles
var (
	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Highlight)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(Muted)

	HelpSeparatorStyle = lipgloss.NewStyle().
				Foreground(BorderColor)
)

// View mode indicator styles
var (
	ViewModeActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Primary).
				Background(lipgloss.Color("#374151")).
				Padding(0, 1)

	ViewModeInactiveStyle = lipgloss.NewStyle().
				Foreground(Muted).
				Padding(0, 1)
)

// Progress bar helper
func RenderProgressBar(percent int, width int) string {
	filled := percent * width / 100
	if filled > width {
		filled = width
	}
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += ProgressFullStyle.Render("█")
	}
	for i := 0; i < empty; i++ {
		bar += ProgressEmptyStyle.Render("░")
	}
	return bar
}

// Tree branch characters
const (
	TreeVertical   = "│"
	TreeHorizontal = "─"
	TreeCorner     = "└"
	TreeTee        = "├"
	TreeExpanded   = "▼"
	TreeCollapsed  = "▶"
)
