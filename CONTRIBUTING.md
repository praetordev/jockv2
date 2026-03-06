# Contributing to Jock

## Development Setup

```bash
git clone https://github.com/praetordev/jockv2.git
cd jockv2
npm ci
make dev
```

## Workflow

1. Pick a task from `.jock/tasks/` or create one
2. Create a feature branch from `main`: `git checkout -b task/NNN-description main`
3. Make your changes
4. Run tests: `npm test && cd backend && go test ./...`
5. Commit with a descriptive message
6. Merge into `main` with `--no-ff`

## Code Style

### TypeScript / React

- React 19 with function components and hooks
- TypeScript strict mode
- Tailwind CSS 4 for styling -- use utility classes, avoid inline styles in new code
- Use the existing `window.electronAPI.invoke()` pattern for IPC calls
- Tests use vitest + @testing-library/react

### Go

- Standard Go formatting (`gofmt`)
- Tests alongside source files (`*_test.go`)
- Git operations in `backend/internal/git/`
- DSL engine in `backend/internal/dsl/`

### Commits

- Use conventional prefixes: `feat:`, `fix:`, `refactor:`, `test:`, `ci:`, `chore:`, `docs:`
- Keep commits focused -- one logical change per commit
- Write messages that explain *why*, not *what*

## Architecture Notes

- `App.tsx` is a thin orchestrator -- state lives in `AppContext.tsx`
- Sidebar, MainContent, BottomPanel, and Taskbar are the four layout regions
- The Go backend (`jockd`) handles all git operations via gRPC
- The Electron main process (`electron/main.ts`) bridges IPC to gRPC
- Some handlers use direct `execSync` for operations that need to be synchronous (e.g., branch switching) -- don't mix with gRPC in the same handler

## Testing

- Frontend: `npm test` (vitest)
- Backend: `cd backend && go test ./...`
- Test files live next to the code they test
- Mock `window.electronAPI` using the setup in `src/test-setup.ts`

## Releasing

1. `make bump-patch` (or `bump-minor`) -- bumps version, creates tag
2. `git push --tags` -- triggers the release workflow
3. Review the draft release on GitHub, then publish
