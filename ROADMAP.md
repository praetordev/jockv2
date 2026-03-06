# Jock Roadmap

> Git-First IDE — Feature Implementation Roadmap
> Last updated: 2026-03-05

---

## How to Read This Roadmap

- **Milestones** are grouped into themed releases
- Each feature lists the **layers** requiring work: `backend`, `proto`, `ipc`, `frontend`
- Effort is estimated as **S** (1–3 days), **M** (3–7 days), **L** (1–2 weeks), **XL** (2–4 weeks)
- Dependencies between features are noted where applicable
- Features marked with `[DSL]` already have partial backend support via the DSL executor

---

## v1.0 — "Solid Foundation" (Current → Month 2)

**Theme:** Complete the core git workflow, polish what exists, close gaps from todo.txt.

The goal is to make Jock fully usable as a daily-driver for solo developers working on git repos. No feature should require falling back to the terminal for common operations.

### 1.1 Merge Conflict Resolution UI
> Backend exists. Frontend needs a proper visual experience.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | Already implemented (GetConflictDetails, ResolveConflict) | — |
| proto | Already defined | — |
| ipc | Already wired | — |
| frontend | 3-pane conflict viewer (ours / theirs / result), inline accept/reject buttons, file-by-file navigation | **L** |

- Show conflict markers visually with syntax highlighting
- Allow choosing ours/theirs/both per hunk, not just per file
- Integrate with commit graph — show which branches are conflicting and why
- Wire "Abort Merge" button prominently

### 1.2 Stash Management UI
> Backend fully implemented. Needs a dedicated frontend panel.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Stash list panel, create stash dialog (with message + include-untracked toggle), apply/pop/drop actions, stash diff preview | **M** |

- Show stash entries with message, date, and parent branch
- Quick-apply from context menu on stash items
- Preview stash diff before applying

### 1.3 Cherry-Pick & Revert (Wire to UI)
> Backend exists via DSL executor. Needs proto/IPC/frontend wiring.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| proto | Add `CherryPick` and `Revert` RPC methods | **S** |
| backend | Expose existing DSL functions as gRPC handlers | **S** |
| ipc | Add IPC handlers | **S** |
| frontend | Right-click commit → "Cherry-pick" / "Revert" in commit graph, confirmation dialog with dry-run preview | **M** |

### 1.4 Tag Management
> DSL has basic tag support. Needs full CRUD and UI.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | `CreateTag`, `DeleteTag`, `PushTag`, `ListTags` functions in git.go | **S** |
| proto | Add Tag RPC methods | **S** |
| ipc | Add IPC handlers | **S** |
| frontend | Tag list in sidebar, create tag dialog (lightweight vs annotated), tag badge on commit graph nodes, push tags action | **M** |

### 1.5 Keyboard Shortcut System
> No implementation exists. Critical for target audience.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Shortcut registry (action → keybinding map), default keymap, user override system (persisted to ~/.jock/keybindings.json), shortcut display in menus/tooltips | **L** |

- Default keybindings for all major actions (stage, commit, push, pull, switch branch, etc.)
- Vim-style navigation option in panels (j/k for up/down, Enter to select)
- Shortcut cheatsheet panel (toggleable)
- Settings UI page for rebinding

### 1.6 Strict IPC TypeScript Typing
> Currently `window.electronAPI` typed as `any`.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Generate typed interface from IPC handler registry, replace `any` casts | **M** |

- Auto-generate types from handler definitions or maintain a shared `.d.ts`
- Eliminates a class of runtime bugs

### 1.7 Settings Panel Completion
> Partially built. Needs remaining sections.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Add keyboard shortcuts tab, git defaults tab (default branch, GPG signing, user.name/email), appearance refinements | **S** |

---

## v1.1 — "Power User" (Month 2 → Month 4)

**Theme:** Ship the advanced git features that power users demand. This is what earns "git-first" credibility.

### 1.1.1 Non-Interactive Rebase (Wire to UI)
> Backend exists via DSL executor. Needs proto/IPC/frontend wiring.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| proto | Add `Rebase` and `AbortRebase` RPC methods | **S** |
| backend | Expose existing DSL rebase as gRPC handler, add abort/continue | **S** |
| ipc | Add IPC handlers | **S** |
| frontend | Rebase dialog (select target branch), progress indicator, conflict resolution flow (reuse v1.0 conflict UI), abort/continue/skip buttons | **M** |

### 1.1.2 Interactive Rebase
> Not implemented anywhere. Most complex feature on the roadmap.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | Implement `git rebase -i` via GIT_SEQUENCE_EDITOR override — generate todo list, parse user edits, execute steps | **L** |
| proto | Add `InteractiveRebase` (start, get todo, update todo, execute) RPC methods | **M** |
| ipc | Add IPC handlers | **S** |
| frontend | Draggable commit reorder list, pick/squash/fixup/reword/drop per commit, real-time preview of resulting history, conflict resolution integration | **XL** |

