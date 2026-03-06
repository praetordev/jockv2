import { useState, useCallback } from 'react';

export interface TerminalEntry {
  id: string;
  label?: string;
  cwd?: string;
}

export function useTerminal() {
  const [terminals, setTerminals] = useState<TerminalEntry[]>([]);
  const [activeTerminalId, setActiveTerminalId] = useState<string | null>(null);

  const createTerminal = useCallback((opts?: { label?: string; cwd?: string }) => {
    const existing = terminals.map(t => {
      const num = parseInt(t.id.split('-')[1], 10);
      return isNaN(num) ? 0 : num;
    });
    const next = existing.length === 0 ? 1 : Math.max(...existing) + 1;
    const id = `terminal-${next}`;
    const entry: TerminalEntry = { id, label: opts?.label, cwd: opts?.cwd };
    setTerminals(prev => [...prev, entry]);
    setActiveTerminalId(id);
    return id;
  }, [terminals]);

  const closeTerminal = useCallback((id: string) => {
    setTerminals(prev => {
      const next = prev.filter(t => t.id !== id);
      setActiveTerminalId(current =>
        current === id ? (next[next.length - 1]?.id ?? null) : current
      );
      return next;
    });
  }, []);

  // Compat: array of just IDs for existing consumers
  const terminalIds = terminals.map(t => t.id);

  return {
    terminals,
    terminalIds,
    activeTerminalId,
    setActiveTerminalId,
    createTerminal,
    closeTerminal,
  };
}
