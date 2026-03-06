import React, { createContext, useContext, useState, useEffect, useMemo, useRef, useCallback } from 'react';
import type { Commit, FileChange, CommitFilters } from '../types';
import { useCommits, useBranches, useCommitDetails, useOpenRepo, useSourceControl, usePush, useBranchManagement, useBlame, useMergeConflicts, useStashes, useRemotes, useCherryPickRevert, useTags, useRebase, useReflog } from '../hooks/useGitData';
import { useTasks } from '../hooks/useTaskData';
import { useEditor } from '../hooks/useEditor';
import { useFileTree } from '../hooks/useFileTree';
import { useTerminal } from '../hooks/useTerminal';
import { useSettings } from '../hooks/useSettings';
import { useKeyboardShortcuts } from '../hooks/useKeyboardShortcuts';
import { isElectron } from '../lib/electron';
import { applyTheme } from '../themes';
import { buildKeymap } from '../lib/defaultKeymap';

export type MainView = 'graph' | 'editor' | 'changes' | 'settings' | 'interactive-rebase' | 'tasks';
export type BottomMode = 'terminal' | 'query' | 'docs';

interface AppContextValue {
  // Repo
  repoPath: string | null;
  repoHistory: string[];
  hasRemote: boolean;
  openRepo: () => Promise<string | null>;
  createRepo: () => Promise<string | null>;
  cloneRepo: (url: string) => Promise<string | null>;
  switchRepo: (path: string) => void;
  checkRemote: () => void;

  // Commits
  commits: Commit[];
  filteredCommits: Commit[];
  selectedCommit: Commit | null;
  setSelectedCommit: (c: Commit | null) => void;
  refreshCommits: (filters?: CommitFilters) => void;
  commitFilters: CommitFilters;
  commitSearch: string;
  setCommitSearch: (s: string) => void;
  graphWidth: number;
  animatedCommitHashes: Set<string>;
  triggerGraphRefresh: () => void;

  // Branches
  branches: ReturnType<typeof useBranches>['branches'];
  refreshBranches: () => void;
  mainBranchWarning: boolean;
  currentBranchName: string;

  // Commit details
  fileChanges: FileChange[];
  selectedFile: FileChange | null;
  setSelectedFile: (f: FileChange | null) => void;

  // Source control
  sourceControl: ReturnType<typeof useSourceControl>;
  amendMode: boolean;
  setAmendMode: (v: boolean) => void;

  // Push
  pushing: boolean;
  pushResult: { success: boolean; error?: string } | null;
  doPush: (remote?: string, branch?: string, force?: boolean, setUpstream?: boolean) => void;

  // Branch management
  creatingBranch: boolean;
  doCreateBranch: (name: string, checkout?: boolean, startPoint?: string) => Promise<any>;
  doDeleteBranch: (name: string, force?: boolean) => Promise<any>;

  // Blame
  blameLines: ReturnType<typeof useBlame>['blameLines'];
  blameFile: string | null;
  blameLoading: boolean;
  fetchBlame: (path: string) => void;
  clearBlame: () => void;

  // Merge conflicts
  mergeConflicts: ReturnType<typeof useMergeConflicts>;

  // Stashes
  stashes: ReturnType<typeof useStashes>;

  // Remotes
  remotes: ReturnType<typeof useRemotes>;

  // Cherry-pick / revert
  cherryPickRevert: ReturnType<typeof useCherryPickRevert>;

  // Tags
  tags: ReturnType<typeof useTags>;

  // Rebase
  rebase: ReturnType<typeof useRebase>;

  // Reflog
  reflog: ReturnType<typeof useReflog>;

  // Tasks
  taskData: ReturnType<typeof useTasks>;

  // Editor
  editorTabs: ReturnType<typeof useEditor>['tabs'];
  activeEditorTabId: string | null;
  setActiveEditorTabId: (id: string) => void;
  openFile: (path: string) => void;
  closeEditorTab: (id: string) => void;

  // File tree
  fileTree: ReturnType<typeof useFileTree>['tree'];
  refreshFileTree: () => void;

  // Terminal
  terminalState: ReturnType<typeof useTerminal>;

  // Views
  mainView: MainView;
  setMainView: (v: MainView) => void;
  bottomMode: BottomMode;
  setBottomMode: (m: BottomMode) => void;

