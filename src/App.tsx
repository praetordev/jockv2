import React from 'react';
import { AlertTriangle } from 'lucide-react';
import { AppProvider, useAppContext } from './context/AppContext';
import Sidebar from './components/Sidebar';
import MainContent from './components/MainContent';
import Taskbar from './components/Taskbar';
import CreateBranchModal from './components/modals/CreateBranchModal';
import MergeDialog from './components/modals/MergeDialog';
import RemoteSetupModal from './components/modals/RemoteSetupModal';
import CreateTagModal from './components/modals/CreateTagModal';

function AppShell() {
  const {
    repoPath, hasRemote, backendError,
    isSidebarOpen, mainView, setMainView,
    pushing, pushResult, doPush, openRepo,
    refreshAll,
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
  } = useAppContext();

  return (
    <div className="flex flex-col h-screen w-full bg-zinc-950 text-zinc-300 overflow-hidden select-none">
      {backendError && (
        <div className="bg-red-900/80 text-red-200 px-4 py-2 text-sm flex items-center gap-2 shrink-0">
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
    </div>
  );
}

export default function App() {
  return (
    <AppProvider>
      <AppShell />
    </AppProvider>
  );
}
