import type { Commit, FileChange, Branch, MergeResult, ConflictDetail, StashEntry, TagInfo, DSLResult, DSLSuggestion, RebaseTodoEntry, ReflogEntry, Task } from './types';
import type { JockSettings } from './settingsTypes';

// --- IPC Channel Type Map ---

interface IPCChannelMap {
  // Repo management
  'git:open-repo': { args: []; result: string | null };
  'git:create-repo': { args: []; result: string | null };
  'git:clone-repo': { args: [url: string]; result: string | null };
  'git:get-repo-path': { args: []; result: string | null };
  'git:get-repo-history': { args: []; result: string[] };
  'git:switch-repo': { args: [repoPath: string]; result: string | null };
  'git:has-remote': { args: []; result: boolean };
  'git:get-user-info': { args: []; result: { gitUser?: string; gitEmail?: string; ghUser?: string } };
  'git:create-github-repo': { args: [repoName: string, isPrivate: boolean]; result: { success: boolean; error?: string } };
  'git:add-remote': { args: [url: string]; result: { success: boolean; error?: string } };

  // Commits & branches
  'git:list-commits': { args: [opts?: { limit?: number; skip?: number; branch?: string; authorPattern?: string; grepPattern?: string; afterDate?: string; beforeDate?: string; pathPattern?: string }]; result: Commit[] };
  'git:list-branches': { args: []; result: Branch[] };
  'git:get-commit-details': { args: [hash: string]; result: { commit: Commit | null; fileChanges: FileChange[] } };
  'git:get-file-diff': { args: [hash: string, filePath: string]; result: string };
  'git:get-status': { args: []; result: { staged: FileChange[]; unstaged: FileChange[]; untracked: string[]; unmerged: FileChange[] } };
  'git:get-working-diff': { args: [filePath: string, staged: boolean]; result: { patch: string; error?: string } };

  // Push / Pull / Remotes
  'git:pull': { args: [remote?: string, branch?: string]; result: { summary: string; updated: number; error?: string } };
  'git:push': { args: [remote?: string, branch?: string, force?: boolean, setUpstream?: boolean]; result: { success: boolean; summary: string; error?: string } };
  'git:list-remotes': { args: []; result: { remotes: { name: string; url: string }[] } };
  'git:list-remote-branches': { args: [remote: string]; result: { branches: string[] } };

  // Stage / Unstage / Commit
  'git:stage-hunk': { args: [patchText: string, reverse?: boolean]; result: { success: boolean; error?: string } };
  'git:stage-files': { args: [paths?: string[]]; result: { success: boolean; error?: string } };
  'git:unstage-files': { args: [paths?: string[]]; result: { success: boolean; error?: string } };
  'git:create-commit': { args: [message: string, amend?: boolean]; result: { hash: string; error?: string } };

  // Merge & conflicts
  'git:merge': { args: [branch: string, noFf?: boolean]; result: MergeResult };
  'git:get-conflict-details': { args: [filePath: string]; result: ConflictDetail };
  'git:resolve-conflict': { args: [filePath: string, strategy: string]; result: { success: boolean; error?: string } };
  'git:abort-merge': { args: []; result: { success: boolean; error?: string } };
  'git:is-merging': { args: []; result: { isMerging: boolean } };

  // Cherry-pick & revert
  'git:cherry-pick': { args: [commitHash: string]; result: MergeResult };
  'git:revert': { args: [commitHash: string]; result: MergeResult };

  // Rebase
  'git:rebase': { args: [ontoBranch: string]; result: MergeResult };
  'git:abort-rebase': { args: []; result: { success: boolean; error?: string } };
  'git:continue-rebase': { args: []; result: MergeResult };
  'git:is-rebasing': { args: []; result: { isRebasing: boolean } };
  'git:interactive-rebase': { args: [baseCommit: string, entries: RebaseTodoEntry[]]; result: MergeResult };
  'git:get-rebase-todo': { args: []; result: { entries: RebaseTodoEntry[] } };

  // Reflog
  'git:list-reflog': { args: [limit?: number]; result: { entries: ReflogEntry[] } };

  // Tags
  'git:list-tags': { args: []; result: { tags: TagInfo[] } };
  'git:create-tag': { args: [tagName: string, commitHash?: string, message?: string]; result: { success: boolean; error?: string } };
  'git:delete-tag': { args: [tagName: string]; result: { success: boolean; error?: string } };
  'git:push-tag': { args: [tagName: string, remote?: string]; result: { success: boolean; error?: string } };

