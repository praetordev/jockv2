import { useState, useEffect, useCallback } from 'react';
import { Commit, FileChange, Branch, MergeResult, ConflictDetail, ConflictStrategy, StashEntry, TagInfo, RebaseTodoEntry, ReflogEntry, CommitFilters } from '../types';
import { mockCommits, mockBranches, mockFileChanges } from '../mockData';
import { isElectron } from '../lib/electron';

export function useCommits(branch?: string, limit?: number) {
  const [commits, setCommits] = useState<Commit[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<CommitFilters>({});

  const refresh = useCallback(async (overrideFilters?: CommitFilters) => {
    if (!isElectron) {
      setCommits(mockCommits);
      setLoading(false);
      return;
    }
    setLoading(true);
    const activeFilters = overrideFilters ?? filters;
    try {
      const data = await window.electronAPI.invoke('git:list-commits', { branch, limit, ...activeFilters });
      if (data && data.length > 0) {
        setCommits(data);
        setError(null);
      } else {
        setCommits([]);
      }
    } catch (err: any) {
      setError(err.message);
      setCommits([]);
    } finally {
      setLoading(false);
    }
  }, [branch, limit, filters]);

  const applyFilters = useCallback((newFilters: CommitFilters) => {
    setFilters(newFilters);
  }, []);

  // Only auto-fetch in browser dev mode (no Electron)
  useEffect(() => { if (!isElectron) refresh(); }, [refresh]);

  return { commits, loading, error, refresh, filters, applyFilters };
}

export function useBranches() {
  const [branches, setBranches] = useState<Branch[]>([]);
  const [loading, setLoading] = useState(false);
  const [headBranch, setHeadBranch] = useState<string | null>(null);

  const currentBranch = headBranch ?? (branches.find(b => b.isCurrent)?.name ?? null);
  // Warn when on main/master
  const mainBranchWarning = currentBranch === 'main' || currentBranch === 'master';

  // Listen for instant branch changes from .git/HEAD watcher
  useEffect(() => {
    if (!isElectron) return;
    const unsub = window.electronAPI.on('git:branch-changed', (branch: string | null) => {
      setHeadBranch(branch);
    });
    return unsub;
  }, []);

  const refresh = useCallback(async () => {
    if (!isElectron) {
      setBranches(mockBranches);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const data = await window.electronAPI.invoke('git:list-branches');
      if (data && data.length > 0) {
        setBranches(data);
      } else {
        setBranches([]);
      }
    } catch {
      setBranches([]);
    } finally {
      setLoading(false);
    }
  }, []);

  // Only auto-fetch in browser dev mode (no Electron)
  useEffect(() => { if (!isElectron) refresh(); }, [refresh]);

  return { branches, loading, refresh, currentBranch, mainBranchWarning };
}

export function useCommitDetails(hash: string | null) {
  const [fileChanges, setFileChanges] = useState<FileChange[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!hash) {
      setFileChanges([]);
      return;
    }
    if (!isElectron) {
      setFileChanges(mockFileChanges);
      return;
    }

    setLoading(true);
    window.electronAPI.invoke('git:get-commit-details', hash)
      .then((data: any) => {
        if (data?.fileChanges?.length > 0) {
          setFileChanges(data.fileChanges);
        } else {
          setFileChanges([]);
        }
      })
      .catch(() => setFileChanges([]))
      .finally(() => setLoading(false));
  }, [hash]);

  return { fileChanges, loading };
}

