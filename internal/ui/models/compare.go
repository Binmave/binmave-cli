package models

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Binmave/binmave-cli/internal/api"
	"github.com/Binmave/binmave-cli/internal/ui"
	"github.com/Binmave/binmave-cli/internal/ui/components"
)

// DiffType represents the type of difference
type DiffType int

const (
	DiffNew DiffType = iota
	DiffRemoved
	DiffModified
	DiffUnchanged
)

// DiffItem represents a single difference item
type DiffItem struct {
	Path       string
	Label      string
	Type       DiffType
	AgentName  string
	Details    string
	AgentCount int // Number of agents affected
}

// CompareModel is the TUI model for comparing executions
type CompareModel struct {
	executionID string
	baselineID  string
	client      *api.Client
	ctx         context.Context

	// Data
	execution       *api.Execution
	baseline        *api.Execution
	currentResults  []api.ExecutionResult
	baselineResults []api.ExecutionResult
	diffs           []DiffItem

	// Counts
	newCount      int
	removedCount  int
	modifiedCount int

	// UI State
	loading       bool
	showDiffsOnly bool
	selectedIdx   int
	scrollOffset  int
	err           error
	dataReady     int // 0 = nothing, 1 = partial, 2 = all loaded

	// Components
	helpBar *components.HelpBar
	spinner spinner.Model

	// Dimensions
	width  int
	height int
	ready  bool
}

// Messages
type compareBaselineMsg struct {
	execution *api.Execution
	results   []api.ExecutionResult
	err       error
}

type compareCurrentMsg struct {
	execution *api.Execution
	results   []api.ExecutionResult
	err       error
}

// NewCompareModel creates a new compare TUI model
func NewCompareModel(executionID, baselineID string, client *api.Client) *CompareModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)

	helpBar := components.NewHelpBar()
	helpBar.SetItems(components.CompareViewHelpItems())

	return &CompareModel{
		executionID:   executionID,
		baselineID:    baselineID,
		client:        client,
		ctx:           context.Background(),
		loading:       true,
		showDiffsOnly: true,
		helpBar:       helpBar,
		spinner:       s,
		width:         80,
		height:        24,
	}
}

// Init initializes the model
func (m *CompareModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchBaseline,
		m.fetchCurrent,
	)
}

// fetchBaseline fetches baseline execution and results
func (m *CompareModel) fetchBaseline() tea.Msg {
	execution, err := m.client.GetExecution(m.ctx, m.baselineID)
	if err != nil {
		return compareBaselineMsg{err: err}
	}

	results, err := m.client.GetAllExecutionResults(m.ctx, m.baselineID)
	return compareBaselineMsg{execution: execution, results: results, err: err}
}

// fetchCurrent fetches current execution and results
func (m *CompareModel) fetchCurrent() tea.Msg {
	execution, err := m.client.GetExecution(m.ctx, m.executionID)
	if err != nil {
		return compareCurrentMsg{err: err}
	}

	results, err := m.client.GetAllExecutionResults(m.ctx, m.executionID)
	return compareCurrentMsg{execution: execution, results: results, err: err}
}

// Update handles messages
func (m *CompareModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "d":
			m.showDiffsOnly = true
			m.selectedIdx = 0
			m.scrollOffset = 0

		case "a":
			m.showDiffsOnly = false
			m.selectedIdx = 0
			m.scrollOffset = 0

		case "up", "k":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.ensureVisible()
			}

		case "down", "j":
			maxIdx := len(m.getVisibleDiffs()) - 1
			if m.selectedIdx < maxIdx {
				m.selectedIdx++
				m.ensureVisible()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.helpBar.SetWidth(m.width)
		m.ready = true

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case compareBaselineMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("baseline: %w", msg.err)
		} else {
			m.baseline = msg.execution
			m.baselineResults = msg.results
			m.dataReady++
			m.tryComputeDiffs()
		}

	case compareCurrentMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("current: %w", msg.err)
		} else {
			m.execution = msg.execution
			m.currentResults = msg.results
			m.dataReady++
			m.tryComputeDiffs()
		}
	}

	return m, tea.Batch(cmds...)
}