  // Branch management
  'git:create-branch': { args: [name: string, checkout?: boolean, startPoint?: string]; result: { success: boolean; name?: string; error?: string } };
  'git:delete-branch': { args: [name: string, force?: boolean]; result: { success: boolean; error?: string } };
  'git:stash-and-switch': { args: [targetBranch: string]; result: { success: boolean; error?: string; warning?: string } };

  // Blame
  'git:blame': { args: [filePath: string]; result: { lines: { hash: string; author: string; date: string; lineNo: number; content: string }[] } };

  // Stash
  'git:list-stashes': { args: []; result: { stashes: StashEntry[] } };
  'git:create-stash': { args: [message?: string, includeUntracked?: boolean]; result: { success: boolean; error?: string } };
  'git:apply-stash': { args: [index: number]; result: { success: boolean; summary?: string; error?: string } };
  'git:pop-stash': { args: [index: number]; result: { success: boolean; summary?: string; error?: string } };
  'git:drop-stash': { args: [index: number]; result: { success: boolean; error?: string } };
  'git:show-stash': { args: [index: number]; result: { patch: string; error?: string } };

  // Git config
  'git:get-config': { args: []; result: { userName: string; userEmail: string; gpgSign: boolean } };

  // Files
  'files:list-tree': { args: []; result: { repoPath: string | null; files: string[] } };
  'files:git-status-map': { args: []; result: Record<string, string> };

  // Shell
  'shell:exec': { args: [command: string]; result: { stdout: string; stderr: string; code: number } };

  // DSL
  'dsl:execute': { args: [query: string, dryRun?: boolean]; result: DSLResult };
  'dsl:autocomplete': { args: [partialQuery: string, cursorPosition: number]; result: { suggestions: DSLSuggestion[] } };
  'dsl:get-history': { args: []; result: string[] };
  'dsl:add-history': { args: [query: string]; result: string[] };

  // Tasks
  'tasks:list': { args: [statusFilter?: string]; result: { tasks: Task[] } };
  'tasks:create': { args: [title: string, description?: string, labels?: string[], priority?: number]; result: { task: Task | null; error?: string } };
  'tasks:update': { args: [id: string, fields: { title?: string; description?: string; status?: string; labels?: string[]; branch?: string; priority?: number }]; result: { task: Task | null; error?: string } };
  'tasks:delete': { args: [id: string]; result: { success: boolean; error?: string } };
  'tasks:start': { args: [id: string, createBranch?: boolean]; result: { task: Task | null; branchName?: string; error?: string } };

  // Tabs
  'tabs:get-state': { args: []; result: { openTabs: string[]; activeIndex: number } };
  'tabs:open': { args: [repoPath: string]; result: { openTabs: string[]; activeIndex: number } };
  'tabs:close': { args: [index: number]; result: { openTabs: string[]; activeIndex: number; repoPath: string | null } };
  'tabs:switch': { args: [index: number]; result: { openTabs: string[]; activeIndex: number; repoPath: string | null } };
  'tabs:reorder': { args: [fromIndex: number, toIndex: number]; result: { openTabs: string[]; activeIndex: number } };

  // Settings
  'settings:get': { args: []; result: JockSettings };
  'settings:set': { args: [newSettings: JockSettings]; result: JockSettings };

  // Auto-update
  'updater:check': { args: []; result: { success: boolean; version?: string; error?: string } };
  'updater:download': { args: []; result: { success: boolean; error?: string } };
  'updater:install': { args: []; result: void };
}

// --- Event Channel Types ---

interface IPCEventMap {
  'repo:file-changed': () => void;
  'git:branch-changed': (branch: string | null) => void;
  'tabs:changed': (state: { openTabs: string[]; activeIndex: number }) => void;
  'settings:changed': (settings: JockSettings) => void;
  'menu:open-settings': () => void;
}

// --- Typed ElectronAPI ---

export interface ElectronAPI {
  platform: string;
  send: (channel: string, ...args: unknown[]) => void;
  invoke<K extends keyof IPCChannelMap>(
    channel: K,
    ...args: IPCChannelMap[K]['args']
  ): Promise<IPCChannelMap[K]['result']>;
  on<K extends keyof IPCEventMap>(
    channel: K,
    listener: IPCEventMap[K]
  ): () => void;
  on(channel: string, listener: (...args: any[]) => void): () => void;
}

declare global {
  interface Window {
    electronAPI: ElectronAPI;
  }
}
