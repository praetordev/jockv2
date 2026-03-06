import React from 'react';
import { useMerge } from '../../hooks/useGitData';
import type { MergeResult } from '../../types';

interface MergeDialogProps {
  show: boolean;
  mergeBranch: string;
  currentBranchName: string;
  onClose: () => void;
  onSuccess: () => void;
  onResolveConflicts: (files: string[]) => void;
  onAbortMerge: () => Promise<void>;
}

export default function MergeDialog({
  show,
  mergeBranch,
  currentBranchName,
  onClose,
  onSuccess,
  onResolveConflicts,
  onAbortMerge,
}: MergeDialogProps) {
  const { merging, mergeResult, doMerge, clearMergeResult } = useMerge();

  if (!show || !mergeBranch) return null;

  const handleClose = () => {
    onClose();
    clearMergeResult();
  };

  return (
    <>
      <div className="fixed inset-0 bg-black/60 z-50" onClick={handleClose} />
      <div className="fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-96 bg-zinc-900 border border-zinc-700/60 rounded-lg shadow-2xl p-5">
        <h3 className="text-sm font-semibold text-zinc-100 mb-1">Merge Branch</h3>
        <p className="text-xs text-zinc-500 mb-4">
          Merge <span className="text-zinc-300 font-medium">{mergeBranch}</span> into{' '}
          <span className="text-zinc-300 font-medium">{currentBranchName}</span>
        </p>

        {mergeResult && (
          <div
            className={`text-xs p-2.5 rounded-md mb-3 ${
              mergeResult.success
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                : 'bg-rose-500/10 text-rose-400 border border-rose-500/20'
            }`}
          >
            {mergeResult.success ? 'Merge completed successfully.' : mergeResult.error || 'Merge failed.'}
            {mergeResult.hasConflicts && (
              <div className="mt-2">
                <div className="font-medium mb-1">Conflict files:</div>
                {mergeResult.conflictFiles.map(f => (
                  <div key={f} className="text-zinc-400 pl-2">{f}</div>
                ))}
              </div>
            )}
          </div>
        )}

        <div className="flex gap-2">
          {!mergeResult && (
            <button
              onClick={async () => {
                const result = await doMerge(mergeBranch, false);
                if (result?.success && !result.hasConflicts) {
                  onSuccess();
                }
              }}
              disabled={merging}
              className="flex-1 px-3 py-2 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors disabled:opacity-40"
            >
              {merging ? 'Merging...' : 'Merge'}
            </button>
          )}
          {mergeResult?.hasConflicts && (
            <button
              onClick={() => {
                onResolveConflicts((mergeResult as MergeResult).conflictFiles);
                handleClose();
              }}
              className="flex-1 px-3 py-2 rounded-md bg-amber-600 hover:bg-amber-500 text-white text-sm font-medium transition-colors"
            >
              Resolve Conflicts
            </button>
          )}
          {mergeResult?.hasConflicts && (
            <button
              onClick={async () => {
                await onAbortMerge();
                clearMergeResult();
                onClose();
                onSuccess();
              }}
              className="px-3 py-2 rounded-md text-sm text-rose-400 hover:text-rose-200 border border-rose-500/30 hover:border-rose-500/60 transition-colors"
            >
              Abort
            </button>
          )}
          {!mergeResult?.hasConflicts && (
            <button
              onClick={handleClose}
              className="px-3 py-2 rounded-md text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
            >
              {mergeResult ? 'Close' : 'Cancel'}
            </button>
          )}
        </div>
      </div>
    </>
  );
}
