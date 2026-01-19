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

// TableRow represents a flattened row for table display
type TableRow struct {
	AgentName string
	AgentID   string
	Data      map[string]string
	RowIndex  int
	HasError  bool
}

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

	// Table data (parsed from JSON)
	tableRows    []TableRow
	tableColumns []string
	isTreeData   bool // True if data is hierarchical (tree/aggregated views applicable)

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
	tableSelectedIdx  int
	tableScrollOffset int

	// Detail view state
	showingDetail  bool
	detailViewport viewport.Model
	detailContent  string

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

type executionMsg struct {
	execution *api.Execution
	err       error
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
		m.fetchExecution,
		m.fetchStatus,
		m.fetchResults,
	)
}

// fetchExecution fetches the execution details (script name, etc.)
func (m *ResultsModel) fetchExecution() tea.Msg {
	exec, err := m.client.GetExecution(m.ctx, m.executionID)
	return executionMsg{execution: exec, err: err}
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
		// Handle detail view mode separately
		if m.showingDetail {
			switch msg.String() {
			case "q", "esc", "enter":
				m.showingDetail = false
				return m, nil
			case "up", "k":
				m.detailViewport.LineUp(1)
			case "down", "j":
				m.detailViewport.LineDown(1)
			case "pgup":
				m.detailViewport.HalfViewUp()
			case "pgdown":
				m.detailViewport.HalfViewDown()
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.nextTab()

		case "1":
			m.setViewMode(TableView)

		case "2":
			// Only allow Tree view if data is hierarchical
			if m.isTreeData {
				m.setViewMode(TreeView)
			}

		case "3":
			// Only allow Aggregated view if data is hierarchical
			if m.isTreeData {
				m.setViewMode(AggregatedView)
			}

		case "up", "k":
			m.navigateUp()

		case "down", "j":
			m.navigateDown()

		case "right", "l":
			if m.viewMode == TreeView || m.viewMode == AggregatedView {
				m.treeView.Expand()
			}

		case "left", "h":
			if m.viewMode == TreeView || m.viewMode == AggregatedView {
				m.treeView.Collapse()
			}

		case "enter", " ":
			if m.viewMode == TableView {
				m.showDetailView()
			} else {
				m.toggleExpand()
			}

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

	case executionMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.execution = msg.execution
		}

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

	// Rebuild table and tree for new tab
	m.buildTableData()
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

	// Rebuild tree data when switching views
	if mode == AggregatedView {
		// Ensure we have agent trees first, then build aggregated
		if len(m.agentTrees) == 0 {
			if m.currentTab == ErrorsTab {
				m.rebuildTreeFromErrors()
			} else {
				m.rebuildTreeFromResults()
			}
		}
		m.rebuildAggregatedTree()
	} else if mode == TreeView {
		// Rebuild per-agent tree view
		if m.currentTab == ErrorsTab {
			m.rebuildTreeFromErrors()
		} else {
			m.rebuildTreeFromResults()
		}
	}
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
		maxIdx := len(m.tableRows) - 1
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

// showDetailView shows the full details of the selected row
func (m *ResultsModel) showDetailView() {
	if m.tableSelectedIdx < 0 || m.tableSelectedIdx >= len(m.tableRows) {
		return
	}

	row := m.tableRows[m.tableSelectedIdx]

	// Build detail content
	var b strings.Builder
	b.WriteString(ui.HeaderStyle.Render("Agent: " + row.AgentName))
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render(fmt.Sprintf("Row %d", row.RowIndex+1)))
	b.WriteString("\n\n")

	// Get the original result for full data
	results := m.getCurrentResults()
	var originalResult *api.ExecutionResult
	for i := range results {
		if results[i].AgentID == row.AgentID {
			originalResult = &results[i]
			break
		}
	}

	if originalResult != nil {
		if originalResult.HasError {
			b.WriteString(ui.ErrorStyle.Render("Error:"))
			b.WriteString("\n")
			// Show full error message with word wrapping
			errorMsg := originalResult.RawStdError
			if errorMsg == "" {
				errorMsg = originalResult.AnswerJSON
			}
			b.WriteString(wrapText(errorMsg, m.width-4))
			b.WriteString("\n")
		} else {
			// Parse JSON and extract just the selected row's item
			var data interface{}
			if err := json.Unmarshal([]byte(originalResult.AnswerJSON), &data); err == nil {
				// If it's an array, get just the item at RowIndex
				var itemToShow interface{}
				if arr, ok := data.([]interface{}); ok {
					if row.RowIndex >= 0 && row.RowIndex < len(arr) {
						itemToShow = arr[row.RowIndex]
					} else {
						itemToShow = data // Fallback to full array
					}
				} else {
					itemToShow = data // Single object, show it
				}

				prettyJSON, _ := json.MarshalIndent(itemToShow, "", "  ")
				b.WriteString(string(prettyJSON))
			} else {
				b.WriteString(originalResult.AnswerJSON)
			}
		}
	} else {
		// Fallback to row data (show all columns with full values)
		for col, val := range row.Data {
			b.WriteString(ui.HeaderStyle.Render(col + ":"))
			b.WriteString("\n")
			b.WriteString(wrapText(val, m.width-4))
			b.WriteString("\n\n")
		}
	}

	m.detailContent = b.String()
	m.detailViewport = viewport.New(m.width-4, m.height-6)
	m.detailViewport.SetContent(m.detailContent)
	m.showingDetail = true
}

// wrapText wraps text at the specified width
func wrapText(text string, width int) string {
	if width <= 0 {
		width = 80
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// Wrap long lines
		for len(line) > width {
			// Find a good break point (look for space to break at)
			breakPoint := width
			if breakPoint >= len(line) {
				breakPoint = len(line)
			}
			// Search backwards for a space
			for i := breakPoint - 1; i > width/2 && i > 0; i-- {
				if line[i] == ' ' {
					breakPoint = i
					break
				}
			}
			if breakPoint > len(line) {
				breakPoint = len(line)
			}
			result.WriteString(line[:breakPoint])
			result.WriteString("\n")
			if breakPoint < len(line) {
				line = strings.TrimPrefix(line[breakPoint:], " ")
			} else {
				line = ""
			}
		}
		if len(line) > 0 {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}

	return result.String()
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

	// Build table rows from JSON
	m.buildTableData()

	// Detect if tree view is applicable (before building trees)
	m.detectTreeApplicability()

	// Build tree data based on current view mode
	if m.viewMode == AggregatedView {
		m.rebuildTreeFromResults() // Need agent trees first
		m.rebuildAggregatedTree()
	} else if m.viewMode == TreeView {
		m.rebuildTreeFromResults()
	}
}

// buildTableData parses JSON results into flat table rows
func (m *ResultsModel) buildTableData() {
	m.tableRows = nil
	m.tableColumns = nil
	columnSet := make(map[string]bool)

	results := m.getCurrentResults()

	for _, r := range results {
		// For errors, use RawStdError instead of AnswerJSON
		if r.HasError {
			errorMsg := r.RawStdError
			if errorMsg == "" {
				errorMsg = r.AnswerJSON // Fallback to AnswerJSON if no stderr
			}
			if errorMsg == "" {
				errorMsg = "(no error message)"
			}
			m.tableRows = append(m.tableRows, TableRow{
				AgentName: r.AgentName,
				AgentID:   r.AgentID,
				Data:      map[string]string{"Error": truncateString(errorMsg, 200)},
				RowIndex:  0,
				HasError:  true,
			})
			columnSet["Error"] = true
			continue
		}

		// Parse JSON
		var data interface{}
		if err := json.Unmarshal([]byte(r.AnswerJSON), &data); err != nil {
			// Not valid JSON - create a single row with raw value
			m.tableRows = append(m.tableRows, TableRow{
				AgentName: r.AgentName,
				AgentID:   r.AgentID,
				Data:      map[string]string{"Value": truncateString(r.AnswerJSON, 100)},
				RowIndex:  0,
				HasError:  r.HasError,
			})
			columnSet["Value"] = true
			continue
		}

		// Normalize to array of objects
		rows := normalizeToRows(data)

		if len(rows) == 0 {
			// Empty result
			m.tableRows = append(m.tableRows, TableRow{
				AgentName: r.AgentName,
				AgentID:   r.AgentID,
				Data:      map[string]string{},
				RowIndex:  0,
				HasError:  r.HasError,
			})
			continue
		}

		// Create a row for each item
		for i, row := range rows {
			flatRow := flattenObject(row, "")
			for k := range flatRow {
				columnSet[k] = true
			}

			m.tableRows = append(m.tableRows, TableRow{
				AgentName: r.AgentName,
				AgentID:   r.AgentID,
				Data:      flatRow,
				RowIndex:  i,
				HasError:  r.HasError,
			})
		}
	}

	// Sort columns with common ones first
	m.tableColumns = sortColumns(columnSet)
}

// normalizeToRows converts JSON data to array of maps
func normalizeToRows(data interface{}) []map[string]interface{} {
	switch v := data.(type) {
	case []interface{}:
		var rows []map[string]interface{}
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				rows = append(rows, m)
			}
		}
		return rows
	case map[string]interface{}:
		return []map[string]interface{}{v}
	}
	return nil
}

