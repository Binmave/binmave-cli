package models

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Binmave/binmave-cli/internal/api"
	"github.com/Binmave/binmave-cli/internal/ui"
	"github.com/Binmave/binmave-cli/internal/ui/components"
)

// ViewMode represents the current view mode
type ViewMode int

const (
	TableView ViewMode = iota
	TreeView
	AggregatedView
)

// TabIndex represents the current tab
type TabIndex int

const (
	ResultsTab TabIndex = iota
	ErrorsTab
)

// ResultsModel is the main TUI model for viewing execution results
type ResultsModel struct {
	executionID string
	client      *api.Client
	ctx         context.Context

	// Data
	execution     *api.Execution
	status        *api.ExecutionStatus
	results       []api.ExecutionResult
	errors        []api.ExecutionResult
	agentTrees    []*components.AgentTree
	aggregateTree []*components.TreeNode

	// UI State
	viewMode         ViewMode
	currentTab       TabIndex
	showAnomaliesOnly bool
	loading          bool
	err              error

	// Components
	tabBar      *components.TabBar
	viewModeBar *components.ViewModeBar
	treeView    *components.TreeView
	helpBar     *components.HelpBar
	progress    *components.ProgressIndicator
	spinner     spinner.Model
	viewport    viewport.Model

	// Table view state
	tableSelectedIdx int
	tableScrollOffset int

	// Dimensions
	width  int
	height int
	ready  bool
}

// Messages
type statusMsg struct {
	status *api.ExecutionStatus
	err    error
}

type resultsMsg struct {
	results []api.ExecutionResult
	err     error
}

type errMsg struct {
	err error
}

type tickMsg time.Time

// SetInitialViewMode sets the view mode before running
func (m *ResultsModel) SetInitialViewMode(mode ViewMode) {
	m.viewMode = mode
	m.viewModeBar.SetActive(int(mode))
}

// SetAnomaliesOnly sets the anomalies filter
func (m *ResultsModel) SetAnomaliesOnly(only bool) {
	m.showAnomaliesOnly = only
}

// NewResultsModel creates a new results TUI model
func NewResultsModel(executionID string, client *api.Client) *ResultsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)

	tabBar := components.NewTabBar([]components.Tab{
		{Label: "Results", Count: 0},
		{Label: "Errors", Count: 0},
	})

	return &ResultsModel{
		executionID: executionID,
		client:      client,
		ctx:         context.Background(),

		viewMode:   TableView,
		currentTab: ResultsTab,
		loading:    true,

		tabBar:      tabBar,
		viewModeBar: components.NewViewModeBar(),
		treeView:    components.NewTreeView(),
		helpBar:     components.NewHelpBar(),
		progress:    components.NewProgressIndicator(),
		spinner:     s,

		width:  80,
		height: 24,
	}
}

// Init initializes the model
func (m *ResultsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.fetchStatus,
		m.fetchResults,
	)
}

// fetchStatus fetches the execution status
func (m *ResultsModel) fetchStatus() tea.Msg {
	status, err := m.client.GetExecutionStatus(m.ctx, m.executionID)
	return statusMsg{status: status, err: err}
}

// fetchResults fetches all execution results
func (m *ResultsModel) fetchResults() tea.Msg {
	results, err := m.client.GetAllExecutionResults(m.ctx, m.executionID)
	return resultsMsg{results: results, err: err}
}

// tick returns a tick command for auto-refresh
func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages
func (m *ResultsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.nextTab()

		case "1":
			m.setViewMode(TableView)

		case "2":
			m.setViewMode(TreeView)

		case "3":
			m.setViewMode(AggregatedView)

		case "up", "k":
			m.navigateUp()

		case "down", "j":
			m.navigateDown()

		case "enter", " ":
			m.toggleExpand()

		case "e":
			if m.viewMode == TreeView || m.viewMode == AggregatedView {
				m.treeView.ExpandAll()
			}

		case "c":
			if m.viewMode == TreeView || m.viewMode == AggregatedView {
				m.treeView.CollapseAll()
			}

		case "a":
			if m.viewMode == AggregatedView {
				m.showAnomaliesOnly = !m.showAnomaliesOnly
				m.rebuildAggregatedTree()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateDimensions()
		m.ready = true

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case statusMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.status = msg.status
			m.progress.SetProgress(msg.status.Received, msg.status.Expected)
			m.tabBar.UpdateCount(0, msg.status.Received-msg.status.Errors)
			m.tabBar.UpdateCount(1, msg.status.Errors)

			// Keep refreshing if not complete
			if msg.status.State != "Completed" && msg.status.State != "Failed" {
				cmds = append(cmds, tick())
			}
		}

	case resultsMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.results = msg.results
			m.processResults()
		}

	case tickMsg:
		// Refresh status
		cmds = append(cmds, m.fetchStatus)
		// Also refresh results if not complete
		if m.status != nil && m.status.State != "Completed" && m.status.State != "Failed" {
			cmds = append(cmds, m.fetchResults)
		}
	}

	return m, tea.Batch(cmds...)
}

