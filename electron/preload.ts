import { contextBridge, ipcRenderer } from 'electron';

contextBridge.exposeInMainWorld('electronAPI', {
  platform: process.platform,
  send: (channel: string, ...args: unknown[]) => ipcRenderer.send(channel, ...args),
  invoke: (channel: string, ...args: unknown[]) => ipcRenderer.invoke(channel, ...args),
  on: (channel: string, listener: (...args: unknown[]) => void) => {
    const wrapped = (_event: any, ...args: unknown[]) => listener(...args);
    ipcRenderer.on(channel, wrapped);
    return () => { ipcRenderer.removeListener(channel, wrapped); };
  },
});
