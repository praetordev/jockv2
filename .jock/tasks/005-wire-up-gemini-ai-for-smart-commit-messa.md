---
id: 005
title: Wire up Gemini AI for smart commit message generation
status: backlog
priority: 2
labels: [ai, feature, P1]
branch: feature/ai-commit-messages
created: 2026-03-06T12:13:52Z
updated: 2026-03-06T12:14:06Z
---

The @google/genai dependency exists but is unused. Implement: (1) Analyze staged diff and generate a conventional-commit message, (2) Add a Generate button next to the commit message input, (3) Settings panel for API key configuration, (4) Fallback gracefully when no key is set. Keep it optional and non-blocking.
