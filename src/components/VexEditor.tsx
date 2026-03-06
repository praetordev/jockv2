import React, { useRef, useEffect } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';

interface VexEditorProps {
  editorId: string;
  filePath: string;
  isVisible: boolean;
  onExit?: (id: string) => void;
  fontSize?: number;
  fontFamily?: string;
  cursorBlink?: boolean;
}

const isElectron = typeof window !== 'undefined' && !!window.electronAPI;

/**
 * Fit terminal to container using ceil instead of floor,
 * so the grid always covers the full width with no gap.
 */
function fitTerminal(xterm: XTerm) {
  const el = xterm.element;
  if (!el || !el.parentElement) return;

  const core = (xterm as any)._core;
  const cellWidth = core._renderService.dimensions.css.cell.width;
  const cellHeight = core._renderService.dimensions.css.cell.height;
  if (cellWidth === 0 || cellHeight === 0) return;

  const parentStyle = window.getComputedStyle(el.parentElement);
  const w = parseInt(parentStyle.width);
  const h = parseInt(parentStyle.height);

  const cols = Math.max(2, Math.ceil(w / cellWidth));
  const rows = Math.max(1, Math.floor(h / cellHeight));

  if (xterm.cols !== cols || xterm.rows !== rows) {
    core._renderService.clear();
    xterm.resize(cols, rows);
  }
}

export default function VexEditor({
  editorId,
  filePath,
  isVisible,
  onExit,
  fontSize = 13,
  fontFamily = '"JetBrains Mono", "Menlo", "Monaco", monospace',
  cursorBlink = true,
}: VexEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const initializedRef = useRef(false);

  useEffect(() => {
    if (!isElectron || !containerRef.current || initializedRef.current) return;
    initializedRef.current = true;

    const xterm = new XTerm({
      cursorBlink,
      fontSize,
      fontFamily,
      scrollback: 0,
      theme: {
        background: '#1a1b26',
        foreground: '#d4d4d8',
        cursor: '#F14E32',
        selectionBackground: '#3f3f46',
        black: '#18181b',
        red: '#ef4444',
        green: '#22c55e',
        yellow: '#eab308',
        blue: '#3b82f6',
        magenta: '#a855f7',
        cyan: '#06b6d4',
        white: '#d4d4d8',
        brightBlack: '#52525b',
        brightRed: '#f87171',
        brightGreen: '#4ade80',
        brightYellow: '#facc15',
        brightBlue: '#60a5fa',
        brightMagenta: '#c084fc',
        brightCyan: '#22d3ee',
        brightWhite: '#fafafa',
      },
    });

    xterm.open(containerRef.current);

    // Hide the scrollbar — Vex handles its own scrolling
    const viewport = containerRef.current.querySelector('.xterm-viewport') as HTMLElement;
    if (viewport) viewport.style.overflowY = 'hidden';

    fitTerminal(xterm);

    xtermRef.current = xterm;

    const { cols, rows } = xterm;
    window.electronAPI.send('pty:create-vex', { id: editorId, cols, rows, filePath });

    xterm.onData((data: string) => {
      window.electronAPI.send('pty:data', { id: editorId, data });
    });

    const removeOutput = window.electronAPI.on('pty:output', (payload: any) => {
      if (payload.id === editorId) {
        xterm.write(payload.data);
      }
    });

    const removeExit = window.electronAPI.on('pty:exit', (payload: any) => {
      if (payload.id === editorId) {
        onExit?.(editorId);
      }
    });

    const resizeObserver = new ResizeObserver(() => {
      if (xtermRef.current) {
        fitTerminal(xtermRef.current);
        const { cols, rows } = xtermRef.current;
        window.electronAPI.send('pty:resize', { id: editorId, cols, rows });
      }
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      removeOutput();
      removeExit();
      window.electronAPI.send('pty:kill', { id: editorId });
      xterm.dispose();
      initializedRef.current = false;
    };
  }, [editorId]);

  useEffect(() => {
    if (isVisible && xtermRef.current) {
      requestAnimationFrame(() => {
        if (xtermRef.current) fitTerminal(xtermRef.current);
      });
    }
  }, [isVisible]);

  // Live-update settings on already-mounted editor
  useEffect(() => {
    if (!xtermRef.current) return;
    xtermRef.current.options.fontSize = fontSize;
    xtermRef.current.options.fontFamily = fontFamily;
    xtermRef.current.options.cursorBlink = cursorBlink;
    fitTerminal(xtermRef.current);
    const { cols, rows } = xtermRef.current;
    window.electronAPI?.send('pty:resize', { id: editorId, cols, rows });
  }, [fontSize, fontFamily, cursorBlink, editorId]);

  if (!isElectron) {
    return (
      <div className="h-full flex items-center justify-center text-zinc-600 italic">
        Editor is only available in the Electron app
      </div>
    );
  }

  return <div ref={containerRef} className="h-full w-full" style={{ backgroundColor: '#1a1b26' }} />;
}
