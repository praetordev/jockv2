import React, { useEffect, useRef, useState } from 'react';
import { AlertTriangle } from 'lucide-react';
import { AppProvider, useAppContext } from './context/AppContext';
import { ToastProvider, useToast } from './context/ToastContext';
import Sidebar from './components/Sidebar';
import MainContent from './components/MainContent';
import Taskbar from './components/Taskbar';
import ToastContainer from './components/ToastContainer';
import CreateBranchModal from './components/modals/CreateBranchModal';
import MergeDialog from './components/modals/MergeDialog';
import RemoteSetupModal from './components/modals/RemoteSetupModal';
import CreateTagModal from './components/modals/CreateTagModal';
import KeyboardShortcutsHelp from './components/KeyboardShortcutsHelp';
import CommandPalette from './components/CommandPalette';

function AppShell() {
  const {
    repoPath, hasRemote, backendError,
    isSidebarOpen, mainView, setMainView,
    pushing, pushResult, doPush, openRepo,
    refreshAll, tabs, switchTab, closeTab,
    // Modals
    showCreateBranch, setShowCreateBranch,
    showMergeDialog, setShowMergeDialog,
    mergeBranch, currentBranchName,
    showRemoteSetup, setShowRemoteSetup,
    showCreateTag, setShowCreateTag,
    createTagCommitHash,
    // Dependencies for modal callbacks
    refreshCommits, refreshBranches, sourceControl,
    doCreateBranch, creatingBranch,
    mergeConflicts, tags, checkRemote,
    settings,
  } = useAppContext();

  const { addToast } = useToast();
  const prevPushResult = useRef(pushResult);
  const [showShortcutsHelp, setShowShortcutsHelp] = useState(false);
  const [showCommandPalette, setShowCommandPalette] = useState(false);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      // Cmd+K / Ctrl+K — command palette (works even in inputs)
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setShowCommandPalette(prev => !prev);
        return;
      }
      // ? — keyboard shortcuts help (skip when typing)
      const tag = (document.activeElement?.tagName ?? '').toLowerCase();
      if (tag === 'input' || tag === 'textarea' || (document.activeElement as HTMLElement)?.isContentEditable) return;
      if (e.key === '?') {
        e.preventDefault();
        setShowShortcutsHelp(prev => !prev);
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, []);

  useEffect(() => {
    if (pushResult && pushResult !== prevPushResult.current) {
      if (pushResult.success) {
        addToast({ type: 'success', title: 'Push successful' });
      } else {
        addToast({ type: 'error', title: 'Push failed', message: pushResult.error });
      }
    }
    prevPushResult.current = pushResult;
  }, [pushResult, addToast]);

  return (
    <div className={`flex flex-col h-screen w-full bg-zinc-950 text-zinc-300 overflow-hidden select-none${settings.appearance.highContrast ? ' high-contrast' : ''}`}>
      <a href="#main-content" className="sr-only focus:not-sr-only focus:absolute focus:z-[100] focus:top-2 focus:left-2 focus:bg-zinc-800 focus:text-zinc-100 focus:px-4 focus:py-2 focus:rounded-md focus:text-sm">
        Skip to main content
      </a>
      {backendError && (
        <div role="alert" className="bg-red-900/80 text-red-200 px-4 py-2 text-sm flex items-center gap-2 shrink-0">
          <AlertTriangle size={16} />
          <span>Backend failed to start: {backendError}</span>
        </div>
      )}
      <div className="flex flex-1 min-h-0">
        {isSidebarOpen && repoPath && <Sidebar />}
        <MainContent />
      </div>

      {repoPath && (
        <Taskbar
          mainView={mainView}
          setMainView={setMainView}
          pushing={pushing}
          pushResult={pushResult}
          hasRemote={hasRemote}
          onPush={() => doPush(undefined, undefined, false, !hasRemote)}
          onOpenRepo={openRepo}
          onRefresh={refreshAll}
          tabs={tabs.openTabs.map(p => ({ path: p, name: p.split('/').pop() || p }))}
          activeTabIndex={tabs.activeIndex}
          onSwitchTab={switchTab}
          onCloseTab={closeTab}
        />
      )}

      <CreateBranchModal
        show={showCreateBranch}
        onClose={() => setShowCreateBranch(false)}
        onSuccess={() => { refreshBranches(); refreshCommits(); sourceControl.refresh(); }}
        doCreateBranch={doCreateBranch}
        creatingBranch={creatingBranch}
      />

      <MergeDialog
        show={showMergeDialog}
        mergeBranch={mergeBranch}
        currentBranchName={currentBranchName}
        onClose={() => setShowMergeDialog(false)}
        onSuccess={() => { refreshCommits(); refreshBranches(); sourceControl.refresh(); }}
        onResolveConflicts={(files) => {
          mergeConflicts.setConflictFiles(files);
          setMainView('changes');
          if (files.length > 0) mergeConflicts.fetchConflictDetail(files[0]);
          sourceControl.refresh();
        }}
        onAbortMerge={mergeConflicts.abortMerge}
      />

      <RemoteSetupModal
        show={showRemoteSetup}
        onClose={() => setShowRemoteSetup(false)}
        initialRepoName={repoPath?.split('/').pop() ?? ''}
        onSuccess={checkRemote}
      />

      <CreateTagModal
        show={showCreateTag}
        onClose={() => setShowCreateTag(false)}
        onSuccess={() => { tags.refresh(); refreshCommits(); }}
        doCreateTag={tags.createTag}
        defaultCommitHash={createTagCommitHash}
      />

      <CommandPalette
        show={showCommandPalette}
        onClose={() => setShowCommandPalette(false)}
      />

      <KeyboardShortcutsHelp
        show={showShortcutsHelp}
        onClose={() => setShowShortcutsHelp(false)}
        keybindings={settings.keybindings}
      />

      <ToastContainer />
    </div>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <AppProvider>
        <AppShell />
      </AppProvider>
    </ToastProvider>
  );
}