// nextTab switches to the next tab
func (m *ResultsModel) nextTab() {
	m.tabBar.Next()
	m.currentTab = TabIndex(m.tabBar.GetActive())
	m.tableSelectedIdx = 0
	m.tableScrollOffset = 0

	// Rebuild tree for new tab
	if m.currentTab == ErrorsTab {
		m.rebuildTreeFromErrors()
	} else {
		m.rebuildTreeFromResults()
	}
}

// setViewMode changes the view mode
func (m *ResultsModel) setViewMode(mode ViewMode) {
	m.viewMode = mode
	m.viewModeBar.SetActive(int(mode))
	m.updateHelpItems()
}

// navigateUp moves selection up
func (m *ResultsModel) navigateUp() {
	switch m.viewMode {
	case TableView:
		if m.tableSelectedIdx > 0 {
			m.tableSelectedIdx--
			m.ensureTableVisible()
		}
	case TreeView, AggregatedView:
		m.treeView.MoveUp()
	}
}

// navigateDown moves selection down
func (m *ResultsModel) navigateDown() {
	switch m.viewMode {
	case TableView:
		maxIdx := len(m.getCurrentResults()) - 1
		if m.tableSelectedIdx < maxIdx {
			m.tableSelectedIdx++
			m.ensureTableVisible()
		}
	case TreeView, AggregatedView:
		m.treeView.MoveDown()
	}
}

// toggleExpand expands/collapses current selection
func (m *ResultsModel) toggleExpand() {
	switch m.viewMode {
	case TreeView, AggregatedView:
		m.treeView.Toggle()
	}
}

// getCurrentResults returns results for the current tab
func (m *ResultsModel) getCurrentResults() []api.ExecutionResult {
	if m.currentTab == ErrorsTab {
		return m.errors
	}
	// Filter to non-errors
	var results []api.ExecutionResult
	for _, r := range m.results {
		if !r.HasError {
			results = append(results, r)
		}
	}
	return results
}

// ensureTableVisible scrolls to keep selection visible
func (m *ResultsModel) ensureTableVisible() {
	viewportHeight := m.height - 10 // Account for header, tabs, etc
	if m.tableSelectedIdx < m.tableScrollOffset {
		m.tableScrollOffset = m.tableSelectedIdx
	}
	if m.tableSelectedIdx >= m.tableScrollOffset+viewportHeight {
		m.tableScrollOffset = m.tableSelectedIdx - viewportHeight + 1
	}
}

// processResults processes fetched results
func (m *ResultsModel) processResults() {
	// Separate errors
	m.errors = nil
	for _, r := range m.results {
		if r.HasError {
			m.errors = append(m.errors, r)
		}
	}

	m.tabBar.UpdateCount(0, len(m.results)-len(m.errors))
	m.tabBar.UpdateCount(1, len(m.errors))

	m.rebuildTreeFromResults()
}

// rebuildTreeFromResults builds tree view from results
func (m *ResultsModel) rebuildTreeFromResults() {
	m.agentTrees = nil

	results := m.getCurrentResults()
	for _, r := range results {
		tree := m.buildAgentTree(r)
		if tree != nil {
			m.agentTrees = append(m.agentTrees, tree)
		}
	}

	m.treeView.SetAgents(m.agentTrees)
	m.rebuildAggregatedTree()
}

// rebuildTreeFromErrors builds tree view from error results
func (m *ResultsModel) rebuildTreeFromErrors() {
	m.agentTrees = nil

	for _, r := range m.errors {
		tree := m.buildAgentTree(r)
		if tree != nil {
			m.agentTrees = append(m.agentTrees, tree)
		}
	}

	m.treeView.SetAgents(m.agentTrees)
}

// buildAgentTree builds a tree from a single agent's result
func (m *ResultsModel) buildAgentTree(result api.ExecutionResult) *components.AgentTree {
	tree := &components.AgentTree{
		AgentID:   result.AgentID,
		AgentName: result.AgentName,
		Expanded:  false,
	}

	// Parse JSON result
	var data interface{}
	if err := json.Unmarshal([]byte(result.AnswerJSON), &data); err != nil {
		// If not valid JSON, create a simple node
		tree.Roots = []*components.TreeNode{{
			ID:    "0",
			Label: truncateString(result.AnswerJSON, 100),
		}}
		tree.NodeCount = 1
		return tree
	}

	// Build tree from JSON
	nodeCount := 0
	tree.Roots = buildTreeNodes(data, 0, &nodeCount)
	tree.NodeCount = nodeCount

	return tree
}

