import {
  FolderOpen, RefreshCw, GitGraph, FileCode, FileEdit, Upload, ClipboardList, ArrowDownCircle, Download, RotateCw, X, FolderGit2,
} from 'lucide-react';
import { useAutoUpdate } from '../hooks/useAutoUpdate';

type MainView = 'graph' | 'editor' | 'changes' | 'settings' | 'interactive-rebase' | 'tasks';

interface TabInfo {
  path: string;
  name: string;
}

interface TaskbarProps {
  mainView: MainView;
  setMainView: (v: MainView) => void;
  pushing: boolean;
  pushResult: { success: boolean; error?: string } | null;
  hasRemote: boolean;
  onPush: () => void;
  onOpenRepo: () => void;
  onRefresh: () => void;
  tabs: TabInfo[];
  activeTabIndex: number;
  onSwitchTab: (index: number) => void;
  onCloseTab: (index: number) => void;
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
  tabs,
  activeTabIndex,
  onSwitchTab,
  onCloseTab,
}: TaskbarProps) {
  const update = useAutoUpdate();
  return (
    <div className="flex-shrink-0 border-t border-zinc-800/60 bg-zinc-900/60">
      {/* Tab bar — only show when there are tabs */}
      {tabs.length > 0 && (
        <div className="h-7 flex items-end border-b border-zinc-800/40 bg-zinc-950/40 overflow-x-auto px-1 gap-0.5">
          {tabs.map((tab, i) => (
            <button
              key={tab.path}
              onClick={() => onSwitchTab(i)}
              className={`group flex items-center gap-1 px-2.5 py-1 text-[11px] rounded-t transition-colors max-w-[180px] shrink-0 ${
                i === activeTabIndex
                  ? 'bg-zinc-900/80 text-zinc-100 border-t border-x border-zinc-700/50'
                  : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/30'
              }`}
              title={tab.path}
            >
              <FolderGit2 className={`w-3 h-3 shrink-0 ${i === activeTabIndex ? 'text-[#F14E32]' : 'text-zinc-600'}`} />
              <span className="truncate">{tab.name}</span>
              {tabs.length > 1 && (
                <span
                  role="button"
                  onClick={(e) => { e.stopPropagation(); onCloseTab(i); }}
                  className="ml-0.5 p-0.5 rounded hover:bg-zinc-700 opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                  title="Close tab"
                >
                  <X className="w-2.5 h-2.5" />
                </span>
              )}
            </button>
          ))}
          <button
            onClick={onOpenRepo}
            className="flex items-center px-1.5 py-1 text-zinc-600 hover:text-zinc-400 transition-colors shrink-0"
            title="Open repository in new tab"
          >
            <FolderOpen className="w-3 h-3" />
          </button>
        </div>
      )}

      {/* Main toolbar */}
      <nav aria-label="Main toolbar" className="h-8 flex items-center px-3 gap-1 text-xs">
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
          aria-label="Refresh"
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

        {update.status === 'available' && (
          <button
            onClick={update.downloadUpdate}
            className="flex items-center px-2 py-1 rounded bg-blue-600/20 text-blue-400 hover:bg-blue-600/30 transition-colors"
            title={`Update v${update.version} available`}
          >
            <ArrowDownCircle className="w-3.5 h-3.5 mr-1" />
            Update v{update.version}
          </button>
        )}
        {update.status === 'downloading' && (
          <span className="flex items-center px-2 py-1 text-blue-400">
            <Download className="w-3.5 h-3.5 mr-1 animate-pulse" />
            {update.progress}%
          </span>
        )}
        {update.status === 'ready' && (
          <button
            onClick={update.installUpdate}
            className="flex items-center px-2 py-1 rounded bg-emerald-600/20 text-emerald-400 hover:bg-emerald-600/30 transition-colors"
            title="Restart to update"
          >
            <RotateCw className="w-3.5 h-3.5 mr-1" />
            Restart to Update
          </button>
        )}
      </nav>
    </div>
  );
}
