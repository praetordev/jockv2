import { useEffect } from 'react';

export interface ShortcutDef {
  chord: string; // e.g. "Cmd+G", "Cmd+Shift+S"
  action: () => void;
  label: string;
}

function parseChord(chord: string) {
  const parts = chord.toLowerCase().split('+');
  return {
    meta: parts.includes('cmd') || parts.includes('meta'),
    ctrl: parts.includes('ctrl'),
    shift: parts.includes('shift'),
    alt: parts.includes('alt'),
    key: parts[parts.length - 1],
  };
}

function isInputElement(el: Element | null): boolean {
  if (!el) return false;
  const tag = el.tagName.toLowerCase();
  return tag === 'input' || tag === 'textarea' || (el as HTMLElement).isContentEditable;
}

export function useKeyboardShortcuts(shortcuts: ShortcutDef[]) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      for (const shortcut of shortcuts) {
        const parsed = parseChord(shortcut.chord);
        const key = e.key.toLowerCase();

        if (
          key === parsed.key &&
          e.metaKey === parsed.meta &&
          e.ctrlKey === parsed.ctrl &&
          e.shiftKey === parsed.shift &&
          e.altKey === parsed.alt
        ) {
          // Allow shortcuts even in inputs for some chords (Cmd+G, etc.)
          // but skip plain letter shortcuts when typing in inputs
          if (!parsed.meta && !parsed.ctrl && isInputElement(document.activeElement)) {
            continue;
          }

          e.preventDefault();
          shortcut.action();
          return;
        }
      }
    };

    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [shortcuts]);
}
