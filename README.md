# Binmave CLI

Command-line interface for Binmave endpoint management and security.

## Installation

### From Releases

Download the latest release for your platform from the [Releases](https://github.com/Binmave/binmave-cli/releases) page.

### From Source

```bash
go install github.com/Binmave/binmave-cli/cmd/binmave@latest
```

## Usage

### Authentication

```bash
# Login via browser (OAuth2 PKCE flow)
binmave login

# Check current user
binmave whoami

# Logout
binmave logout
```

### Agents

```bash
# List all agents
binmave agents

# Show agent statistics
binmave agents stats

# JSON output for scripting
binmave agents --json
```

### Scripts

```bash
# List all scripts
binmave scripts

# Show script details
binmave scripts show 42
```

### Executions

```bash
# List recent executions
binmave executions

# Show execution details
binmave executions show abc123

# View agent results
binmave executions results abc123

# Watch execution in real-time
binmave watch abc123
```

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format |
| `--server <url>` | Override the server URL |
| `--help` | Show help |

## Configuration

Configuration is stored in `~/.binmave/`:

- `config.yaml` - Server URL and settings
- `credentials.json` - Authentication tokens (auto-managed)

### Environment Variables

| Variable | Description |
|----------|-------------|
| `BINMAVE_SERVER` | Override server URL |

## Development

### Building

```bash
# Build for current platform
go build -o binmave ./cmd/binmave

# Build for all platforms
make build-all
```

### Running Tests

```bash
go test ./...
```

## License

Proprietary - Binmave
