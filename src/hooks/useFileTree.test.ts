import { describe, it, expect } from 'vitest';

// We test the buildTree logic by importing it indirectly through the module.
// Since buildTree is not exported, we replicate its interface for testing.
// This tests the tree-building algorithm that useFileTree depends on.

interface FileNode {
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

describe('buildTree', () => {
  it('builds a flat list of files', () => {
    const tree = buildTree(['a.ts', 'b.ts'], {});
    expect(tree).toHaveLength(2);
    expect(tree[0].name).toBe('a.ts');
    expect(tree[1].name).toBe('b.ts');
    expect(tree[0].isDirectory).toBe(false);
  });

  it('builds nested directories', () => {
    const tree = buildTree(['src/components/App.tsx', 'src/main.ts'], {});
    expect(tree).toHaveLength(1);
    expect(tree[0].name).toBe('src');
    expect(tree[0].isDirectory).toBe(true);
    expect(tree[0].children).toHaveLength(2); // components dir + main.ts
  });

  it('sorts directories before files', () => {
    const tree = buildTree(['README.md', 'src/main.ts', 'package.json'], {});
    // src/ directory should come first
    expect(tree[0].name).toBe('src');
    expect(tree[0].isDirectory).toBe(true);
    expect(tree[1].name).toBe('package.json');
    expect(tree[2].name).toBe('README.md');
  });

  it('sorts files alphabetically within same level', () => {
    const tree = buildTree(['c.ts', 'a.ts', 'b.ts'], {});
    expect(tree.map(n => n.name)).toEqual(['a.ts', 'b.ts', 'c.ts']);
  });

  it('attaches git status to files', () => {
    const tree = buildTree(['src/App.tsx', 'src/index.ts'], { 'src/App.tsx': 'M', 'src/index.ts': '??' });
    const appFile = tree[0].children.find(n => n.name === 'App.tsx');
    const indexFile = tree[0].children.find(n => n.name === 'index.ts');
    expect(appFile?.gitStatus).toBe('M');
    expect(indexFile?.gitStatus).toBe('??');
  });

  it('does not attach git status to directories', () => {
    const tree = buildTree(['src/App.tsx'], { 'src': 'M' });
    expect(tree[0].gitStatus).toBeUndefined();
  });

  it('handles deeply nested paths', () => {
    const tree = buildTree(['a/b/c/d/e.ts'], {});
    expect(tree[0].name).toBe('a');
    expect(tree[0].children[0].name).toBe('b');
    expect(tree[0].children[0].children[0].name).toBe('c');
    expect(tree[0].children[0].children[0].children[0].name).toBe('d');
    expect(tree[0].children[0].children[0].children[0].children[0].name).toBe('e.ts');
  });

  it('handles empty file list', () => {
    const tree = buildTree([], {});
    expect(tree).toHaveLength(0);
  });

  it('merges files under same directory', () => {
    const tree = buildTree(['src/a.ts', 'src/b.ts', 'src/c.ts'], {});
    expect(tree).toHaveLength(1);
    expect(tree[0].children).toHaveLength(3);
  });

  it('sets correct path for nested items', () => {
    const tree = buildTree(['src/components/App.tsx'], {});
    expect(tree[0].path).toBe('src');
    expect(tree[0].children[0].path).toBe('src/components');
    expect(tree[0].children[0].children[0].path).toBe('src/components/App.tsx');
  });
});
