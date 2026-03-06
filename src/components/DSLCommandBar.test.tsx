import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import DSLCommandBar from './DSLCommandBar';

describe('DSLCommandBar', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (window.electronAPI.invoke as ReturnType<typeof vi.fn>).mockResolvedValue(null);
  });

  it('renders the input with placeholder', () => {
    render(<DSLCommandBar />);

    expect(screen.getByPlaceholderText(/commits \| where author/)).toBeInTheDocument();
  });

  it('renders the jock prompt', () => {
    render(<DSLCommandBar />);

    expect(screen.getByText('jock>')).toBeInTheDocument();
  });

  it('shows empty state message when no history', () => {
    render(<DSLCommandBar />);

    expect(screen.getByText(/Type a query like/)).toBeInTheDocument();
  });

  it('executes query on form submit', async () => {
    const mockResult = {
      resultKind: 'count',
      count: 5,
    };
    (window.electronAPI.invoke as ReturnType<typeof vi.fn>).mockResolvedValue(mockResult);

    render(<DSLCommandBar />);

    const input = screen.getByPlaceholderText(/commits \| where author/);
    fireEvent.change(input, { target: { value: 'commits | count' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(window.electronAPI.invoke).toHaveBeenCalledWith('dsl:execute', 'commits | count', false);
    });
  });

  it('clears input after execution', async () => {
    (window.electronAPI.invoke as ReturnType<typeof vi.fn>).mockResolvedValue({ resultKind: 'count', count: 1 });

    render(<DSLCommandBar />);

    const input = screen.getByPlaceholderText(/commits \| where author/) as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'commits | count' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(input.value).toBe('');
    });
  });

  it('does not execute empty query', async () => {
    render(<DSLCommandBar />);

    const input = screen.getByPlaceholderText(/commits \| where author/);
    fireEvent.submit(input.closest('form')!);

    expect(window.electronAPI.invoke).not.toHaveBeenCalledWith('dsl:execute', expect.anything(), expect.anything());
  });

  it('handles shell escape with ! prefix', async () => {
    (window.electronAPI.invoke as ReturnType<typeof vi.fn>).mockResolvedValue({ stdout: 'output', stderr: '' });

    render(<DSLCommandBar />);

    const input = screen.getByPlaceholderText(/commits \| where author/);
    fireEvent.change(input, { target: { value: '!ls -la' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(window.electronAPI.invoke).toHaveBeenCalledWith('shell:exec', 'ls -la');
    });
  });

  it('handles execution errors gracefully', async () => {
    (window.electronAPI.invoke as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('parse error'));

    render(<DSLCommandBar />);

    const input = screen.getByPlaceholderText(/commits \| where author/);
    fireEvent.change(input, { target: { value: 'invalid query' } });
    fireEvent.submit(input.closest('form')!);

    await waitFor(() => {
      expect(screen.getByText('parse error')).toBeInTheDocument();
    });
  });
});
