import React, { useState, useEffect, useMemo, useRef } from 'react';
import {
  GitBranch, GitMerge, Tag,
  FolderGit2, Settings,
  Download, Upload, Plus, Minus, FolderOpen, Check,
  FilePlus, FileEdit, FileMinus, ChevronRight, ChevronDown,
  X, FileCode,
  AlertTriangle, Archive, Trash2, Play, RotateCcw, Clock
} from 'lucide-react';
import { Commit, FileChange } from './types';
import { useCommits, useBranches, useCommitDetails, useOpenRepo, useSourceControl, usePush, useBranchManagement, useBlame, useMergeConflicts, useStashes, useRemotes, useCherryPickRevert, useTags, useRebase, useReflog } from './hooks/useGitData';
import { useTasks } from './hooks/useTaskData';
import { useEditor } from './hooks/useEditor';
import { useFileTree } from './hooks/useFileTree';
import VexEditor from './components/VexEditor';
import FileExplorer from './components/FileExplorer';
import SettingsView from './components/SettingsView';
import { useTerminal } from './hooks/useTerminal';
import { useSettings } from './hooks/useSettings';
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts';
import { isElectron } from './lib/electron';
import { applyTheme } from './themes';
import { dragPayload } from './lib/dragState';
import { buildKeymap } from './lib/defaultKeymap';

// Extracted components
import CommitGraph from './components/CommitGraph';
import CommitDetailPanel from './components/CommitDetailPanel';
import ChangesView from './components/ChangesView';
import BottomPanel from './components/BottomPanel';
import Taskbar from './components/Taskbar';
import CreateBranchModal from './components/modals/CreateBranchModal';
import MergeDialog from './components/modals/MergeDialog';
import RemoteSetupModal from './components/modals/RemoteSetupModal';
import CreateTagModal from './components/modals/CreateTagModal';
import InteractiveRebaseView from './components/InteractiveRebaseView';
import DSLExplorer from './components/DSLExplorer';
import TaskBoard from './components/TaskBoard';

