import { useEffect, useRef, useCallback } from 'react';

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'textarea:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

export function useFocusTrap(active: boolean) {
  const containerRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!active) return;

    previousFocus.current = document.activeElement as HTMLElement;

    // Focus first focusable element after mount
    const timer = requestAnimationFrame(() => {
      const container = containerRef.current;
      if (!container) return;
      const autofocus = container.querySelector('[autofocus]') as HTMLElement;
      if (autofocus) {
        autofocus.focus();
      } else {
        const first = container.querySelector(FOCUSABLE_SELECTOR) as HTMLElement;
        first?.focus();
      }
    });

    return () => {
      cancelAnimationFrame(timer);
      // Restore focus on unmount
      previousFocus.current?.focus();
    };
  }, [active]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key !== 'Tab' || !containerRef.current) return;

    const focusable = Array.from(
      containerRef.current.querySelectorAll(FOCUSABLE_SELECTOR)
    ) as HTMLElement[];

    if (focusable.length === 0) {
      e.preventDefault();
      return;
    }

    const first = focusable[0];
    const last = focusable[focusable.length - 1];

    if (e.shiftKey) {
      if (document.activeElement === first) {
        e.preventDefault();
        last.focus();
      }
    } else {
      if (document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }, []);

  return { containerRef, handleKeyDown };
}
