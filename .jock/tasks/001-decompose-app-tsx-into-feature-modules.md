---
id: 001
title: Decompose App.tsx into feature modules
status: in-progress
priority: 3
labels: [refactor, architecture, P0]
branch: task/001-decompose-app-tsx-into-feature
created: 2026-03-06T12:13:52Z
updated: 2026-03-06T12:15:39Z
---

Break the monolithic App.tsx (~2000+ lines) into focused feature modules. Extract state management into a central store or context providers. Create dedicated route/view components for each major panel (commits, source control, tasks, DSL). Target: App.tsx under 300 lines.
