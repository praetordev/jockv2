import React from 'react';
import { FileCode, X } from 'lucide-react';
import { useAppContext } from '../context/AppContext';
import VexEditor from './VexEditor';

export default function EditorView() {
  const { editorTabs, activeEditorTabId, setActiveEditorTabId, closeEditorTab, mainView, settings } = useAppContext();

  if (editorTabs.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-zinc-600 italic">
        Open a file from the sidebar to start editing
      </div>
    );
  }

  return (
    <>
      <div className="h-9 flex items-center border-b border-zinc-800/60 bg-zinc-900/40 px-2 gap-0.5 flex-shrink-0 overflow-x-auto">
        {editorTabs.map(tab => (
          <button
            key={tab.id}
            onClick={() => setActiveEditorTabId(tab.id)}
            className={`flex items-center px-3 py-1 text-xs font-medium rounded transition-colors gap-1.5 flex-shrink-0 ${
              activeEditorTabId === tab.id
                ? 'bg-zinc-800 text-zinc-100'
                : 'text-zinc-500 hover:text-zinc-300'
            }`}
          >
            <FileCode className="w-3 h-3" />
            {tab.laneName && (
              <span className="px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 text-[10px] font-semibold uppercase tracking-wide">
                {tab.laneName}
              </span>
            )}
            {tab.fileName}
            <X
              className="w-3 h-3 hover:text-rose-400"
              onClick={(e) => { e.stopPropagation(); closeEditorTab(tab.id); }}
            />
          </button>
        ))}
      </div>
      <div className="flex-1 relative min-h-0 overflow-hidden">
        {editorTabs.map(tab => (
          <div
            key={tab.id}
            className="absolute inset-0"
            style={{ display: activeEditorTabId === tab.id ? 'block' : 'none' }}
          >
            <VexEditor
              editorId={tab.id}
              filePath={tab.filePath}
              isVisible={activeEditorTabId === tab.id && mainView === 'editor'}
              onExit={(id) => closeEditorTab(id)}
              fontSize={settings.editor.fontSize}
              fontFamily={settings.editor.fontFamily}
              cursorBlink={settings.editor.cursorBlink}
            />
          </div>
        ))}
      </div>
    </>
  );
}
