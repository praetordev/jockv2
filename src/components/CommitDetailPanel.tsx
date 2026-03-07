import { useState, useEffect } from 'react';
import { X, GitCommit, User, Clock, GitBranch, Tag, FileEdit, FilePlus, FileMinus, ChevronLeft } from 'lucide-react';
import DiffViewer from './DiffViewer';
import type { Commit, FileChange } from '../types';

interface CommitDetailPanelProps {
  commit: Commit;
  fileChanges: FileChange[];
  onClose: () => void;
}

export default function CommitDetailPanel({ commit, fileChanges, onClose }: CommitDetailPanelProps) {
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [diffPatch, setDiffPatch] = useState<string | null>(null);
  const [diffLoading, setDiffLoading] = useState(false);

  // Reset selection when commit changes
  useEffect(() => {
    setSelectedFile(null);
    setDiffPatch(null);
  }, [commit.hash]);

  const handleFileClick = async (filePath: string) => {
    if (selectedFile === filePath) {
      setSelectedFile(null);
      setDiffPatch(null);
      return;
    }
    setSelectedFile(filePath);
    setDiffPatch(null);
    setDiffLoading(true);
    try {
      const patch = await window.electronAPI.invoke('git:get-file-diff', commit.hash, filePath);
      setDiffPatch(patch || 'No diff available');
    } catch {
      setDiffPatch('Failed to load diff');
    } finally {
      setDiffLoading(false);
    }
  };

  return (
    <div className="w-80 border-l border-zinc-800/60 bg-zinc-950 flex flex-col overflow-hidden animate-slide-in-right">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-zinc-800/60">
        <span className="text-xs font-medium text-zinc-300">
          {selectedFile ? selectedFile.split('/').pop() : 'Commit Details'}
        </span>
        <div className="flex items-center gap-1">
          {selectedFile && (
            <button
              onClick={() => { setSelectedFile(null); setDiffPatch(null); }}
              className="p-0.5 rounded hover:bg-zinc-800 text-zinc-500 hover:text-zinc-300"
              aria-label="Back to details"
            >
              <ChevronLeft size={14} />
            </button>
          )}
          <button
            onClick={onClose}
            className="p-0.5 rounded hover:bg-zinc-800 text-zinc-500 hover:text-zinc-300"
            aria-label="Close commit details"
          >
            <X size={14} />
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {selectedFile ? (
          /* Diff view */
          <div className="min-w-0">
            {diffLoading ? (
              <div className="p-3 text-xs text-zinc-500 italic">Loading diff...</div>
            ) : diffPatch ? (
              <DiffViewer patch={diffPatch} filePath={selectedFile} showToolbar={false} />
            ) : null}
          </div>
        ) : (
          /* Details view */
          <div className="p-3 space-y-4 text-xs">
            {/* Hash */}
            <div className="flex items-center gap-2">
              <GitCommit size={12} className="text-zinc-500 flex-shrink-0" />
              <span className="text-amber-400 font-mono select-all">{commit.hash}</span>
            </div>

            {/* Author */}
            <div className="flex items-center gap-2">
              <User size={12} className="text-zinc-500 flex-shrink-0" />
              <span className="text-cyan-400">{commit.author}</span>
            </div>

            {/* Date */}
            <div className="flex items-center gap-2">
              <Clock size={12} className="text-zinc-500 flex-shrink-0" />
              <span className="text-zinc-400">{commit.date}</span>
            </div>

            {/* Message */}
            <div className="bg-zinc-900 rounded px-2 py-1.5 text-zinc-300 whitespace-pre-wrap break-words">
              {commit.message}
            </div>

            {/* Branches */}
            {commit.branches && commit.branches.length > 0 && (
              <div className="flex items-start gap-2">
                <GitBranch size={12} className="text-zinc-500 flex-shrink-0 mt-0.5" />
                <div className="flex flex-wrap gap-1">
                  {commit.branches.map((b) => (
                    <span key={b} className="text-emerald-400 bg-emerald-400/10 px-1.5 py-0.5 rounded text-[10px]">
                      {b}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {/* Tags */}
            {commit.tags && commit.tags.length > 0 && (
              <div className="flex items-start gap-2">
                <Tag size={12} className="text-zinc-500 flex-shrink-0 mt-0.5" />
                <div className="flex flex-wrap gap-1">
                  {commit.tags.map((t) => (
                    <span key={t} className="text-yellow-400 bg-yellow-400/10 px-1.5 py-0.5 rounded text-[10px]">
                      {t}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {/* Parents */}
            {commit.parents && commit.parents.length > 0 && (
              <div>
                <div className="text-zinc-500 mb-1">Parents</div>
                {commit.parents.map((p) => (
                  <div key={p} className="text-amber-400/70 font-mono text-[10px]">{p.slice(0, 12)}</div>
                ))}
              </div>
            )}

            {/* File Changes */}
            {fileChanges.length > 0 && (
              <div>
                <div className="text-zinc-500 mb-1">Changed Files ({fileChanges.length})</div>
                <div className="space-y-0.5">
                  {fileChanges.map((fc) => (
                    <button
                      key={fc.path}
                      onClick={() => handleFileClick(fc.path)}
                      className="w-full flex items-center gap-1.5 px-1.5 py-1 rounded hover:bg-zinc-800/50 text-left group"
                    >
                      <FileStatusIcon status={fc.status} />
                      <span className="text-zinc-300 truncate flex-1 group-hover:text-zinc-100">{fc.path}</span>
                      <span className="text-emerald-500 text-[10px]">+{fc.additions}</span>
                      <span className="text-rose-500 text-[10px]">-{fc.deletions}</span>
                    </button>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

function FileStatusIcon({ status }: { status: string }) {
  switch (status) {
    case 'A':
      return <FilePlus size={12} className="text-emerald-400 flex-shrink-0" />;
    case 'D':
      return <FileMinus size={12} className="text-rose-400 flex-shrink-0" />;
    default:
      return <FileEdit size={12} className="text-amber-400 flex-shrink-0" />;
  }
}