// tryComputeDiffs computes diffs once both datasets are loaded
func (m *CompareModel) tryComputeDiffs() {
	if m.dataReady < 2 {
		return
	}
	m.loading = false
	m.computeDiffs()
}

// computeDiffs compares baseline and current results
func (m *CompareModel) computeDiffs() {
	m.diffs = nil
	m.newCount = 0
	m.removedCount = 0
	m.modifiedCount = 0

	// Build path sets for baseline and current
	baselinePaths := make(map[string]map[string]bool) // path -> agentID -> exists
	currentPaths := make(map[string]map[string]bool)

	for _, r := range m.baselineResults {
		paths := extractPaths(r.AnswerJSON)
		for _, p := range paths {
			if baselinePaths[p] == nil {
				baselinePaths[p] = make(map[string]bool)
			}
			baselinePaths[p][r.AgentName] = true
		}
	}

	for _, r := range m.currentResults {
		paths := extractPaths(r.AnswerJSON)
		for _, p := range paths {
			if currentPaths[p] == nil {
				currentPaths[p] = make(map[string]bool)
			}
			currentPaths[p][r.AgentName] = true
		}
	}

	// Find new items (in current but not baseline)
	for path, agents := range currentPaths {
		if _, exists := baselinePaths[path]; !exists {
			agentList := mapKeys(agents)
			m.diffs = append(m.diffs, DiffItem{
				Path:       path,
				Label:      pathToLabel(path),
				Type:       DiffNew,
				AgentName:  strings.Join(agentList, ", "),
				AgentCount: len(agentList),
			})
			m.newCount++
		}
	}

	// Find removed items (in baseline but not current)
	for path, agents := range baselinePaths {
		if _, exists := currentPaths[path]; !exists {
			agentList := mapKeys(agents)
			m.diffs = append(m.diffs, DiffItem{
				Path:       path,
				Label:      pathToLabel(path),
				Type:       DiffRemoved,
				AgentName:  strings.Join(agentList, ", "),
				AgentCount: len(agentList),
			})
			m.removedCount++
		}
	}

	// Sort by type then path
	sort.Slice(m.diffs, func(i, j int) bool {
		if m.diffs[i].Type != m.diffs[j].Type {
			return m.diffs[i].Type < m.diffs[j].Type
		}
		return m.diffs[i].Path < m.diffs[j].Path
	})
}

// extractPaths extracts all paths from JSON data
func extractPaths(jsonStr string) []string {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil
	}

	var paths []string
	extractPathsRecursive(data, "", &paths)
	return paths
}

func extractPathsRecursive(data interface{}, prefix string, paths *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			newPath := prefix + "/" + key
			*paths = append(*paths, newPath)

			switch child := value.(type) {
			case map[string]interface{}, []interface{}:
				extractPathsRecursive(child, newPath, paths)
			default:
				// Include leaf values as paths
				leafPath := fmt.Sprintf("%s=%v", newPath, value)
				*paths = append(*paths, leafPath)
			}
		}

	case []interface{}:
		for i, item := range v {
			newPath := fmt.Sprintf("%s[%d]", prefix, i)

			// Try to get a meaningful label from the item
			if m, ok := item.(map[string]interface{}); ok {
				if label := findLabel(m); label != "" {
					newPath = prefix + "/" + label
				}
			}

			*paths = append(*paths, newPath)
			extractPathsRecursive(item, newPath, paths)
		}
	}
}