export default function App() {
  const { repoPath, repoHistory, hasRemote, openRepo, createRepo, cloneRepo, switchRepo, checkRemote } = useOpenRepo();
  const [isRepoDropdownOpen, setIsRepoDropdownOpen] = useState(false);
  const [dropdownPos, setDropdownPos] = useState({ left: 0, top: 0 });
  const repoButtonRef = useRef<HTMLButtonElement>(null);
  const [cloneUrl, setCloneUrl] = useState('');
  const { settings, updateSetting, updateKeybinding } = useSettings();

  // Apply theme when settings change
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
  const [mainView, setMainView] = useState<'graph' | 'editor' | 'changes' | 'settings' | 'interactive-rebase' | 'tasks'>('graph');
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
  const terminalState = useTerminal();
  const [bottomMode, setBottomMode] = useState<'terminal' | 'query' | 'docs'>('terminal');
  const [initialQuery, setInitialQuery] = useState<string | undefined>(undefined);
  const [dslHighlightHashes, setDslHighlightHashes] = useState<Set<string> | null>(null);
  useEffect(() => { if (bottomMode !== 'query') setDslHighlightHashes(null); }, [bottomMode]);
  const dslInputRef = useRef<HTMLInputElement>(null);
  const commitMsgRef = useRef<HTMLTextAreaElement>(null);

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

  // Graph animation tracking
  const prevCommitFingerprintsRef = useRef<Map<string, string>>(new Map());
  const [graphGeneration, setGraphGeneration] = useState(0);
  const [commitSearch, setCommitSearch] = useState('');

  // Filter commits by search query (message, author, hash)
  const filteredCommits = useMemo(() => {
    const q = commitSearch.trim().toLowerCase();
    if (!q) return commits;
    return commits.filter(c =>
      c.message.toLowerCase().includes(q) ||
      c.author.toLowerCase().includes(q) ||
      c.hash.toLowerCase().startsWith(q)
    );
  }, [commits, commitSearch]);

  // Compute dynamic graph column width based on max lane column
  const graphWidth = useMemo(() => {
    let maxCol = 0;
    for (const c of filteredCommits) {
      if (c.graph && c.graph.column > maxCol) maxCol = c.graph.column;
    }
    return 32 + (maxCol + 1) * 24 + 16;
  }, [filteredCommits]);

  // Track which commits are new or changed for animation
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

  // Select first commit when commits load
  useEffect(() => {
    if (commits.length > 0 && !selectedCommit) {
      setSelectedCommit(commits[0]);
    }
  }, [commits, selectedCommit]);

  // Select first file when file changes load
  useEffect(() => {
    if (fileChanges.length > 0) {
      setSelectedFile(fileChanges[0]);
    }
  }, [fileChanges]);

  // Listen for Preferences menu item (Cmd+,)
  useEffect(() => {
    if (!isElectron) return;
    const unsub = window.electronAPI.on('menu:open-settings', () => {
      setMainView('settings');
    });
    return unsub;
  }, []);

  // Refresh data when repo changes
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
    } else {
      mergeConflicts.clear();
    }
  }, [repoPath, refreshCommits, refreshBranches, refreshFileTree, sourceControl.refresh, mergeConflicts.checkMergeState, mergeConflicts.clear, stashes.refresh, remotes.refresh, tags.refresh]);

  // Prompt to add remote if repo has none
  useEffect(() => {
    if (!repoPath || !isElectron) return;
    if (hasRemote) return;
    const timer = setTimeout(() => {
      if (!hasRemote) {
        setShowRemoteSetup(true);
      }
    }, 800);
    return () => clearTimeout(timer);
  }, [repoPath, hasRemote]);

  // Listen for file changes and debounce all data refreshes
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
      }, 300);
    };
    const unsub = window.electronAPI.on('repo:file-changed', doRefresh);
    return () => {
      unsub();
      clearTimeout(debounceTimer);
    };
  }, [refreshFileTree, refreshBranches, refreshCommits, sourceControl.refresh, stashes.refresh, remotes.refresh, tags.refresh]);

  const currentBranchName = branches.find(b => b.isCurrent)?.name ?? 'current branch';

  return (
    <div className="flex flex-col h-screen w-full bg-zinc-950 text-zinc-300 overflow-hidden select-none">
      <div className="flex flex-1 min-h-0">

      {/* Sidebar */}
      {isSidebarOpen && repoPath && (
        <div className="w-64 flex-shrink-0 border-r border-zinc-800/60 bg-zinc-900/40 flex flex-col">
          <div className="h-14 flex items-center px-4 pl-24 border-b border-zinc-800/60 font-medium text-zinc-100 relative" style={{ WebkitAppRegion: 'drag' } as React.CSSProperties}>
            <button
              ref={repoButtonRef}
              onClick={() => {
                if (!isRepoDropdownOpen && repoButtonRef.current) {
                  const rect = repoButtonRef.current.getBoundingClientRect();
                  setDropdownPos({ left: rect.left, top: rect.bottom + 4 });
                }
                setIsRepoDropdownOpen(!isRepoDropdownOpen);
              }}
              className="flex items-center hover:bg-zinc-800/50 rounded px-1.5 py-0.5 transition-colors"
              style={{ WebkitAppRegion: 'no-drag' } as React.CSSProperties}
            >
              <FolderGit2 className={`w-5 h-5 mr-2 ${hasRemote ? 'text-emerald-500' : 'text-amber-500'}`} />
              {repoPath?.split('/').pop()}
              <ChevronDown className="w-3.5 h-3.5 ml-1.5 text-zinc-500" />
            </button>
            {isRepoDropdownOpen && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setIsRepoDropdownOpen(false)} />
                <div className="fixed z-50 w-56 bg-zinc-900 border border-zinc-700/60 rounded-lg shadow-xl py-0.5 max-h-80 overflow-y-auto" style={{ left: dropdownPos.left, top: dropdownPos.top }}>
                  {repoHistory.length === 0 ? (
                    <div className="px-2 py-1.5 text-xs text-zinc-600 italic">No history</div>
                  ) : (
                    repoHistory.map(p => (
                      <button
                        key={p}
                        onClick={() => { switchRepo(p); setIsRepoDropdownOpen(false); }}
                        className={`w-full text-left px-2 py-1 text-xs hover:bg-zinc-800/80 transition-colors truncate ${
                          p === repoPath ? 'text-[#F14E32]' : 'text-zinc-300'
                        }`}
                      >
                        {p.split('/').pop()}
                      </button>
                    ))
                  )}
                  <div className="border-t border-zinc-800/60">
                    <button
                      onClick={() => { openRepo(); setIsRepoDropdownOpen(false); }}
                      className="w-full text-left px-2 py-1 text-xs text-zinc-400 hover:bg-zinc-800/80 hover:text-zinc-200 transition-colors flex items-center"
                    >
                      <FolderOpen className="w-3 h-3 mr-1.5" />
                      Open...
                    </button>
                  </div>
                </div>
              </>
            )}
          </div>

          <div className="flex-1 overflow-y-auto py-2">
            <SidebarSection title="FILES" defaultOpen>
              <FileExplorer
                tree={fileTree}
                repoPath={repoPath}
                onFileOpen={(path) => { setMainView('editor'); openFile(path); }}
                onQueryFile={(filePath) => {
                  setBottomMode('query');
                  if (filePath.startsWith('blame:')) {
                    setInitialQuery(`blame "${filePath.slice(6)}"`);
                  } else {
                    setInitialQuery(`commits | where files contains "${filePath}"`);
                  }
                }}
                isOnMain={mainBranchWarning}
              />
            </SidebarSection>
          </div>

          {/* Source Control */}
          <div className="border-t border-zinc-800/60 py-2">
            <SidebarSection title="SOURCE CONTROL" defaultOpen>
              {sourceControl.staged.length > 0 && (
                <div className="px-4 mb-2">
                  <textarea
                    value={sourceControl.commitMessage}
                    onChange={(e) => sourceControl.setCommitMessage(e.target.value)}
                    onKeyDown={(e) => {
                      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                        e.preventDefault();
                        sourceControl.commit({ amend: amendMode }).then((hash) => {
                          if (hash) { setAmendMode(false); refreshCommits(); refreshBranches(); }
                        });
                      }
                    }}
                    placeholder="Commit message..."
                    rows={2}
                    className="w-full bg-zinc-800 border border-zinc-700 rounded-md px-2 py-1.5 text-xs focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] resize-none"
                  />
                  <div className="flex items-center gap-2 mt-1.5">
                    <button
                      onClick={() => sourceControl.commit({ amend: amendMode }).then((hash) => {
                        if (hash) { setAmendMode(false); refreshCommits(); refreshBranches(); }
                      })}
                      disabled={sourceControl.committing || !sourceControl.commitMessage.trim()}
                      className="flex-1 flex items-center justify-center px-3 py-1.5 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-xs font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      <Check className="w-3 h-3 mr-1" />
                      {sourceControl.committing ? 'Committing...' : amendMode ? 'Amend' : 'Commit'}
                    </button>
                    <label className="flex items-center gap-1 text-[10px] text-zinc-500 cursor-pointer select-none">
                      <input
                        type="checkbox"
                        checked={amendMode}
                        onChange={(e) => setAmendMode(e.target.checked)}
                        className="rounded border-zinc-600 bg-zinc-800 text-[#F14E32] focus:ring-[#F14E32] w-3 h-3"
                      />
                      Amend
                    </label>
                  </div>
                </div>
              )}

              {/* Merge Conflicts */}
              {sourceControl.unmerged.length > 0 && (
                <div className="mb-1">
                  <div className="flex items-center justify-between px-4 py-1">
                    <span className="text-[10px] font-semibold text-amber-500 uppercase tracking-wider">Merge Conflicts ({sourceControl.unmerged.length})</span>
                    <button
                      onClick={async () => {
                        await mergeConflicts.abortMerge();
                        refreshCommits();
                        refreshBranches();
                      }}
                      className="text-[10px] text-rose-500 hover:text-rose-300 transition-colors"
                    >
                      Abort Merge
                    </button>
                  </div>
                  {sourceControl.unmerged.map(f => (
                    <button
                      key={f.path}
                      onClick={() => {
                        mergeConflicts.fetchConflictDetail(f.path);
                        setMainView('changes');
                      }}
                      className={`w-full flex items-center px-6 py-1 text-xs hover:bg-zinc-800/50 group ${
                        mergeConflicts.selectedConflict === f.path ? 'bg-zinc-800/80 text-amber-300' : 'text-amber-400/80'
                      }`}
                    >
                      <AlertTriangle className="w-3 h-3 mr-1.5 text-amber-500 flex-shrink-0" />
                      <span className="truncate flex-1 text-left">{f.path.split('/').pop()}</span>
                    </button>
                  ))}
                </div>
              )}

              {/* Rebase in progress */}
              {rebase.isRebasing && (
                <div className="mb-1">
                  <div className="flex items-center justify-between px-4 py-1">
                    <span className="text-[10px] font-semibold text-purple-400 uppercase tracking-wider">Rebase in Progress</span>
                  </div>
                  <div className="flex items-center gap-2 px-4 pb-1">
                    <button
                      onClick={async () => {
                        const result = await rebase.continueRebase();
                        if (result?.success) {
                          refreshCommits(); refreshBranches(); sourceControl.refresh();
                        } else if (result?.hasConflicts) {
                          mergeConflicts.setConflictFiles(result.conflictFiles);
                          if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
                        }
                      }}
                      className="flex-1 px-2 py-1 text-[10px] rounded bg-emerald-600 hover:bg-emerald-500 text-white transition-colors"
                    >
                      Continue Rebase
                    </button>
                    <button
                      onClick={async () => {
                        await rebase.abortRebase();
                        refreshCommits(); refreshBranches(); sourceControl.refresh();
                      }}
                      className="flex-1 px-2 py-1 text-[10px] rounded bg-rose-600 hover:bg-rose-500 text-white transition-colors"
                    >
                      Abort Rebase
                    </button>
                  </div>
                </div>
              )}

              {/* Staged files — drop zone for staging */}
              <div
                className="mb-1"
                onDragOver={(e) => { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; }}
                onDrop={(e) => {
                  e.preventDefault();
                  const payload = dragPayload.current;
                  if (payload?.type === 'file' && payload.filePath) {
                    sourceControl.stageFiles([payload.filePath]);
                    dragPayload.current = null;
                  }
                }}
              >
                {sourceControl.staged.length > 0 && (
                  <>
                  <div className="flex items-center justify-between px-4 py-1">
                    <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider">Staged ({sourceControl.staged.length})</span>
                    <button onClick={() => sourceControl.unstageAll()} className="text-[10px] text-zinc-500 hover:text-zinc-300 transition-colors">Unstage All</button>
                  </div>
                  {sourceControl.staged.map(f => (
                    <button
                      key={f.path}
                      draggable
                      onDragStart={(e) => {
                        dragPayload.current = { type: 'file', filePath: f.path, sourceBranchId: null };
                        e.dataTransfer.effectAllowed = 'move';
                      }}
                      onClick={() => { sourceControl.setSelectedWorkingFile({ path: f.path, staged: true }); setMainView('changes'); }}
                      className="w-full flex items-center px-6 py-1 text-xs hover:bg-zinc-800/50 text-zinc-400 group cursor-grab active:cursor-grabbing"
                    >
                      {f.status === 'added' && <FilePlus className="w-3 h-3 mr-1.5 text-emerald-500 flex-shrink-0" />}
                      {f.status === 'modified' && <FileEdit className="w-3 h-3 mr-1.5 text-zinc-400 flex-shrink-0" />}
                      {f.status === 'deleted' && <FileMinus className="w-3 h-3 mr-1.5 text-rose-500 flex-shrink-0" />}
                      <span className="truncate flex-1 text-left">{f.path.split('/').pop()}</span>
                      <span
                        role="button"
                        onClick={(e) => { e.stopPropagation(); sourceControl.unstageFiles([f.path]); }}
                        className="opacity-0 group-hover:opacity-100 text-zinc-500 hover:text-zinc-200 transition-all"
                        title="Unstage"
                      >
                        <Minus className="w-3 h-3" />
                      </span>
                    </button>
                  ))}
                </>
                )}
              </div>

              {/* Unstaged (changed) files — drop zone for unstaging */}
              <div
                className="mb-1"
                onDragOver={(e) => { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; }}
                onDrop={(e) => {
                  e.preventDefault();
                  const payload = dragPayload.current;
                  if (payload?.type === 'file' && payload.filePath) {
                    sourceControl.unstageFiles([payload.filePath]);
                    dragPayload.current = null;
                  }
                }}
              >
                {sourceControl.unstaged.length > 0 && (
                  <>
                  <div className="flex items-center justify-between px-4 py-1">
                    <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider">Changes ({sourceControl.unstaged.length})</span>
                    <button onClick={() => sourceControl.stageAll()} className="text-[10px] text-zinc-500 hover:text-zinc-300 transition-colors">Stage All</button>
                  </div>
                  {sourceControl.unstaged.map(f => (
                    <button
                      key={f.path}
                      draggable
                      onDragStart={(e) => {
                        dragPayload.current = { type: 'file', filePath: f.path, sourceBranchId: null };
                        e.dataTransfer.effectAllowed = 'move';
                      }}
                      onClick={() => { sourceControl.setSelectedWorkingFile({ path: f.path, staged: false }); setMainView('changes'); }}
                      className="w-full flex items-center px-6 py-1 text-xs hover:bg-zinc-800/50 text-zinc-400 group cursor-grab active:cursor-grabbing"
                    >
                      {f.status === 'added' && <FilePlus className="w-3 h-3 mr-1.5 text-emerald-500 flex-shrink-0" />}
                      {f.status === 'modified' && <FileEdit className="w-3 h-3 mr-1.5 text-zinc-400 flex-shrink-0" />}
                      {f.status === 'deleted' && <FileMinus className="w-3 h-3 mr-1.5 text-rose-500 flex-shrink-0" />}
                      <span className="truncate flex-1 text-left">{f.path.split('/').pop()}</span>
                      <span
                        role="button"
                        onClick={(e) => { e.stopPropagation(); sourceControl.stageFiles([f.path]); }}
                        className="opacity-0 group-hover:opacity-100 text-zinc-500 hover:text-zinc-200 transition-all"
                        title="Stage"
                      >
                        <Plus className="w-3 h-3" />
                      </span>
                    </button>
                  ))}
                  </>
                )}
              </div>

              {/* Untracked files */}
              {sourceControl.untracked.length > 0 && (
                <div className="mb-1">
                  <div className="flex items-center justify-between px-4 py-1">
                    <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider">Untracked ({sourceControl.untracked.length})</span>
                  </div>
                  {sourceControl.untracked.map(path => (
                    <div
                      key={path}
                      className="flex items-center px-6 py-1 text-xs text-zinc-400 group"
                    >
                      <FilePlus className="w-3 h-3 mr-1.5 text-zinc-600 flex-shrink-0" />
                      <span className="truncate flex-1">{path.split('/').pop()}</span>
                      <button
                        onClick={(e) => { e.stopPropagation(); sourceControl.stageFiles([path]); }}
                        className="opacity-0 group-hover:opacity-100 text-zinc-500 hover:text-zinc-200 transition-all"
                        title="Stage"
                      >
                        <Plus className="w-3 h-3" />
                      </button>
                    </div>
                  ))}
                </div>
              )}

              {sourceControl.staged.length === 0 && sourceControl.unstaged.length === 0 && sourceControl.untracked.length === 0 && !sourceControl.loading && (
                <div className="px-6 py-1.5 text-xs text-zinc-500 italic">No changes</div>
              )}
            </SidebarSection>
          </div>

          <div className="border-t border-zinc-800/60 overflow-y-auto max-h-[40%] py-2">
            <SidebarSection title="LOCAL BRANCHES" defaultOpen>
              <div className="flex items-center justify-end px-4 -mt-1 mb-1">
                <button
                  onClick={() => setShowCreateBranch(true)}
                  className="text-[10px] text-zinc-500 hover:text-zinc-300 transition-colors flex items-center gap-0.5"
                >
                  <Plus className="w-3 h-3" /> New
                </button>
              </div>
              {branches.map(b => (
                <div
                  key={b.name}
                  role="button"
                  tabIndex={0}
                  onClick={async () => {
                    if (b.isCurrent) return;
                    const result = await window.electronAPI.invoke('git:stash-and-switch', b.name);
                    if (result.success) {
                      refreshBranches();
                      refreshCommits();
                      refreshFileTree();
                      sourceControl.refresh();
                    }
                  }}
                  onKeyDown={async (e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      if (b.isCurrent) return;
                      const result = await window.electronAPI.invoke('git:stash-and-switch', b.name);
                      if (result.success) {
                        refreshBranches(); refreshCommits(); refreshFileTree(); sourceControl.refresh();
                      }
                    }
                  }}
                  className={`flex items-center px-6 py-1.5 text-sm cursor-pointer hover:bg-zinc-800/50 group ${b.isCurrent ? 'text-[#F14E32] font-medium' : 'text-zinc-400'}`}
                >
                  <GitBranch className="w-4 h-4 mr-2 opacity-70" />
                  <span className="truncate flex-1">{b.name}</span>
                  {b.remote && !b.ahead && !b.behind && (
                    <span className="text-[10px] text-emerald-500 mr-1.5" title="In sync with remote">●</span>
                  )}
                  {(b.ahead ?? 0) > 0 && (
                    <span className="text-[10px] text-amber-400 mr-0.5" title={`${b.ahead} ahead of remote`}>↑{b.ahead}</span>
                  )}
                  {(b.behind ?? 0) > 0 && (
                    <span className="text-[10px] text-blue-400 mr-0.5" title={`${b.behind} behind remote`}>↓{b.behind}</span>
                  )}
                  {b.isCurrent ? (
                    <span className={`ml-auto w-1.5 h-1.5 rounded-full ${mainBranchWarning ? 'bg-amber-400' : 'bg-[#F14E32]'}`} />
                  ) : (
                    <div className="ml-auto flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-all">
                      <button
                        onClick={(e) => { e.stopPropagation(); setMergeBranch(b.name); setShowMergeDialog(true); }}
                        className="text-zinc-500 hover:text-zinc-200"
                        title={`Merge ${b.name} into current branch`}
                      >
                        <GitMerge className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={async (e) => {
                          e.stopPropagation();
                          const result = await rebase.rebase(b.name);
                          if (result?.success) {
                            refreshCommits(); refreshBranches();
                          } else if (result?.hasConflicts) {
                            mergeConflicts.setConflictFiles(result.conflictFiles);
                            setMainView('changes');
                            if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
                          }
                        }}
                        className="text-zinc-500 hover:text-zinc-200"
                        title={`Rebase onto ${b.name}`}
                      >
                        <RotateCcw className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={async (e) => {
                          e.stopPropagation();
                          const result = await doDeleteBranch(b.name);
                          if (result?.success) { refreshBranches(); }
                        }}
                        className="text-zinc-500 hover:text-rose-400"
                        title={`Delete ${b.name}`}
                      >
                        <X className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  )}
                </div>
              ))}
            </SidebarSection>

            <SidebarSection title={`REMOTES${remotes.remotes.length > 0 ? ` (${remotes.remotes.length})` : ''}`}>
              {remotes.remotes.length === 0 ? (
                <div className="px-6 py-1.5 text-xs text-zinc-500 italic">No remotes</div>
              ) : (
                remotes.remotes.map((r) => (
                  <div key={r.name}>
                    <button
                      onClick={() => remotes.toggleRemote(r.name)}
                      className="w-full flex items-center px-6 py-1.5 text-sm hover:bg-zinc-800/50 text-zinc-400"
                      title={r.url}
                    >
                      {remotes.expandedRemote === r.name
                        ? <ChevronDown className="w-4 h-4 mr-1 opacity-50" />
                        : <ChevronRight className="w-4 h-4 mr-1 opacity-50" />}
                      <span className="truncate flex-1">{r.name}</span>
                      <span className="text-[10px] text-zinc-600 truncate max-w-[120px] ml-2">{r.url}</span>
                    </button>
                    {remotes.expandedRemote === r.name && (
                      <div className="ml-4">
                        {remotes.loadingBranches ? (
                          <div className="px-6 py-1 text-xs text-zinc-600 italic">Loading...</div>
                        ) : remotes.remoteBranches.length === 0 ? (
                          <div className="px-6 py-1 text-xs text-zinc-600 italic">No branches</div>
                        ) : (
                          remotes.remoteBranches.map((branch) => {
                            const trackingBranch = branches.find(b => b.remote === `${r.name}/${branch}`);
                            const synced = trackingBranch && !trackingBranch.ahead && !trackingBranch.behind;
                            return (
                              <div key={branch} className="flex items-center px-6 py-1 text-xs text-zinc-500 hover:bg-zinc-800/50 hover:text-zinc-300">
                                <GitBranch className="w-3 h-3 mr-1.5 opacity-50" />
                                <span className="truncate flex-1">{branch}</span>
                                {trackingBranch ? (
                                  synced ? (
                                    <span className="text-[10px] text-emerald-500 ml-1" title={`In sync with ${trackingBranch.name}`}>●</span>
                                  ) : (
                                    <span className="flex items-center gap-0.5 ml-1">
                                      {(trackingBranch.ahead ?? 0) > 0 && (
                                        <span className="text-[10px] text-amber-400" title={`${trackingBranch.name} is ${trackingBranch.ahead} ahead`}>↑{trackingBranch.ahead}</span>
                                      )}
                                      {(trackingBranch.behind ?? 0) > 0 && (
                                        <span className="text-[10px] text-blue-400" title={`${trackingBranch.name} is ${trackingBranch.behind} behind`}>↓{trackingBranch.behind}</span>
                                      )}
                                    </span>
                                  )
                                ) : (
                                  <span className="text-[10px] text-zinc-700 ml-1" title="No local branch tracking">—</span>
                                )}
                              </div>
                            );
                          })
                        )}
                      </div>
                    )}
                  </div>
                ))
              )}
            </SidebarSection>

            <SidebarSection title={`TAGS${tags.tags.length > 0 ? ` (${tags.tags.length})` : ''}`}>
              <div className="flex items-center justify-end px-4 -mt-1 mb-1">
                <button
                  onClick={() => { setCreateTagCommitHash(''); setShowCreateTag(true); }}
                  className="text-[10px] text-zinc-500 hover:text-zinc-300 transition-colors flex items-center gap-0.5"
                >
                  <Plus className="w-3 h-3" /> New
                </button>
              </div>
              {tags.tags.length === 0 ? (
                <div className="px-6 py-1.5 text-xs text-zinc-500 italic">No tags</div>
              ) : (
                tags.tags.map((tag) => (
                  <div
                    key={tag.name}
                    className="group flex items-center px-6 py-1 text-xs hover:bg-zinc-800/50 text-zinc-400"
                  >
                    <Tag className="w-3 h-3 mr-2 text-zinc-500 flex-shrink-0" />
                    <span className="truncate flex-1" title={`${tag.name} → ${tag.hash}${tag.message ? ` — ${tag.message}` : ''}`}>
                      {tag.name}
                    </span>
                    <span className="font-mono text-[10px] text-zinc-600 mr-2 flex-shrink-0">{tag.hash}</span>
                    <div className="hidden group-hover:flex items-center gap-0.5 flex-shrink-0">
                      <button
                        title="Push tag"
                        onClick={(e) => { e.stopPropagation(); tags.pushTag(tag.name); }}
                        className="text-emerald-500 hover:text-emerald-400 p-0.5"
                      >
                        <Upload className="w-3 h-3" />
                      </button>
                      <button
                        title="Delete tag"
                        onClick={async (e) => {
                          e.stopPropagation();
                          await tags.deleteTag(tag.name);
                          refreshCommits();
                        }}
                        className="text-rose-500 hover:text-rose-400 p-0.5"
                      >
                        <Trash2 className="w-3 h-3" />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </SidebarSection>

            <SidebarSection title={`STASHES${stashes.stashes.length > 0 ? ` (${stashes.stashes.length})` : ''}`}>
              <div className="px-4 pb-1">
                {showStashInput ? (
                  <form
                    className="flex items-center gap-1"
                    onSubmit={async (e) => {
                      e.preventDefault();
                      await stashes.createStash(stashMessageInput || 'Stashed changes', false);
                      setStashMessageInput('');
                      setShowStashInput(false);
                      sourceControl.refresh();
                    }}
                  >
                    <input
                      autoFocus
                      value={stashMessageInput}
                      onChange={(e) => setStashMessageInput(e.target.value)}
                      placeholder="Stash message..."
                      className="flex-1 bg-zinc-800 border border-zinc-700 rounded px-2 py-0.5 text-xs text-zinc-300 outline-none focus:border-zinc-500"
                      onKeyDown={(e) => { if (e.key === 'Escape') { setShowStashInput(false); setStashMessageInput(''); } }}
                    />
                    <button type="submit" className="text-emerald-400 hover:text-emerald-300 p-0.5"><Check className="w-3 h-3" /></button>
                    <button type="button" onClick={() => { setShowStashInput(false); setStashMessageInput(''); }} className="text-zinc-500 hover:text-zinc-300 p-0.5"><X className="w-3 h-3" /></button>
                  </form>
                ) : (
                  <button
                    onClick={() => setShowStashInput(true)}
                    className="flex items-center text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
                  >
                    <Archive className="w-3 h-3 mr-1" />
                    Stash Changes
                  </button>
                )}
              </div>
              {stashes.stashes.length === 0 ? (
                <div className="px-6 py-1.5 text-xs text-zinc-500 italic">No stashes</div>
              ) : (
                stashes.stashes.map((stash) => (
                  <div
                    key={stash.index}
                    role="button"
                    tabIndex={0}
                    className={`group flex items-center px-6 py-1 text-xs cursor-pointer hover:bg-zinc-800/50 ${
                      selectedStashDiff !== null ? 'text-zinc-300' : 'text-zinc-400'
                    }`}
                    onClick={async () => {
                      const patch = await stashes.showStash(stash.index);
                      setSelectedStashDiff(patch);
                      sourceControl.setSelectedWorkingFile(null);
                      setSelectedFile(null);
                      setMainView('changes');
                    }}
                    onKeyDown={async (e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        const patch = await stashes.showStash(stash.index);
                        setSelectedStashDiff(patch);
                        sourceControl.setSelectedWorkingFile(null);
                        setSelectedFile(null);
                        setMainView('changes');
                      }
                    }}
                  >
                    <Archive className="w-3 h-3 mr-2 text-zinc-500 flex-shrink-0" />
                    <span className="truncate flex-1" title={stash.message}>
                      {stash.message}
                    </span>
                    <span className="text-zinc-600 text-[10px] mr-2 flex-shrink-0">{stash.date}</span>
                    <div className="hidden group-hover:flex items-center gap-0.5 flex-shrink-0">
                      <button
                        title="Apply"
                        onClick={async (e) => { e.stopPropagation(); await stashes.applyStash(stash.index); sourceControl.refresh(); }}
                        className="text-emerald-500 hover:text-emerald-400 p-0.5"
                      >
                        <Play className="w-3 h-3" />
                      </button>
                      <button
                        title="Pop"
                        onClick={async (e) => { e.stopPropagation(); await stashes.popStash(stash.index); sourceControl.refresh(); }}
                        className="text-blue-500 hover:text-blue-400 p-0.5"
                      >
                        <Download className="w-3 h-3" />
                      </button>
                      <button
                        title="Drop"
                        onClick={async (e) => {
                          e.stopPropagation();
                          await stashes.dropStash(stash.index);
                          if (selectedStashDiff !== null) setSelectedStashDiff(null);
                        }}
                        className="text-rose-500 hover:text-rose-400 p-0.5"
                      >
                        <Trash2 className="w-3 h-3" />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </SidebarSection>

            <SidebarSection title={`REFLOG${reflog.entries.length > 0 ? ` (${reflog.entries.length})` : ''}`}>
              {reflog.entries.length === 0 ? (
                <div className="px-6 py-1.5 text-xs text-zinc-500 italic">No reflog entries</div>
              ) : (
                reflog.entries.slice(0, 50).map((entry, i) => (
                  <div
                    key={`${entry.hash}-${i}`}
                    role="button"
                    tabIndex={0}
                    className="flex items-center px-6 py-1 text-xs hover:bg-zinc-800/50 text-zinc-400 group cursor-pointer"
                    onClick={() => {
                      const commit = commits.find(c => c.hash.startsWith(entry.hash.slice(0, 7)));
                      if (commit) {
                        setSelectedCommit(commit);
                        setMainView('graph');
                      }
                    }}
                  >
                    <Clock className="w-3 h-3 mr-2 text-zinc-600 flex-shrink-0" />
                    <span className="font-mono text-[10px] text-zinc-500 mr-2 flex-shrink-0">{entry.hash.slice(0, 7)}</span>
                    <span className="truncate flex-1" title={entry.message}>{entry.message}</span>
                    <span className="text-zinc-600 text-[10px] ml-1 flex-shrink-0">{entry.date}</span>
                  </div>
                ))
              )}
            </SidebarSection>
          </div>

          <div className="p-3 border-t border-zinc-800/60 flex items-center justify-between text-zinc-500">
            <button onClick={() => setMainView('settings')} aria-label="Settings" className="hover:text-zinc-300 transition-colors">
              <Settings className="w-4 h-4" />
            </button>
            <div className="text-xs">Git Client v0.1.0</div>
          </div>
        </div>
      )}

      {/* Main Content */}
      <div className="flex-1 flex flex-col min-w-0">

        {/* Drag region when no toolbar */}
        {!repoPath && (
          <div className="h-8" style={{ WebkitAppRegion: 'drag' } as React.CSSProperties} />
        )}

        <div className="flex-1 flex flex-col min-h-0">
        {/* Editor — always mounted to preserve PTY connections across view switches */}
        {repoPath && (
          <div
            className="flex-1 flex flex-col bg-zinc-950 min-h-0"
            style={{ display: mainView === 'editor' ? undefined : 'none' }}
          >
            {editorTabs.length > 0 ? (
              <>
                <div className="h-9 flex items-center border-b border-zinc-800/60 bg-zinc-900/40 px-2 gap-0.5 flex-shrink-0 overflow-x-auto">
                  {editorTabs.map(tab => (
                    <button
                      key={tab.id}
                      onClick={() => setActiveEditorTabId(tab.id)}
                      className={`flex items-center px-3 py-1 text-xs font-medium rounded transition-colors gap-1.5 flex-shrink-0 ${
                        activeEditorTabId === tab.id
                          ? 'bg-zinc-800 text-zinc-100'
                          : 'text-zinc-500 hover:text-zinc-300'
                      }`}
                    >
                      <FileCode className="w-3 h-3" />
                      {tab.laneName && (
                        <span className="px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 text-[10px] font-semibold uppercase tracking-wide">
                          {tab.laneName}
                        </span>
                      )}
                      {tab.fileName}
                      <X
                        className="w-3 h-3 hover:text-rose-400"
                        onClick={(e) => { e.stopPropagation(); closeEditorTab(tab.id); }}
                      />
                    </button>
                  ))}
                </div>
                <div className="flex-1 relative min-h-0 overflow-hidden">
                  {editorTabs.map(tab => (
                    <div
                      key={tab.id}
                      className="absolute inset-0"
                      style={{ display: activeEditorTabId === tab.id ? 'block' : 'none' }}
                    >
                      <VexEditor
                        editorId={tab.id}
                        filePath={tab.filePath}
                        isVisible={activeEditorTabId === tab.id && mainView === 'editor'}
                        onExit={(id) => closeEditorTab(id)}
                        fontSize={settings.editor.fontSize}
                        fontFamily={settings.editor.fontFamily}
                        cursorBlink={settings.editor.cursorBlink}
                      />
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <div className="flex-1 flex items-center justify-center text-zinc-600 italic">
                Open a file from the sidebar to start editing
              </div>
            )}
          </div>
        )}

        {/* Main Content Area - Switchable */}
        {!repoPath ? (
          <div className="flex-1 flex flex-col items-center justify-center bg-zinc-950 text-center">
            <FolderGit2 className="w-16 h-16 text-zinc-800 mb-4" />
            <h2 className="text-xl font-semibold text-zinc-400 mb-2">Welcome to Jock</h2>
            <p className="text-sm text-zinc-600 mb-6">Open a Git repository to get started</p>
            <div className="flex items-center gap-3 mb-6">
              <button
                onClick={openRepo}
                className="flex items-center px-5 py-2.5 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors"
              >
                <FolderOpen className="w-4 h-4 mr-2" />
                Open Repository
              </button>
              <button
                onClick={async () => {
                  const path = await createRepo();
                  if (path) {
                    setShowRemoteSetup(true);
                  }
                }}
                className="flex items-center px-5 py-2.5 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors"
              >
                <Plus className="w-4 h-4 mr-2" />
                Create Repository
              </button>
            </div>
            <form
              className="flex items-center gap-2"
              onSubmit={(e) => { e.preventDefault(); if (cloneUrl.trim()) cloneRepo(cloneUrl.trim()); }}
            >
              <input
                type="text"
                value={cloneUrl}
                onChange={(e) => setCloneUrl(e.target.value)}
                placeholder="https://github.com/user/repo.git"
                className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-2 text-sm focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] w-80 transition-all"
              />
              <button
                type="submit"
                disabled={!cloneUrl.trim()}
                className="flex items-center px-5 py-2.5 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <Download className="w-4 h-4 mr-2" />
                Clone
              </button>
            </form>
          </div>
        ) : mainView === 'graph' ? (
          <div className="flex-1 min-h-0 flex">
            <CommitGraph
              filteredCommits={filteredCommits}
              graphWidth={graphWidth}
              selectedCommit={selectedCommit}
              animatedCommitHashes={animatedCommitHashes}
              onSelectCommit={setSelectedCommit}
              commitSearch={commitSearch}
              onSearchChange={setCommitSearch}
              onCherryPick={async (commit) => {
                const result = await cherryPickRevert.cherryPick(commit.hash);
                if (result?.success) {
                  refreshCommits(); refreshBranches();
                } else if (result?.hasConflicts) {
                  mergeConflicts.setConflictFiles(result.conflictFiles);
                  setMainView('changes');
                  if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
                }
              }}
              onRevert={async (commit) => {
                const result = await cherryPickRevert.revert(commit.hash);
                if (result?.success) {
                  refreshCommits(); refreshBranches();
                } else if (result?.hasConflicts) {
                  mergeConflicts.setConflictFiles(result.conflictFiles);
                  setMainView('changes');
                  if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
                }
              }}
              onCreateTag={(commit) => {
                setCreateTagCommitHash(commit.hash);
                setShowCreateTag(true);
              }}
              onInteractiveRebase={(commit) => {
                setInteractiveRebaseBase(commit);
                setMainView('interactive-rebase');
              }}
              onQueryFromGraph={(query) => {
                setBottomMode('query');
                setInitialQuery(query);
              }}
              dslHighlightHashes={dslHighlightHashes}
              filters={commitFilters}
              onFiltersChange={(newFilters) => {
                refreshCommits(newFilters);
              }}
            />
            {selectedCommit && (
              <CommitDetailPanel
                commit={selectedCommit}
                fileChanges={fileChanges}
                onClose={() => setSelectedCommit(null)}
              />
            )}
          </div>
        ) : mainView === 'interactive-rebase' && interactiveRebaseBase ? (
          <InteractiveRebaseView
            commits={commits.slice(0, commits.findIndex(c => c.hash === interactiveRebaseBase.hash))}
            baseCommit={interactiveRebaseBase}
            onExecute={async (baseHash, entries) => {
              const result = await rebase.interactiveRebase(baseHash, entries);
              if (result?.success) {
                setInteractiveRebaseBase(null);
                setMainView('graph');
                refreshCommits(); refreshBranches();
              } else if (result?.hasConflicts) {
                mergeConflicts.setConflictFiles(result.conflictFiles);
                setMainView('changes');
                if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
              }
            }}
            onCancel={() => {
              setInteractiveRebaseBase(null);
              setMainView('graph');
            }}
          />
        ) : mainView === 'tasks' ? (
          <TaskBoard
            backlog={taskData.backlog}
            inProgress={taskData.inProgress}
            done={taskData.done}
            loading={taskData.loading}
            onCreateTask={taskData.createTask}
            onUpdateTask={taskData.updateTask}
            onDeleteTask={taskData.deleteTask}
            onStartTask={taskData.startTask}
          />
        ) : mainView === 'settings' ? (
          <SettingsView settings={settings} onUpdateSetting={updateSetting} onUpdateKeybinding={updateKeybinding} />
        ) : mainView !== 'editor' ? (
          <ChangesView
            fileChanges={fileChanges}
            selectedFile={selectedFile}
            onSelectFile={(file) => { setSelectedFile(file); clearBlame(); }}
            sourceControl={sourceControl}
            mergeConflicts={mergeConflicts}
            blameLines={blameLines}
            blameFile={blameFile}
            blameLoading={blameLoading}
            fetchBlame={fetchBlame}
            clearBlame={clearBlame}
            selectedStashDiff={selectedStashDiff}
            onCloseStashDiff={() => setSelectedStashDiff(null)}
            onOpenInEditor={(filePath) => { setMainView('editor'); openFile(filePath); }}
            mergeBranch={mergeBranch}
          />
        ) : null}

        {/* Bottom Panel */}
        {repoPath && (
          <BottomPanel
            settings={settings}
            commits={commits}
            onCommitSelect={setSelectedCommit}
            terminals={terminalState.terminals}
            activeTerminalId={terminalState.activeTerminalId}
            setActiveTerminalId={terminalState.setActiveTerminalId}
            createTerminal={terminalState.createTerminal}
            closeTerminal={terminalState.closeTerminal}
            bottomMode={bottomMode}
            setBottomMode={setBottomMode}
            initialQuery={initialQuery}
            onInitialQueryConsumed={() => setInitialQuery(undefined)}
            onRunDSLQuery={(query) => {
              setInitialQuery(query);
              setBottomMode('query');
            }}
            onDSLResultHashes={setDslHighlightHashes}
          />
        )}
      </div>
      </div>
      </div>

      {/* Taskbar */}
      {repoPath && (
        <Taskbar
          mainView={mainView}
          setMainView={setMainView}
          pushing={pushing}
          pushResult={pushResult}
          hasRemote={hasRemote}
          onPush={() => doPush(undefined, undefined, false, !hasRemote)}
          onOpenRepo={openRepo}
          onRefresh={() => {
            prevCommitFingerprintsRef.current = new Map();
            setGraphGeneration(g => g + 1);
            refreshCommits();
            refreshBranches();
          }}
        />
      )}

      {/* Modals */}
      <CreateBranchModal
        show={showCreateBranch}
        onClose={() => setShowCreateBranch(false)}
        onSuccess={() => { refreshBranches(); refreshCommits(); sourceControl.refresh(); }}
        doCreateBranch={doCreateBranch}
        creatingBranch={creatingBranch}
      />

      <MergeDialog
        show={showMergeDialog}
        mergeBranch={mergeBranch}
        currentBranchName={currentBranchName}
        onClose={() => setShowMergeDialog(false)}
        onSuccess={() => { refreshCommits(); refreshBranches(); sourceControl.refresh(); }}
        onResolveConflicts={(files) => {
          mergeConflicts.setConflictFiles(files);
          setMainView('changes');
          if (files.length > 0) mergeConflicts.fetchConflictDetail(files[0]);
          sourceControl.refresh();
        }}
        onAbortMerge={mergeConflicts.abortMerge}
      />

      <RemoteSetupModal
        show={showRemoteSetup}
        onClose={() => setShowRemoteSetup(false)}
        initialRepoName={repoPath?.split('/').pop() ?? ''}
        onSuccess={checkRemote}
      />

      <CreateTagModal
        show={showCreateTag}
        onClose={() => setShowCreateTag(false)}
        onSuccess={() => { tags.refresh(); refreshCommits(); }}
        doCreateTag={tags.createTag}
        defaultCommitHash={createTagCommitHash}
      />
    </div>
  );
}

function SidebarSection({ title, children, defaultOpen = false }: { title: string, children: React.ReactNode, defaultOpen?: boolean }) {
  const [isOpen, setIsOpen] = useState(defaultOpen);

  return (
    <div className="mb-2">
      <button
        className="w-full flex items-center px-4 py-1.5 hover:bg-zinc-800/30 text-zinc-500 group"
        onClick={() => setIsOpen(!isOpen)}
        aria-expanded={isOpen}
      >
        <ChevronRight className={`w-3.5 h-3.5 mr-1.5 transition-transform ${isOpen ? 'rotate-90' : ''}`} />
        <span className="text-xs font-semibold tracking-wider group-hover:text-zinc-400 transition-colors">{title}</span>
      </button>
      {isOpen && <div className="mt-1">{children}</div>}
    </div>
  );
}
