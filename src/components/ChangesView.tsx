import { useState, useCallback } from 'react';
import { FileEdit, FilePlus, FileMinus, X, User } from 'lucide-react';
import ConflictResolutionView from './ConflictResolutionView';
import DiffViewer from './DiffViewer';
import type { DiffViewMode, DiffAnnotation } from './DiffViewer';
import type { FileChange } from '../types';
import type { BlameLine, useSourceControl, useMergeConflicts } from '../hooks/useGitData';
import { parseDiff, buildHunkPatch } from '../utils/diffParser';

interface ChangesViewProps {
  fileChanges: FileChange[];
  selectedFile: FileChange | null;
  onSelectFile: (file: FileChange) => void;
  sourceControl: ReturnType<typeof useSourceControl>;
  mergeConflicts: ReturnType<typeof useMergeConflicts>;
  blameLines: BlameLine[];
  blameFile: string | null;
  blameLoading: boolean;
  fetchBlame: (path: string) => void;
  clearBlame: () => void;
  selectedStashDiff: string | null;
  onCloseStashDiff: () => void;
  onOpenInEditor: (path: string) => void;
  mergeBranch: string;
}

export default function ChangesView({
  fileChanges,
  selectedFile,
  onSelectFile,
  sourceControl,
  mergeConflicts,
  blameLines,
  blameFile,
  blameLoading,
  fetchBlame,
  clearBlame,
  selectedStashDiff,
  onCloseStashDiff,
  onOpenInEditor,
  mergeBranch,
}: ChangesViewProps) {
  const [diffMode, setDiffMode] = useState<DiffViewMode>('unified');
  const [annotations, setAnnotations] = useState<DiffAnnotation[]>([]);

  const currentPatch = sourceControl.selectedWorkingFile
    ? sourceControl.workingDiff
    : selectedFile?.patch ?? '';

  const currentFilePath = sourceControl.selectedWorkingFile
    ? sourceControl.selectedWorkingFile.path
    : selectedFile?.path;

  const isWorkingDiff = !!sourceControl.selectedWorkingFile;

  const handleStageHunk = useCallback(async (hunkIndex: number) => {
    if (!currentPatch || !currentFilePath) return;
    const parsed = parseDiff(currentPatch);
    if (hunkIndex >= parsed.hunks.length) return;
    const patchText = buildHunkPatch(parsed.headers, parsed.hunks[hunkIndex]);
    try {
      await window.electronAPI.invoke('git:stage-hunk', patchText, false);
      // Refresh the working diff
      if (sourceControl.selectedWorkingFile) {
        sourceControl.setSelectedWorkingFile({ ...sourceControl.selectedWorkingFile });
      }
    } catch (err) {
      console.error('Failed to stage hunk:', err);
    }
  }, [currentPatch, currentFilePath, sourceControl]);

  const handleUnstageHunk = useCallback(async (hunkIndex: number) => {
    if (!currentPatch || !currentFilePath) return;
    const parsed = parseDiff(currentPatch);
    if (hunkIndex >= parsed.hunks.length) return;
    const patchText = buildHunkPatch(parsed.headers, parsed.hunks[hunkIndex]);
    try {
      await window.electronAPI.invoke('git:stage-hunk', patchText, true);
      if (sourceControl.selectedWorkingFile) {
        sourceControl.setSelectedWorkingFile({ ...sourceControl.selectedWorkingFile });
      }
    } catch (err) {
      console.error('Failed to unstage hunk:', err);
    }
  }, [currentPatch, currentFilePath, sourceControl]);

  const handleAddAnnotation = useCallback((hunkIndex: number, lineIndex: number, text: string) => {
    setAnnotations(prev => [
      ...prev,
      {
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
        hunkIndex,
        lineIndex,
        text,
        timestamp: new Date().toISOString(),
      },
    ]);
  }, []);

  const handleDeleteAnnotation = useCallback((id: string) => {
    setAnnotations(prev => prev.filter(a => a.id !== id));
  }, []);

  return (
    <div className="flex-1 flex min-h-0 bg-zinc-950">
      {/* File list */}
      <div className="w-64 border-r border-zinc-800/60 flex flex-col bg-zinc-900/20">
        <div className="px-4 py-2 border-b border-zinc-800/60 flex items-center justify-between">
          <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Changed Files</span>
          <span className="text-xs text-zinc-500">{fileChanges.length}</span>
        </div>
        <div className="flex-1 overflow-y-auto py-2">
          {/* Working changes from source control */}
          {sourceControl.selectedWorkingFile && (
            <div className="border-b border-zinc-800/60 pb-1 mb-1">
              <div className="px-4 py-1 text-[10px] font-semibold text-zinc-500 uppercase tracking-wider">
                Working {sourceControl.selectedWorkingFile.staged ? '(Staged)' : '(Unstaged)'}
              </div>
              <button
                className="w-full flex items-center px-4 py-1.5 text-sm bg-zinc-800/80 text-zinc-100"
                onClick={() => sourceControl.setSelectedWorkingFile(null)}
              >
                <FileEdit className="w-4 h-4 mr-2 text-zinc-400 flex-shrink-0" />
                <span className="truncate flex-1 text-left">{sourceControl.selectedWorkingFile.path.split('/').pop()}</span>
                <X className="w-3 h-3 text-zinc-500 hover:text-zinc-300" />
              </button>
            </div>
          )}
          {/* Commit file changes */}
          {fileChanges.map(file => (
            <button
              key={file.path}
              onClick={() => { onSelectFile(file); sourceControl.setSelectedWorkingFile(null); clearBlame(); }}
              className={`w-full flex items-center px-4 py-1.5 text-sm hover:bg-zinc-800/50
                ${selectedFile?.path === file.path && !sourceControl.selectedWorkingFile ? 'bg-zinc-800/80 text-zinc-100' : 'text-zinc-400'}`}
            >
              {file.status === 'added' && <FilePlus className="w-4 h-4 mr-2 text-emerald-500 flex-shrink-0" />}
              {file.status === 'modified' && <FileEdit className="w-4 h-4 mr-2 text-zinc-400 flex-shrink-0" />}
              {file.status === 'deleted' && <FileMinus className="w-4 h-4 mr-2 text-rose-500 flex-shrink-0" />}
              <span className="truncate flex-1 text-left">{file.path.split('/').pop()}</span>
              <div className="flex items-center space-x-1.5 text-xs ml-2">
                {file.additions > 0 && <span className="text-emerald-500">+{file.additions}</span>}
                {file.deletions > 0 && <span className="text-rose-500">-{file.deletions}</span>}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Diff viewer / Blame / Conflict Resolution */}
      {mergeConflicts.selectedConflict && mergeConflicts.conflictDetail ? (
        <ConflictResolutionView
          detail={mergeConflicts.conflictDetail}
          onResolve={(strategy) => mergeConflicts.resolveConflict(mergeConflicts.selectedConflict!, strategy)}
          onOpenInEditor={() => {
            onOpenInEditor(mergeConflicts.selectedConflict!);
          }}
          resolving={mergeConflicts.resolving}
          mergeBranch={mergeBranch}
          conflictFiles={mergeConflicts.conflictFiles}
          currentFile={mergeConflicts.selectedConflict ?? undefined}
          onNavigateConflict={mergeConflicts.fetchConflictDetail}
        />
      ) : (
        <div className="flex-1 flex flex-col min-w-0">
          <div className="h-10 border-b border-zinc-800/60 flex items-center px-4 bg-zinc-900/40 gap-2">
            <span className="text-sm font-mono text-zinc-300 truncate flex-1">
              {selectedStashDiff !== null
                ? 'Stash Diff'
                : sourceControl.selectedWorkingFile
                  ? sourceControl.selectedWorkingFile.path
                  : selectedFile?.path}
            </span>
            {selectedStashDiff !== null && (
              <button
                onClick={onCloseStashDiff}
                className="flex items-center px-2 py-1 text-xs rounded transition-colors text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800"
              >
                <X className="w-3 h-3 mr-1" />
                Close
              </button>
            )}
            {selectedFile && !sourceControl.selectedWorkingFile && (
              <button
                onClick={() => {
                  if (blameFile === selectedFile.path) {
                    clearBlame();
                  } else {
                    fetchBlame(selectedFile.path);
                  }
                }}
                className={`flex items-center px-2 py-1 text-xs rounded transition-colors ${
                  blameFile === selectedFile.path
                    ? 'bg-[#F14E32]/20 text-[#F14E32]'
                    : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800'
                }`}
              >
                <User className="w-3 h-3 mr-1" />
                {blameLoading ? 'Loading...' : 'Blame'}
              </button>
            )}
          </div>

          <div className="flex-1 overflow-hidden font-mono text-sm leading-relaxed">
            {blameFile && blameLines.length > 0 && !sourceControl.selectedWorkingFile ? (
              <div className="h-full overflow-auto p-4 whitespace-pre">
                {blameLines.map((line, i) => {
                  const prevLine = i > 0 ? blameLines[i - 1] : null;
                  const isNewBlock = !prevLine || prevLine.hash !== line.hash;
                  return (
                    <div key={i} className={`flex hover:bg-zinc-900 ${isNewBlock ? 'border-t border-zinc-800/40' : ''}`}>
                      <div
                        className="w-52 flex-shrink-0 flex items-baseline pr-3 text-[11px] select-none"
                        title={`${line.hash} — ${line.author} — ${line.date}`}
                      >
                        {isNewBlock ? (
                          <>
                            <span className="text-[#F14E32] w-16 inline-block">{line.hash}</span>
                            <span className="text-zinc-500 truncate flex-1 mx-1.5">{line.author}</span>
                            <span className="text-zinc-600 flex-shrink-0">{line.date.split(' ')[0]}</span>
                          </>
                        ) : (
                          <span className="text-zinc-800">|</span>
                        )}
                      </div>
                      <div className="w-8 flex-shrink-0 text-right pr-2 text-zinc-600 text-xs select-none">{line.lineNo}</div>
                      <div className="flex-1 text-zinc-300 px-2 py-0.5">{line.content}</div>
                    </div>
                  );
                })}
              </div>
            ) : selectedStashDiff ? (
              <DiffViewer
                patch={selectedStashDiff}
                mode={diffMode}
                onModeChange={setDiffMode}
                className="h-full"
              />
            ) : currentPatch ? (
              <DiffViewer
                patch={currentPatch}
                filePath={currentFilePath}
                mode={diffMode}
                onModeChange={setDiffMode}
                showStaging={isWorkingDiff}
                onStageHunk={isWorkingDiff ? handleStageHunk : undefined}
                onUnstageHunk={isWorkingDiff && sourceControl.selectedWorkingFile?.staged ? handleUnstageHunk : undefined}
                annotations={annotations}
                onAddAnnotation={handleAddAnnotation}
                onDeleteAnnotation={handleDeleteAnnotation}
                className="h-full"
              />
            ) : (
              <div className="h-full flex items-center justify-center text-zinc-600 italic">
                No diff available
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
