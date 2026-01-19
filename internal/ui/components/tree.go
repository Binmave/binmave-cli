package components

import (
	"fmt"
	"strings"

	"github.com/Binmave/binmave-cli/internal/ui"
)

// TreeNode represents a node in a tree structure
type TreeNode struct {
	ID       string
	Label    string
	Data     map[string]interface{}
	Children []*TreeNode
	Expanded bool
	Depth    int

	// For aggregated view
	Count      int      // Number of agents with this node
	TotalCount int      // Total number of agents
	AgentNames []string // List of agent names
	IsAnomaly  bool     // True if this node appears on few agents
}

// AgentTree represents a tree of results for a single agent
type AgentTree struct {
	AgentID   string
	AgentName string
	Roots     []*TreeNode
	NodeCount int
	Expanded  bool
}

// TreeView manages the display of tree data
type TreeView struct {
	agents         []*AgentTree
	flatNodes      []*flatNode // Flattened for navigation
	selectedIdx    int
	viewportStart  int
	viewportHeight int
	width          int
}

// flatNode represents a node in the flattened tree for navigation
type flatNode struct {
	node       *TreeNode
	agent      *AgentTree
	isAgent    bool // true if this represents an agent header
	indent     int
	isLastNode bool
	parentPath []bool // track which parents are "last" nodes
}

// NewTreeView creates a new tree view
func NewTreeView() *TreeView {
	return &TreeView{
		agents:         make([]*AgentTree, 0),
		flatNodes:      make([]*flatNode, 0),
		selectedIdx:    0,
		viewportStart:  0,
		viewportHeight: 20,
		width:          80,
	}
}

// SetAgents sets the agent trees and rebuilds the flat list
func (t *TreeView) SetAgents(agents []*AgentTree) {
	t.agents = agents
	t.rebuildFlatList()
}

// SetViewportHeight sets the number of visible lines
func (t *TreeView) SetViewportHeight(height int) {
	t.viewportHeight = height
	t.ensureSelectedVisible()
}

// SetWidth sets the available width
func (t *TreeView) SetWidth(width int) {
	t.width = width
}

// MoveUp moves selection up
func (t *TreeView) MoveUp() {
	if t.selectedIdx > 0 {
		t.selectedIdx--
		t.ensureSelectedVisible()
	}
}

// MoveDown moves selection down
func (t *TreeView) MoveDown() {
	if t.selectedIdx < len(t.flatNodes)-1 {
		t.selectedIdx++
		t.ensureSelectedVisible()
	}
}

// Toggle expands/collapses the selected node
func (t *TreeView) Toggle() {
	if t.selectedIdx >= len(t.flatNodes) {
		return
	}

	fn := t.flatNodes[t.selectedIdx]
	if fn.isAgent {
		fn.agent.Expanded = !fn.agent.Expanded
	} else if fn.node != nil && len(fn.node.Children) > 0 {
		fn.node.Expanded = !fn.node.Expanded
	}
	t.rebuildFlatList()
}

// Expand expands the currently selected node
func (t *TreeView) Expand() {
	if t.selectedIdx >= len(t.flatNodes) {
		return
	}

	fn := t.flatNodes[t.selectedIdx]
	if fn.isAgent && !fn.agent.Expanded {
		fn.agent.Expanded = true
		t.rebuildFlatList()
	} else if fn.node != nil && len(fn.node.Children) > 0 && !fn.node.Expanded {
		fn.node.Expanded = true
		t.rebuildFlatList()
	}
}

// Collapse collapses the currently selected node
func (t *TreeView) Collapse() {
	if t.selectedIdx >= len(t.flatNodes) {
		return
	}

	fn := t.flatNodes[t.selectedIdx]
	if fn.isAgent && fn.agent.Expanded {
		fn.agent.Expanded = false
		t.rebuildFlatList()
	} else if fn.node != nil && fn.node.Expanded {
		fn.node.Expanded = false
		t.rebuildFlatList()
	}
}

// ExpandAll expands all nodes
func (t *TreeView) ExpandAll() {
	for _, agent := range t.agents {
		agent.Expanded = true
		expandAllNodes(agent.Roots)
	}
	t.rebuildFlatList()
}

// CollapseAll collapses all nodes
func (t *TreeView) CollapseAll() {
	for _, agent := range t.agents {
		agent.Expanded = false
		collapseAllNodes(agent.Roots)
	}
	t.rebuildFlatList()
}

func expandAllNodes(nodes []*TreeNode) {
	for _, n := range nodes {
		n.Expanded = true
		expandAllNodes(n.Children)
	}
}

func collapseAllNodes(nodes []*TreeNode) {
	for _, n := range nodes {
		n.Expanded = false
		collapseAllNodes(n.Children)
	}
}

// rebuildFlatList rebuilds the flattened node list for navigation
func (t *TreeView) rebuildFlatList() {
	t.flatNodes = make([]*flatNode, 0)

	for _, agent := range t.agents {
		// Add agent header
		t.flatNodes = append(t.flatNodes, &flatNode{
			agent:   agent,
			isAgent: true,
			indent:  0,
		})

		// Add child nodes if expanded
		if agent.Expanded {
			for i, root := range agent.Roots {
				isLast := i == len(agent.Roots)-1
				t.flattenNode(root, agent, 1, isLast, nil)
			}
		}
	}

	// Ensure selection is valid
	if t.selectedIdx >= len(t.flatNodes) {
		t.selectedIdx = len(t.flatNodes) - 1
	}
	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}
}