  // UI state
  isSidebarOpen: boolean;
  setIsSidebarOpen: (v: boolean | ((prev: boolean) => boolean)) => void;
  initialQuery: string | undefined;
  setInitialQuery: (q: string | undefined) => void;
  dslHighlightHashes: Set<string> | null;
  setDslHighlightHashes: (h: Set<string> | null) => void;

  // Modals
  showCreateBranch: boolean;
  setShowCreateBranch: (v: boolean) => void;
  showMergeDialog: boolean;
  setShowMergeDialog: (v: boolean) => void;
  mergeBranch: string;
  setMergeBranch: (v: string) => void;
  showRemoteSetup: boolean;
  setShowRemoteSetup: (v: boolean) => void;
  showCreateTag: boolean;
  setShowCreateTag: (v: boolean) => void;
  createTagCommitHash: string;
  setCreateTagCommitHash: (v: string) => void;
  interactiveRebaseBase: Commit | null;
  setInteractiveRebaseBase: (c: Commit | null) => void;

  // Stash UI
  selectedStashDiff: string | null;
  setSelectedStashDiff: (v: string | null) => void;
  showStashInput: boolean;
  setShowStashInput: (v: boolean) => void;
  stashMessageInput: string;
  setStashMessageInput: (v: string) => void;

  // Settings
  settings: ReturnType<typeof useSettings>['settings'];
  updateSetting: ReturnType<typeof useSettings>['updateSetting'];
  updateKeybinding: ReturnType<typeof useSettings>['updateKeybinding'];

  // Backend error
  backendError: string | null;

  // Refs
  dslInputRef: React.RefObject<HTMLInputElement | null>;
  commitMsgRef: React.RefObject<HTMLTextAreaElement | null>;

  // Refresh all
  refreshAll: () => void;
}

const AppContext = createContext<AppContextValue | null>(null);

export function useAppContext() {
  const ctx = useContext(AppContext);
  if (!ctx) throw new Error('useAppContext must be used within AppProvider');
  return ctx;
}

