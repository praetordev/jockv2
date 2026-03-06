import React, { useState, useMemo } from 'react';
import { Search, User, Filter, X } from 'lucide-react';
import type { Commit, CommitFilters } from '../types';
import ContextMenu, { ContextMenuItem } from './ContextMenu';

function estimatePathLength(x1: number, y1: number, x2: number, y2: number): number {
  if (x1 === x2) return Math.abs(y2 - y1);
  const dx = x2 - x1;
  const dy = y2 - y1;
  return Math.ceil(Math.sqrt(dx * dx + dy * dy) * 1.2);
}

interface CommitGraphProps {
  filteredCommits: Commit[];
  graphWidth: number;
  selectedCommit: Commit | null;
  animatedCommitHashes: Set<string>;
  onSelectCommit: (commit: Commit) => void;
  commitSearch: string;
  onSearchChange: (q: string) => void;
  onCherryPick?: (commit: Commit) => void;
  onRevert?: (commit: Commit) => void;
  onCreateTag?: (commit: Commit) => void;
  onInteractiveRebase?: (commit: Commit) => void;
  onQueryFromGraph?: (query: string) => void;
  dslHighlightHashes?: Set<string> | null;
  filters?: CommitFilters;
  onFiltersChange?: (filters: CommitFilters) => void;
}

const ROW_HEIGHT = 48;

