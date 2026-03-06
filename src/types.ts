export interface Commit {
  hash: string;
  message: string;
  author: string;
  date: string;
  branches?: string[];
  tags?: string[];
  parents?: string[];
  graph?: {
    color: string;
    column: number;
    connections: {
      toColumn: number;
      toRow: number;
      color: string;
    }[];
  };
}

export interface FileChange {
  path: string;
  status: 'added' | 'modified' | 'deleted' | 'renamed' | 'conflicted';
  additions: number;
  deletions: number;
  patch?: string;
}

export interface Branch {
  name: string;
  isCurrent: boolean;
  remote?: string;
  ahead?: number;
  behind?: number;
}

// --- DSL Types ---

export interface DSLAggregateRow {
  group: string;
  value: number;
}

export interface DSLResult {
  resultKind: 'commits' | 'branches' | 'files' | 'blame' | 'stashes' | 'count' | 'aggregate' | 'formatted' | 'action_report' | 'error';
  commits?: DSLCommitResult[];
  branches?: Branch[];
  blameLines?: DSLBlameLineResult[];
  stashes?: DSLStashResult[];
  count?: number;
  actionReport?: DSLActionReport;
  error?: string;
  aggregates?: DSLAggregateRow[];
  aggFunc?: string;
  aggField?: string;
  groupField?: string;
  formattedOutput?: string;
  formatType?: string;
}

export interface DSLCommitResult {
  hash: string;
  message: string;
  author: string;
  date: string;
  branches?: string[];
  tags?: string[];
  additions: number;
  deletions: number;
  parents?: string[];
  graph?: {
    color: string;
    column: number;
    connections: {
      toColumn: number;
      toRow: number;
      color: string;
    }[];
  };
}

export interface DSLActionReport {
  action: string;
  affectedHashes: string[];
  success: boolean;
  dryRun: boolean;
  description: string;
  errors: string[];
}

export interface DSLBlameLineResult {
  hash: string;
  author: string;
  date: string;
  lineNo: number;
  content: string;
}

export interface DSLStashResult {
  index: number;
  message: string;
  branch: string;
  date: string;
}

export interface DSLSuggestion {
  text: string;
  kind: string;
  description: string;
}

// --- Git Operation Result Types ---

export interface MergeResult {
  success: boolean;
  summary: string;
  hasConflicts: boolean;
  conflictFiles: string[];
  error?: string;
}

export interface ConflictDetail {
  path: string;
  oursContent: string;
  theirsContent: string;
  rawContent: string;
  error?: string;
}

export type ConflictStrategy = 'ours' | 'theirs' | 'both';

export interface StashEntry {
  index: number;
  message: string;
  branch: string;
  date: string;
}

export interface TagInfo {
  name: string;
  hash: string;
  date: string;
  message: string;
  isAnnotated: boolean;
}

export interface RebaseTodoEntry {
  action: 'pick' | 'squash' | 'fixup' | 'drop' | 'reword';
  hash: string;
  message: string;
}

export interface ReflogEntry {
  hash: string;
  action: string;
  message: string;
  date: string;
}

export interface CommitFilters {
  authorPattern?: string;
  grepPattern?: string;
  afterDate?: string;
  beforeDate?: string;
  pathPattern?: string;
}

// --- Task Types ---

export interface Task {
  id: string;
  title: string;
  description: string;
  status: 'backlog' | 'in-progress' | 'done';
  labels: string[];
  branch: string;
  commits: string[];
  created: string;
  updated: string;
  priority: number; // 0=none, 1=low, 2=medium, 3=high
}

