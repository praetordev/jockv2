import React, { useState, useRef, useCallback } from 'react';
import { GripVertical, ChevronDown, Play, X } from 'lucide-react';
import type { Commit, RebaseTodoEntry } from '../types';

const ACTIONS: RebaseTodoEntry['action'][] = ['pick', 'squash', 'fixup', 'drop', 'reword'];

const ACTION_COLORS: Record<string, string> = {
  pick: 'text-emerald-400',
  squash: 'text-amber-400',
  fixup: 'text-blue-400',
  drop: 'text-rose-400',
  reword: 'text-purple-400',
};

interface InteractiveRebaseViewProps {
  commits: Commit[];        // commits between HEAD and base (newest-first from graph)
  baseCommit: Commit;
  onExecute: (baseCommit: string, entries: RebaseTodoEntry[]) => void;
  onCancel: () => void;
}

interface RebaseEntry {
  action: RebaseTodoEntry['action'];
  hash: string;
  message: string;
}

export default function InteractiveRebaseView({ commits, baseCommit, onExecute, onCancel }: InteractiveRebaseViewProps) {
  // Reverse: git todo is oldest-first
  const [entries, setEntries] = useState<RebaseEntry[]>(() =>
    [...commits].reverse().map(c => ({ action: 'pick', hash: c.hash, message: c.message }))
  );

  const [dragIdx, setDragIdx] = useState<number | null>(null);
  const [dropIdx, setDropIdx] = useState<number | null>(null);
  const dragRef = useRef<number | null>(null);

  const handleDragStart = useCallback((idx: number) => {
    dragRef.current = idx;
    setDragIdx(idx);
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent, idx: number) => {
    e.preventDefault();
    setDropIdx(idx);
  }, []);

  const handleDrop = useCallback((targetIdx: number) => {
    const sourceIdx = dragRef.current;
    if (sourceIdx === null || sourceIdx === targetIdx) {
      setDragIdx(null);
      setDropIdx(null);
      return;
    }
    setEntries(prev => {
      const next = [...prev];
      const [moved] = next.splice(sourceIdx, 1);
      next.splice(targetIdx, 0, moved);
      return next;
    });
    setDragIdx(null);
    setDropIdx(null);
    dragRef.current = null;
  }, []);

  const handleDragEnd = useCallback(() => {
    setDragIdx(null);
    setDropIdx(null);
    dragRef.current = null;
  }, []);

  const setAction = (idx: number, action: RebaseTodoEntry['action']) => {
    setEntries(prev => prev.map((e, i) => i === idx ? { ...e, action } : e));
  };

  const execute = () => {
    onExecute(baseCommit.hash, entries.map(e => ({
      action: e.action,
      hash: e.hash,
      message: e.message,
    })));
  };

  return (
    <div className="flex-1 flex flex-col bg-zinc-950 overflow-hidden">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-800/60">
        <div className="text-sm text-zinc-300">
          Interactive Rebase — <span className="font-mono text-zinc-500">{baseCommit.hash.slice(0, 7)}</span>{' '}
          <span className="text-zinc-500">({entries.length} commits)</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onCancel}
            className="px-3 py-1 text-xs rounded bg-zinc-800 text-zinc-300 hover:bg-zinc-700 transition-colors flex items-center gap-1"
          >
            <X className="w-3 h-3" /> Cancel
          </button>
          <button
            onClick={execute}
            className="px-3 py-1 text-xs rounded bg-[#F14E32] text-white hover:bg-[#d4432b] transition-colors flex items-center gap-1"
          >
            <Play className="w-3 h-3" /> Execute Rebase
          </button>
        </div>
      </div>

      {/* Entry list */}
      <div className="flex-1 overflow-auto p-2">
        {entries.map((entry, idx) => (
          <div
            key={`${entry.hash}-${idx}`}
            draggable
            onDragStart={() => handleDragStart(idx)}
            onDragOver={(e) => handleDragOver(e, idx)}
            onDrop={() => handleDrop(idx)}
            onDragEnd={handleDragEnd}
            className={`flex items-center gap-2 px-3 py-2 rounded mb-1 transition-colors cursor-grab active:cursor-grabbing
              ${dragIdx === idx ? 'opacity-40' : ''}
              ${dropIdx === idx ? 'bg-[#F14E32]/10 border border-[#F14E32]/30' : 'bg-zinc-900/50 border border-transparent hover:bg-zinc-900'}
              ${entry.action === 'drop' ? 'opacity-50' : ''}`}
          >
            <GripVertical className="w-4 h-4 text-zinc-600 flex-shrink-0" />

            {/* Action dropdown */}
            <div className="relative">
              <select
                value={entry.action}
                onChange={(e) => setAction(idx, e.target.value as RebaseTodoEntry['action'])}
                className={`bg-zinc-800 border border-zinc-700 rounded px-2 py-0.5 text-xs font-mono appearance-none cursor-pointer pr-6 ${ACTION_COLORS[entry.action] || 'text-zinc-300'}`}
              >
                {ACTIONS.map(a => (
                  <option key={a} value={a}>{a}</option>
                ))}
              </select>
              <ChevronDown className="w-3 h-3 absolute right-1.5 top-1/2 -translate-y-1/2 pointer-events-none text-zinc-500" />
            </div>

            {/* Hash */}
            <span className="font-mono text-xs text-zinc-500 w-16 flex-shrink-0">{entry.hash.slice(0, 7)}</span>

            {/* Message */}
            <span className={`text-sm truncate ${entry.action === 'drop' ? 'line-through text-zinc-600' : 'text-zinc-300'}`}>
              {entry.message}
            </span>
          </div>
        ))}
      </div>

      {/* Legend */}
      <div className="px-4 py-2 border-t border-zinc-800/60 flex items-center gap-4 text-[10px] text-zinc-500">
        {ACTIONS.map(a => (
          <span key={a} className={`font-mono ${ACTION_COLORS[a]}`}>{a}</span>
        ))}
        <span className="ml-auto">Drag to reorder</span>
      </div>
    </div>
  );
}