// flattenObject flattens nested objects with dot notation
func flattenObject(obj map[string]interface{}, prefix string) map[string]string {
	result := make(map[string]string)

	for key, value := range obj {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			// Flatten nested objects
			nested := flattenObject(v, fullKey)
			for k, val := range nested {
				result[k] = val
			}
		case []interface{}:
			// For arrays, show count or first few items
			if len(v) == 0 {
				result[fullKey] = "[]"
			} else if len(v) <= 3 {
				// Show short arrays inline
				var items []string
				for _, item := range v {
					items = append(items, fmt.Sprintf("%v", item))
				}
				result[fullKey] = strings.Join(items, ", ")
			} else {
				result[fullKey] = fmt.Sprintf("[%d items]", len(v))
			}
		case nil:
			result[fullKey] = ""
		default:
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}

	return result
}

// sortColumns returns columns sorted with priority columns first
func sortColumns(columnSet map[string]bool) []string {
	// Priority columns that should appear first
	priority := []string{"Name", "name", "User", "user", "Group", "group", "Path", "path",
		"Value", "value", "Status", "status", "Type", "type", "ID", "id", "Id"}

	var columns []string
	added := make(map[string]bool)

	// Add priority columns first
	for _, p := range priority {
		if columnSet[p] && !added[p] {
			columns = append(columns, p)
			added[p] = true
		}
	}

	// Add remaining columns alphabetically
	var remaining []string
	for col := range columnSet {
		if !added[col] {
			remaining = append(remaining, col)
		}
	}
	sort.Strings(remaining)
	columns = append(columns, remaining...)

	return columns
}