- This is the marquee feature. The UI must be better than any existing tool.
- Show before/after commit graph side-by-side
- Support squash with combined message editing
- Support fixup (auto-discard message)
- Reword inline without full rebase

**Depends on:** v1.0 Merge Conflict Resolution UI (reused for rebase conflicts)

### 1.1.3 Reflog Viewer
> Not implemented. Important safety net for power users.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | `ListReflog` function — parse `git reflog` output | **S** |
| proto | Add `ListReflog` RPC | **S** |
| ipc | Add IPC handler | **S** |
| frontend | Reflog panel showing HEAD movement history, "restore to this point" action (creates branch at ref), search/filter | **M** |

- Position as the "undo history for git" — makes dangerous operations feel safe
- Link reflog entries to commit graph nodes

### 1.1.4 DSL Documentation & Query Explorer
> DSL engine is mature but undiscoverable.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | DSL panel upgrade: syntax reference sidebar, example query library (10–20 prebuilt queries), query history with re-run, save/name queries, result export (JSON/CSV/clipboard) | **L** |

- Example queries: "commits by author this week", "branches with no recent activity", "files changed most in last 100 commits", "blame hotspots"
- Autocomplete already exists — surface it more prominently
- Add inline documentation on hover for DSL keywords

### 1.1.5 Commit Search & Filtering
> ListCommits exists but search is basic.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | Add `--grep`, `--author`, `--since`, `--until`, `--all` flags to ListCommits | **S** |
| proto | Extend `ListCommitsRequest` with filter fields | **S** |
| frontend | Search bar above commit graph with filter chips (author, date range, message text, file path), persist recent searches | **M** |

---

## v1.2 — "Connected" (Month 4 → Month 6)

**Theme:** Integrate with the platforms developers actually use. Git doesn't exist in isolation — PRs, issues, and CI are part of the workflow.

### 1.2.1 GitHub Integration
> Currently relies on external `gh` CLI.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | GitHub REST/GraphQL API client — OAuth device flow auth, PR CRUD, issue listing, CI status polling | **XL** |
| proto | Add GitHub-specific RPCs (ListPRs, CreatePR, GetPRDetails, ListChecks, etc.) | **M** |
| ipc | Add IPC handlers | **S** |
| frontend | PR list panel, create PR dialog (base/head branch, title, body, reviewers, labels), PR detail view with diff, CI status badges on branches, review comments inline | **XL** |

**Sub-milestones:**
1. OAuth device flow authentication (no external `gh` dependency)
2. PR list and detail view (read-only)
3. Create PR from current branch
4. CI check status on branches and PRs
5. Review comments and approval workflow

### 1.2.2 GitLab Integration
> Not started.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | GitLab REST API client — OAuth flow, MR CRUD, pipeline status | **L** |
| proto | Reuse GitHub-like RPCs with provider abstraction | **M** |
| frontend | Same UI as GitHub, provider-agnostic | **M** |

- Abstract the forge provider interface so GitHub and GitLab share UI components
- Detect remote URL to auto-select provider

### 1.2.3 Notifications & Activity Feed
> Not started.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Activity panel: recent pushes from teammates, PR review requests, CI failures, branch updates from remotes | **L** |
| backend | Polling service for remote changes and forge notifications | **M** |

---

## v1.3 — "Extensible" (Month 6 → Month 9)

**Theme:** Open up Jock for community contribution and long-tail use cases.

### 1.3.1 Plugin/Extension System
> Not started. Highest-leverage feature for long-term growth.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | Plugin API definition — lifecycle hooks, git event subscriptions, custom command registration | **XL** |
| frontend | Extension manager UI, extension sidebar panels, command palette integration | **XL** |

**Design principles:**
- Plugins should be able to: add sidebar panels, register commands, subscribe to git events (commit, push, branch switch), add context menu items, contribute DSL functions
- Sandboxed execution (no arbitrary filesystem access without permission)
- Plugin manifest format (name, version, permissions, entry point)
- Distribution: Git repo URL or future marketplace

### 1.3.2 Custom Themes
> Basic theme selection exists in settings.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Theme file format (JSON/CSS variables), theme editor/preview, import/export themes, community theme gallery | **M** |

### 1.3.3 LSP Integration for Vex
> Not started. Bridges the editor capability gap.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend (Vex) | LSP client in Vex — initialize, textDocument/didOpen, completion, hover, diagnostics, go-to-definition | **XL** |

- Start with Go and TypeScript LSP support
- This single feature dramatically increases Vex's viability as a real editor
- Can be shipped incrementally: completion first, then hover, then diagnostics

### 1.3.4 Command Palette
> Not started.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Fuzzy-search command palette (Cmd+Shift+P), registered from all features and plugins, recent commands, keyboard-navigable | **M** |

