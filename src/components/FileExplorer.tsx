import React, { useState, useCallback } from 'react';
import { ChevronRight, ChevronDown, File, Folder, FolderOpen, AlertTriangle } from 'lucide-react';
import { FileNode } from '../hooks/useFileTree';
import { dragPayload } from '../lib/dragState';
import ContextMenu from './ContextMenu';

interface FileExplorerProps {
  tree: FileNode[];
  repoPath: string;
  onFileOpen: (absolutePath: string) => void;
  onQueryFile?: (filePath: string) => void;
  isOnMain?: boolean;
}

function statusColor(status?: string): string {
  if (!status) return '';
  if (status.includes('M')) return 'text-amber-400';
  if (status.includes('A') || status === '??') return 'text-emerald-400';
  if (status.includes('D')) return 'text-rose-400';
  return 'text-zinc-400';
}

function statusBadge(status?: string): string {
  if (!status) return '';
  if (status === '??') return 'U';
  return status.trim().charAt(0);
}

function TreeNode({ node, repoPath, depth, onFileOpen, onContextMenu }: {
  node: FileNode;
  repoPath: string;
  depth: number;
  onFileOpen: (absolutePath: string) => void;
  onContextMenu?: (e: React.MouseEvent, filePath: string) => void;
}) {
  const [isOpen, setIsOpen] = useState(depth < 1);

  if (node.isDirectory) {
    return (
      <div>
        <div
          className="flex items-center py-1 px-2 text-sm cursor-pointer hover:bg-zinc-800/50 text-zinc-400"
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
          onClick={() => setIsOpen(!isOpen)}
        >
          {isOpen ? (
            <ChevronDown className="w-3.5 h-3.5 mr-1 flex-shrink-0" />
          ) : (
            <ChevronRight className="w-3.5 h-3.5 mr-1 flex-shrink-0" />
          )}
          {isOpen ? (
            <FolderOpen className="w-4 h-4 mr-1.5 text-[#F14E32] flex-shrink-0" />
          ) : (
            <Folder className="w-4 h-4 mr-1.5 text-zinc-500 flex-shrink-0" />
          )}
          <span className="truncate">{node.name}</span>
        </div>
        {isOpen && node.children.map(child => (
          <TreeNode
            key={child.path}
            node={child}
            repoPath={repoPath}
            depth={depth + 1}
            onFileOpen={onFileOpen}
            onContextMenu={onContextMenu}
          />
        ))}
      </div>
    );
  }

  const isDraggable = !!node.gitStatus;

  return (
    <div
      draggable={isDraggable}
      onDragStart={isDraggable ? (e) => {
        dragPayload.current = { type: 'file', filePath: node.path, sourceBranchId: null };
        e.dataTransfer.effectAllowed = 'move';
        e.dataTransfer.setData('text/plain', node.path);
      } : undefined}
      className={`flex items-center py-1 px-2 text-sm hover:bg-zinc-800/50 ${statusColor(node.gitStatus) || 'text-zinc-400'} ${isDraggable ? 'cursor-grab active:cursor-grabbing select-none' : 'cursor-default'}`}
      style={{ paddingLeft: `${depth * 16 + 8 + 14}px` }}
      onClick={() => onFileOpen(`${repoPath}/${node.path}`)}
      onContextMenu={(e) => { e.preventDefault(); onContextMenu?.(e, node.path); }}
    >
      <File className="w-4 h-4 mr-1.5 flex-shrink-0 opacity-60 pointer-events-none" />
      <span className="truncate flex-1">{node.name}</span>
      {node.gitStatus && (
        <span className={`text-[10px] font-mono ml-1 ${statusColor(node.gitStatus)}`}>
          {statusBadge(node.gitStatus)}
        </span>
      )}
    </div>
  );
}

export default function FileExplorer({ tree, repoPath, onFileOpen, onQueryFile, isOnMain }: FileExplorerProps) {
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; filePath: string } | null>(null);

  const handleContextMenu = useCallback((e: React.MouseEvent, filePath: string) => {
    setContextMenu({ x: e.clientX, y: e.clientY, filePath });
  }, []);

  return (
    <div className="flex-1 overflow-y-auto py-1">
      {isOnMain && (
        <div className="flex items-center gap-2 text-amber-400 text-xs px-3 py-1.5 mx-2 my-1">
          <AlertTriangle className="w-3.5 h-3.5 flex-shrink-0" />
          <span>Editing on main</span>
        </div>
      )}
      {tree.map(node => (
        <TreeNode
          key={node.path}
          node={node}
          repoPath={repoPath}
          depth={0}
          onFileOpen={onFileOpen}
          onContextMenu={handleContextMenu}
        />
      ))}
      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          items={[
            { label: 'Open in Editor', action: () => onFileOpen(`${repoPath}/${contextMenu.filePath}`) },
            ...(onQueryFile ? [
              { label: 'Query commits for this file', action: () => onQueryFile(contextMenu.filePath) },
              { label: 'Blame this file', action: () => onQueryFile(`blame:${contextMenu.filePath}`) },
            ] : []),
          ]}
          onClose={() => setContextMenu(null)}
        />
      )}
    </div>
  );
}