func pathToLabel(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// getVisibleDiffs returns diffs based on filter
func (m *CompareModel) getVisibleDiffs() []DiffItem {
	if !m.showDiffsOnly {
		return m.diffs
	}

	var filtered []DiffItem
	for _, d := range m.diffs {
		if d.Type != DiffUnchanged {
			filtered = append(filtered, d)
		}
	}
	return filtered
}

// ensureVisible scrolls to keep selection visible
func (m *CompareModel) ensureVisible() {
	viewportHeight := m.height - 10
	if m.selectedIdx < m.scrollOffset {
		m.scrollOffset = m.selectedIdx
	}
	if m.selectedIdx >= m.scrollOffset+viewportHeight {
		m.scrollOffset = m.selectedIdx - viewportHeight + 1
	}
}

// View renders the model
func (m *CompareModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Title bar
	execID := m.executionID
	if len(execID) > 8 {
		execID = execID[:8]
	}
	baseID := m.baselineID
	if len(baseID) > 8 {
		baseID = baseID[:8]
	}
	title := fmt.Sprintf(" Compare: %s vs %s ", execID, baseID)
	b.WriteString(ui.TitleStyle.Render(title))
	b.WriteString("\n")

	// Summary
	if m.loading {
		b.WriteString(m.spinner.View() + " Loading results...")
		b.WriteString("\n")
	} else {
		summary := fmt.Sprintf("Changes: %s new | %s removed | %s modified",
			ui.SuccessStyle.Render(fmt.Sprintf("%d", m.newCount)),
			ui.ErrorStyle.Render(fmt.Sprintf("%d", m.removedCount)),
			ui.WarningStyle.Render(fmt.Sprintf("%d", m.modifiedCount)),
		)
		b.WriteString(summary)
		b.WriteString("\n")
	}

	// Filter indicator
	filterText := "Showing: "
	if m.showDiffsOnly {
		filterText += ui.HeaderStyle.Render("Diffs only")
	} else {
		filterText += ui.HeaderStyle.Render("All items")
	}
	b.WriteString(filterText)
	b.WriteString("\n")

	// Error display
	if m.err != nil {
		b.WriteString(ui.ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n")
	}

	// Divider
	b.WriteString(ui.MutedStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Content
	contentHeight := m.height - 9
	content := m.renderDiffs(contentHeight)
	b.WriteString(content)

	// Ensure content fills height
	lines := strings.Count(content, "\n")
	for i := lines; i < contentHeight; i++ {
		b.WriteString("\n")
	}

	// Footer divider
	b.WriteString(ui.MutedStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Help bar
	b.WriteString(m.helpBar.Render())

	return b.String()
}

// renderDiffs renders the diff list
func (m *CompareModel) renderDiffs(height int) string {
	diffs := m.getVisibleDiffs()

	if len(diffs) == 0 {
		if m.loading {
			return ""
		}
		return ui.MutedStyle.Render("No differences found")
	}

	var b strings.Builder

	endIdx := m.scrollOffset + height
	if endIdx > len(diffs) {
		endIdx = len(diffs)
	}

	for i := m.scrollOffset; i < endIdx; i++ {
		d := diffs[i]
		isSelected := i == m.selectedIdx

		// Type indicator
		var typeIndicator string
		var typeStyle lipgloss.Style

		switch d.Type {
		case DiffNew:
			typeIndicator = "+ "
			typeStyle = lipgloss.NewStyle().Foreground(ui.Success)
		case DiffRemoved:
			typeIndicator = "- "
			typeStyle = lipgloss.NewStyle().Foreground(ui.Error)
		case DiffModified:
			typeIndicator = "~ "
			typeStyle = lipgloss.NewStyle().Foreground(ui.Warning)
		default:
			typeIndicator = "  "
			typeStyle = lipgloss.NewStyle().Foreground(ui.Muted)
		}

		// Truncate label if needed
		label := d.Label
		agentInfo := ""
		if d.AgentCount > 0 {
			if d.AgentCount == 1 {
				agentInfo = fmt.Sprintf(" (%s)", d.AgentName)
			} else {
				agentInfo = fmt.Sprintf(" (%d agents)", d.AgentCount)
			}
		}

		maxLabelLen := m.width - len(typeIndicator) - len(agentInfo) - 4
		if maxLabelLen < 20 {
			maxLabelLen = 20
		}
		if len(label) > maxLabelLen {
			label = label[:maxLabelLen-3] + "..."
		}

		line := typeIndicator + label + agentInfo

		if isSelected {
			b.WriteString(ui.SelectedStyle.Render(line))
		} else {
			b.WriteString(typeStyle.Render(typeIndicator) + label + ui.MutedStyle.Render(agentInfo))
		}
		b.WriteString("\n")
	}

	return b.String()
}