func (t *TreeView) flattenNode(node *TreeNode, agent *AgentTree, indent int, isLast bool, parentPath []bool) {
	path := append(parentPath, isLast)

	t.flatNodes = append(t.flatNodes, &flatNode{
		node:       node,
		agent:      agent,
		isAgent:    false,
		indent:     indent,
		isLastNode: isLast,
		parentPath: path,
	})

	if node.Expanded {
		for i, child := range node.Children {
			childIsLast := i == len(node.Children)-1
			t.flattenNode(child, agent, indent+1, childIsLast, path)
		}
	}
}

// ensureSelectedVisible scrolls to keep selection visible
func (t *TreeView) ensureSelectedVisible() {
	if t.selectedIdx < t.viewportStart {
		t.viewportStart = t.selectedIdx
	}
	if t.selectedIdx >= t.viewportStart+t.viewportHeight {
		t.viewportStart = t.selectedIdx - t.viewportHeight + 1
	}
}

// Render returns the rendered tree view
func (t *TreeView) Render() string {
	if len(t.flatNodes) == 0 {
		return ui.MutedStyle.Render("No data to display")
	}

	var lines []string
	endIdx := t.viewportStart + t.viewportHeight
	if endIdx > len(t.flatNodes) {
		endIdx = len(t.flatNodes)
	}

	for i := t.viewportStart; i < endIdx; i++ {
		fn := t.flatNodes[i]
		isSelected := i == t.selectedIdx
		line := t.renderNode(fn, isSelected)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderNode renders a single node line
func (t *TreeView) renderNode(fn *flatNode, isSelected bool) string {
	var line string

	if fn.isAgent {
		// Render agent header
		expandChar := ui.TreeCollapsed
		if fn.agent.Expanded {
			expandChar = ui.TreeExpanded
		}

		agentLine := fmt.Sprintf("%s %s (%d items)",
			expandChar,
			fn.agent.AgentName,
			fn.agent.NodeCount,
		)

		if isSelected {
			line = ui.SelectedStyle.Render(agentLine)
		} else {
			line = ui.TreeExpandedStyle.Render(expandChar) + " " +
				ui.HeaderStyle.Render(fn.agent.AgentName) + " " +
				ui.MutedStyle.Render(fmt.Sprintf("(%d items)", fn.agent.NodeCount))
		}
	} else {
		// Render tree node
		prefix := t.buildPrefix(fn)

		expandChar := ""
		if len(fn.node.Children) > 0 {
			if fn.node.Expanded {
				expandChar = ui.TreeExpanded + " "
			} else {
				expandChar = ui.TreeCollapsed + " "
			}
		}

		nodeLabel := fn.node.Label

		// For aggregated view, add count badge
		var countBadge string
		if fn.node.TotalCount > 0 {
			countBadge = fmt.Sprintf(" [%d/%d]", fn.node.Count, fn.node.TotalCount)
		}

		// Truncate if too long
		maxLabelLen := t.width - len(prefix) - len(expandChar) - len(countBadge) - 4
		if maxLabelLen < 10 {
			maxLabelLen = 10
		}
		if len(nodeLabel) > maxLabelLen {
			nodeLabel = nodeLabel[:maxLabelLen-3] + "..."
		}

		if isSelected {
			fullLine := prefix + expandChar + nodeLabel + countBadge
			line = ui.SelectedStyle.Render(fullLine)
		} else {
			styledPrefix := ui.TreeBranchStyle.Render(prefix)
			styledExpand := ""
			if expandChar != "" {
				if fn.node.Expanded {
					styledExpand = ui.TreeExpandedStyle.Render(ui.TreeExpanded) + " "
				} else {
					styledExpand = ui.TreeCollapsedStyle.Render(ui.TreeCollapsed) + " "
				}
			}

			styledLabel := ui.TreeNodeStyle.Render(nodeLabel)

			styledCount := ""
			if fn.node.TotalCount > 0 {
				if fn.node.IsAnomaly {
					styledCount = ui.AnomalyBadgeStyle.Render(countBadge + " " + "⚠")
				} else {
					styledCount = ui.CountBadgeStyle.Render(countBadge)
				}
			}

			line = styledPrefix + styledExpand + styledLabel + styledCount
		}
	}

	return line
}

// buildPrefix builds the tree branch characters for a node
func (t *TreeView) buildPrefix(fn *flatNode) string {
	if fn.indent == 0 {
		return ""
	}

	var prefix strings.Builder

	// Add vertical lines for parent levels
	for i := 0; i < len(fn.parentPath)-1; i++ {
		if fn.parentPath[i] {
			prefix.WriteString("    ") // Parent was last, no line
		} else {
			prefix.WriteString(ui.TreeVertical + "   ")
		}
	}

	// Add branch character for this level
	if fn.isLastNode {
		prefix.WriteString(ui.TreeCorner + ui.TreeHorizontal + " ")
	} else {
		prefix.WriteString(ui.TreeTee + ui.TreeHorizontal + " ")
	}

	return prefix.String()
}

// GetSelectedNode returns the currently selected node (or nil if agent header selected)
func (t *TreeView) GetSelectedNode() *TreeNode {
	if t.selectedIdx >= len(t.flatNodes) {
		return nil
	}
	fn := t.flatNodes[t.selectedIdx]
	if fn.isAgent {
		return nil
	}
	return fn.node
}

// GetSelectedAgent returns the agent for the currently selected item
func (t *TreeView) GetSelectedAgent() *AgentTree {
	if t.selectedIdx >= len(t.flatNodes) {
		return nil
	}
	return t.flatNodes[t.selectedIdx].agent
}
