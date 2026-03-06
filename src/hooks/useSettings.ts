import { useState, useEffect, useCallback } from 'react';
import { JockSettings, DEFAULT_SETTINGS } from '../settingsTypes';

const isElectron = typeof window !== 'undefined' && !!window.electronAPI;

export function useSettings() {
  const [settings, setSettings] = useState<JockSettings>(DEFAULT_SETTINGS);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    if (!isElectron) {
      setLoaded(true);
      return;
    }
    window.electronAPI.invoke('settings:get').then((data: JockSettings) => {
      setSettings(data);
      setLoaded(true);
    });
  }, []);

  useEffect(() => {
    if (!isElectron) return;
    const unsub = window.electronAPI.on('settings:changed', (data: unknown) => {
      setSettings(data as JockSettings);
    });
    return unsub;
  }, []);

  const updateSetting = useCallback(
    <K extends keyof JockSettings>(
      category: K,
      key: keyof JockSettings[K],
      value: JockSettings[K][keyof JockSettings[K]],
    ) => {
      setSettings((prev) => {
        const next = {
          ...prev,
          [category]: { ...prev[category], [key]: value },
        };
        if (isElectron) {
          window.electronAPI.invoke('settings:set', next);
        }
        return next;
      });
    },
    [],
  );

  const updateKeybinding = useCallback((actionId: string, chord: string | undefined) => {
    setSettings((prev) => {
      const bindings = { ...prev.keybindings };
      if (chord === undefined) {
        delete bindings[actionId];
      } else {
        bindings[actionId] = chord;
      }
      const next = { ...prev, keybindings: Object.keys(bindings).length > 0 ? bindings : undefined };
      if (isElectron) {
        window.electronAPI.invoke('settings:set', next);
      }
      return next;
    });
  }, []);

  return { settings, loaded, updateSetting, updateKeybinding };
}