// detectTreeApplicability checks if tree/aggregated views make sense for this data
func (m *ResultsModel) detectTreeApplicability() {
	// Tree view is applicable if:
	// 1. Data has nested objects/arrays, OR
	// 2. Data has self-referential structure (parent/child IDs)

	m.isTreeData = false

	results := m.getCurrentResults()
	if len(results) == 0 {
		m.viewModeBar.SetTreeEnabled(false)
		return
	}

	// Check first result for nested structure
	var data interface{}
	if err := json.Unmarshal([]byte(results[0].AnswerJSON), &data); err != nil {
		m.viewModeBar.SetTreeEnabled(false)
		return
	}

	m.isTreeData = hasNestedStructure(data)
	m.viewModeBar.SetTreeEnabled(m.isTreeData)
}

// hasNestedStructure checks if data contains nested objects/arrays
func hasNestedStructure(data interface{}) bool {
	switch v := data.(type) {
	case []interface{}:
		if len(v) == 0 {
			return false
		}
		// Check if array items have nested objects/arrays
		if first, ok := v[0].(map[string]interface{}); ok {
			for _, val := range first {
				switch val.(type) {
				case map[string]interface{}, []interface{}:
					return true
				}
			}
			// Also check for self-referential fields (parent/child)
			return hasSelfReferentialFields(v)
		}
	case map[string]interface{}:
		for _, val := range v {
			switch val.(type) {
			case map[string]interface{}, []interface{}:
				return true
			}
		}
	}
	return false
}