export function useOpenRepo() {
  const [repoPath, setRepoPath] = useState<string | null>(null);
  const [repoHistory, setRepoHistory] = useState<string[]>([]);
  const [hasRemote, setHasRemote] = useState(false);

  const checkRemote = useCallback(async () => {
    if (!isElectron) return;
    const result = await window.electronAPI.invoke('git:has-remote');
    setHasRemote(result);
  }, []);

  const refreshHistory = useCallback(async () => {
    if (!isElectron) return;
    const history = await window.electronAPI.invoke('git:get-repo-history');
    setRepoHistory(history || []);
  }, []);

  const openRepo = useCallback(async () => {
    if (!isElectron) return null;
    const path = await window.electronAPI.invoke('git:open-repo');
    if (path) {
      setRepoPath(path);
      refreshHistory();
      checkRemote();
    }
    return path;
  }, [refreshHistory, checkRemote]);

  const createRepo = useCallback(async () => {
    if (!isElectron) return null;
    const path = await window.electronAPI.invoke('git:create-repo');
    if (path) {
      setRepoPath(path);
      refreshHistory();
      setHasRemote(false);
    }
    return path;
  }, [refreshHistory]);

  const cloneRepo = useCallback(async (url: string) => {
    if (!isElectron) return null;
    const path = await window.electronAPI.invoke('git:clone-repo', url);
    if (path) {
      setRepoPath(path);
      refreshHistory();
      setHasRemote(true);
    }
    return path;
  }, [refreshHistory]);

  const switchRepo = useCallback(async (targetPath: string) => {
    if (!isElectron) return null;
    const path = await window.electronAPI.invoke('git:switch-repo', targetPath);
    if (path) {
      setRepoPath(path);
      refreshHistory();
      checkRemote();
    }
    return path;
  }, [refreshHistory, checkRemote]);

  // Check if a repo is already open on mount + load history
  useEffect(() => {
    if (!isElectron) return;
    window.electronAPI.invoke('git:get-repo-path').then((path: string | null) => {
      if (path) {
        setRepoPath(path);
        checkRemote();
      }
    });
    refreshHistory();
  }, [refreshHistory, checkRemote]);

  return { repoPath, repoHistory, hasRemote, openRepo, createRepo, cloneRepo, switchRepo, checkRemote };
}

// --- Source Control Hooks ---

export function useStatus() {
  const [staged, setStaged] = useState<FileChange[]>([]);
  const [unstaged, setUnstaged] = useState<FileChange[]>([]);
  const [untracked, setUntracked] = useState<string[]>([]);
  const [unmerged, setUnmerged] = useState<FileChange[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    if (!isElectron) return;
    setLoading(true);
    try {
      const data = await window.electronAPI.invoke('git:get-status');
      setStaged(data?.staged || []);
      setUnstaged(data?.unstaged || []);
      setUntracked(data?.untracked || []);
      setUnmerged(data?.unmerged || []);
    } catch {
      setStaged([]);
      setUnstaged([]);
      setUntracked([]);
      setUnmerged([]);
    } finally {
      setLoading(false);
    }
  }, []);

  return { staged, unstaged, untracked, unmerged, loading, refresh };
}

export function useSourceControl() {
  const status = useStatus();
  const [commitMessage, setCommitMessage] = useState('');
  const [committing, setCommitting] = useState(false);
  const [selectedWorkingFile, setSelectedWorkingFile] = useState<{ path: string; staged: boolean } | null>(null);
  const [workingDiff, setWorkingDiff] = useState('');

  const stageFiles = useCallback(async (paths: string[]) => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:stage-files', paths);
    status.refresh();
  }, [status.refresh]);

  const unstageFiles = useCallback(async (paths: string[]) => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:unstage-files', paths);
    status.refresh();
  }, [status.refresh]);

  const stageAll = useCallback(async () => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:stage-files');
    status.refresh();
  }, [status.refresh]);

  const unstageAll = useCallback(async () => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:unstage-files');
    status.refresh();
  }, [status.refresh]);

  const commit = useCallback(async (opts?: { amend?: boolean }) => {
    if (!isElectron || !commitMessage.trim()) return null;
    setCommitting(true);
    try {
      const result = await window.electronAPI.invoke('git:create-commit', commitMessage.trim(), opts?.amend ?? false);
      if (result?.hash) {
        setCommitMessage('');
        status.refresh();
        return result.hash;
      }
      return null;
    } finally {
      setCommitting(false);
    }
  }, [commitMessage, status.refresh]);

  useEffect(() => {
    if (!selectedWorkingFile || !isElectron) {
      setWorkingDiff('');
      return;
    }
    window.electronAPI.invoke('git:get-working-diff', selectedWorkingFile.path, selectedWorkingFile.staged)
      .then((data: any) => setWorkingDiff(data?.patch || ''))
      .catch(() => setWorkingDiff(''));
  }, [selectedWorkingFile]);

  return {
    ...status,
    commitMessage,
    setCommitMessage,
    committing,
    stageFiles,
    unstageFiles,
    stageAll,
    unstageAll,
    commit,
    selectedWorkingFile,
    setSelectedWorkingFile,
    workingDiff,
  };
}

