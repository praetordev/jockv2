import { useState, useCallback, useEffect } from 'react';

const isElectron = typeof window !== 'undefined' && !!window.electronAPI;

export interface FileNode {
  name: string;
  path: string;
  isDirectory: boolean;
  children: FileNode[];
  gitStatus?: string;
}

function buildTree(files: string[], statusMap: Record<string, string>): FileNode[] {
  const root: FileNode[] = [];

  for (const filePath of files) {
    const parts = filePath.split('/');
    let current = root;

    for (let i = 0; i < parts.length; i++) {
      const name = parts[i];
      const partialPath = parts.slice(0, i + 1).join('/');
      const isLast = i === parts.length - 1;

      let existing = current.find(n => n.name === name);
      if (!existing) {
        existing = {
          name,
          path: partialPath,
          isDirectory: !isLast,
          children: [],
          gitStatus: isLast ? statusMap[filePath] : undefined,
        };
        current.push(existing);
      }
      current = existing.children;
    }
  }

  function sortNodes(nodes: FileNode[]): void {
    nodes.sort((a, b) => {
      if (a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    nodes.forEach(n => sortNodes(n.children));
  }
  sortNodes(root);

  return root;
}

export function useFileTree(repoPath: string | null) {
  const [tree, setTree] = useState<FileNode[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    if (!isElectron || !repoPath) {
      setTree([]);
      return;
    }
    setLoading(true);
    try {
      const [fileResult, statusMap] = await Promise.all([
        window.electronAPI.invoke('files:list-tree'),
        window.electronAPI.invoke('files:git-status-map'),
      ]);
      const built = buildTree(fileResult.files, statusMap as Record<string, string>);
      setTree(built);
    } catch {
      setTree([]);
    } finally {
      setLoading(false);
    }
  }, [repoPath]);

  useEffect(() => { refresh(); }, [refresh]);

  return { tree, loading, refresh };
}
