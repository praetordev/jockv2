export interface KeymapAction {
  id: string;
  label: string;
  defaultChord: string;
  category: 'navigation' | 'git' | 'editor';
}

export const KEYMAP_ACTIONS: KeymapAction[] = [
  // Navigation
  { id: 'showGraph', label: 'Show commit graph', defaultChord: 'Cmd+g', category: 'navigation' },
  { id: 'showTerminal', label: 'Show terminal', defaultChord: 'Cmd+t', category: 'navigation' },
  { id: 'focusDSL', label: 'Focus DSL query bar', defaultChord: 'Cmd+q', category: 'navigation' },
  { id: 'commandPalette', label: 'Command palette', defaultChord: 'Cmd+k', category: 'navigation' },
  { id: 'toggleSidebar', label: 'Toggle sidebar', defaultChord: 'Cmd+\\', category: 'navigation' },
  { id: 'openSettings', label: 'Open settings', defaultChord: 'Cmd+,', category: 'navigation' },

  // Git operations
  { id: 'quickStash', label: 'Quick stash', defaultChord: 'Cmd+Shift+s', category: 'git' },
  { id: 'push', label: 'Push', defaultChord: 'Cmd+Shift+p', category: 'git' },
  { id: 'pull', label: 'Pull', defaultChord: 'Cmd+Shift+l', category: 'git' },
  { id: 'newBranch', label: 'New branch', defaultChord: 'Cmd+b', category: 'git' },
  { id: 'refresh', label: 'Refresh all', defaultChord: 'Cmd+r', category: 'git' },
];

export function buildKeymap(overrides?: Record<string, string>): Record<string, string> {
  const keymap: Record<string, string> = {};
  for (const action of KEYMAP_ACTIONS) {
    keymap[action.id] = overrides?.[action.id] ?? action.defaultChord;
  }
  return keymap;
}
