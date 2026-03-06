---
id: 003
title: Increase test coverage to 60%+ across frontend and backend
status: in-progress
priority: 3
labels: [testing, quality, P0]
branch: feature/test-coverage
created: 2026-03-06T12:13:52Z
updated: 2026-03-06T12:14:06Z
---

Currently only 3 test files exist. Add unit tests for: all React hooks (useGitData, useTaskData, useTerminal, etc.), DSL parser/evaluator (backend), git operations layer, TaskBoard interactions, CommitGraph rendering edge cases. Set up coverage reporting in CI.
