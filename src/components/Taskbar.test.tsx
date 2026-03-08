import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import Taskbar from './Taskbar';

const defaultProps = {
  mainView: 'graph' as const,
  setMainView: vi.fn(),
  pushing: false,
  pushResult: null,
  hasRemote: true,
  onPush: vi.fn(),
  onOpenRepo: vi.fn(),
  onRefresh: vi.fn(),
  tabs: [{ path: '/repos/myrepo', name: 'myrepo' }],
  activeTabIndex: 0,
  onSwitchTab: vi.fn(),
  onCloseTab: vi.fn(),
};

describe('Taskbar', () => {
  it('renders view buttons', () => {
    render(<Taskbar {...defaultProps} />);

    expect(screen.getByText('Graph')).toBeInTheDocument();
    expect(screen.getByText('Editor')).toBeInTheDocument();
    expect(screen.getByText('Changes')).toBeInTheDocument();
    expect(screen.getByText('Tasks')).toBeInTheDocument();
  });

  it('renders Open and Push buttons', () => {
    render(<Taskbar {...defaultProps} />);

    expect(screen.getByText('Open')).toBeInTheDocument();
    expect(screen.getByText('Push')).toBeInTheDocument();
  });

  it('highlights active view', () => {
    render(<Taskbar {...defaultProps} mainView="editor" />);

    const editorBtn = screen.getByText('Editor').closest('button');
    expect(editorBtn?.className).toContain('text-zinc-200');
    expect(editorBtn?.className).toContain('bg-zinc-800');
  });

  it('calls setMainView when a view button is clicked', () => {
    const setMainView = vi.fn();
    render(<Taskbar {...defaultProps} setMainView={setMainView} />);

    fireEvent.click(screen.getByText('Changes'));
    expect(setMainView).toHaveBeenCalledWith('changes');
  });

  it('calls onOpenRepo when Open is clicked', () => {
    const onOpenRepo = vi.fn();
    render(<Taskbar {...defaultProps} onOpenRepo={onOpenRepo} />);

    fireEvent.click(screen.getByText('Open'));
    expect(onOpenRepo).toHaveBeenCalledTimes(1);
  });

  it('calls onPush when Push is clicked', () => {
    const onPush = vi.fn();
    render(<Taskbar {...defaultProps} onPush={onPush} />);

    fireEvent.click(screen.getByText('Push'));
    expect(onPush).toHaveBeenCalledTimes(1);
  });

  it('calls onRefresh when refresh is clicked', () => {
    const onRefresh = vi.fn();
    render(<Taskbar {...defaultProps} onRefresh={onRefresh} />);

    const refreshBtn = screen.getByLabelText('Refresh');
    fireEvent.click(refreshBtn);
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

  it('shows "Pushing..." when push is in progress', () => {
    render(<Taskbar {...defaultProps} pushing={true} />);

    expect(screen.getByText('Pushing...')).toBeInTheDocument();
    expect(screen.queryByText('Push')).not.toBeInTheDocument();
  });

  it('disables push button while pushing', () => {
    render(<Taskbar {...defaultProps} pushing={true} />);

    const pushBtn = screen.getByText('Pushing...').closest('button');
    expect(pushBtn).toBeDisabled();
  });

  it('shows success message after push', () => {
    render(<Taskbar {...defaultProps} pushResult={{ success: true }} />);

    expect(screen.getByText('Pushed')).toBeInTheDocument();
  });

  it('shows error message after push failure', () => {
    render(<Taskbar {...defaultProps} pushResult={{ success: false, error: 'rejected: non-fast-forward' }} />);

    expect(screen.getByText('rejected: non-fast-forward')).toBeInTheDocument();
  });

  it('truncates long error messages', () => {
    const longError = 'a'.repeat(100);
    render(<Taskbar {...defaultProps} pushResult={{ success: false, error: longError }} />);

    const errorEl = screen.getByText(longError.slice(0, 40));
    expect(errorEl).toBeInTheDocument();
  });
});
