package components

import (
	"strings"

	"github.com/Binmave/binmave-cli/internal/ui"
)

// HelpItem represents a keyboard shortcut help item
type HelpItem struct {
	Key  string
	Desc string
}

// HelpBar renders a horizontal help bar with keyboard shortcuts
type HelpBar struct {
	items []HelpItem
	width int
}

// NewHelpBar creates a new help bar
func NewHelpBar() *HelpBar {
	return &HelpBar{
		items: make([]HelpItem, 0),
		width: 80,
	}
}

// SetItems sets the help items to display
func (h *HelpBar) SetItems(items []HelpItem) {
	h.items = items
}

// SetWidth sets the available width
func (h *HelpBar) SetWidth(width int) {
	h.width = width
}

// Render returns the rendered help bar
func (h *HelpBar) Render() string {
	if len(h.items) == 0 {
		return ""
	}

	var parts []string
	for _, item := range h.items {
		key := ui.HelpKeyStyle.Render(item.Key)
		desc := ui.HelpDescStyle.Render(item.Desc)
		parts = append(parts, key+" "+desc)
	}

	separator := ui.HelpSeparatorStyle.Render("  ")
	return strings.Join(parts, separator)
}

// StandardHelpItems returns common help items
func StandardHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "↑↓", Desc: "Navigate"},
		{Key: "Tab", Desc: "Tabs"},
		{Key: "q", Desc: "Quit"},
	}
}

// TableViewHelpItems returns help items for table view
func TableViewHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "↑↓", Desc: "Navigate"},
		{Key: "Enter", Desc: "Details"},
		{Key: "/", Desc: "Search"},
		{Key: "Tab", Desc: "Tabs"},
		{Key: "1/2/3", Desc: "View"},
		{Key: "q", Desc: "Quit"},
	}
}

// TreeViewHelpItems returns help items for tree view
func TreeViewHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "↑↓←→", Desc: "Navigate/Expand"},
		{Key: "e/c", Desc: "Expand/Collapse All"},
		{Key: "/", Desc: "Search"},
		{Key: "Tab", Desc: "Tabs"},
		{Key: "1/2/3", Desc: "View"},
		{Key: "q", Desc: "Quit"},
	}
}

// AggregatedViewHelpItems returns help items for aggregated view
func AggregatedViewHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "↑↓←→", Desc: "Navigate/Expand"},
		{Key: "a", Desc: "Anomalies"},
		{Key: "/", Desc: "Search"},
		{Key: "Tab", Desc: "Tabs"},
		{Key: "1/2/3", Desc: "View"},
		{Key: "q", Desc: "Quit"},
	}
}

// CompareViewHelpItems returns help items for compare view
func CompareViewHelpItems() []HelpItem {
	return []HelpItem{
		{Key: "↑↓", Desc: "Navigate"},
		{Key: "d", Desc: "Show Diffs Only"},
		{Key: "a", Desc: "Show All"},
		{Key: "q", Desc: "Quit"},
	}
}

// ProgressIndicator renders a progress indicator with percentage
type ProgressIndicator struct {
	current int
	total   int
	width   int
}

// NewProgressIndicator creates a new progress indicator
func NewProgressIndicator() *ProgressIndicator {
	return &ProgressIndicator{
		current: 0,
		total:   0,
		width:   40,
	}
}

// SetProgress updates the progress values
func (p *ProgressIndicator) SetProgress(current, total int) {
	p.current = current
	p.total = total
}

// SetWidth sets the bar width
func (p *ProgressIndicator) SetWidth(width int) {
	p.width = width
}

// Render returns the rendered progress bar
func (p *ProgressIndicator) Render() string {
	if p.total == 0 {
		return ui.MutedStyle.Render("Progress: N/A")
	}

	percent := p.current * 100 / p.total

	// Render the bar
	bar := ui.RenderProgressBar(percent, p.width)

	// Format label
	label := "Progress: " + intToString(p.current) + "/" + intToString(p.total) + " "

	// Percent text
	percentText := " " + intToString(percent) + "%"

	return label + bar + percentText
}

// IsComplete returns true if progress is 100%
func (p *ProgressIndicator) IsComplete() bool {
	return p.total > 0 && p.current >= p.total
}