// hasSelfReferentialFields detects self-referential tree structure by analyzing data values
// This matches the web frontend's approach: look for a field where values reference another field's values
func hasSelfReferentialFields(rows []interface{}) bool {
	if len(rows) < 2 {
		return false
	}

	// Convert to maps
	var rowMaps []map[string]interface{}
	for _, r := range rows {
		if m, ok := r.(map[string]interface{}); ok {
			rowMaps = append(rowMaps, m)
		}
	}

	if len(rowMaps) < 2 {
		return false
	}

	// Get column names from first row
	columns := make([]string, 0)
	for k := range rowMaps[0] {
		columns = append(columns, k)
	}

	if len(columns) < 2 {
		return false
	}

	// Try each pair of columns to find id/parent relationship
	for _, idCol := range columns {
		// Collect all values in the potential ID column (as strings to avoid unhashable types)
		idValues := make(map[string]bool)
		for _, row := range rowMaps {
			if val, ok := row[idCol]; ok && val != nil {
				// Only use scalar values (strings, numbers) - skip maps and slices
				strVal := toStringKey(val)
				if strVal != "" {
					idValues[strVal] = true
				}
			}
		}

		// Skip if no valid IDs
		if len(idValues) == 0 {
			continue
		}

		for _, parentCol := range columns {
			if idCol == parentCol {
				continue
			}

			// Count valid references:
			// - null/nil/empty/0 = root node (valid)
			// - references an existing ID (valid)
			validRefs := 0
			hasRoot := false

			for _, row := range rowMaps {
				parentVal := row[parentCol]

				// Check for root indicators
				if parentVal == nil {
					validRefs++
					hasRoot = true
				} else {
					strVal := toStringKey(parentVal)
					if strVal == "" || strVal == "0" {
						validRefs++
						hasRoot = true
					} else if idValues[strVal] {
						validRefs++
					}
				}
			}

			// Must have: 80%+ valid references AND at least one root
			refRatio := float64(validRefs) / float64(len(rowMaps))
			if refRatio >= 0.8 && hasRoot {
				return true
			}
		}
	}

	return false
}

// toStringKey converts a value to a string key, returning empty string for non-scalar types
func toStringKey(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		// Maps, slices, etc. - not usable as keys
		return ""
	}
}

