import React, { useRef, useEffect } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';

interface TerminalProps {
  terminalId: string;
  isVisible: boolean;
  fontSize?: number;
  fontFamily?: string;
  cursorBlink?: boolean;
  cwd?: string;
  onNewTerminal?: () => void;
}

const isElectron = typeof window !== 'undefined' && !!window.electronAPI;

export default function Terminal({
  terminalId,
  isVisible,
  fontSize = 13,
  fontFamily = '"JetBrains Mono", "Menlo", "Monaco", monospace',
  cursorBlink = true,
  cwd,
  onNewTerminal,
}: TerminalProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const initializedRef = useRef(false);
  const onNewTerminalRef = useRef(onNewTerminal);
  onNewTerminalRef.current = onNewTerminal;

  useEffect(() => {
    if (!isElectron || !containerRef.current || initializedRef.current) return;
    initializedRef.current = true;

    const xterm = new XTerm({
      cursorBlink,
      fontSize,
      fontFamily,
      theme: {
        background: '#09090b',
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

    const fitAddon = new FitAddon();
    xterm.loadAddon(fitAddon);
    xterm.loadAddon(new WebLinksAddon());

    xterm.open(containerRef.current);
    fitAddon.fit();

    xtermRef.current = xterm;
    fitAddonRef.current = fitAddon;

    // Intercept Ctrl+T to create new terminal tab
    xterm.attachCustomKeyEventHandler((e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 't' && e.type === 'keydown') {
        e.preventDefault();
        onNewTerminalRef.current?.();
        return false;
      }
      return true;
    });

    // Send initial dimensions to create PTY
    const { cols, rows } = xterm;
    window.electronAPI.send('pty:create', { id: terminalId, cols, rows, ...(cwd ? { cwd } : {}) });

    // Forward keystrokes to PTY
    xterm.onData((data: string) => {
      window.electronAPI.send('pty:data', { id: terminalId, data });
    });

    // Receive PTY output
    const removeOutput = window.electronAPI.on('pty:output', (payload: any) => {
      if (payload.id === terminalId) {
        xterm.write(payload.data);
      }
    });

    // Handle PTY exit
    const removeExit = window.electronAPI.on('pty:exit', (payload: any) => {
      if (payload.id === terminalId) {
        xterm.write('\r\n\x1b[90m[Process exited]\x1b[0m\r\n');
      }
    });

    // Handle resize
    const resizeObserver = new ResizeObserver(() => {
      if (fitAddonRef.current && xtermRef.current) {
        fitAddonRef.current.fit();
        const { cols, rows } = xtermRef.current;
        window.electronAPI.send('pty:resize', { id: terminalId, cols, rows });
      }
    });
    resizeObserver.observe(containerRef.current);

    return () => {
      resizeObserver.disconnect();
      removeOutput();
      removeExit();
      window.electronAPI.send('pty:kill', { id: terminalId });
      xterm.dispose();
      initializedRef.current = false;
    };
  }, [terminalId]);

  // Re-fit when visibility changes
  useEffect(() => {
    if (isVisible && fitAddonRef.current) {
      requestAnimationFrame(() => {
        fitAddonRef.current?.fit();
      });
    }
  }, [isVisible]);

  // Live-update settings on already-mounted terminal
  useEffect(() => {
    if (!xtermRef.current) return;
    xtermRef.current.options.fontSize = fontSize;
    xtermRef.current.options.fontFamily = fontFamily;
    xtermRef.current.options.cursorBlink = cursorBlink;
    if (fitAddonRef.current) {
      fitAddonRef.current.fit();
      const { cols, rows } = xtermRef.current;
      window.electronAPI?.send('pty:resize', { id: terminalId, cols, rows });
    }
  }, [fontSize, fontFamily, cursorBlink, terminalId]);

  if (!isElectron) {
    return (
      <div className="h-full flex items-center justify-center text-zinc-600 italic">
        Terminal is only available in the Electron app
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="h-full w-full overflow-hidden"
      style={{ padding: '8px 0 0 8px' }}
    />
  );
}
