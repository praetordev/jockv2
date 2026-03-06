export interface ThemeVars {
  '--bg-primary': string;
  '--bg-secondary': string;
  '--bg-tertiary': string;
  '--bg-hover': string;
  '--border': string;
  '--text-primary': string;
  '--text-secondary': string;
  '--text-muted': string;
}

export const themes: Record<'dark' | 'light', ThemeVars> = {
  dark: {
    '--bg-primary': '#09090b',
    '--bg-secondary': '#18181b',
    '--bg-tertiary': '#27272a',
    '--bg-hover': 'rgba(39, 39, 42, 0.5)',
    '--border': 'rgba(39, 39, 42, 0.6)',
    '--text-primary': '#e4e4e7',
    '--text-secondary': '#a1a1aa',
    '--text-muted': '#71717a',
  },
  light: {
    '--bg-primary': '#ffffff',
    '--bg-secondary': '#f4f4f5',
    '--bg-tertiary': '#e4e4e7',
    '--bg-hover': 'rgba(228, 228, 231, 0.5)',
    '--border': 'rgba(212, 212, 216, 0.6)',
    '--text-primary': '#18181b',
    '--text-secondary': '#52525b',
    '--text-muted': '#a1a1aa',
  },
};

export function applyTheme(theme: 'dark' | 'light') {
  const vars = themes[theme];
  const root = document.documentElement;
  for (const [key, value] of Object.entries(vars)) {
    root.style.setProperty(key, value);
  }
  // Also set a data attribute for CSS selectors
  root.setAttribute('data-theme', theme);
}
