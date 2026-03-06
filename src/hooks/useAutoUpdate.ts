import { useState, useEffect, useCallback } from 'react';
import { isElectron } from '../lib/electron';

export type UpdateStatus = 'idle' | 'checking' | 'available' | 'downloading' | 'ready' | 'error' | 'up-to-date';

interface UpdateState {
  status: UpdateStatus;
  version: string | null;
  progress: number;
  error: string | null;
}

export function useAutoUpdate() {
  const [state, setState] = useState<UpdateState>({
    status: 'idle',
    version: null,
    progress: 0,
    error: null,
  });

  useEffect(() => {
    if (!isElectron) return;
    const api = (window as any).electronAPI;

    const unsubs = [
      api.on('updater:update-available', (info: { version: string }) => {
        setState({ status: 'available', version: info.version, progress: 0, error: null });
      }),
      api.on('updater:up-to-date', () => {
        setState(prev => ({ ...prev, status: 'up-to-date', error: null }));
        // Reset to idle after 3 seconds
        setTimeout(() => setState(prev => prev.status === 'up-to-date' ? { ...prev, status: 'idle' } : prev), 3000);
      }),
      api.on('updater:download-progress', (info: { percent: number }) => {
        setState(prev => ({ ...prev, status: 'downloading', progress: info.percent }));
      }),
      api.on('updater:update-downloaded', () => {
        setState(prev => ({ ...prev, status: 'ready', progress: 100 }));
      }),
      api.on('updater:error', (message: string) => {
        setState(prev => ({ ...prev, status: 'error', error: message }));
      }),
    ];

    return () => unsubs.forEach(unsub => unsub());
  }, []);

  const checkForUpdates = useCallback(async () => {
    if (!isElectron) return;
    setState(prev => ({ ...prev, status: 'checking', error: null }));
    await window.electronAPI.invoke('updater:check');
  }, []);

  const downloadUpdate = useCallback(async () => {
    if (!isElectron) return;
    setState(prev => ({ ...prev, status: 'downloading', progress: 0 }));
    await window.electronAPI.invoke('updater:download');
  }, []);

  const installUpdate = useCallback(async () => {
    if (!isElectron) return;
    await window.electronAPI.invoke('updater:install');
  }, []);

  return {
    ...state,
    checkForUpdates,
    downloadUpdate,
    installUpdate,
  };
}
