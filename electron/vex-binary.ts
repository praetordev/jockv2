import path from 'path';
import { app } from 'electron';

export function getVexBinaryPath(): string {
  if (app.isPackaged) {
    const ext = process.platform === 'win32' ? '.exe' : '';
    return path.join(process.resourcesPath, 'bin', `vex${ext}`);
  }
  return path.join(__dirname, '..', 'backend', 'bin', 'vex');
}
