import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// Mock window.electronAPI for all component tests
const mockElectronAPI = {
  invoke: vi.fn().mockResolvedValue(null),
  send: vi.fn(),
  on: vi.fn().mockReturnValue(() => {}),
  platform: 'darwin',
};

Object.defineProperty(window, 'electronAPI', {
  value: mockElectronAPI,
  writable: true,
});
