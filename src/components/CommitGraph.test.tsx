import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import CommitGraph from './CommitGraph';
import type { Commit } from '../types';

const makeCommit = (hash: string, message: string, author: string, overrides?: Partial<Commit>): Commit => ({
  hash,
  message,
  author,
  date: '2 days ago',
  graph: {
    color: '#4ade80',
    column: 0,
    connections: [],
  },
  ...overrides,
});

describe('CommitGraph', () => {
  const defaultProps = {
    graphWidth: 80,
    selectedCommit: null,
    animatedCommitHashes: new Set<string>(),
    onSelectCommit: vi.fn(),
    commitSearch: '',
    onSearchChange: vi.fn(),
  };

  it('renders commit rows', () => {
    const commits = [
      makeCommit('abc1234', 'initial commit', 'Alice'),
      makeCommit('def5678', 'add tests', 'Bob'),
    ];

    render(<CommitGraph {...defaultProps} filteredCommits={commits} />);

    expect(screen.getByText('initial commit')).toBeInTheDocument();
    expect(screen.getByText('add tests')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('Bob')).toBeInTheDocument();
  });

  it('renders branch badges', () => {
    const commits = [
      makeCommit('abc1234', 'latest', 'Alice', { branches: ['main', 'develop'] }),
    ];

    render(<CommitGraph {...defaultProps} filteredCommits={commits} />);

    expect(screen.getByText('main')).toBeInTheDocument();
    expect(screen.getByText('develop')).toBeInTheDocument();
  });

  it('renders tag badges', () => {
    const commits = [
      makeCommit('abc1234', 'release', 'Alice', { tags: ['v1.0.0'] }),
    ];

    render(<CommitGraph {...defaultProps} filteredCommits={commits} />);

    expect(screen.getByText('v1.0.0')).toBeInTheDocument();
  });

  it('calls onSelectCommit when a commit row is clicked', () => {
    const onSelectCommit = vi.fn();
    const commits = [makeCommit('abc1234', 'click me', 'Alice')];

    render(<CommitGraph {...defaultProps} filteredCommits={commits} onSelectCommit={onSelectCommit} />);

    fireEvent.click(screen.getByText('click me'));

    expect(onSelectCommit).toHaveBeenCalledTimes(1);
    expect(onSelectCommit).toHaveBeenCalledWith(commits[0]);
  });

  it('highlights selected commit', () => {
    const commits = [
      makeCommit('abc1234', 'selected', 'Alice'),
      makeCommit('def5678', 'not selected', 'Bob'),
    ];

    render(<CommitGraph {...defaultProps} filteredCommits={commits} selectedCommit={commits[0]} />);

    const selectedButton = screen.getByText('selected').closest('button');
    expect(selectedButton?.className).toContain('bg-[#F14E32]/10');
  });

  it('renders search input', () => {
    render(<CommitGraph {...defaultProps} filteredCommits={[]} />);

    expect(screen.getByPlaceholderText('Search commits...')).toBeInTheDocument();
  });

  it('calls onSearchChange when search input changes', () => {
    const onSearchChange = vi.fn();

    render(<CommitGraph {...defaultProps} filteredCommits={[]} onSearchChange={onSearchChange} />);

    fireEvent.change(screen.getByPlaceholderText('Search commits...'), { target: { value: 'fix' } });

    expect(onSearchChange).toHaveBeenCalledWith('fix');
  });

  it('renders column headers', () => {
    render(<CommitGraph {...defaultProps} filteredCommits={[]} />);

    expect(screen.getByText('Description')).toBeInTheDocument();
    expect(screen.getByText('Author')).toBeInTheDocument();
    expect(screen.getByText('Date')).toBeInTheDocument();
    expect(screen.getByText('Commit')).toBeInTheDocument();
  });
});