export function usePush() {
  const [pushing, setPushing] = useState(false);
  const [pushResult, setPushResult] = useState<{ success: boolean; error?: string } | null>(null);

  const doPush = useCallback(async (remote?: string, branch?: string, force?: boolean, setUpstream?: boolean) => {
    if (!isElectron) return;
    setPushing(true);
    setPushResult(null);
    try {
      const result = await window.electronAPI.invoke('git:push', remote, branch, force, setUpstream);
      setPushResult({ success: result.success, error: result.error });
      return result;
    } catch (err: any) {
      setPushResult({ success: false, error: err.message });
    } finally {
      setPushing(false);
    }
  }, []);

  return { pushing, pushResult, doPush, clearPushResult: () => setPushResult(null) };
}

export function useMerge() {
  const [merging, setMerging] = useState(false);
  const [mergeResult, setMergeResult] = useState<MergeResult | null>(null);

  const doMerge = useCallback(async (branch: string, noFf?: boolean) => {
    if (!isElectron) return;
    setMerging(true);
    setMergeResult(null);
    try {
      const result = await window.electronAPI.invoke('git:merge', branch, noFf);
      setMergeResult(result);
      return result;
    } catch (err: any) {
      setMergeResult({ success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.message });
    } finally {
      setMerging(false);
    }
  }, []);

  return { merging, mergeResult, doMerge, clearMergeResult: () => setMergeResult(null) };
}

export function useBranchManagement() {
  const [creating, setCreating] = useState(false);

  const doCreateBranch = useCallback(async (name: string, checkout = true, startPoint = '') => {
    if (!isElectron) return;
    setCreating(true);
    try {
      const result = await window.electronAPI.invoke('git:create-branch', name, checkout, startPoint);
      return result;
    } finally {
      setCreating(false);
    }
  }, []);

  const doDeleteBranch = useCallback(async (name: string, force = false) => {
    if (!isElectron) return;
    const result = await window.electronAPI.invoke('git:delete-branch', name, force);
    return result;
  }, []);

  return { creating, doCreateBranch, doDeleteBranch };
}

export interface BlameLine {
  hash: string;
  author: string;
  date: string;
  lineNo: number;
  content: string;
}