// buildTreeNodes recursively builds tree nodes from JSON data
func buildTreeNodes(data interface{}, depth int, count *int) []*components.TreeNode {
	var nodes []*components.TreeNode

	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			*count++
			node := &components.TreeNode{
				ID:       fmt.Sprintf("%d-%s", *count, key),
				Label:    key,
				Depth:    depth,
				Expanded: false,
			}

			switch child := value.(type) {
			case map[string]interface{}, []interface{}:
				node.Children = buildTreeNodes(child, depth+1, count)
			default:
				// Leaf value
				node.Label = fmt.Sprintf("%s: %v", key, value)
			}

			nodes = append(nodes, node)
		}

	case []interface{}:
		for i, item := range v {
			*count++
			node := &components.TreeNode{
				ID:       fmt.Sprintf("%d-[%d]", *count, i),
				Depth:    depth,
				Expanded: false,
			}

			switch child := item.(type) {
			case map[string]interface{}:
				// Try to find a good label from the map
				label := findLabel(child)
				if label == "" {
					label = fmt.Sprintf("[%d]", i)
				}
				node.Label = label
				node.Children = buildTreeNodes(child, depth+1, count)
			case []interface{}:
				node.Label = fmt.Sprintf("[%d] (%d items)", i, len(child))
				node.Children = buildTreeNodes(child, depth+1, count)
			default:
				node.Label = fmt.Sprintf("[%d]: %v", i, item)
			}

			nodes = append(nodes, node)
		}
	}

	return nodes
}

