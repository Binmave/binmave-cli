package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/Binmave/binmave-cli/internal/ui"
)

// Tab represents a single tab with a label and optional count
type Tab struct {
	Label string
	Count int
}

// TabBar renders a horizontal tab bar with active tab highlighting
type TabBar struct {
	tabs      []Tab
	activeIdx int
	width     int
}

// NewTabBar creates a new tab bar with the given tabs
func NewTabBar(tabs []Tab) *TabBar {
	return &TabBar{
		tabs:      tabs,
		activeIdx: 0,
		width:     80,
	}
}

// SetActive sets the active tab by index
func (t *TabBar) SetActive(idx int) {
	if idx >= 0 && idx < len(t.tabs) {
		t.activeIdx = idx
	}
}

// GetActive returns the index of the active tab
func (t *TabBar) GetActive() int {
	return t.activeIdx
}

// Next moves to the next tab
func (t *TabBar) Next() {
	t.activeIdx = (t.activeIdx + 1) % len(t.tabs)
}

// Prev moves to the previous tab
func (t *TabBar) Prev() {
	t.activeIdx = (t.activeIdx - 1 + len(t.tabs)) % len(t.tabs)
}

// SetWidth sets the available width for rendering
func (t *TabBar) SetWidth(width int) {
	t.width = width
}

// UpdateCount updates the count for a specific tab
func (t *TabBar) UpdateCount(idx int, count int) {
	if idx >= 0 && idx < len(t.tabs) {
		t.tabs[idx].Count = count
	}
}

// Render returns the rendered tab bar string
func (t *TabBar) Render() string {
	var tabs []string

	for i, tab := range t.tabs {
		var label string
		if tab.Count > 0 {
			label = tab.Label + " (" + formatCount(tab.Count) + ")"
		} else {
			label = tab.Label
		}

		if i == t.activeIdx {
			tabs = append(tabs, ui.ActiveTabStyle.Render(label))
		} else {
			tabs = append(tabs, ui.InactiveTabStyle.Render(label))
		}
	}

	return strings.Join(tabs, ui.TabGapStyle.Render(""))
}

// ViewModeBar renders the view mode selector (Table/Tree/Aggregated)
type ViewModeBar struct {
	modes       []string
	keys        []string // keyboard shortcuts
	activeIdx   int
	treeEnabled bool // whether Tree/Aggregated modes are available
}

// NewViewModeBar creates a view mode bar with the standard modes
func NewViewModeBar() *ViewModeBar {
	return &ViewModeBar{
		modes:       []string{"Table", "Tree", "Aggregated"},
		keys:        []string{"1", "2", "3"},
		activeIdx:   0,
		treeEnabled: true,
	}
}

// SetTreeEnabled enables or disables Tree/Aggregated modes
func (v *ViewModeBar) SetTreeEnabled(enabled bool) {
	v.treeEnabled = enabled
	// If tree is disabled and we're in a tree mode, switch to table
	if !enabled && v.activeIdx > 0 {
		v.activeIdx = 0
	}
}

// SetActive sets the active view mode by index
func (v *ViewModeBar) SetActive(idx int) {
	if idx >= 0 && idx < len(v.modes) {
		// Don't allow setting tree modes if disabled
		if !v.treeEnabled && idx > 0 {
			return
		}
		v.activeIdx = idx
	}
}

// GetActive returns the index of the active mode
func (v *ViewModeBar) GetActive() int {
	return v.activeIdx
}

// GetActiveMode returns the name of the active mode
func (v *ViewModeBar) GetActiveMode() string {
	return v.modes[v.activeIdx]
}

// IsTreeEnabled returns whether tree modes are enabled
func (v *ViewModeBar) IsTreeEnabled() bool {
	return v.treeEnabled
}

// Render returns the rendered view mode bar
func (v *ViewModeBar) Render() string {
	// If tree modes are disabled, only show Table
	if !v.treeEnabled {
		return lipgloss.NewStyle().
			Foreground(ui.Muted).
			Render("View: ") + ui.ViewModeActiveStyle.Render("[1]Table")
	}

	viewLabel := lipgloss.NewStyle().
		Foreground(ui.Muted).
		Render("View: ")

	var modes []string
	for i, mode := range v.modes {
		key := v.keys[i]
		if i == v.activeIdx {
			// Active mode: highlighted with brackets
			modes = append(modes, ui.ViewModeActiveStyle.Render("["+key+"]"+mode))
		} else {
			// Inactive: just the key and shortened mode name
			shortMode := mode
			if len(shortMode) > 5 {
				shortMode = shortMode[:3]
			}
			modes = append(modes, ui.ViewModeInactiveStyle.Render(key+" "+shortMode))
		}
	}

	return viewLabel + strings.Join(modes, " ")
}

// formatCount formats a count for display
func formatCount(count int) string {
	if count >= 1000 {
		return formatThousands(count)
	}
	return intToString(count)
}

// intToString converts an int to string without importing strconv
func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// formatThousands formats large numbers with K suffix
func formatThousands(n int) string {
	if n >= 1000 {
		return intToString(n/1000) + "K"
	}
	return intToString(n)
}
