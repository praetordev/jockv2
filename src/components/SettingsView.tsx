import React, { useState, useEffect, useCallback } from 'react';
import { JockSettings } from '../settingsTypes';
import { KEYMAP_ACTIONS, buildKeymap } from '../lib/defaultKeymap';
import { isElectron } from '../lib/electron';
import { useAutoUpdate } from '../hooks/useAutoUpdate';

interface SettingsViewProps {
  settings: JockSettings;
  onUpdateSetting: <K extends keyof JockSettings>(
    category: K,
    key: keyof JockSettings[K],
    value: JockSettings[K][keyof JockSettings[K]],
  ) => void;
  onUpdateKeybinding: (actionId: string, chord: string | undefined) => void;
}

type SettingsCategory = 'general' | 'editor' | 'appearance' | 'keyboard' | 'git';

function chordToDisplay(chord: string): string {
  return chord
    .replace(/Cmd/gi, '\u2318')
    .replace(/Shift/gi, '\u21E7')
    .replace(/Alt/gi, '\u2325')
    .replace(/Ctrl/gi, '\u2303')
    .replace(/\+/g, ' ')
    .replace(/\\/g, '\\');
}

function keyEventToChord(e: KeyboardEvent): string | null {
  const key = e.key;
  if (['Meta', 'Shift', 'Alt', 'Control'].includes(key)) return null;

  const parts: string[] = [];
  if (e.metaKey) parts.push('Cmd');
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');

  // Require at least one modifier
  if (parts.length === 0) return null;

  parts.push(key.length === 1 ? key.toLowerCase() : key.toLowerCase());
  return parts.join('+');
}