export default function CommitGraph({
  filteredCommits,
  graphWidth,
  selectedCommit,
  animatedCommitHashes,
  onSelectCommit,
  commitSearch,
  onSearchChange,
  onCherryPick,
  onRevert,
  onCreateTag,
  onInteractiveRebase,
  onQueryFromGraph,
  dslHighlightHashes,
  filters,
  onFiltersChange,
}: CommitGraphProps) {
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; commit: Commit } | null>(null);
  const [showFilterBar, setShowFilterBar] = useState(false);
  const [filterAuthor, setFilterAuthor] = useState('');
  const [filterAfterDate, setFilterAfterDate] = useState('');
  const [filterBeforeDate, setFilterBeforeDate] = useState('');
  const [filterPath, setFilterPath] = useState('');

  const contextMenuItems: ContextMenuItem[] = contextMenu ? [
    { label: `Cherry-pick ${contextMenu.commit.hash.slice(0, 7)}`, action: () => onCherryPick?.(contextMenu.commit) },
    { label: `Revert ${contextMenu.commit.hash.slice(0, 7)}`, action: () => onRevert?.(contextMenu.commit) },
    { label: 'Create tag here...', action: () => onCreateTag?.(contextMenu.commit) },
    { type: 'separator' as const },
    { label: 'Interactive rebase from here...', action: () => onInteractiveRebase?.(contextMenu.commit) },
    ...(onQueryFromGraph ? [
      { type: 'separator' as const },
      { label: `Query commits by ${contextMenu.commit.author}`, action: () => onQueryFromGraph(`commits | where author == "${contextMenu.commit.author}"`) },
      ...(contextMenu.commit.branches?.length ? [
        { label: `Query commits on ${contextMenu.commit.branches[0]}`, action: () => onQueryFromGraph(`commits[${contextMenu.commit.branches![0]}]`) },
      ] : []),
      { label: 'Query files in this commit', action: () => onQueryFromGraph(`commits | where hash == "${contextMenu.commit.hash.slice(0, 7)}" | select files`) },
    ] : []),
  ] : [];

  const hasActiveFilters = !!(filters?.authorPattern || filters?.grepPattern || filters?.afterDate || filters?.beforeDate || filters?.pathPattern);

  const applyFilters = () => {
    onFiltersChange?.({
      authorPattern: filterAuthor || undefined,
      grepPattern: commitSearch || undefined,
      afterDate: filterAfterDate || undefined,
      beforeDate: filterBeforeDate || undefined,
      pathPattern: filterPath || undefined,
    });
  };

  const clearFilters = () => {
    setFilterAuthor('');
    setFilterAfterDate('');
    setFilterBeforeDate('');
    setFilterPath('');
    onFiltersChange?.({});
  };

  const columns = useMemo(
    () => `${graphWidth}px minmax(100px, 1fr) 120px 80px 120px 90px 72px`,
    [graphWidth],
  );

  return (
    <div className="flex-1 overflow-hidden bg-zinc-950 flex flex-col relative">
      {/* Scrollable area containing header + rows */}
      <div className="flex-1 overflow-auto">
        {/* Column headers */}
        <div
          className="grid gap-3 px-4 py-1.5 border-b border-zinc-800/60 sticky top-0 bg-zinc-950/95 backdrop-blur z-20 text-xs font-semibold text-zinc-500 uppercase tracking-wider"
          style={{ gridTemplateColumns: columns }}
        >
          <div className="text-center">Graph</div>
          <div>Description</div>
          <div>Branches</div>
          <div>Tags</div>
          <div>Author</div>
          <div>Date</div>
          <div>Commit</div>
        </div>

        {/* Rows wrapper — SVG is positioned relative to this */}
        <div className="relative">
          {/* SVG graph lines and nodes */}
          <svg
            className="absolute top-0 left-0 h-full pointer-events-none z-10"
            style={{ width: graphWidth + 16, minHeight: filteredCommits.length * ROW_HEIGHT }}
          >
            {/* Connection paths */}
            {filteredCommits.map((commit, i) => {
              if (!commit.graph) return null;
              const startX = 32 + (commit.graph.column * 24);
              const startY = (i * ROW_HEIGHT) + ROW_HEIGHT / 2;
              return commit.graph.connections.map((conn, connIdx) => {
                const endX = 32 + (conn.toColumn * 24);
                const endY = (conn.toRow * ROW_HEIGHT) + ROW_HEIGHT / 2;
                let path = '';
                if (startX === endX) {
                  path = `M ${startX} ${startY} L ${endX} ${endY}`;
                } else if (endY - startY <= ROW_HEIGHT) {
                  const midY = (startY + endY) / 2;
                  path = `M ${startX} ${startY} C ${startX} ${midY}, ${endX} ${midY}, ${endX} ${endY}`;
                } else {
                  path = `M ${startX} ${startY} C ${startX} ${startY + ROW_HEIGHT * 0.6}, ${endX} ${startY + ROW_HEIGHT * 0.4}, ${endX} ${startY + ROW_HEIGHT} L ${endX} ${endY}`;
                }
                const isNew = animatedCommitHashes.has(commit.hash);
                const len = isNew ? estimatePathLength(startX, startY, endX, endY) : 0;
                return (
                  <path
                    key={`path-${commit.hash}-${connIdx}`}
                    d={path}
                    fill="none"
                    stroke={conn.color}
                    strokeWidth="2"
                    strokeLinecap="round"
                    opacity={dslHighlightHashes && !dslHighlightHashes.has(commit.hash) ? 0.15 : 1}
                    {...(isNew ? {
                      className: 'graph-path-animate',
                      style: {
                        '--path-length': len,
                        strokeDasharray: len,
                        strokeDashoffset: len,
                        animationDelay: `${Math.min(i * 0.06, 1.2)}s`,
                      } as React.CSSProperties,
                    } : {})}
                  />
                );
              });
            })}
            {/* Commit nodes */}
            {filteredCommits.map((commit, i) => {
              if (!commit.graph) return null;
              const cx = 32 + (commit.graph.column * 24);
              const cy = (i * ROW_HEIGHT) + ROW_HEIGHT / 2;
              const isNew = animatedCommitHashes.has(commit.hash);
              return (
                <circle
                  key={`node-${commit.hash}`}
                  cx={cx}
                  cy={cy}
                  r="4.5"
                  fill={commit.graph.color}
                  stroke="#09090b"
                  strokeWidth="2.5"
                  opacity={dslHighlightHashes && !dslHighlightHashes.has(commit.hash) ? 0.25 : 1}
                  {...(isNew ? {
                    className: 'graph-node-animate',
                    style: { animationDelay: `${Math.min(i * 0.06, 1.2) + 0.15}s` },
                  } : {})}
                />
              );
            })}
          </svg>

          {/* Commit rows */}
          {filteredCommits.map((commit, i) => (
            <button
              key={commit.hash}
              onClick={() => onSelectCommit(commit)}
              onContextMenu={(e) => {
                e.preventDefault();
                setContextMenu({ x: e.clientX, y: e.clientY, commit });
              }}
              className={`w-full grid gap-3 px-4 h-12 text-sm border-b border-zinc-800/30 transition-colors items-center relative z-0
                ${selectedCommit?.hash === commit.hash
                  ? 'bg-[#F14E32]/10 border-[#F14E32]/20'
                  : dslHighlightHashes && !dslHighlightHashes.has(commit.hash)
                    ? 'opacity-25'
                    : dslHighlightHashes?.has(commit.hash)
                      ? 'bg-amber-900/10 hover:bg-amber-900/20'
                      : 'hover:bg-zinc-900/50'
                }`}
              style={{ gridTemplateColumns: columns }}
            >
              <div />
              <div className="min-w-0">
                <span className={`block truncate font-medium ${selectedCommit?.hash === commit.hash ? 'text-zinc-100' : 'text-zinc-300'}`}>
                  {commit.message}
                </span>
              </div>
              <div className="flex items-center gap-1 min-w-0 overflow-hidden">
                {commit.branches?.map((b, bIdx) => (
                  <span
                    key={b}
                    className={`px-1.5 py-0.5 rounded text-[10px] font-mono border truncate${animatedCommitHashes.has(commit.hash) ? ' graph-badge-animate' : ''}`}
                    style={{
                      color: commit.graph?.color,
                      borderColor: commit.graph?.color,
                      backgroundColor: `${commit.graph?.color}15`,
                      ...(animatedCommitHashes.has(commit.hash)
                        ? { animationDelay: `${Math.min(i * 0.06, 1.2) + 0.2 + bIdx * 0.1}s` }
                        : {}),
                    }}
                  >
                    {b}
                  </span>
                ))}
              </div>
              <div className="flex items-center gap-1 min-w-0 overflow-hidden">
                {commit.tags?.map(t => (
                  <span key={t} className="px-1.5 py-0.5 rounded text-[10px] font-mono bg-amber-900/20 text-amber-400 border border-amber-700/40 truncate">
                    {t}
                  </span>
                ))}
              </div>
              <div className="flex items-center min-w-0">
                <User className="w-3 h-3 mr-1.5 opacity-50 flex-shrink-0" />
                <span className="truncate text-zinc-400">{commit.author}</span>
              </div>
              <div className="truncate text-zinc-500 text-xs">{commit.date}</div>
              <div className="font-mono text-xs text-zinc-500 truncate">{commit.hash}</div>
            </button>
          ))}
        </div>
      </div>

      {/* Filter bar (expandable) */}
      {showFilterBar && (
        <div className="px-4 py-2 border-t border-zinc-800/60 bg-zinc-900/50 flex flex-wrap gap-2 items-center text-xs">
          <label className="flex items-center gap-1 text-zinc-400">
            Author
            <input
              type="text"
              value={filterAuthor}
              onChange={(e) => setFilterAuthor(e.target.value)}
              placeholder="e.g. John"
              className="bg-zinc-900 border border-zinc-800 rounded px-2 py-0.5 w-28 focus:outline-none focus:border-[#F14E32]"
            />
          </label>
          <label className="flex items-center gap-1 text-zinc-400">
            After
            <input
              type="date"
              value={filterAfterDate}
              onChange={(e) => setFilterAfterDate(e.target.value)}
              className="bg-zinc-900 border border-zinc-800 rounded px-2 py-0.5 w-32 focus:outline-none focus:border-[#F14E32]"
            />
          </label>
          <label className="flex items-center gap-1 text-zinc-400">
            Before
            <input
              type="date"
              value={filterBeforeDate}
              onChange={(e) => setFilterBeforeDate(e.target.value)}
              className="bg-zinc-900 border border-zinc-800 rounded px-2 py-0.5 w-32 focus:outline-none focus:border-[#F14E32]"
            />
          </label>
          <label className="flex items-center gap-1 text-zinc-400">
            Path
            <input
              type="text"
              value={filterPath}
              onChange={(e) => setFilterPath(e.target.value)}
              placeholder="e.g. src/"
              className="bg-zinc-900 border border-zinc-800 rounded px-2 py-0.5 w-28 focus:outline-none focus:border-[#F14E32]"
            />
          </label>
          <button onClick={applyFilters} className="px-2 py-0.5 bg-[#F14E32] text-white rounded hover:bg-[#d4432b] transition-colors">Apply</button>
          <button onClick={clearFilters} className="px-2 py-0.5 bg-zinc-800 text-zinc-300 rounded hover:bg-zinc-700 transition-colors">Clear</button>
        </div>
      )}

      {/* Search bar */}
      <div className="flex items-center justify-between px-4 py-1.5 border-t border-zinc-800/60">
        <div className="flex items-center gap-2">
          {hasActiveFilters && (
            <div className="flex items-center gap-1 text-xs">
              {filters?.authorPattern && (
                <span className="px-1.5 py-0.5 rounded bg-[#F14E32]/10 text-[#F14E32] border border-[#F14E32]/30 flex items-center gap-1">
                  author: {filters.authorPattern}
                  <X className="w-3 h-3 cursor-pointer" onClick={() => onFiltersChange?.({ ...filters, authorPattern: undefined })} />
                </span>
              )}
              {filters?.afterDate && (
                <span className="px-1.5 py-0.5 rounded bg-[#F14E32]/10 text-[#F14E32] border border-[#F14E32]/30 flex items-center gap-1">
                  after: {filters.afterDate}
                  <X className="w-3 h-3 cursor-pointer" onClick={() => onFiltersChange?.({ ...filters, afterDate: undefined })} />
                </span>
              )}
              {filters?.beforeDate && (
                <span className="px-1.5 py-0.5 rounded bg-[#F14E32]/10 text-[#F14E32] border border-[#F14E32]/30 flex items-center gap-1">
                  before: {filters.beforeDate}
                  <X className="w-3 h-3 cursor-pointer" onClick={() => onFiltersChange?.({ ...filters, beforeDate: undefined })} />
                </span>
              )}
              {filters?.pathPattern && (
                <span className="px-1.5 py-0.5 rounded bg-[#F14E32]/10 text-[#F14E32] border border-[#F14E32]/30 flex items-center gap-1">
                  path: {filters.pathPattern}
                  <X className="w-3 h-3 cursor-pointer" onClick={() => onFiltersChange?.({ ...filters, pathPattern: undefined })} />
                </span>
              )}
            </div>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowFilterBar(p => !p)}
            className={`p-1 rounded transition-colors ${showFilterBar || hasActiveFilters ? 'text-[#F14E32]' : 'text-zinc-500 hover:text-zinc-300'}`}
            title="Toggle filters"
          >
            <Filter className="w-3.5 h-3.5" />
          </button>
          <div className="relative">
            <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
            <input
              type="text"
              value={commitSearch}
              onChange={(e) => onSearchChange(e.target.value)}
              placeholder="Search commits..."
              className="bg-zinc-900 border border-zinc-800 rounded-md pl-8 pr-3 py-1 text-xs w-52 focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] transition-all"
            />
          </div>
        </div>
      </div>

      {/* Context menu */}
      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          items={contextMenuItems}
          onClose={() => setContextMenu(null)}
        />
      )}
    </div>
  );
}
