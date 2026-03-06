import { renderHook, act } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { useEditor } from './useEditor';

describe('useEditor', () => {
  it('starts with no tabs', () => {
    const { result } = renderHook(() => useEditor());
    expect(result.current.tabs).toHaveLength(0);
    expect(result.current.activeTabId).toBeNull();
    expect(result.current.activeTab).toBeNull();
  });

  it('opens a file and creates a tab', () => {
    const { result } = renderHook(() => useEditor());

    act(() => {
      result.current.openFile('/src/App.tsx');
    });

    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.tabs[0].filePath).toBe('/src/App.tsx');
    expect(result.current.tabs[0].fileName).toBe('App.tsx');
    expect(result.current.activeTab?.filePath).toBe('/src/App.tsx');
  });

  it('does not duplicate tabs for the same file', () => {
    const { result } = renderHook(() => useEditor());

    act(() => {
      result.current.openFile('/src/App.tsx');
    });
    act(() => {
      result.current.openFile('/src/App.tsx');
    });

    expect(result.current.tabs).toHaveLength(1);
  });

  it('switches active tab when opening existing file', () => {
    const { result } = renderHook(() => useEditor());

    act(() => {
      result.current.openFile('/src/App.tsx');
    });
    act(() => {
      result.current.openFile('/src/main.ts');
    });

    expect(result.current.activeTab?.filePath).toBe('/src/main.ts');

    act(() => {
      result.current.openFile('/src/App.tsx');
    });

    expect(result.current.activeTab?.filePath).toBe('/src/App.tsx');
  });

  it('closes a tab', () => {
    const { result } = renderHook(() => useEditor());

    act(() => {
      result.current.openFile('/src/App.tsx');
    });
    const tabId = result.current.tabs[0].id;

    act(() => {
      result.current.closeTab(tabId);
    });

    expect(result.current.tabs).toHaveLength(0);
    expect(result.current.activeTabId).toBeNull();
  });

  it('activates previous tab after closing active tab', () => {
    const { result } = renderHook(() => useEditor());

    act(() => {
      result.current.openFile('/src/a.ts');
    });
    act(() => {
      result.current.openFile('/src/b.ts');
    });

    const bId = result.current.tabs[1].id;
    expect(result.current.activeTab?.filePath).toBe('/src/b.ts');

    act(() => {
      result.current.closeTab(bId);
    });

    expect(result.current.tabs).toHaveLength(1);
    expect(result.current.activeTab?.filePath).toBe('/src/a.ts');
  });

  it('preserves lane info when opening file with lane', () => {
    const { result } = renderHook(() => useEditor());

    act(() => {
      result.current.openFile('/src/App.tsx', { id: 'lane-1', name: 'Main' });
    });

    expect(result.current.tabs[0].laneId).toBe('lane-1');
    expect(result.current.tabs[0].laneName).toBe('Main');
  });

  it('allows manual tab switching', () => {
    const { result } = renderHook(() => useEditor());

    act(() => {
      result.current.openFile('/src/a.ts');
    });
    act(() => {
      result.current.openFile('/src/b.ts');
    });

    const aId = result.current.tabs[0].id;

    act(() => {
      result.current.setActiveTabId(aId);
    });

    expect(result.current.activeTab?.filePath).toBe('/src/a.ts');
  });
});
