import { ChildProcess, spawn } from 'child_process';
import path from 'path';
import { app } from 'electron';

let sidecarProcess: ChildProcess | null = null;

function getBinaryPath(): string {
  if (app.isPackaged) {
    const ext = process.platform === 'win32' ? '.exe' : '';
    return path.join(process.resourcesPath, 'bin', `jockd${ext}`);
  }
  return path.join(__dirname, '..', 'backend', 'bin', 'jockd');
}

export function startSidecar(): Promise<number> {
  return new Promise((resolve, reject) => {
    const binPath = getBinaryPath();
    sidecarProcess = spawn(binPath, [], {
      stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env },
      detached: true,
    });

    let resolved = false;
    let stdoutBuffer = '';

    sidecarProcess.stdout!.on('data', (data: Buffer) => {
      stdoutBuffer += data.toString();
      const match = stdoutBuffer.match(/JOCK_PORT=(\d+)/);
      if (match && !resolved) {
        resolved = true;
        resolve(parseInt(match[1], 10));
      }
    });

    sidecarProcess.stderr!.on('data', (data: Buffer) => {
      console.error('[jockd]', data.toString().trim());
    });

    sidecarProcess.on('error', (err) => {
      if (!resolved) {
        resolved = true;
        reject(new Error(`Failed to start sidecar: ${err.message}`));
      }
    });

    sidecarProcess.on('exit', (code) => {
      if (!resolved) {
        resolved = true;
        reject(new Error(`Sidecar exited with code ${code} before announcing port`));
      }
      sidecarProcess = null;
    });

    setTimeout(() => {
      if (!resolved) {
        resolved = true;
        stopSidecar();
        reject(new Error('Sidecar startup timed out'));
      }
    }, 10_000);
  });
}

export function stopSidecar(): void {
  if (sidecarProcess) {
    sidecarProcess.kill('SIGTERM');
    sidecarProcess = null;
  }
}
