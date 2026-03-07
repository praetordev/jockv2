import { KEYMAP_ACTIONS, type KeymapAction } from '../lib/defaultKeymap';
import AccessibleModal from './AccessibleModal';

interface KeyboardShortcutsHelpProps {
  show: boolean;
  onClose: () => void;
  keybindings?: Record<string, string>;
}

function chordToDisplay(chord: string): string {
  return chord
    .replace(/Cmd/gi, '\u2318')
    .replace(/Shift/gi, '\u21E7')
    .replace(/Alt/gi, '\u2325')
    .replace(/Ctrl/gi, '\u2303')
    .replace(/\+/g, ' ');
}

const categories: { key: KeymapAction['category']; label: string }[] = [
  { key: 'navigation', label: 'Navigation' },
  { key: 'git', label: 'Git Operations' },
  { key: 'editor', label: 'Editor' },
];

export default function KeyboardShortcutsHelp({ show, onClose, keybindings }: KeyboardShortcutsHelpProps) {
  return (
    <AccessibleModal show={show} onClose={onClose} title="Keyboard Shortcuts" className="w-96 max-h-[80vh] overflow-y-auto">
      <h3 className="text-sm font-semibold text-zinc-100 mb-4">Keyboard Shortcuts</h3>
      {categories.map(cat => {
        const actions = KEYMAP_ACTIONS.filter(a => a.category === cat.key);
        if (actions.length === 0) return null;
        return (
          <div key={cat.key} className="mb-4">
            <div className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider mb-2">{cat.label}</div>
            <div className="space-y-1">
              {actions.map(action => {
                const chord = keybindings?.[action.id] ?? action.defaultChord;
                return (
                  <div key={action.id} className="flex items-center justify-between py-1 px-2 rounded hover:bg-zinc-800/50">
                    <span className="text-xs text-zinc-300">{action.label}</span>
                    <kbd className="text-[10px] font-mono bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5 text-zinc-400">
                      {chordToDisplay(chord)}
                    </kbd>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
      <div className="mt-2 pt-3 border-t border-zinc-800/60">
        <div className="flex items-center justify-between py-1 px-2">
          <span className="text-xs text-zinc-300">Show this help</span>
          <kbd className="text-[10px] font-mono bg-zinc-800 border border-zinc-700 rounded px-1.5 py-0.5 text-zinc-400">?</kbd>
        </div>
      </div>
      <button
        onClick={onClose}
        className="mt-3 w-full px-3 py-2 rounded-md text-sm text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition-colors"
      >
        Close
      </button>
    </AccessibleModal>
  );
}