**Depends on:** Keyboard Shortcut System (v1.0)

---

## v1.4 — "Team Ready" (Month 9 → Month 12)

**Theme:** Features that make Jock valuable for teams, not just individuals. This is also the monetization path.

### 1.4.1 Shared DSL Query Library
> Not started.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | Query storage format, import/export, optional sync to repo (.jock/queries/) | **M** |
| frontend | Shared query browser, fork/edit workflow, categorization | **M** |

- Store queries in `.jock/queries/` in the repo so they're version-controlled and shared via git itself
- Zero additional infrastructure required

### 1.4.2 Branch Policies & Conventions
> Not started.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Branch naming convention enforcement (regex), protected branch warnings, required review reminders, commit message templates | **L** |

- Configuration stored in `.jock/config.json` in repo root
- Warn before pushing to protected branches
- Suggest branch names based on conventions

### 1.4.3 DSL-Powered Dashboards
> Not started. Killer differentiator.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| frontend | Dashboard builder: pin DSL query results as widgets (charts, tables, counters), layout editor, auto-refresh, export/share dashboard configs | **XL** |

- Example dashboards: "Team Activity" (commits per author per week), "Code Hotspots" (most-changed files), "Branch Health" (stale branches, ahead/behind)
- Dashboards stored as JSON in `.jock/dashboards/`

### 1.4.4 Worktree Support
> Not started.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | `ListWorktrees`, `CreateWorktree`, `RemoveWorktree` in git.go | **S** |
| proto | Add Worktree RPCs | **S** |
| frontend | Worktree list panel, create worktree dialog, switch between worktrees (opens new window or tab) | **M** |

### 1.4.5 Bisect Wizard
> Not started.

| Layer | Work Required | Effort |
|-------|--------------|--------|
| backend | `BisectStart`, `BisectGood`, `BisectBad`, `BisectReset` functions | **M** |
| proto | Add Bisect RPCs | **S** |
| frontend | Guided bisect workflow — visual narrowing on commit graph, "good/bad" buttons, automated bisect with test command | **L** |

---

## v2.0 — "The Git-First IDE" (Month 12+)

**Theme:** Ambitious features that establish Jock as a category-defining product.

### 2.0.1 AI-Assisted Git Workflows
- Commit message generation from staged diffs
- PR description generation
- Conflict resolution suggestions
- "Explain this history" — natural language summary of a branch's changes
- DSL query generation from natural language ("show me who changed auth code this month")

### 2.0.2 Git Analytics & Insights
- Code churn visualization over time
- Author contribution heatmaps
- Merge frequency and conflict rate tracking
- Branch lifecycle metrics (time to merge, review cycles)

### 2.0.3 Multi-Repo Workspace
- Open multiple repos in one window
- Cross-repo search and DSL queries
- Monorepo support (sparse checkout, path-scoped views)

### 2.0.4 Collaboration Features
- Real-time branch activity from teammates
- Shared annotations on commits
- Code review inside Jock (not just PR links)

### 2.0.5 Submodule Management
- Visual submodule dependency graph
- Bulk update/init submodules
- Status indicators for submodule state

---

## Milestone Summary

```
v1.0  "Solid Foundation"  ██████████░░░░░░░░░░  Month 0-2
v1.1  "Power User"        ░░░░██████████░░░░░░  Month 2-4
v1.2  "Connected"         ░░░░░░░░██████████░░  Month 4-6
v1.3  "Extensible"        ░░░░░░░░░░░░██████░░  Month 6-9
v1.4  "Team Ready"        ░░░░░░░░░░░░░░████░░  Month 9-12
v2.0  "The Git-First IDE" ░░░░░░░░░░░░░░░░████  Month 12+
```

## Priority Legend

| Priority | Meaning |
|----------|---------|
| v1.0 | Must-have for credible launch. Closing existing gaps. |
| v1.1 | Differentiating features that earn "git-first" positioning. |
| v1.2 | Integration features that fit Jock into real workflows. |
| v1.3 | Platform features that unlock community growth. |
| v1.4 | Team features that unlock monetization. |
| v2.0 | Vision features that define the category. |

---

## Decision Log

| Decision | Rationale | Date |
|----------|-----------|------|
| Ship conflict UI before interactive rebase | Rebase reuses the conflict resolution flow; build the dependency first | 2026-03-05 |
| GitHub before GitLab | Larger market share; validate the integration pattern first | 2026-03-05 |
| Plugin system after core git features | Avoid premature abstraction; understand what plugins need by building features first | 2026-03-05 |
| LSP in Vex over building language features manually | Leverage existing LSP ecosystem instead of reinventing syntax/completion per language | 2026-03-05 |
| Store shared configs in .jock/ repo directory | Zero infrastructure, git-native, works offline — aligns with "git-first" philosophy | 2026-03-05 |
