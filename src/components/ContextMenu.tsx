import { useEffect, useRef, useCallback } from 'react';

export type ContextMenuItem = {
  label: string;
  action: () => void;
  danger?: boolean;
  type?: never;
} | {
  type: 'separator';
};

interface ContextMenuProps {
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
}

export default function ContextMenu({ x, y, items, onClose }: ContextMenuProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose();
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  useEffect(() => {
    // Focus first menu item on mount
    const firstButton = ref.current?.querySelector('button[role="menuitem"]') as HTMLElement;
    firstButton?.focus();
  }, []);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
      return;
    }

    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const buttons = Array.from(
        ref.current?.querySelectorAll('button[role="menuitem"]') ?? []
      ) as HTMLElement[];
      if (buttons.length === 0) return;

      const currentIndex = buttons.indexOf(document.activeElement as HTMLElement);
      let nextIndex: number;
      if (e.key === 'ArrowDown') {
        nextIndex = currentIndex < buttons.length - 1 ? currentIndex + 1 : 0;
      } else {
        nextIndex = currentIndex > 0 ? currentIndex - 1 : buttons.length - 1;
      }
      buttons[nextIndex].focus();
    }
  }, [onClose]);

  return (
    <div
      ref={ref}
      role="menu"
      aria-label="Context menu"
      onKeyDown={handleKeyDown}
      className="fixed z-50 bg-zinc-900 border border-zinc-700 rounded shadow-xl py-1 min-w-[180px]"
      style={{ left: x, top: y }}
    >
      {items.map((item, i) =>
        'type' in item && item.type === 'separator' ? (
          <div key={i} role="separator" className="border-t border-zinc-700/50 my-1" />
        ) : (
          <button
            key={i}
            role="menuitem"
            onClick={() => { (item as Exclude<ContextMenuItem, { type: 'separator' }>).action(); onClose(); }}
            className={`w-full text-left px-3 py-1.5 text-xs hover:bg-zinc-800 focus:bg-zinc-800 focus:outline-none ${
              (item as Exclude<ContextMenuItem, { type: 'separator' }>).danger ? 'text-rose-400' : 'text-zinc-300'
            }`}
          >
            {(item as Exclude<ContextMenuItem, { type: 'separator' }>).label}
          </button>
        )
      )}
    </div>
  );
}