// findLabel tries to find a good label from a map
func findLabel(m map[string]interface{}) string {
	// Common label keys
	labelKeys := []string{"name", "Name", "label", "Label", "title", "Title", "path", "Path", "key", "Key", "id", "Id", "ID"}
	for _, key := range labelKeys {
		if val, ok := m[key]; ok {
			if s, ok := val.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// rebuildAggregatedTree builds the aggregated tree view
func (m *ResultsModel) rebuildAggregatedTree() {
	if m.viewMode != AggregatedView {
		return
	}

	totalAgents := len(m.agentTrees)
	if totalAgents == 0 {
		return
	}

	// Build aggregated tree by path
	pathCounts := make(map[string]*aggregatedPath)
	for _, agentTree := range m.agentTrees {
		collectPaths(agentTree.Roots, "", agentTree.AgentName, pathCounts)
	}

	// Convert to tree nodes with counts
	m.aggregateTree = buildAggregatedNodes(pathCounts, "", totalAgents, m.showAnomaliesOnly)

	// Create single agent tree for the aggregated view
	aggregatedAgent := &components.AgentTree{
		AgentID:   "aggregated",
		AgentName: fmt.Sprintf("All Agents (%d)", totalAgents),
		Roots:     m.aggregateTree,
		NodeCount: len(pathCounts),
		Expanded:  true,
	}

	m.treeView.SetAgents([]*components.AgentTree{aggregatedAgent})
}

type aggregatedPath struct {
	label      string
	count      int
	agentNames []string
	children   map[string]*aggregatedPath
}

func collectPaths(nodes []*components.TreeNode, prefix string, agentName string, paths map[string]*aggregatedPath) {
	for _, node := range nodes {
		path := prefix + "/" + node.Label
		if paths[path] == nil {
			paths[path] = &aggregatedPath{
				label:    node.Label,
				children: make(map[string]*aggregatedPath),
			}
		}
		paths[path].count++
		paths[path].agentNames = append(paths[path].agentNames, agentName)

		if len(node.Children) > 0 {
			collectPaths(node.Children, path, agentName, paths)
		}
	}
}

func buildAggregatedNodes(paths map[string]*aggregatedPath, prefix string, totalAgents int, anomaliesOnly bool) []*components.TreeNode {
	var nodes []*components.TreeNode

	// Find direct children of this prefix
	childPaths := make(map[string]*aggregatedPath)
	for path, p := range paths {
		// Check if this is a direct child
		if strings.HasPrefix(path, prefix+"/") {
			remainder := strings.TrimPrefix(path, prefix+"/")
			if !strings.Contains(remainder, "/") {
				childPaths[path] = p
			}
		}
	}

	// Sort by count (descending)
	type pathEntry struct {
		path string
		p    *aggregatedPath
	}
	var entries []pathEntry
	for path, p := range childPaths {
		entries = append(entries, pathEntry{path, p})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].p.count > entries[j].p.count
	})

	anomalyThreshold := totalAgents / 10 // 10% threshold
	if anomalyThreshold < 1 {
		anomalyThreshold = 1
	}

	for _, e := range entries {
		isAnomaly := e.p.count <= anomalyThreshold

		// Skip non-anomalies if showing anomalies only
		if anomaliesOnly && !isAnomaly {
			continue
		}

		node := &components.TreeNode{
			ID:         e.path,
			Label:      e.p.label,
			Count:      e.p.count,
			TotalCount: totalAgents,
			AgentNames: e.p.agentNames,
			IsAnomaly:  isAnomaly,
			Expanded:   false,
		}

		// Recursively build children
		node.Children = buildAggregatedNodes(paths, e.path, totalAgents, anomaliesOnly)

		nodes = append(nodes, node)
	}

	return nodes
}

// updateDimensions updates component dimensions
func (m *ResultsModel) updateDimensions() {
	m.treeView.SetWidth(m.width - 4)
	m.treeView.SetViewportHeight(m.height - 10)
	m.tabBar.SetWidth(m.width)
	m.helpBar.SetWidth(m.width)
	m.progress.SetWidth(m.width - 30)
	m.updateHelpItems()
}

// updateHelpItems updates help bar based on current mode
func (m *ResultsModel) updateHelpItems() {
	switch m.viewMode {
	case TableView:
		m.helpBar.SetItems(components.TableViewHelpItems())
	case TreeView:
		m.helpBar.SetItems(components.TreeViewHelpItems())
	case AggregatedView:
		m.helpBar.SetItems(components.AggregatedViewHelpItems())
	}
}

// View renders the model
func (m *ResultsModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var b strings.Builder

	// Title bar
	title := fmt.Sprintf(" Results: %s ", m.executionID[:8])
	b.WriteString(ui.TitleStyle.Render(title))
	b.WriteString("\n")

	// Progress bar
	if m.status != nil {
		b.WriteString(m.progress.Render())
		b.WriteString("\n")
	} else if m.loading {
		b.WriteString(m.spinner.View() + " Loading...")
		b.WriteString("\n")
	}

	// Error display
	if m.err != nil {
		b.WriteString(ui.ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Tabs and view mode
	tabsLine := m.tabBar.Render()
	viewModeLine := m.viewModeBar.Render()

	// Calculate spacing
	tabsWidth := lipgloss.Width(tabsLine)
	viewModeWidth := lipgloss.Width(viewModeLine)
	spacing := m.width - tabsWidth - viewModeWidth - 2
	if spacing < 0 {
		spacing = 0
	}

	b.WriteString(tabsLine)
	b.WriteString(strings.Repeat(" ", spacing))
	b.WriteString(viewModeLine)
	b.WriteString("\n")

	// Divider
	b.WriteString(ui.MutedStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Content area
	contentHeight := m.height - 8
	content := m.renderContent(contentHeight)
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

// renderContent renders the main content based on view mode
func (m *ResultsModel) renderContent(height int) string {
	switch m.viewMode {
	case TableView:
		return m.renderTableView(height)
	case TreeView, AggregatedView:
		return m.treeView.Render()
	}
	return ""
}

// renderTableView renders the table view
func (m *ResultsModel) renderTableView(height int) string {
	results := m.getCurrentResults()

	if len(results) == 0 {
		return ui.MutedStyle.Render("No results to display")
	}

	var b strings.Builder

	// Header
	header := fmt.Sprintf("%-20s │ %-40s │ %-6s │ %-8s",
		"AGENT", "RESULT", "TIME", "DURATION")
	b.WriteString(ui.HeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render(strings.Repeat("─", m.width-2)))
	b.WriteString("\n")

	// Rows
	endIdx := m.tableScrollOffset + height - 4
	if endIdx > len(results) {
		endIdx = len(results)
	}

	for i := m.tableScrollOffset; i < endIdx; i++ {
		r := results[i]
		isSelected := i == m.tableSelectedIdx

		agentName := truncateString(r.AgentName, 18)
		resultPreview := truncateString(r.AnswerJSON, 38)
		timeSince := formatTimeSince(r.ResultReceived)
		duration := fmt.Sprintf("%ds", r.ExecutionTimeSeconds)

		row := fmt.Sprintf("%-20s │ %-40s │ %-6s │ %-8s",
			agentName, resultPreview, timeSince, duration)

		if isSelected {
			b.WriteString(ui.SelectedStyle.Render(row))
		} else if r.HasError {
			b.WriteString(ui.ErrorStyle.Render(row))
		} else {
			b.WriteString(row)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Helper functions
func truncateString(s string, max int) string {
	// Remove newlines
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")

	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func formatTimeSince(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
