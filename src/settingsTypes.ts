export interface JockSettings {
  general: {
    commitListLimit: number;
    defaultShell: string;
  };
  editor: {
    fontSize: number;
    fontFamily: string;
    cursorBlink: boolean;
  };
  appearance: {
    theme: 'dark' | 'light';
    highContrast: boolean;
  };
  keybindings?: Record<string, string>;
}

export const DEFAULT_SETTINGS: JockSettings = {
  general: {
    commitListLimit: 100,
    defaultShell: '',
  },
  editor: {
    fontSize: 13,
    fontFamily: '"JetBrains Mono", "Menlo", "Monaco", monospace',
    cursorBlink: true,
  },
  appearance: {
    theme: 'dark',
    highContrast: false,
  },
};