// rebuildTreeFromResults builds tree view from results (per-agent trees)
func (m *ResultsModel) rebuildTreeFromResults() {
	m.agentTrees = nil

	results := m.getCurrentResults()
	for _, r := range results {
		tree := m.buildAgentTree(r)
		if tree != nil {
			m.agentTrees = append(m.agentTrees, tree)
		}
	}

	// Set per-agent trees for Tree view (NOT aggregated)
	m.treeView.SetAgents(m.agentTrees)
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

	// Render detail view if showing
	if m.showingDetail {
		return m.renderDetailView()
	}

	var b strings.Builder

	// Title bar with script info
	execID := m.executionID
	if len(execID) > 8 {
		execID = execID[:8]
	}

	var title string
	if m.execution != nil {
		title = fmt.Sprintf(" %s (ID: %d) - %s ", m.execution.ScriptName, m.execution.ScriptID, execID)
	} else {
		title = fmt.Sprintf(" Results: %s ", execID)
	}
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

// renderTableView renders the table view with parsed JSON columns
func (m *ResultsModel) renderTableView(height int) string {
	if len(m.tableRows) == 0 {
		return ui.MutedStyle.Render("No results to display")
	}

	var b strings.Builder

	// Calculate column widths
	colWidths := m.calculateColumnWidths()

	// Header
	var headerParts []string
	headerParts = append(headerParts, fmt.Sprintf("%-*s", colWidths["_agent"], "AGENT"))
	for _, col := range m.tableColumns {
		width := colWidths[col]
		headerParts = append(headerParts, fmt.Sprintf("%-*s", width, strings.ToUpper(col)))
	}
	header := strings.Join(headerParts, " │ ")
	b.WriteString(ui.HeaderStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(ui.MutedStyle.Render(strings.Repeat("─", min(len(header)+10, m.width-2))))
	b.WriteString("\n")

	// Rows
	endIdx := m.tableScrollOffset + height - 4
	if endIdx > len(m.tableRows) {
		endIdx = len(m.tableRows)
	}

	for i := m.tableScrollOffset; i < endIdx; i++ {
		row := m.tableRows[i]
		isSelected := i == m.tableSelectedIdx

		var rowParts []string
		rowParts = append(rowParts, fmt.Sprintf("%-*s", colWidths["_agent"], truncateString(row.AgentName, colWidths["_agent"])))

		for _, col := range m.tableColumns {
			width := colWidths[col]
			value := row.Data[col]
			rowParts = append(rowParts, fmt.Sprintf("%-*s", width, truncateString(value, width)))
		}

		rowStr := strings.Join(rowParts, " │ ")

		if isSelected {
			b.WriteString(ui.SelectedStyle.Render(rowStr))
		} else if row.HasError {
			b.WriteString(ui.ErrorStyle.Render(rowStr))
		} else {
			b.WriteString(rowStr)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// calculateColumnWidths determines optimal column widths
func (m *ResultsModel) calculateColumnWidths() map[string]int {
	widths := make(map[string]int)

	// Agent column
	widths["_agent"] = 15

	// Start with header widths
	for _, col := range m.tableColumns {
		widths[col] = len(col)
	}

	// Check data widths (sample first 50 rows)
	sampleSize := min(50, len(m.tableRows))
	for i := 0; i < sampleSize; i++ {
		row := m.tableRows[i]
		if len(row.AgentName) > widths["_agent"] {
			widths["_agent"] = min(len(row.AgentName), 20)
		}
		for col, value := range row.Data {
			if len(value) > widths[col] {
				widths[col] = len(value)
			}
		}
	}

	// Cap column widths and distribute available space
	totalWidth := widths["_agent"] + 3 // Agent + separator
	for _, col := range m.tableColumns {
		totalWidth += widths[col] + 3 // Column + separator
	}

	// If total is too wide, scale down
	availableWidth := m.width - 4
	if totalWidth > availableWidth && len(m.tableColumns) > 0 {
		// Distribute space more evenly
		perColWidth := (availableWidth - widths["_agent"] - 3) / len(m.tableColumns) - 3
		if perColWidth < 8 {
			perColWidth = 8
		}
		for _, col := range m.tableColumns {
			if widths[col] > perColWidth {
				widths[col] = perColWidth
			}
		}
	}

	// Ensure minimum widths
	for col := range widths {
		if widths[col] < 5 {
			widths[col] = 5
		}
		if widths[col] > 40 {
			widths[col] = 40
		}
	}

	return widths
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

// renderDetailView renders the full detail view for a selected row
func (m *ResultsModel) renderDetailView() string {
	var b strings.Builder

	// Title bar
	b.WriteString(ui.TitleStyle.Render(" Detail View "))
	b.WriteString("\n")

	// Divider
	b.WriteString(ui.MutedStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Viewport content
	b.WriteString(m.detailViewport.View())

	// Fill remaining space
	contentLines := strings.Count(m.detailViewport.View(), "\n")
	for i := contentLines; i < m.height-4; i++ {
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(ui.MutedStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// Help text
	scrollInfo := ""
	if m.detailViewport.TotalLineCount() > m.detailViewport.Height {
		scrollInfo = fmt.Sprintf(" (%d/%d) ", m.detailViewport.YOffset+1, m.detailViewport.TotalLineCount()-m.detailViewport.Height+1)
	}
	helpText := ui.HelpKeyStyle.Render("↑↓") + ui.HelpDescStyle.Render(" scroll  ") +
		ui.HelpKeyStyle.Render("q/esc/enter") + ui.HelpDescStyle.Render(" back") +
		ui.MutedStyle.Render(scrollInfo)
	b.WriteString(helpText)

	return b.String()
}
