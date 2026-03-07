import { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import {
  GitBranch, GitCommit, Tag, FileText, Terminal, Search,
  GitGraph, FileCode, FileEdit, Settings, ClipboardList, RefreshCw,
  Archive, Upload, Download, Plus,
} from 'lucide-react';
import { useAppContext } from '../context/AppContext';
import { useToast } from '../context/ToastContext';
import { isElectron } from '../lib/electron';
import type { FileNode } from '../hooks/useFileTree';

interface CommandPaletteProps {
  show: boolean;
  onClose: () => void;
}

type ResultKind = 'action' | 'commit' | 'branch' | 'tag' | 'file' | 'dsl';

interface PaletteResult {
  id: string;
  kind: ResultKind;
  label: string;
  detail?: string;
  icon: React.ReactNode;
  action: () => void;
}

function fuzzyMatch(query: string, target: string): boolean {
  const q = query.toLowerCase();
  const t = target.toLowerCase();
  let qi = 0;
  for (let ti = 0; ti < t.length && qi < q.length; ti++) {
    if (t[ti] === q[qi]) qi++;
  }
  return qi === q.length;
}

function fuzzyScore(query: string, target: string): number {
  const q = query.toLowerCase();
  const t = target.toLowerCase();
  // Exact prefix match scores highest
  if (t.startsWith(q)) return 100;
  // Contains as substring
  if (t.includes(q)) return 80;
  // Fuzzy match — score based on consecutive matches
  let qi = 0;
  let score = 0;
  let lastMatchIdx = -2;
  for (let ti = 0; ti < t.length && qi < q.length; ti++) {
    if (t[ti] === q[qi]) {
      score += ti === lastMatchIdx + 1 ? 10 : 5; // consecutive matches score more
      lastMatchIdx = ti;
      qi++;
    }
  }
  return qi === q.length ? score : 0;
}

function flattenFileTree(nodes: FileNode[]): { name: string; path: string }[] {
  const result: { name: string; path: string }[] = [];
  function walk(node: FileNode) {
    if (!node.isDirectory) {
      result.push({ name: node.name, path: node.path });
    }
    node.children.forEach(walk);
  }
  nodes.forEach(walk);
  return result;
}

const ICON_CLASS = 'w-4 h-4 flex-shrink-0';

export default function CommandPalette({ show, onClose }: CommandPaletteProps) {
  const ctx = useAppContext();
  const { addToast } = useToast();
  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [dslHistory, setDslHistory] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Load DSL history when opened
  useEffect(() => {
    if (show && isElectron) {
      window.electronAPI.invoke('dsl:get-history').then((h: string[]) => setDslHistory(h || []));
    }
  }, [show]);

  // Focus input and reset state when opened
  useEffect(() => {
    if (show) {
      setQuery('');
      setSelectedIndex(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [show]);

  // Close on Escape
  useEffect(() => {
    if (!show) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.preventDefault(); onClose(); }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [show, onClose]);

  const flatFiles = useMemo(() => flattenFileTree(ctx.fileTree), [ctx.fileTree]);

  // Build static actions
  const actions: PaletteResult[] = useMemo(() => [
    { id: 'act:graph', kind: 'action' as const, label: 'Show Commit Graph', icon: <GitGraph className={ICON_CLASS} />, action: () => ctx.setMainView('graph') },
    { id: 'act:editor', kind: 'action' as const, label: 'Show Editor', icon: <FileCode className={ICON_CLASS} />, action: () => ctx.setMainView('editor') },
    { id: 'act:changes', kind: 'action' as const, label: 'Show Changes', icon: <FileEdit className={ICON_CLASS} />, action: () => ctx.setMainView('changes') },
    { id: 'act:tasks', kind: 'action' as const, label: 'Show Tasks', icon: <ClipboardList className={ICON_CLASS} />, action: () => ctx.setMainView('tasks') },
    { id: 'act:settings', kind: 'action' as const, label: 'Open Settings', icon: <Settings className={ICON_CLASS} />, action: () => ctx.setMainView('settings') },
    { id: 'act:terminal', kind: 'action' as const, label: 'Show Terminal', icon: <Terminal className={ICON_CLASS} />, action: () => ctx.setBottomMode('terminal') },
    { id: 'act:dsl', kind: 'action' as const, label: 'Focus DSL Query Bar', icon: <Search className={ICON_CLASS} />, action: () => ctx.setBottomMode('query') },
    { id: 'act:refresh', kind: 'action' as const, label: 'Refresh All', icon: <RefreshCw className={ICON_CLASS} />, action: () => ctx.refreshAll() },
    { id: 'act:newbranch', kind: 'action' as const, label: 'New Branch', icon: <Plus className={ICON_CLASS} />, action: () => ctx.setShowCreateBranch(true) },
    { id: 'act:push', kind: 'action' as const, label: 'Push to Remote', icon: <Upload className={ICON_CLASS} />, action: () => ctx.doPush(undefined, undefined, false, !ctx.hasRemote) },
    { id: 'act:stash', kind: 'action' as const, label: 'Quick Stash', icon: <Archive className={ICON_CLASS} />, action: () => { ctx.stashes.createStash('Quick stash', false).then(() => ctx.sourceControl.refresh()); } },
  ], [ctx]);

  const results = useMemo<PaletteResult[]>(() => {
    const q = query.trim();
    if (!q) {
      // Show actions when empty
      return actions;
    }

    const items: (PaletteResult & { score: number })[] = [];

    // Search actions
    for (const a of actions) {
      const score = fuzzyScore(q, a.label);
      if (score > 0) items.push({ ...a, score });
    }

    // Search commits (limit to top 50 for performance)
    for (let i = 0; i < Math.min(ctx.commits.length, 500); i++) {
      const c = ctx.commits[i];
      const msgScore = fuzzyScore(q, c.message);
      const hashScore = c.hash.toLowerCase().startsWith(q.toLowerCase()) ? 100 : 0;
      const authorScore = fuzzyScore(q, c.author);
      const score = Math.max(msgScore, hashScore, authorScore);
      if (score > 0) {
        items.push({
          id: `commit:${c.hash}`,
          kind: 'commit',
          label: c.message.split('\n')[0].slice(0, 80),
          detail: `${c.hash.slice(0, 7)} by ${c.author}`,
          icon: <GitCommit className={ICON_CLASS} />,
          action: () => { ctx.setSelectedCommit(c); ctx.setMainView('graph'); },
          score,
        });
      }
    }

    // Search branches
    for (const b of ctx.branches) {
      const score = fuzzyScore(q, b.name);
      if (score > 0) {
        items.push({
          id: `branch:${b.name}`,
          kind: 'branch',
          label: b.name,
          detail: b.isCurrent ? 'current' : undefined,
          icon: <GitBranch className={ICON_CLASS} />,
          action: async () => {
            if (b.isCurrent) return;
            const result = await window.electronAPI.invoke('git:stash-and-switch', b.name);
            if (result.success) {
              ctx.refreshBranches(); ctx.refreshCommits(); ctx.refreshFileTree(); ctx.sourceControl.refresh();
              addToast({ type: 'success', title: `Switched to ${b.name}` });
            } else {
              addToast({ type: 'error', title: 'Switch failed', message: result.error });
            }
          },
          score,
        });
      }
    }

    // Search tags
    for (const t of ctx.tags.tags) {
      const score = fuzzyScore(q, t.name);
      if (score > 0) {
        items.push({
          id: `tag:${t.name}`,
          kind: 'tag',
          label: t.name,
          detail: t.message || t.hash.slice(0, 7),
          icon: <Tag className={ICON_CLASS} />,
          action: () => {
            const commit = ctx.commits.find(c => c.hash.startsWith(t.hash.slice(0, 7)));
            if (commit) { ctx.setSelectedCommit(commit); ctx.setMainView('graph'); }
          },
          score,
        });
      }
    }

    // Search files
    for (const f of flatFiles) {
      const nameScore = fuzzyScore(q, f.name);
      const pathScore = fuzzyScore(q, f.path);
      const score = Math.max(nameScore, pathScore * 0.8);
      if (score > 0) {
        items.push({
          id: `file:${f.path}`,
          kind: 'file',
          label: f.name,
          detail: f.path,
          icon: <FileText className={ICON_CLASS} />,
          action: () => { ctx.setMainView('editor'); ctx.openFile(f.path); },
          score,
        });
      }
    }

    // Search DSL history
    for (const h of dslHistory) {
      const score = fuzzyScore(q, h);
      if (score > 0) {
        items.push({
          id: `dsl:${h}`,
          kind: 'dsl',
          label: h,
          detail: 'DSL query',
          icon: <Terminal className={ICON_CLASS} />,
          action: () => { ctx.setInitialQuery(h); ctx.setBottomMode('query'); },
          score,
        });
      }
    }

    // Sort by score descending, cap at 50
    items.sort((a, b) => b.score - a.score);
    return items.slice(0, 50);
  }, [query, actions, ctx, flatFiles, dslHistory, addToast]);

  // Reset selection when results change
  useEffect(() => {
    setSelectedIndex(0);
  }, [results.length, query]);

  // Scroll selected item into view
  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    const selected = list.children[selectedIndex] as HTMLElement;
    selected?.scrollIntoView({ block: 'nearest' });
  }, [selectedIndex]);

  const executeSelected = useCallback(() => {
    const item = results[selectedIndex];
    if (item) {
      onClose();
      item.action();
    }
  }, [results, selectedIndex, onClose]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex(i => (i < results.length - 1 ? i + 1 : 0));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex(i => (i > 0 ? i - 1 : results.length - 1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      executeSelected();
    }
  }, [results.length, executeSelected]);

  if (!show) return null;

  const kindLabels: Record<ResultKind, string> = {
    action: 'Action',
    commit: 'Commit',
    branch: 'Branch',
    tag: 'Tag',
    file: 'File',
    dsl: 'Query',
  };

  return (
    <>
      <div className="fixed inset-0 bg-black/50 z-[60]" onClick={onClose} />
      <div className="fixed z-[60] top-[15%] left-1/2 -translate-x-1/2 w-[560px] bg-zinc-900 border border-zinc-700/60 rounded-xl shadow-2xl overflow-hidden">
        {/* Search input */}
        <div className="flex items-center px-4 py-3 border-b border-zinc-800/60">
          <Search className="w-4 h-4 text-zinc-500 mr-3 flex-shrink-0" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search commits, files, branches, tags, or run an action..."
            className="flex-1 bg-transparent text-sm text-zinc-200 placeholder-zinc-500 outline-none"
            aria-label="Command palette search"
          />
        </div>

        {/* Results */}
        <div ref={listRef} className="max-h-[400px] overflow-y-auto py-1" role="listbox">
          {results.length === 0 && query.trim() && (
            <div className="px-4 py-6 text-center text-xs text-zinc-500">No results found</div>
          )}
          {results.map((item, i) => (
            <button
              key={item.id}
              role="option"
              aria-selected={i === selectedIndex}
              onClick={() => { onClose(); item.action(); }}
              onMouseEnter={() => setSelectedIndex(i)}
              className={`w-full flex items-center gap-3 px-4 py-2 text-left transition-colors ${
                i === selectedIndex ? 'bg-zinc-800/80' : 'hover:bg-zinc-800/40'
              }`}
            >
              <span className={i === selectedIndex ? 'text-[#F14E32]' : 'text-zinc-500'}>
                {item.icon}
              </span>
              <div className="flex-1 min-w-0">
                <div className="text-sm text-zinc-200 truncate">{item.label}</div>
                {item.detail && (
                  <div className="text-[11px] text-zinc-500 truncate">{item.detail}</div>
                )}
              </div>
              <span className="text-[10px] text-zinc-600 uppercase flex-shrink-0">
                {kindLabels[item.kind]}
              </span>
            </button>
          ))}
        </div>

        {/* Footer hints */}
        <div className="flex items-center gap-4 px-4 py-2 border-t border-zinc-800/60 text-[10px] text-zinc-600">
          <span><kbd className="font-mono bg-zinc-800 px-1 rounded">↑↓</kbd> navigate</span>
          <span><kbd className="font-mono bg-zinc-800 px-1 rounded">↵</kbd> select</span>
          <span><kbd className="font-mono bg-zinc-800 px-1 rounded">esc</kbd> close</span>
        </div>
      </div>
    </>
  );
}