export function AppProvider({ children }: { children: React.ReactNode }) {
  const { repoPath, repoHistory, hasRemote, openRepo, createRepo, cloneRepo, switchRepo, checkRemote } = useOpenRepo();
  const { settings, updateSetting, updateKeybinding } = useSettings();

  // Apply theme
  useEffect(() => {
    applyTheme(settings.appearance?.theme ?? 'dark');
  }, [settings.appearance?.theme]);

  const { commits, refresh: refreshCommits, filters: commitFilters } = useCommits(undefined, settings.general.commitListLimit);
  const { branches, refresh: refreshBranches, mainBranchWarning } = useBranches();
  const [selectedCommit, setSelectedCommit] = useState<Commit | null>(null);
  const { fileChanges } = useCommitDetails(selectedCommit?.hash ?? null);
  const [selectedFile, setSelectedFile] = useState<FileChange | null>(null);
  const [isSidebarOpen, setIsSidebarOpen] = useState(true);
  const [amendMode, setAmendMode] = useState(false);
  const [mainView, setMainView] = useState<MainView>('graph');
  const [showRemoteSetup, setShowRemoteSetup] = useState(false);
  const { tabs: editorTabs, activeTabId: activeEditorTabId, setActiveTabId: setActiveEditorTabId, openFile, closeTab: closeEditorTab } = useEditor();
  const { tree: fileTree, refresh: refreshFileTree } = useFileTree(repoPath);
  const sourceControl = useSourceControl();
  const { pushing, pushResult, doPush } = usePush();
  const { creating: creatingBranch, doCreateBranch, doDeleteBranch } = useBranchManagement();
  const [showCreateBranch, setShowCreateBranch] = useState(false);
  const [showMergeDialog, setShowMergeDialog] = useState(false);
  const [mergeBranch, setMergeBranch] = useState('');
  const { blameLines, blameFile, loading: blameLoading, fetchBlame, clearBlame } = useBlame();
  const mergeConflicts = useMergeConflicts(sourceControl.refresh);
  const stashes = useStashes();
  const remotes = useRemotes();
  const cherryPickRevert = useCherryPickRevert();
  const tags = useTags();
  const rebase = useRebase();
  const reflog = useReflog();
  const taskData = useTasks();
  const [interactiveRebaseBase, setInteractiveRebaseBase] = useState<Commit | null>(null);
  const [showCreateTag, setShowCreateTag] = useState(false);
  const [createTagCommitHash, setCreateTagCommitHash] = useState('');
  const [selectedStashDiff, setSelectedStashDiff] = useState<string | null>(null);
  const [showStashInput, setShowStashInput] = useState(false);
  const [stashMessageInput, setStashMessageInput] = useState('');
  const [backendError, setBackendError] = useState<string | null>(null);

  const terminalState = useTerminal();
  const [bottomMode, setBottomMode] = useState<BottomMode>('terminal');
  const [initialQuery, setInitialQuery] = useState<string | undefined>(undefined);
  const [dslHighlightHashes, setDslHighlightHashes] = useState<Set<string> | null>(null);
  useEffect(() => { if (bottomMode !== 'query') setDslHighlightHashes(null); }, [bottomMode]);
  const dslInputRef = useRef<HTMLInputElement>(null);
  const commitMsgRef = useRef<HTMLTextAreaElement>(null);

  // Graph animation tracking
  const prevCommitFingerprintsRef = useRef<Map<string, string>>(new Map());
  const [graphGeneration, setGraphGeneration] = useState(0);
  const [commitSearch, setCommitSearch] = useState('');

  const filteredCommits = useMemo(() => {
    const q = commitSearch.trim().toLowerCase();
    if (!q) return commits;
    return commits.filter(c =>
      c.message.toLowerCase().includes(q) ||
      c.author.toLowerCase().includes(q) ||
      c.hash.toLowerCase().startsWith(q)
    );
  }, [commits, commitSearch]);

  const graphWidth = useMemo(() => {
    let maxCol = 0;
    for (const c of filteredCommits) {
      if (c.graph && c.graph.column > maxCol) maxCol = c.graph.column;
    }
    return 32 + (maxCol + 1) * 24 + 16;
  }, [filteredCommits]);

  const animatedCommitHashes = useMemo(() => {
    const prevMap = prevCommitFingerprintsRef.current;
    const newMap = new Map<string, string>();
    const animated = new Set<string>();
    for (const c of commits) {
      const fp = c.hash;
      newMap.set(c.hash, fp);
      if (prevMap.size === 0 || prevMap.get(c.hash) !== fp) {
        animated.add(c.hash);
      }
    }
    prevCommitFingerprintsRef.current = newMap;
    return animated;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [commits, graphGeneration]);

  const triggerGraphRefresh = useCallback(() => {
    prevCommitFingerprintsRef.current = new Map();
    setGraphGeneration(g => g + 1);
  }, []);

  // Backend error check
  useEffect(() => {
    if (!isElectron) return;
    const api = (window as any).electronAPI;
    api.invoke('backend:status').then((s: { ok: boolean; error: string | null }) => {
      if (!s.ok) setBackendError(s.error);
    });
    const unsub = api.on('backend:error', (err: string) => setBackendError(err));
    return unsub;
  }, []);

  // Select first commit
  useEffect(() => {
    if (commits.length > 0 && !selectedCommit) {
      setSelectedCommit(commits[0]);
    }
  }, [commits, selectedCommit]);

  // Select first file
  useEffect(() => {
    if (fileChanges.length > 0) {
      setSelectedFile(fileChanges[0]);
    }
  }, [fileChanges]);

  // Listen for Preferences menu
  useEffect(() => {
    if (!isElectron) return;
    const unsub = window.electronAPI.on('menu:open-settings', () => {
      setMainView('settings');
    });
    return unsub;
  }, []);

  // Refresh on repo change
  useEffect(() => {
    if (repoPath) {
      setSelectedCommit(null);
      setSelectedFile(null);
      prevCommitFingerprintsRef.current = new Map();
      setGraphGeneration(g => g + 1);
      refreshCommits();
      refreshBranches();
      refreshFileTree();
      sourceControl.refresh();
      mergeConflicts.checkMergeState();
      rebase.checkRebaseState();
      stashes.refresh();
      remotes.refresh();
      tags.refresh();
      reflog.refresh();
      taskData.refresh();
    } else {
      mergeConflicts.clear();
    }
  }, [repoPath, refreshCommits, refreshBranches, refreshFileTree, sourceControl.refresh, mergeConflicts.checkMergeState, mergeConflicts.clear, stashes.refresh, remotes.refresh, tags.refresh, taskData.refresh]);

  // Prompt remote setup
  useEffect(() => {
    if (!repoPath || !isElectron) return;
    if (hasRemote) return;
    const timer = setTimeout(() => {
      if (!hasRemote) setShowRemoteSetup(true);
    }, 800);
    return () => clearTimeout(timer);
  }, [repoPath, hasRemote]);

  // File change listener
  useEffect(() => {
    if (!isElectron) return;
    let debounceTimer: ReturnType<typeof setTimeout>;
    const doRefresh = () => {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        refreshFileTree();
        refreshBranches();
        refreshCommits();
        sourceControl.refresh();
        stashes.refresh();
        remotes.refresh();
        tags.refresh();
        taskData.refresh();
      }, 300);
    };
    const unsub = window.electronAPI.on('repo:file-changed', doRefresh);
    return () => {
      unsub();
      clearTimeout(debounceTimer);
    };
  }, [refreshFileTree, refreshBranches, refreshCommits, sourceControl.refresh, stashes.refresh, remotes.refresh, tags.refresh, taskData.refresh]);

  // Keyboard shortcuts
  const keymap = useMemo(() => buildKeymap(settings.keybindings), [settings.keybindings]);

  useKeyboardShortcuts(useMemo(() => [
    { chord: keymap.showGraph, action: () => setMainView('graph'), label: 'Show commit graph' },
    { chord: keymap.showTerminal, action: () => { setBottomMode('terminal'); }, label: 'Show terminal' },
    { chord: keymap.focusDSL, action: () => { setBottomMode('query'); setTimeout(() => dslInputRef.current?.focus(), 50); }, label: 'Focus DSL query bar' },
    { chord: keymap.focusCommitMsg, action: () => { commitMsgRef.current?.focus(); }, label: 'Focus commit message' },
    { chord: keymap.quickStash, action: () => { stashes.createStash('Quick stash', false).then(() => sourceControl.refresh()); }, label: 'Quick stash' },
    { chord: keymap.toggleSidebar, action: () => setIsSidebarOpen(prev => !prev), label: 'Toggle sidebar' },
    { chord: keymap.push, action: () => doPush(undefined, undefined, false, !hasRemote), label: 'Push' },
    { chord: keymap.newBranch, action: () => setShowCreateBranch(true), label: 'New branch' },
    { chord: keymap.refresh, action: () => { refreshCommits(); refreshBranches(); refreshFileTree(); sourceControl.refresh(); }, label: 'Refresh all' },
  ], [keymap, stashes, sourceControl, doPush, hasRemote, refreshCommits, refreshBranches, refreshFileTree]));

  const currentBranchName = branches.find(b => b.isCurrent)?.name ?? 'current branch';

  const refreshAll = useCallback(() => {
    triggerGraphRefresh();
    refreshCommits();
    refreshBranches();
  }, [triggerGraphRefresh, refreshCommits, refreshBranches]);

  const value: AppContextValue = {
    repoPath, repoHistory, hasRemote, openRepo, createRepo, cloneRepo, switchRepo, checkRemote,
    commits, filteredCommits, selectedCommit, setSelectedCommit, refreshCommits, commitFilters,
    commitSearch, setCommitSearch, graphWidth, animatedCommitHashes, triggerGraphRefresh,
    branches, refreshBranches, mainBranchWarning, currentBranchName,
    fileChanges, selectedFile, setSelectedFile,
    sourceControl, amendMode, setAmendMode,
    pushing, pushResult, doPush,
    creatingBranch, doCreateBranch, doDeleteBranch,
    blameLines, blameFile, blameLoading, fetchBlame, clearBlame,
    mergeConflicts, stashes, remotes, cherryPickRevert, tags, rebase, reflog, taskData,
    editorTabs, activeEditorTabId, setActiveEditorTabId, openFile, closeEditorTab,
    fileTree, refreshFileTree,
    terminalState,
    mainView, setMainView, bottomMode, setBottomMode,
    isSidebarOpen, setIsSidebarOpen,
    initialQuery, setInitialQuery, dslHighlightHashes, setDslHighlightHashes,
    showCreateBranch, setShowCreateBranch,
    showMergeDialog, setShowMergeDialog,
    mergeBranch, setMergeBranch,
    showRemoteSetup, setShowRemoteSetup,
    showCreateTag, setShowCreateTag,
    createTagCommitHash, setCreateTagCommitHash,
    interactiveRebaseBase, setInteractiveRebaseBase,
    selectedStashDiff, setSelectedStashDiff,
    showStashInput, setShowStashInput,
    stashMessageInput, setStashMessageInput,
    settings, updateSetting, updateKeybinding,
    backendError,
    dslInputRef, commitMsgRef,
    refreshAll,
  };

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}
