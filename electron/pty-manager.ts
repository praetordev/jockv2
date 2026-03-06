import * as pty from 'node-pty';
import { BrowserWindow } from 'electron';
import os from 'os';
import { getVexBinaryPath } from './vex-binary';

interface PtyInstance {
  process: pty.IPty;
  id: string;
}

const terminals = new Map<string, PtyInstance>();

function getDefaultShell(): string {
  if (process.platform === 'win32') {
    return process.env.COMSPEC || 'cmd.exe';
  }
  return process.env.SHELL || '/bin/zsh';
}

export function createPty(
  window: BrowserWindow,
  id: string,
  cols: number,
  rows: number,
  cwd?: string | null,
  shellOverride?: string,
): void {
  // Kill any existing PTY with the same ID (handles StrictMode remount)
  const existing = terminals.get(id);
  if (existing) {
    existing.process.kill();
    terminals.delete(id);
  }

  const shell = shellOverride || getDefaultShell();
  const ptyProcess = pty.spawn(shell, [], {
    name: 'xterm-256color',
    cols,
    rows,
    cwd: cwd || os.homedir(),
    env: process.env as Record<string, string>,
  });

  terminals.set(id, { process: ptyProcess, id });

  ptyProcess.onData((data: string) => {
    const current = terminals.get(id);
    if (current && current.process === ptyProcess && !window.isDestroyed()) {
      window.webContents.send('pty:output', { id, data });
    }
  });

  ptyProcess.onExit(({ exitCode }) => {
    const current = terminals.get(id);
    if (current && current.process === ptyProcess) {
      terminals.delete(id);
      if (!window.isDestroyed()) {
        window.webContents.send('pty:exit', { id, exitCode });
      }
    }
  });
}

export function createVexPty(
  window: BrowserWindow,
  id: string,
  cols: number,
  rows: number,
  filePath: string,
  cwd?: string | null,
): void {
  const existing = terminals.get(id);
  if (existing) {
    existing.process.kill();
    terminals.delete(id);
  }

  const vexBin = getVexBinaryPath();
  const ptyProcess = pty.spawn(vexBin, [filePath], {
    name: 'xterm-256color',
    cols,
    rows,
    cwd: cwd || os.homedir(),
    env: process.env as Record<string, string>,
  });

  terminals.set(id, { process: ptyProcess, id });

  ptyProcess.onData((data: string) => {
    const current = terminals.get(id);
    if (current && current.process === ptyProcess && !window.isDestroyed()) {
      window.webContents.send('pty:output', { id, data });
    }
  });

  ptyProcess.onExit(({ exitCode }) => {
    const current = terminals.get(id);
    if (current && current.process === ptyProcess) {
      terminals.delete(id);
      if (!window.isDestroyed()) {
        window.webContents.send('pty:exit', { id, exitCode });
      }
    }
  });
}

export function writeToPty(id: string, data: string): void {
  terminals.get(id)?.process.write(data);
}

export function resizePty(id: string, cols: number, rows: number): void {
  terminals.get(id)?.process.resize(cols, rows);
}

export function killPty(id: string): void {
  const instance = terminals.get(id);
  if (instance) {
    terminals.delete(id);
    instance.process.kill();
  }
}

export function killAllPtys(): void {
  for (const [, instance] of terminals) {
    instance.process.kill();
  }
  terminals.clear();
}
