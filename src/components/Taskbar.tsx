import React from 'react';
import {
  FolderOpen, RefreshCw, GitGraph, FileCode, FileEdit, GitBranch, Upload, ClipboardList,
} from 'lucide-react';

type MainView = 'graph' | 'editor' | 'changes' | 'settings' | 'interactive-rebase' | 'tasks';

interface TaskbarProps {
  mainView: MainView;
  setMainView: (v: MainView) => void;
  pushing: boolean;
  pushResult: { success: boolean; error?: string } | null;
  hasRemote: boolean;
  onPush: () => void;
  onOpenRepo: () => void;
  onRefresh: () => void;
}

export default function Taskbar({
  mainView,
  setMainView,
  pushing,
  pushResult,
  hasRemote: _hasRemote,
  onPush,
  onOpenRepo,
  onRefresh,
}: TaskbarProps) {
  return (
    <div className="h-8 flex-shrink-0 border-t border-zinc-800/60 bg-zinc-900/60 flex items-center px-3 gap-1 text-xs">
      <button
        onClick={onOpenRepo}
        className="flex items-center px-2 py-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition-colors"
        title="Open Repository"
      >
        <FolderOpen className="w-3.5 h-3.5 mr-1" /> Open
      </button>
      <div className="w-px h-3.5 bg-zinc-800" />
      <button
        onClick={onRefresh}
        className="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition-colors"
        title="Refresh"
      >
        <RefreshCw className="w-3.5 h-3.5" />
      </button>
      <div className="w-px h-3.5 bg-zinc-800" />
      <button
        onClick={() => setMainView('graph')}
        className={`flex items-center px-2 py-1 rounded transition-colors ${
          mainView === 'graph' ? 'text-zinc-200 bg-zinc-800' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800'
        }`}
        title="Commit Graph"
      >
        <GitGraph className="w-3.5 h-3.5 mr-1" />
        Graph
      </button>
      <button
        onClick={() => setMainView('editor')}
        className={`flex items-center px-2 py-1 rounded transition-colors ${
          mainView === 'editor' ? 'text-zinc-200 bg-zinc-800' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800'
        }`}
        title="Editor"
      >
        <FileCode className="w-3.5 h-3.5 mr-1" />
        Editor
      </button>
      <button
        onClick={() => setMainView('changes')}
        className={`flex items-center px-2 py-1 rounded transition-colors ${
          mainView === 'changes' ? 'text-zinc-200 bg-zinc-800' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800'
        }`}
        title="Changes"
      >
        <FileEdit className="w-3.5 h-3.5 mr-1" />
        Changes
      </button>
      <button
        onClick={() => setMainView('tasks')}
        className={`flex items-center px-2 py-1 rounded transition-colors ${
          mainView === 'tasks' ? 'text-zinc-200 bg-zinc-800' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800'
        }`}
        title="Tasks"
      >
        <ClipboardList className="w-3.5 h-3.5 mr-1" />
        Tasks
      </button>
      <div className="w-px h-3.5 bg-zinc-800" />
      <button
        onClick={onPush}
        disabled={pushing}
        className="flex items-center px-2 py-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition-colors disabled:opacity-40"
        title="Push to remote"
      >
        <Upload className="w-3.5 h-3.5 mr-1" />
        {pushing ? 'Pushing...' : 'Push'}
      </button>
      {pushResult && (
        <span className={`text-[10px] ${pushResult.success ? 'text-emerald-400' : 'text-rose-400'}`}>
          {pushResult.success ? 'Pushed' : pushResult.error?.slice(0, 40)}
        </span>
      )}
      <div className="flex-1" />
    </div>
  );
}
