import React from 'react';
import { useAppContext } from '../context/AppContext';
import CommitGraph from './CommitGraph';
import CommitDetailPanel from './CommitDetailPanel';
import ChangesView from './ChangesView';
import InteractiveRebaseView from './InteractiveRebaseView';
import TaskBoard from './TaskBoard';
import SettingsView from './SettingsView';
import EditorView from './EditorView';
import WelcomeScreen from './WelcomeScreen';
import BottomPanel from './BottomPanel';

export default function MainContent() {
  const ctx = useAppContext();
  const {
    repoPath, mainView, setMainView,
    filteredCommits, graphWidth, selectedCommit, setSelectedCommit,
    animatedCommitHashes, commitSearch, setCommitSearch,
    fileChanges, selectedFile, setSelectedFile,
    cherryPickRevert, mergeConflicts, rebase,
    refreshCommits, refreshBranches, sourceControl,
    interactiveRebaseBase, setInteractiveRebaseBase,
    commits, taskData, settings, updateSetting, updateKeybinding,
    blameLines, blameFile, blameLoading, fetchBlame, clearBlame,
    selectedStashDiff, setSelectedStashDiff,
    openFile, mergeBranch,
    setBottomMode, setInitialQuery,
    dslHighlightHashes, commitFilters,
    setShowCreateTag, setCreateTagCommitHash,
    terminalState, bottomMode,
    initialQuery, setDslHighlightHashes,
  } = ctx;

  return (
    <div className="flex-1 flex flex-col min-w-0">
      {/* Drag region when no toolbar */}
      {!repoPath && (
        <div className="h-8" style={{ WebkitAppRegion: 'drag' } as React.CSSProperties} />
      )}

      <div className="flex-1 flex flex-col min-h-0">
        {/* Editor — always mounted to preserve PTY connections */}
        {repoPath && (
          <div
            className="flex-1 flex flex-col bg-zinc-950 min-h-0"
            style={{ display: mainView === 'editor' ? undefined : 'none' }}
          >
            <EditorView />
          </div>
        )}

        {/* Main Content Area - Switchable */}
        {!repoPath ? (
          <WelcomeScreen />
        ) : mainView === 'graph' ? (
          <div className="flex-1 min-h-0 flex">
            <CommitGraph
              filteredCommits={filteredCommits}
              graphWidth={graphWidth}
              selectedCommit={selectedCommit}
              animatedCommitHashes={animatedCommitHashes}
              onSelectCommit={setSelectedCommit}
              commitSearch={commitSearch}
              onSearchChange={setCommitSearch}
              onCherryPick={async (commit) => {
                const result = await cherryPickRevert.cherryPick(commit.hash);
                if (result?.success) {
                  refreshCommits(); refreshBranches();
                } else if (result?.hasConflicts) {
                  mergeConflicts.setConflictFiles(result.conflictFiles);
                  setMainView('changes');
                  if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
                }
              }}
              onRevert={async (commit) => {
                const result = await cherryPickRevert.revert(commit.hash);
                if (result?.success) {
                  refreshCommits(); refreshBranches();
                } else if (result?.hasConflicts) {
                  mergeConflicts.setConflictFiles(result.conflictFiles);
                  setMainView('changes');
                  if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
                }
              }}
              onCreateTag={(commit) => {
                setCreateTagCommitHash(commit.hash);
                setShowCreateTag(true);
              }}
              onInteractiveRebase={(commit) => {
                setInteractiveRebaseBase(commit);
                setMainView('interactive-rebase');
              }}
              onQueryFromGraph={(query) => {
                setBottomMode('query');
                setInitialQuery(query);
              }}
              dslHighlightHashes={dslHighlightHashes}
              filters={commitFilters}
              onFiltersChange={(newFilters) => {
                refreshCommits(newFilters);
              }}
            />
            {selectedCommit && (
              <CommitDetailPanel
                commit={selectedCommit}
                fileChanges={fileChanges}
                onClose={() => setSelectedCommit(null)}
              />
            )}
          </div>
        ) : mainView === 'interactive-rebase' && interactiveRebaseBase ? (
          <InteractiveRebaseView
            commits={commits.slice(0, commits.findIndex(c => c.hash === interactiveRebaseBase.hash))}
            baseCommit={interactiveRebaseBase}
            onExecute={async (baseHash, entries) => {
              const result = await rebase.interactiveRebase(baseHash, entries);
              if (result?.success) {
                setInteractiveRebaseBase(null);
                setMainView('graph');
                refreshCommits(); refreshBranches();
              } else if (result?.hasConflicts) {
                mergeConflicts.setConflictFiles(result.conflictFiles);
                setMainView('changes');
                if (result.conflictFiles.length > 0) mergeConflicts.fetchConflictDetail(result.conflictFiles[0]);
              }
            }}
            onCancel={() => {
              setInteractiveRebaseBase(null);
              setMainView('graph');
            }}
          />
        ) : mainView === 'tasks' ? (
          <TaskBoard
            backlog={taskData.backlog}
            inProgress={taskData.inProgress}
            done={taskData.done}
            loading={taskData.loading}
            onCreateTask={taskData.createTask}
            onUpdateTask={taskData.updateTask}
            onDeleteTask={taskData.deleteTask}
            onStartTask={taskData.startTask}
          />
        ) : mainView === 'settings' ? (
          <SettingsView settings={settings} onUpdateSetting={updateSetting} onUpdateKeybinding={updateKeybinding} />
        ) : mainView !== 'editor' ? (
          <ChangesView
            fileChanges={fileChanges}
            selectedFile={selectedFile}
            onSelectFile={(file) => { setSelectedFile(file); clearBlame(); }}
            sourceControl={sourceControl}
            mergeConflicts={mergeConflicts}
            blameLines={blameLines}
            blameFile={blameFile}
            blameLoading={blameLoading}
            fetchBlame={fetchBlame}
            clearBlame={clearBlame}
            selectedStashDiff={selectedStashDiff}
            onCloseStashDiff={() => setSelectedStashDiff(null)}
            onOpenInEditor={(filePath) => { setMainView('editor'); openFile(filePath); }}
            mergeBranch={mergeBranch}
          />
        ) : null}

        {/* Bottom Panel */}
        {repoPath && (
          <BottomPanel
            settings={settings}
            commits={commits}
            onCommitSelect={setSelectedCommit}
            terminals={terminalState.terminals}
            activeTerminalId={terminalState.activeTerminalId}
            setActiveTerminalId={terminalState.setActiveTerminalId}
            createTerminal={terminalState.createTerminal}
            closeTerminal={terminalState.closeTerminal}
            bottomMode={bottomMode}
            setBottomMode={setBottomMode}
            initialQuery={initialQuery}
            onInitialQueryConsumed={() => setInitialQuery(undefined)}
            onRunDSLQuery={(query) => {
              setInitialQuery(query);
              setBottomMode('query');
            }}
            onDSLResultHashes={setDslHighlightHashes}
          />
        )}
      </div>
    </div>
  );
}
