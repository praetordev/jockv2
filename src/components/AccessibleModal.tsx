import React, { useEffect } from 'react';
import { useFocusTrap } from '../hooks/useFocusTrap';

interface AccessibleModalProps {
  show: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  className?: string;
}

export default function AccessibleModal({ show, onClose, title, children, className = 'w-80' }: AccessibleModalProps) {
  const { containerRef, handleKeyDown } = useFocusTrap(show);

  useEffect(() => {
    if (!show) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [show, onClose]);

  if (!show) return null;

  return (
    <>
      <div className="fixed inset-0 bg-black/60 z-50" onClick={onClose} aria-hidden="true" />
      <div
        ref={containerRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        onKeyDown={handleKeyDown}
        className={`fixed z-50 top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 bg-zinc-900 border border-zinc-700/60 rounded-lg shadow-2xl p-5 ${className}`}
      >
        {children}
      </div>
    </>
  );
}
