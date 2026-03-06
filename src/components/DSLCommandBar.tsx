import { useState, useRef, useEffect, useCallback } from 'react';
import type { DSLResult, DSLSuggestion } from '../types';
import DSLResultView from './DSLResultView';
import DSLHighlighter from './DSLHighlighter';

interface DSLCommandBarProps {
  onCommitClick?: (hash: string) => void;
  onQueryResultHashes?: (hashes: Set<string> | null) => void;
  initialQuery?: string;
  onInitialQueryConsumed?: () => void;
}

interface HistoryEntry {
  query: string;
  result: DSLResult;
}

export default function DSLCommandBar({ onCommitClick, onQueryResultHashes, initialQuery, onInitialQueryConsumed }: DSLCommandBarProps) {
  const [query, setQuery] = useState('');
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const [suggestions, setSuggestions] = useState<DSLSuggestion[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedSuggestion, setSelectedSuggestion] = useState(0);
  const [historyIdx, setHistoryIdx] = useState(-1);
  const [queryHistory, setQueryHistory] = useState<string[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);
  const outputRef = useRef<HTMLDivElement>(null);
  const autocompleteTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Load persisted query history on mount
  useEffect(() => {
    window.electronAPI.invoke('dsl:get-history')
      .then((h: string[]) => { if (Array.isArray(h)) setQueryHistory(h); })
      .catch(() => {});
  }, []);

  // Debounced autocomplete
  useEffect(() => {
    if (autocompleteTimer.current) {
      clearTimeout(autocompleteTimer.current);
    }

    if (query.length === 0) {
      setSuggestions([]);
      setShowSuggestions(false);
      return;
    }

    autocompleteTimer.current = setTimeout(async () => {
      try {
        const cursorPos = inputRef.current?.selectionStart ?? query.length;
        const result = await window.electronAPI.invoke('dsl:autocomplete', query, cursorPos);
        const items = result?.suggestions ?? [];
        setSuggestions(items);
        setShowSuggestions(items.length > 0);
        setSelectedSuggestion(0);
      } catch {
        setSuggestions([]);
        setShowSuggestions(false);
      }
    }, 150);

    return () => {
      if (autocompleteTimer.current) {
        clearTimeout(autocompleteTimer.current);
      }
    };
  }, [query]);

  const execute = useCallback(async (input: string) => {
    const trimmed = input.trim();
    if (!trimmed) return;

    setShowSuggestions(false);

    // Shell escape: ! prefix delegates to shell:exec
    if (trimmed.startsWith('!')) {
      const cmd = trimmed.slice(1).trim();
      const shellResult = await window.electronAPI.invoke('shell:exec', cmd);
      setHistory((prev) => [
        ...prev,
        {
          query: trimmed,
          result: {
            resultKind: shellResult.stderr && !shellResult.stdout ? 'error' : 'commits',
            error: shellResult.stderr || undefined,
            commits: shellResult.stdout
              ? [{ hash: '', message: shellResult.stdout, author: '', date: '', additions: 0, deletions: 0 }]
              : undefined,
          },
        },
      ]);
      onQueryResultHashes?.(null);
    } else {
      try {
        const result = await window.electronAPI.invoke('dsl:execute', trimmed, false);
        setHistory((prev) => [...prev, { query: trimmed, result }]);
        if (result.resultKind === 'commits' && result.commits?.length) {
          onQueryResultHashes?.(new Set(result.commits.map((c: { hash: string }) => c.hash)));
        } else {
          onQueryResultHashes?.(null);
        }
      } catch (err: any) {
        setHistory((prev) => [
          ...prev,
          { query: trimmed, result: { resultKind: 'error', error: err.message || String(err) } },
        ]);
        onQueryResultHashes?.(null);
      }
    }

    setQuery('');
    setHistoryIdx(-1);
    // Persist to disk
    window.electronAPI.invoke('dsl:add-history', trimmed)
      .then((h: string[]) => { if (Array.isArray(h)) setQueryHistory(h); })
      .catch(() => {});
    setTimeout(() => outputRef.current?.scrollTo(0, outputRef.current.scrollHeight), 50);
  }, []);

  // Handle initialQuery from external triggers (e.g., file tree context menu)
  useEffect(() => {
    if (initialQuery) {
      execute(initialQuery);
      onInitialQueryConsumed?.();
    }
  }, [initialQuery, execute, onInitialQueryConsumed]);

  const acceptSuggestion = useCallback(
    (suggestion: DSLSuggestion) => {
      // Insert suggestion at cursor position, replacing the current partial word
      const cursorPos = inputRef.current?.selectionStart ?? query.length;
      const before = query.slice(0, cursorPos);

      // Find the start of the current word
      const wordStart = Math.max(before.lastIndexOf(' '), before.lastIndexOf('|'), before.lastIndexOf('(')) + 1;
      const newQuery = before.slice(0, wordStart) + suggestion.text + ' ' + query.slice(cursorPos);

      setQuery(newQuery);
      setShowSuggestions(false);
      inputRef.current?.focus();
    },
    [query],
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      // Autocomplete navigation
      if (showSuggestions && suggestions.length > 0) {
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          setSelectedSuggestion((prev) => (prev + 1) % suggestions.length);
          return;
        }
        if (e.key === 'ArrowUp') {
          e.preventDefault();
          setSelectedSuggestion((prev) => (prev - 1 + suggestions.length) % suggestions.length);
          return;
        }
        if (e.key === 'Tab') {
          e.preventDefault();
          acceptSuggestion(suggestions[selectedSuggestion]);
          return;
        }
        if (e.key === 'Escape') {
          e.preventDefault();
          setShowSuggestions(false);
          return;
        }
      }

      // Command history navigation (uses persisted queryHistory)
      if (e.key === 'ArrowUp' && !showSuggestions) {
        e.preventDefault();
        if (queryHistory.length === 0) return;
        const newIdx = historyIdx === -1 ? 0 : Math.min(queryHistory.length - 1, historyIdx + 1);
        setHistoryIdx(newIdx);
        setQuery(queryHistory[newIdx]);
      }
      if (e.key === 'ArrowDown' && !showSuggestions) {
        e.preventDefault();
        if (historyIdx === -1) return;
        if (historyIdx <= 0) {
          setHistoryIdx(-1);
          setQuery('');
        } else {
          const newIdx = historyIdx - 1;
          setHistoryIdx(newIdx);
          setQuery(queryHistory[newIdx]);
        }
      }
    },
    [showSuggestions, suggestions, selectedSuggestion, queryHistory, historyIdx, acceptSuggestion],
  );

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      {/* Result history */}
      <div ref={outputRef} className="flex-1 overflow-y-auto p-3 font-mono text-xs space-y-3">
        {history.map((entry, i) => (
          <DSLResultView key={i} query={entry.query} result={entry.result} onCommitClick={onCommitClick} />
        ))}
        {history.length === 0 && (
          <div className="text-zinc-600 italic">
            Type a query like: commits | where author == &quot;name&quot; | first 10
          </div>
        )}
      </div>

      {/* Autocomplete dropdown */}
      {showSuggestions && suggestions.length > 0 && (
        <div className="border-t border-zinc-800/60 bg-zinc-900 max-h-32 overflow-y-auto">
          {suggestions.map((s, i) => (
            <div
              key={i}
              className={`px-3 py-1 text-xs flex items-center gap-2 cursor-pointer ${
                i === selectedSuggestion ? 'bg-zinc-800 text-zinc-100' : 'hover:bg-zinc-800/50'
              }`}
              onMouseDown={(e) => {
                e.preventDefault();
                acceptSuggestion(s);
              }}
            >
              <span className="text-zinc-500 w-16 flex-shrink-0">{s.kind}</span>
              <span className="text-zinc-200">{s.text}</span>
              <span className="text-zinc-600 ml-auto">{s.description}</span>
            </div>
          ))}
        </div>
      )}

      {/* Input */}
      <form
        className="flex items-center border-t border-zinc-800/60 px-3 py-2 gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          execute(query);
        }}
      >
        <span className="text-[#F14E32] font-mono text-xs">jock&gt;</span>
        <div className="flex-1 relative">
          {/* Syntax-highlighted overlay (behind input) */}
          <div className="absolute inset-0 pointer-events-none overflow-hidden">
            <DSLHighlighter value={query} />
          </div>
          {/* Transparent input on top */}
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder='commits | where author == "name" | first 10'
            className="relative w-full bg-transparent text-xs font-mono text-transparent caret-zinc-200 focus:outline-none placeholder-zinc-600"
            style={{ caretColor: '#e4e4e7' }}
            autoFocus
          />
        </div>
      </form>
    </div>
  );
}