export default function SettingsView({ settings, onUpdateSetting, onUpdateKeybinding }: SettingsViewProps) {
  const [activeCategory, setActiveCategory] = useState<SettingsCategory>('general');
  const [recordingAction, setRecordingAction] = useState<string | null>(null);
  const [gitConfig, setGitConfig] = useState<{ userName: string; userEmail: string; gpgSign: boolean } | null>(null);
  const keymap = buildKeymap(settings.keybindings);
  const update = useAutoUpdate();

  useEffect(() => {
    if (activeCategory === 'git' && isElectron && !gitConfig) {
      window.electronAPI.invoke('git:get-config').then(setGitConfig);
    }
  }, [activeCategory, gitConfig]);

  const handleKeyCapture = useCallback((e: KeyboardEvent) => {
    if (!recordingAction) return;
    e.preventDefault();
    e.stopPropagation();

    if (e.key === 'Escape') {
      setRecordingAction(null);
      return;
    }

    const chord = keyEventToChord(e);
    if (chord) {
      onUpdateKeybinding(recordingAction, chord);
      setRecordingAction(null);
    }
  }, [recordingAction, onUpdateKeybinding]);

  useEffect(() => {
    if (!recordingAction) return;
    document.addEventListener('keydown', handleKeyCapture, true);
    return () => document.removeEventListener('keydown', handleKeyCapture, true);
  }, [recordingAction, handleKeyCapture]);

  return (
    <div className="flex-1 flex bg-zinc-950 overflow-hidden">
      {/* Category sidebar */}
      <div className="w-48 border-r border-zinc-800/60 p-4 space-y-1">
        <h2 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-3">
          Settings
        </h2>
        {(['general', 'editor', 'appearance', 'git', 'keyboard'] as const).map((cat) => {
          const labels: Record<SettingsCategory, string> = {
            general: 'General',
            editor: 'Editor & Terminal',
            appearance: 'Appearance',
            git: 'Git',
            keyboard: 'Keyboard',
          };
          return (
            <button
              key={cat}
              onClick={() => setActiveCategory(cat)}
              className={`w-full text-left px-3 py-1.5 rounded text-sm transition-colors ${
                activeCategory === cat
                  ? 'bg-zinc-800 text-zinc-200'
                  : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50'
              }`}
            >
              {labels[cat]}
            </button>
          );
        })}
      </div>

      {/* Settings content */}
      <div className="flex-1 overflow-auto p-6 max-w-xl">
        {activeCategory === 'general' && (
          <div className="space-y-6">
            <h3 className="text-sm font-semibold text-zinc-100">General</h3>

            <div className="space-y-1.5">
              <label className="text-xs text-zinc-400">Commit list limit</label>
              <p className="text-[11px] text-zinc-600">
                Maximum number of commits to load in the graph view.
              </p>
              <input
                type="number"
                min={10}
                max={5000}
                value={settings.general.commitListLimit}
                onChange={(e) =>
                  onUpdateSetting('general', 'commitListLimit', parseInt(e.target.value) || 100)
                }
                className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-1.5 text-sm text-zinc-200
                  focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] w-32"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs text-zinc-400">Default shell</label>
              <p className="text-[11px] text-zinc-600">
                Shell for new terminals. Leave empty to auto-detect.
              </p>
              <input
                type="text"
                value={settings.general.defaultShell}
                onChange={(e) => onUpdateSetting('general', 'defaultShell', e.target.value)}
                placeholder="/bin/zsh"
                className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-1.5 text-sm text-zinc-200
                  focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] w-64"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs text-zinc-400">Updates</label>
              <p className="text-[11px] text-zinc-600">
                Check for new versions from GitHub Releases.
              </p>
              <div className="flex items-center gap-3">
                <button
                  onClick={update.checkForUpdates}
                  disabled={update.status === 'checking' || update.status === 'downloading'}
                  className="px-3 py-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 text-sm text-zinc-200 transition-colors disabled:opacity-40"
                >
                  {update.status === 'checking' ? 'Checking...' : 'Check for Updates'}
                </button>
                {update.status === 'up-to-date' && (
                  <span className="text-xs text-emerald-400">You&apos;re up to date</span>
                )}
                {update.status === 'available' && (
                  <button
                    onClick={update.downloadUpdate}
                    className="px-3 py-1.5 rounded-md bg-blue-600 hover:bg-blue-500 text-sm text-white transition-colors"
                  >
                    Download v{update.version}
                  </button>
                )}
                {update.status === 'downloading' && (
                  <span className="text-xs text-blue-400">Downloading... {update.progress}%</span>
                )}
                {update.status === 'ready' && (
                  <button
                    onClick={update.installUpdate}
                    className="px-3 py-1.5 rounded-md bg-emerald-600 hover:bg-emerald-500 text-sm text-white transition-colors"
                  >
                    Restart to Update
                  </button>
                )}
                {update.status === 'error' && (
                  <span className="text-xs text-rose-400">{update.error}</span>
                )}
              </div>
            </div>
          </div>
        )}

        {activeCategory === 'editor' && (
          <div className="space-y-6">
            <h3 className="text-sm font-semibold text-zinc-100">Editor & Terminal</h3>

            <div className="space-y-1.5">
              <label className="text-xs text-zinc-400">Font size</label>
              <input
                type="number"
                min={8}
                max={32}
                value={settings.editor.fontSize}
                onChange={(e) =>
                  onUpdateSetting('editor', 'fontSize', parseInt(e.target.value) || 13)
                }
                className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-1.5 text-sm text-zinc-200
                  focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] w-32"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs text-zinc-400">Font family</label>
              <input
                type="text"
                value={settings.editor.fontFamily}
                onChange={(e) => onUpdateSetting('editor', 'fontFamily', e.target.value)}
                className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-1.5 text-sm text-zinc-200
                  focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] w-full"
              />
            </div>

            <div className="flex items-center gap-3">
              <label className="text-xs text-zinc-400">Cursor blink</label>
              <button
                onClick={() => onUpdateSetting('editor', 'cursorBlink', !settings.editor.cursorBlink)}
                className={`relative w-8 h-4 rounded-full transition-colors ${
                  settings.editor.cursorBlink ? 'bg-[#F14E32]' : 'bg-zinc-700'
                }`}
              >
                <span
                  className={`absolute top-0.5 left-0.5 w-3 h-3 rounded-full bg-white transition-transform ${
                    settings.editor.cursorBlink ? 'translate-x-4' : 'translate-x-0'
                  }`}
                />
              </button>
            </div>
          </div>
        )}

        {activeCategory === 'appearance' && (
          <div className="space-y-6">
            <h3 className="text-sm font-semibold text-zinc-100">Appearance</h3>

            <div className="space-y-1.5">
              <label className="text-xs text-zinc-400">Theme</label>
              <p className="text-[11px] text-zinc-600">
                Choose between dark and light mode.
              </p>
              <div className="flex gap-2">
                {(['dark', 'light'] as const).map((theme) => (
                  <button
                    key={theme}
                    onClick={() => onUpdateSetting('appearance', 'theme', theme)}
                    className={`px-4 py-2 rounded-md text-sm capitalize transition-colors ${
                      settings.appearance?.theme === theme
                        ? 'bg-[#F14E32] text-white'
                        : 'bg-zinc-800 text-zinc-400 hover:text-zinc-200'
                    }`}
                  >
                    {theme}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center gap-3">
              <label className="text-xs text-zinc-400">High contrast</label>
              <button
                onClick={() => onUpdateSetting('appearance', 'highContrast', !settings.appearance.highContrast)}
                role="switch"
                aria-checked={settings.appearance.highContrast}
                className={`relative w-8 h-4 rounded-full transition-colors ${
                  settings.appearance.highContrast ? 'bg-[#F14E32]' : 'bg-zinc-700'
                }`}
              >
                <span
                  className={`absolute top-0.5 left-0.5 w-3 h-3 rounded-full bg-white transition-transform ${
                    settings.appearance.highContrast ? 'translate-x-4' : 'translate-x-0'
                  }`}
                />
              </button>
              <span className="text-[11px] text-zinc-600">
                Increases contrast for better visibility
              </span>
            </div>
          </div>
        )}

        {activeCategory === 'keyboard' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-zinc-100">Keyboard Shortcuts</h3>
              {settings.keybindings && Object.keys(settings.keybindings).length > 0 && (
                <button
                  onClick={() => {
                    for (const id of Object.keys(settings.keybindings || {})) {
                      onUpdateKeybinding(id, undefined);
                    }
                  }}
                  className="text-[11px] text-zinc-500 hover:text-zinc-300 transition-colors"
                >
                  Reset all
                </button>
              )}
            </div>
            <p className="text-[11px] text-zinc-600">
              Click a shortcut to rebind it. Press Escape to cancel.
            </p>

            {(['navigation', 'git'] as const).map((category) => {
              const actions = KEYMAP_ACTIONS.filter((a) => a.category === category);
              if (actions.length === 0) return null;
              return (
                <div key={category} className="space-y-2">
                  <h4 className="text-xs font-semibold text-zinc-500 uppercase tracking-wider">
                    {category}
                  </h4>
                  <div className="space-y-0.5">
                    {actions.map((action) => {
                      const isRecording = recordingAction === action.id;
                      const isCustom = settings.keybindings?.[action.id] !== undefined;
                      const currentChord = keymap[action.id];
                      return (
                        <div
                          key={action.id}
                          className="flex items-center justify-between px-3 py-2 rounded hover:bg-zinc-900/50"
                        >
                          <span className="text-sm text-zinc-300">{action.label}</span>
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => setRecordingAction(isRecording ? null : action.id)}
                              className={`px-2.5 py-1 rounded text-xs font-mono transition-colors ${
                                isRecording
                                  ? 'bg-[#F14E32]/20 text-[#F14E32] border border-[#F14E32]/40 animate-pulse'
                                  : isCustom
                                    ? 'bg-zinc-800 text-zinc-200 border border-zinc-700'
                                    : 'bg-zinc-900 text-zinc-400 border border-zinc-800'
                              }`}
                            >
                              {isRecording ? 'Press keys...' : chordToDisplay(currentChord)}
                            </button>
                            {isCustom && (
                              <button
                                onClick={() => onUpdateKeybinding(action.id, undefined)}
                                className="text-zinc-600 hover:text-zinc-400 text-[10px]"
                                title="Reset to default"
                              >
                                Reset
                              </button>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {activeCategory === 'git' && (
          <div className="space-y-6">
            <h3 className="text-sm font-semibold text-zinc-100">Git</h3>

            {!gitConfig ? (
              <p className="text-xs text-zinc-500">Loading git config...</p>
            ) : (
              <>
                <div className="space-y-1.5">
                  <label className="text-xs text-zinc-400">User name</label>
                  <p className="text-[11px] text-zinc-600">
                    From <code className="text-zinc-500">git config --global user.name</code>
                  </p>
                  <div className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-1.5 text-sm text-zinc-300">
                    {gitConfig.userName || <span className="text-zinc-600 italic">Not set</span>}
                  </div>
                </div>

                <div className="space-y-1.5">
                  <label className="text-xs text-zinc-400">User email</label>
                  <p className="text-[11px] text-zinc-600">
                    From <code className="text-zinc-500">git config --global user.email</code>
                  </p>
                  <div className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-1.5 text-sm text-zinc-300">
                    {gitConfig.userEmail || <span className="text-zinc-600 italic">Not set</span>}
                  </div>
                </div>

                <div className="flex items-center gap-3">
                  <label className="text-xs text-zinc-400">GPG signing</label>
                  <span className={`text-xs px-2 py-0.5 rounded ${
                    gitConfig.gpgSign
                      ? 'bg-green-900/30 text-green-400 border border-green-800/40'
                      : 'bg-zinc-800 text-zinc-500 border border-zinc-700/40'
                  }`}>
                    {gitConfig.gpgSign ? 'Enabled' : 'Disabled'}
                  </span>
                </div>

                <p className="text-[11px] text-zinc-600">
                  These values are read from your global git configuration. Edit them with <code className="text-zinc-500">git config --global</code>.
                </p>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
