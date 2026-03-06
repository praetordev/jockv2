import { useEffect, useRef } from 'react';

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
    const keyHandler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', handler);
    document.addEventListener('keydown', keyHandler);
    return () => {
      document.removeEventListener('mousedown', handler);
      document.removeEventListener('keydown', keyHandler);
    };
  }, [onClose]);

  return (
    <div
      ref={ref}
      className="fixed z-50 bg-zinc-900 border border-zinc-700 rounded shadow-xl py-1 min-w-[180px]"
      style={{ left: x, top: y }}
    >
      {items.map((item, i) =>
        'type' in item && item.type === 'separator' ? (
          <div key={i} className="border-t border-zinc-700/50 my-1" />
        ) : (
          <button
            key={i}
            onClick={() => { (item as Exclude<ContextMenuItem, { type: 'separator' }>).action(); onClose(); }}
            className={`w-full text-left px-3 py-1.5 text-xs hover:bg-zinc-800 ${
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
