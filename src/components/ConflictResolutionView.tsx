import React, { useRef, useCallback, useMemo } from 'react';
import { Check, GitBranch, FileEdit, ChevronLeft, ChevronRight } from 'lucide-react';
import { ConflictDetail, ConflictStrategy } from '../types';

interface ConflictResolutionViewProps {
  detail: ConflictDetail;
  onResolve: (strategy: ConflictStrategy) => void;
  onOpenInEditor: () => void;
  resolving: boolean;
  mergeBranch: string;
  conflictFiles?: string[];
  currentFile?: string;
  onNavigateConflict?: (filePath: string) => void;
}

function LineNumberedPre({
  content,
  scrollRef,
  onScroll,
}: {
  content: string;
  scrollRef: React.RefObject<HTMLDivElement | null>;
  onScroll: () => void;
}) {
  const lines = useMemo(() => (content || '(empty)').split('\n'), [content]);
  return (
    <div
      ref={scrollRef}
      onScroll={onScroll}
      className="flex-1 overflow-auto font-mono text-sm leading-relaxed"
    >
      <table className="border-collapse">
        <tbody>
          {lines.map((line, i) => (
            <tr key={i} className="hover:bg-zinc-800/30">
              <td className="px-2 text-right text-zinc-600 select-none w-10 align-top whitespace-nowrap border-r border-zinc-800/40">
                {i + 1}
              </td>
              <td className="px-3 text-zinc-300 whitespace-pre">{line}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default function ConflictResolutionView({
  detail,
  onResolve,
  onOpenInEditor,
  resolving,
  mergeBranch,
  conflictFiles,
  currentFile,
  onNavigateConflict,
}: ConflictResolutionViewProps) {
  const oursRef = useRef<HTMLDivElement>(null);
  const theirsRef = useRef<HTMLDivElement>(null);
  const syncing = useRef(false);

  const handleScroll = useCallback((source: 'ours' | 'theirs') => {
    if (syncing.current) return;
    syncing.current = true;
    const from = source === 'ours' ? oursRef.current : theirsRef.current;
    const to = source === 'ours' ? theirsRef.current : oursRef.current;
    if (from && to) {
      to.scrollTop = from.scrollTop;
      to.scrollLeft = from.scrollLeft;
    }
    syncing.current = false;
  }, []);

  const currentIndex = conflictFiles && currentFile ? conflictFiles.indexOf(currentFile) : -1;
  const hasPrev = currentIndex > 0;
  const hasNext = conflictFiles ? currentIndex < conflictFiles.length - 1 : false;

  return (
    <div className="flex-1 flex flex-col min-w-0">
      {/* Toolbar */}
      <div className="h-10 border-b border-zinc-800/60 flex items-center px-4 bg-zinc-900/40 gap-2 flex-shrink-0">
        {conflictFiles && conflictFiles.length > 1 && onNavigateConflict && (
          <div className="flex items-center gap-1 mr-2">
            <button
              onClick={() => hasPrev && onNavigateConflict(conflictFiles[currentIndex - 1])}
              disabled={!hasPrev}
              className="p-0.5 rounded text-zinc-500 hover:text-zinc-300 disabled:opacity-30 disabled:cursor-not-allowed"
              title="Previous conflict file"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <span className="text-[11px] text-zinc-500 tabular-nums">
              {currentIndex + 1}/{conflictFiles.length}
            </span>
            <button
              onClick={() => hasNext && onNavigateConflict(conflictFiles[currentIndex + 1])}
              disabled={!hasNext}
              className="p-0.5 rounded text-zinc-500 hover:text-zinc-300 disabled:opacity-30 disabled:cursor-not-allowed"
              title="Next conflict file"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        )}
        <span className="text-sm font-mono text-zinc-300 truncate flex-1">
          {detail.path}
        </span>
        <button
          onClick={() => onResolve('ours')}
          disabled={resolving}
          className="flex items-center px-2 py-1 text-xs rounded transition-colors bg-blue-500/10 text-blue-400 hover:bg-blue-500/20 border border-blue-500/20 disabled:opacity-40"
        >
          <Check className="w-3 h-3 mr-1" />
          Accept Current
        </button>
        <button
          onClick={() => onResolve('theirs')}
          disabled={resolving}
          className="flex items-center px-2 py-1 text-xs rounded transition-colors bg-emerald-500/10 text-emerald-400 hover:bg-emerald-500/20 border border-emerald-500/20 disabled:opacity-40"
        >
          <Check className="w-3 h-3 mr-1" />
          Accept Incoming
        </button>
        <button
          onClick={() => onResolve('both')}
          disabled={resolving}
          className="flex items-center px-2 py-1 text-xs rounded transition-colors bg-zinc-700/30 text-zinc-400 hover:bg-zinc-700/50 border border-zinc-600/30 disabled:opacity-40"
        >
          Accept Both
        </button>
        <button
          onClick={onOpenInEditor}
          className="flex items-center px-2 py-1 text-xs rounded transition-colors text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800"
        >
          <FileEdit className="w-3 h-3 mr-1" />
          Open in Editor
        </button>
      </div>

      {/* Side-by-side content */}
      <div className="flex-1 flex min-h-0">
        {/* Ours (Current / HEAD) */}
        <div className="flex-1 flex flex-col min-w-0 border-r border-zinc-800/60">
          <div className="h-8 flex items-center px-3 bg-blue-500/5 border-b border-blue-500/20 flex-shrink-0">
            <GitBranch className="w-3 h-3 mr-1.5 text-blue-400" />
            <span className="text-xs font-medium text-blue-400">Current (HEAD)</span>
          </div>
          <LineNumberedPre
            content={detail.oursContent}
            scrollRef={oursRef}
            onScroll={() => handleScroll('ours')}
          />
        </div>

        {/* Theirs (Incoming / merge branch) */}
        <div className="flex-1 flex flex-col min-w-0">
          <div className="h-8 flex items-center px-3 bg-emerald-500/5 border-b border-emerald-500/20 flex-shrink-0">
            <GitBranch className="w-3 h-3 mr-1.5 text-emerald-400" />
            <span className="text-xs font-medium text-emerald-400">
              Incoming{mergeBranch ? ` (${mergeBranch})` : ''}
            </span>
          </div>
          <LineNumberedPre
            content={detail.theirsContent}
            scrollRef={theirsRef}
            onScroll={() => handleScroll('theirs')}
          />
        </div>
      </div>
    </div>
  );
}
