# Jock

A Git GUI client built with Electron, React, and a Go backend. Designed for developers who want a powerful, keyboard-driven interface with an integrated query language for exploring repositories.

![CI](https://github.com/praetordev/jockv2/actions/workflows/ci.yml/badge.svg)

## Features

- **Commit graph** -- column-based visualization with branch/tag labels and connections
- **Source control** -- stage, unstage, commit, amend, diff viewer
- **Branch management** -- create, delete, switch (auto-stash), merge, rebase (including interactive)
- **Conflict resolution** -- ours/theirs/both strategies with visual conflict view
- **Stash management** -- create, apply, pop, drop, show
- **Cherry-pick & revert** with conflict handling
- **Tag management** -- lightweight and annotated, push to remote
- **Blame viewer** -- per-line annotation
- **Reflog viewer**
- **Remote management** -- push, pull, add remote, list remote branches
- **GitHub integration** -- create repo, clone, credential-based auth
- **Task board** -- kanban (backlog/in-progress/done), linked to branches, priorities, labels
- **DSL query engine** -- custom query language with autocomplete and syntax highlighting
- **File explorer** -- git ls-files based tree with status indicators
- **Integrated terminal** -- xterm.js with node-pty
- **Vex editor** -- embedded terminal-based code editor
- **MCP server** -- full git operations + DSL + tasks exposed via Model Context Protocol for AI agents
- **Keyboard shortcuts** -- configurable keymap system
- **Theming** -- dark/light

## Architecture

```
+------------------+       gRPC        +------------------+
|   Electron App   | <===============> |   Go Backend     |
|                  |                   |   (jockd)        |
|  React 19        |                   |                  |
|  TypeScript      |                   |  git operations  |
|  Tailwind CSS 4  |                   |  DSL engine      |
|  xterm.js        |                   |  task management |
+------------------+                   +------------------+
        |
        |  node-pty
        v
+------------------+       stdio       +------------------+
|   Terminal / Vex |                   |   jockmcp        |
+------------------+                   |   (MCP server)   |
                                       +------------------+
```

**Frontend**: React 19, TypeScript, Tailwind CSS 4, Vite, packaged with Electron 33

**Backend**: Go 1.25 serving gRPC via `jockd`. Separate binaries for CLI queries (`jockq`) and AI agent integration (`jockmcp`).

**Communication**: Protobuf over gRPC, defined in `proto/jock.proto`

## Getting Started

### Prerequisites

- Node.js 20+
- Go 1.22+
- npm

### Install

```bash
git clone https://github.com/praetordev/jockv2.git
cd jockv2
npm ci
```

### Development

```bash
make dev
```

This builds the Go backend and starts the Vite dev server with Electron.

### Run Tests

```bash
# Frontend
npm test

# Backend
cd backend && go test ./...
```

### Build for Release

```bash
# macOS
make release-mac

# Linux
make release-linux
```

Packaged apps are output to `release/`.

## DSL Query Language

Jock includes a query language for exploring repositories. Access it from the Query tab in the bottom panel.

```
# List recent commits by an author
commits | where author contains "alice" | first 10

# Count commits after a date
commits | where after("2025-01-01") | count

# Aggregate additions by author
commits | stats additions by author

# List merged branches
branches | where merged

# Blame a file
blame "src/App.tsx"

# List tasks
tasks | where status == "in-progress"

# Shell escape
!git shortlog -sn
```

## MCP Server

Jock exposes 31 tools via the Model Context Protocol for use with AI agents (Claude, etc.).

### Setup

Add to your MCP client config:

```json
{
  "mcpServers": {
    "jock": {
      "command": "/path/to/jockmcp",
      "args": []
    }
  }
}
```

The server auto-detects the git repository from the working directory. Tools include full git operations (`jock_git_*`), DSL queries (`jock_dsl_query`), and task management (`jock_task_*`).

### Tool Groups

| Group | Tools | Description |
|-------|-------|-------------|
| Read | 11 | status, log, diff, blame, branches, tags, stashes, reflog, remotes, commit details, show stash |
| Write | 16 | stage, unstage, commit, push, pull, merge, branch create/delete, tag, stash, cherry-pick, revert, rebase, conflict resolution |
| DSL | 1 | Execute any Jock DSL query |
| Tasks | 5 | list, create, update, delete, start |

## Project Structure

```
jockv2/
  backend/           # Go backend
    cmd/
      jockd/         # gRPC server
      jockq/         # CLI query tool
      jockmcp/       # MCP server
    internal/
      dsl/           # DSL parser and evaluator
      git/           # Git operations
      tasks/         # Task management
  electron/          # Electron main process
  proto/             # Protobuf definitions
  src/               # React frontend
    components/      # UI components
    context/         # State management (AppContext)
    hooks/           # React hooks
  .github/workflows/ # CI/CD pipelines
  .jock/tasks/       # Task board data
```

## Make Targets

```bash
make help          # Show all targets
make dev           # Build backend + start dev server
make backend       # Build all Go binaries
make test          # Run frontend tests
make backend-test  # Run Go tests
make lint          # TypeScript type check
make electron      # Full build + package
make release-mac   # Package for macOS
make release-linux # Package for Linux
make bump-patch    # Bump version, tag, ready to push
make clean         # Remove build artifacts
```

## License

Private. All rights reserved.
