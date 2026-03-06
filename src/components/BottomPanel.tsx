import React from 'react';
import {
  Terminal as TerminalIcon, Search, Plus, X, BookOpen,
} from 'lucide-react';
import Terminal from './Terminal';
import DSLCommandBar from './DSLCommandBar';
import DSLExplorer from './DSLExplorer';
import type { TerminalEntry } from '../hooks/useTerminal';
import type { Commit } from '../types';

interface BottomPanelProps {
  settings: { editor: { fontSize: number; fontFamily: string; cursorBlink: boolean } };
  commits: Commit[];
  onCommitSelect: (commit: Commit) => void;
  terminals: TerminalEntry[];
  activeTerminalId: string | null;
  setActiveTerminalId: (id: string) => void;
  createTerminal: (opts?: { label?: string; cwd?: string }) => string;
  closeTerminal: (id: string) => void;
  bottomMode: 'terminal' | 'query' | 'docs';
  setBottomMode: (mode: 'terminal' | 'query' | 'docs') => void;
  initialQuery?: string;
  onInitialQueryConsumed?: () => void;
  onRunDSLQuery?: (query: string) => void;
  onDSLResultHashes?: (hashes: Set<string> | null) => void;
}

export default function BottomPanel({ settings, commits, onCommitSelect, terminals, activeTerminalId, setActiveTerminalId, createTerminal, closeTerminal, bottomMode, setBottomMode, initialQuery, onInitialQueryConsumed, onRunDSLQuery, onDSLResultHashes }: BottomPanelProps) {

  const hasExtraTabs = terminals.length > 0;
  const showingExtra = hasExtraTabs && bottomMode === 'terminal';

  return (
    <div className="h-1/4 min-h-[200px] border-t border-zinc-800/60 bg-zinc-900/30 flex flex-col">
      {/* Tab bar */}
      <div className="h-9 flex items-center border-b border-zinc-800/60 bg-zinc-900/40 px-2 gap-1 flex-shrink-0">
        <button
          onClick={() => setBottomMode('terminal')}
          className={`flex items-center px-2 py-1 text-xs font-medium rounded transition-colors gap-1 ${
            bottomMode === 'terminal' ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
          }`}
        >
          <TerminalIcon className="w-3 h-3" />
          Terminal
        </button>
        <button
          onClick={() => setBottomMode('query')}
          className={`flex items-center px-2 py-1 text-xs font-medium rounded transition-colors gap-1 ${
            bottomMode === 'query' ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
          }`}
        >
          <Search className="w-3 h-3" />
          Query
        </button>
        <button
          onClick={() => setBottomMode('docs')}
          className={`flex items-center px-2 py-1 text-xs font-medium rounded transition-colors gap-1 ${
            bottomMode === 'docs' ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
          }`}
        >
          <BookOpen className="w-3 h-3" />
          Docs
        </button>
        {showingExtra && (
          <>
            <div className="w-px h-3.5 bg-zinc-800 mx-1" />
            {terminals.map(t => (
              <button
                key={t.id}
                onClick={() => setActiveTerminalId(t.id)}
                className={`flex items-center px-3 py-1 text-xs font-medium rounded transition-colors gap-1.5 ${
                  activeTerminalId === t.id ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
                }`}
              >
                {t.label || `Terminal ${t.id.split('-')[1]}`}
                <X
                  className="w-3 h-3 hover:text-rose-400"
                  onClick={(e) => { e.stopPropagation(); closeTerminal(t.id); }}
                />
              </button>
            ))}
            <button
              onClick={() => createTerminal()}
              className="flex items-center gap-1.5 px-3 py-1 text-xs font-medium text-zinc-500 hover:text-zinc-300 transition-colors rounded hover:bg-zinc-800/50"
              title="New Terminal"
            >
              <Plus className="w-3 h-3" />
            </button>
          </>
        )}
      </div>

      {bottomMode === 'terminal' && (
        <div className="flex-1 min-h-0 relative">
          {/* Default terminal — always present */}
          <div
            className="absolute inset-0"
            style={{ display: !activeTerminalId ? 'block' : 'none' }}
          >
            <Terminal
              terminalId="default-terminal"
              isVisible={bottomMode === 'terminal' && !activeTerminalId}
              fontSize={settings.editor.fontSize}
              fontFamily={settings.editor.fontFamily}
              cursorBlink={settings.editor.cursorBlink}
              onNewTerminal={createTerminal}
            />
          </div>
          {/* Extra terminals from tabs */}
          {terminals.map(t => (
            <div
              key={t.id}
              className="absolute inset-0"
              style={{ display: activeTerminalId === t.id ? 'block' : 'none' }}
            >
              <Terminal
                terminalId={t.id}
                isVisible={activeTerminalId === t.id}
                fontSize={settings.editor.fontSize}
                fontFamily={settings.editor.fontFamily}
                cursorBlink={settings.editor.cursorBlink}
                cwd={t.cwd}
                onNewTerminal={createTerminal}
              />
            </div>
          ))}
        </div>
      )}

      {bottomMode === 'query' && (
        <DSLCommandBar
          onCommitClick={(hash) => {
            const commit = commits.find(c => c.hash.startsWith(hash));
            if (commit) onCommitSelect(commit);
          }}
          onQueryResultHashes={onDSLResultHashes}
          initialQuery={initialQuery}
          onInitialQueryConsumed={onInitialQueryConsumed}
        />
      )}

      {bottomMode === 'docs' && (
        <DSLExplorer
          onRunQuery={(query) => {
            if (onRunDSLQuery) {
              onRunDSLQuery(query);
            } else {
              setBottomMode('query');
            }
          }}
        />
      )}
    </div>
  );
}
