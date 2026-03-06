import { useState, useCallback } from 'react';

export interface EditorTab {
  id: string;
  filePath: string;
  fileName: string;
  laneName?: string;
  laneId?: string;
}

let nextVexId = 0;

export function useEditor() {
  const [tabs, setTabs] = useState<EditorTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);

  const openFile = useCallback((filePath: string, lane?: { id: string; name: string }) => {
    setTabs(prev => {
      const existing = prev.find(t => t.filePath === filePath);
      if (existing) {
        setActiveTabId(existing.id);
        return prev;
      }
      const id = `vex-${++nextVexId}`;
      const fileName = filePath.split('/').pop() || filePath;
      const newTab: EditorTab = { id, filePath, fileName, laneName: lane?.name, laneId: lane?.id };
      setActiveTabId(id);
      return [...prev, newTab];
    });
  }, []);

  const closeTab = useCallback((id: string) => {
    setTabs(prev => {
      const next = prev.filter(t => t.id !== id);
      setActiveTabId(current =>
        current === id ? (next[next.length - 1]?.id ?? null) : current
      );
      return next;
    });
  }, []);

  const activeTab = tabs.find(t => t.id === activeTabId) ?? null;

  return {
    tabs,
    activeTabId,
    activeTab,
    setActiveTabId,
    openFile,
    closeTab,
  };
}