export function useBlame() {
  const [blameLines, setBlameLines] = useState<BlameLine[]>([]);
  const [loading, setLoading] = useState(false);
  const [blameFile, setBlameFile] = useState<string | null>(null);

  const fetchBlame = useCallback(async (filePath: string) => {
    if (!isElectron) return;
    setLoading(true);
    setBlameFile(filePath);
    try {
      const data = await window.electronAPI.invoke('git:blame', filePath);
      setBlameLines(data?.lines || []);
    } catch {
      setBlameLines([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const clearBlame = useCallback(() => {
    setBlameLines([]);
    setBlameFile(null);
  }, []);

  return { blameLines, blameFile, loading, fetchBlame, clearBlame };
}

export function useMergeConflicts(refreshStatus: () => void) {
  const [isMergingState, setIsMergingState] = useState(false);
  const [conflictFiles, setConflictFiles] = useState<string[]>([]);
  const [selectedConflict, setSelectedConflict] = useState<string | null>(null);
  const [conflictDetail, setConflictDetail] = useState<ConflictDetail | null>(null);
  const [resolving, setResolving] = useState(false);

  const checkMergeState = useCallback(async () => {
    if (!isElectron) return;
    const result = await window.electronAPI.invoke('git:is-merging');
    setIsMergingState(result?.isMerging ?? false);
  }, []);

  const fetchConflictDetail = useCallback(async (filePath: string) => {
    if (!isElectron) return;
    setSelectedConflict(filePath);
    try {
      const detail = await window.electronAPI.invoke('git:get-conflict-details', filePath);
      setConflictDetail(detail);
    } catch {
      setConflictDetail(null);
    }
  }, []);

  const resolveConflict = useCallback(async (filePath: string, strategy: ConflictStrategy) => {
    if (!isElectron) return;
    setResolving(true);
    try {
      await window.electronAPI.invoke('git:resolve-conflict', filePath, strategy);
      setConflictFiles(prev => prev.filter(f => f !== filePath));
      if (selectedConflict === filePath) {
        setSelectedConflict(null);
        setConflictDetail(null);
      }
      refreshStatus();
    } finally {
      setResolving(false);
    }
  }, [selectedConflict, refreshStatus]);

  const abortMerge = useCallback(async () => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:abort-merge');
    setIsMergingState(false);
    setConflictFiles([]);
    setSelectedConflict(null);
    setConflictDetail(null);
    refreshStatus();
  }, [refreshStatus]);

  const clear = useCallback(() => {
    setIsMergingState(false);
    setConflictFiles([]);
    setSelectedConflict(null);
    setConflictDetail(null);
  }, []);

  return {
    isMerging: isMergingState,
    conflictFiles,
    setConflictFiles,
    selectedConflict,
    conflictDetail,
    resolving,
    checkMergeState,
    fetchConflictDetail,
    resolveConflict,
    abortMerge,
    clear,
  };
}

// --- Stash Hook ---

export function useStashes() {
  const [stashes, setStashes] = useState<StashEntry[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    if (!isElectron) return;
    setLoading(true);
    try {
      const data = await window.electronAPI.invoke('git:list-stashes');
      setStashes(data?.stashes || []);
    } catch {
      setStashes([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const doCreateStash = useCallback(async (message: string, includeUntracked = false) => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:create-stash', message, includeUntracked);
    refresh();
  }, [refresh]);

  const doApplyStash = useCallback(async (index: number) => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:apply-stash', index);
    refresh();
  }, [refresh]);

  const doPopStash = useCallback(async (index: number) => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:pop-stash', index);
    refresh();
  }, [refresh]);

  const doDropStash = useCallback(async (index: number) => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:drop-stash', index);
    refresh();
  }, [refresh]);

  const doShowStash = useCallback(async (index: number): Promise<string> => {
    if (!isElectron) return '';
    const data = await window.electronAPI.invoke('git:show-stash', index);
    return data?.patch || '';
  }, []);

  return {
    stashes,
    loading,
    refresh,
    createStash: doCreateStash,
    applyStash: doApplyStash,
    popStash: doPopStash,
    dropStash: doDropStash,
    showStash: doShowStash,
  };
}

// --- Remotes Hook ---

export function useRemotes() {
  const [remotes, setRemotes] = useState<{ name: string; url: string }[]>([]);
  const [loading, setLoading] = useState(false);
  const [expandedRemote, setExpandedRemote] = useState<string | null>(null);
  const [remoteBranches, setRemoteBranches] = useState<string[]>([]);
  const [loadingBranches, setLoadingBranches] = useState(false);

  const refresh = useCallback(async () => {
    if (!isElectron) return;
    setLoading(true);
    try {
      const data = await window.electronAPI.invoke('git:list-remotes');
      setRemotes(data?.remotes || []);
    } catch {
      setRemotes([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const toggleRemote = useCallback(async (name: string) => {
    if (expandedRemote === name) {
      setExpandedRemote(null);
      setRemoteBranches([]);
      return;
    }
    setExpandedRemote(name);
    setLoadingBranches(true);
    try {
      const data = await window.electronAPI.invoke('git:list-remote-branches', name);
      setRemoteBranches(data?.branches || []);
    } catch {
      setRemoteBranches([]);
    } finally {
      setLoadingBranches(false);
    }
  }, [expandedRemote]);

  return { remotes, loading, refresh, expandedRemote, remoteBranches, loadingBranches, toggleRemote };
}

// --- Cherry-Pick / Revert Hook ---

export function useCherryPickRevert() {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<MergeResult | null>(null);

  const doCherryPick = useCallback(async (commitHash: string): Promise<MergeResult | undefined> => {
    if (!isElectron) return;
    setLoading(true);
    setResult(null);
    try {
      const data = await window.electronAPI.invoke('git:cherry-pick', commitHash);
      setResult(data);
      return data;
    } catch (err: any) {
      const errorResult: MergeResult = { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.message };
      setResult(errorResult);
      return errorResult;
    } finally {
      setLoading(false);
    }
  }, []);

  const doRevert = useCallback(async (commitHash: string): Promise<MergeResult | undefined> => {
    if (!isElectron) return;
    setLoading(true);
    setResult(null);
    try {
      const data = await window.electronAPI.invoke('git:revert', commitHash);
      setResult(data);
      return data;
    } catch (err: any) {
      const errorResult: MergeResult = { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.message };
      setResult(errorResult);
      return errorResult;
    } finally {
      setLoading(false);
    }
  }, []);

  const clearResult = useCallback(() => setResult(null), []);

  return { loading, result, cherryPick: doCherryPick, revert: doRevert, clearResult };
}

// --- Tags Hook ---

export function useTags() {
  const [tags, setTags] = useState<TagInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    if (!isElectron) return;
    setLoading(true);
    try {
      const data = await window.electronAPI.invoke('git:list-tags');
      setTags(data?.tags || []);
    } catch {
      setTags([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const doCreateTag = useCallback(async (tagName: string, commitHash?: string, message?: string) => {
    if (!isElectron) return;
    const result = await window.electronAPI.invoke('git:create-tag', tagName, commitHash || '', message || '');
    if (result?.success) refresh();
    return result;
  }, [refresh]);

  const doDeleteTag = useCallback(async (tagName: string) => {
    if (!isElectron) return;
    const result = await window.electronAPI.invoke('git:delete-tag', tagName);
    if (result?.success) refresh();
    return result;
  }, [refresh]);

  const doPushTag = useCallback(async (tagName: string, remote?: string) => {
    if (!isElectron) return;
    return await window.electronAPI.invoke('git:push-tag', tagName, remote || 'origin');
  }, []);

  return {
    tags,
    loading,
    refresh,
    createTag: doCreateTag,
    deleteTag: doDeleteTag,
    pushTag: doPushTag,
  };
}

// --- Rebase Hook ---

export function useRebase() {
  const [rebasing, setRebasing] = useState(false);
  const [isRebasingState, setIsRebasingState] = useState(false);
  const [result, setResult] = useState<MergeResult | null>(null);

  const checkRebaseState = useCallback(async () => {
    if (!isElectron) return;
    const data = await window.electronAPI.invoke('git:is-rebasing');
    setIsRebasingState(data?.isRebasing ?? false);
  }, []);

  const doRebase = useCallback(async (ontoBranch: string): Promise<MergeResult | undefined> => {
    if (!isElectron) return;
    setRebasing(true);
    setResult(null);
    try {
      const data = await window.electronAPI.invoke('git:rebase', ontoBranch);
      setResult(data);
      if (data.hasConflicts) setIsRebasingState(true);
      return data;
    } catch (err: any) {
      const errorResult: MergeResult = { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.message };
      setResult(errorResult);
      return errorResult;
    } finally {
      setRebasing(false);
    }
  }, []);

  const doAbortRebase = useCallback(async () => {
    if (!isElectron) return;
    await window.electronAPI.invoke('git:abort-rebase');
    setIsRebasingState(false);
    setResult(null);
  }, []);

  const doContinueRebase = useCallback(async (): Promise<MergeResult | undefined> => {
    if (!isElectron) return;
    setRebasing(true);
    try {
      const data = await window.electronAPI.invoke('git:continue-rebase');
      setResult(data);
      if (!data.hasConflicts) setIsRebasingState(false);
      return data;
    } catch (err: any) {
      const errorResult: MergeResult = { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.message };
      setResult(errorResult);
      return errorResult;
    } finally {
      setRebasing(false);
    }
  }, []);

  const doInteractiveRebase = useCallback(async (baseCommit: string, entries: RebaseTodoEntry[]): Promise<MergeResult | undefined> => {
    if (!isElectron) return;
    setRebasing(true);
    setResult(null);
    try {
      const data = await window.electronAPI.invoke('git:interactive-rebase', baseCommit, entries);
      setResult(data);
      if (data.hasConflicts) setIsRebasingState(true);
      return data;
    } catch (err: any) {
      const errorResult: MergeResult = { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.message };
      setResult(errorResult);
      return errorResult;
    } finally {
      setRebasing(false);
    }
  }, []);

  const clearResult = useCallback(() => setResult(null), []);

  return {
    rebasing,
    isRebasing: isRebasingState,
    result,
    rebase: doRebase,
    abortRebase: doAbortRebase,
    continueRebase: doContinueRebase,
    interactiveRebase: doInteractiveRebase,
    checkRebaseState,
    clearResult,
  };
}

// --- Reflog Hook ---

export function useReflog() {
  const [entries, setEntries] = useState<ReflogEntry[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async (limit = 50) => {
    if (!isElectron) return;
    setLoading(true);
    try {
      const data = await window.electronAPI.invoke('git:list-reflog', limit);
      setEntries(data?.entries || []);
    } catch {
      setEntries([]);
    } finally {
      setLoading(false);
    }
  }, []);

  return { entries, loading, refresh };
}
