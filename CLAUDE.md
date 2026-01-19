# CLAUDE.md - Binmave CLI

## Overview

**Binmave CLI** (`binmave`) is a command-line interface for the Binmave endpoint management platform. It provides terminal-based access to the same functionality available in the web console, with interactive TUI (Text User Interface) views for exploring execution results.

## Design Philosophy

### Web Frontend Parity

The CLI should match the web frontend's capabilities and visualizations. When implementing new features, reference the React frontend at `/home/claude/projects/binmave/Frontend` for:

- **Visualization patterns** - How data is displayed (tables, trees, aggregations)
- **Data detection** - When to show tree vs flat views (see `VisualizationToggle.tsx`)
- **Anomaly detection** - Highlighting items found on few agents (see `AggregatedTreeView.tsx`)
- **Comparison logic** - Baseline diff calculations (see `compare.tsx`)

### Key Frontend Files to Reference

| File | Purpose |
|------|---------|
| `src/app/modules/results/tree-view/tree-view.component.ts` | Per-agent tree display |
| `src/app/modules/results/aggregated-tree-view/` | Aggregated view with counts |
| `src/app/modules/results/compare/` | Baseline comparison |
| `src/app/modules/results/visualization-toggle/` | View mode switching logic |

## Architecture

```
binmave-cli/
├── cmd/binmave/main.go           # Entry point
├── internal/
│   ├── api/
│   │   ├── client.go             # HTTP client with auth
│   │   └── types.go              # API data types
│   ├── auth/
│   │   ├── oauth.go              # OAuth2 PKCE flow
│   │   └── token.go              # Token storage
│   ├── commands/
│   │   ├── root.go               # CLI root command
│   │   ├── login.go              # Authentication
│   │   ├── agents.go             # Agent listing
│   │   ├── scripts.go            # Script management
│   │   ├── executions.go         # Execution listing
│   │   ├── results.go            # Results TUI
│   │   ├── compare.go            # Baseline comparison
│   │   └── watch.go              # Real-time execution watch
│   ├── config/
│   │   └── config.go             # Configuration management
│   └── ui/
│       ├── styles.go             # Lipgloss styles/theme
│       ├── components/
│       │   ├── tabs.go           # Tab bar, view mode selector
│       │   ├── tree.go           # Tree renderer
│       │   └── help.go           # Help bar, progress indicator
│       └── models/
│           ├── results.go        # Main results TUI model
│           └── compare.go        # Compare TUI model
└── go.mod
```

## Commands

| Command | Description |
|---------|-------------|
| `binmave login` | OAuth2 PKCE authentication |
| `binmave logout` | Clear stored credentials |
| `binmave whoami` | Show current user |
| `binmave agents` | List agents |
| `binmave scripts` | List scripts |
| `binmave scripts run <id>` | Execute a script |
| `binmave executions` | List executions |
| `binmave watch <id>` | Watch execution in real-time |
| `binmave results <id>` | Interactive results viewer |
| `binmave compare <id> -b <baseline>` | Compare executions |

## Results TUI

The `binmave results` command opens an interactive viewer with three view modes:

### Table View (Key: 1)
- Flat grid showing parsed JSON columns
- Auto-detects column names from data
- Press Enter to see full details of selected row

### Tree View (Key: 2)
- Per-agent hierarchical display
- Each agent is a collapsible section
- Navigate with ↑↓, expand/collapse with ←→

### Aggregated View (Key: 3)
- Merged tree across all agents
- Shows counts like `[45/47]` for prevalence
- Highlights anomalies (items on <10% of agents)
- Press `a` to toggle "anomalies only" filter

### View Mode Detection

Tree/Aggregated views are only enabled when data is hierarchical. Detection logic:
1. Check for nested objects/arrays in JSON
2. Check for self-referential fields (e.g., ProcessId/ParentProcessId)
3. Use value analysis to find id→parent relationships

Reference: `hasSelfReferentialFields()` in `results.go` mirrors the web frontend's approach.

## Keyboard Shortcuts

### All Views
| Key | Action |
|-----|--------|
| `Tab` | Switch Results/Errors tabs |
| `1/2/3` | Switch view modes |
| `q` | Quit |

### Table View
| Key | Action |
|-----|--------|
| `↑↓` | Navigate rows |
| `Enter` | Show full details |

### Tree/Aggregated Views
| Key | Action |
|-----|--------|
| `↑↓` | Navigate nodes |
| `←→` | Collapse/Expand |
| `e` | Expand all |
| `c` | Collapse all |
| `a` | Toggle anomalies (Aggregated only) |

## Dependencies

- **Cobra** - CLI framework
- **Viper** - Configuration
- **Bubbletea** - TUI framework (Elm architecture)
- **Lipgloss** - Terminal styling
- **Bubbles** - TUI components (spinner, viewport)

## Authentication

Uses OAuth2 PKCE flow:
1. CLI generates code verifier/challenge
2. Opens browser to IdentityServer
3. Receives callback on `localhost:8765`
4. Exchanges code for tokens
5. Stores tokens in `~/.binmave/credentials.json`

## Backend API

Connects to the same backend as the web frontend:
- Production: `https://dib3oav9kh29t.cloudfront.net`
- Uses Bearer token authentication
- Auto-refreshes expired tokens

## Building

```bash
# Build for current platform
go build -o binmave ./cmd/binmave

# Cross-compile
GOOS=darwin GOARCH=arm64 go build -o binmave-darwin-arm64 ./cmd/binmave
GOOS=linux GOARCH=amd64 go build -o binmave-linux-amd64 ./cmd/binmave
```

## CI/CD

GitHub Actions workflow (`.github/workflows/build.yml`):
- Builds for linux/darwin/windows (amd64/arm64)
- Creates GitHub release on version tags
- Publishes binaries as release assets

## Future Enhancements

Potential features to maintain web parity:

| Feature | Web Reference |
|---------|---------------|
| Search/filter in trees | Tree view search box |
| Export to CSV/JSON | Export button in results |
| Agent filtering | Filter dropdown in results |
| Script input prompts | Script execution modal |
| Job orchestration | Jobs module |

## Version History

- **v0.1.0** - Initial release with basic commands
- **v0.2.0** - Added `scripts run` and `watch` commands
- **v0.3.x** - Interactive TUI results viewer with Table/Tree/Aggregated views

---

*Last updated: 2026-01-19*
