import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import TaskBoard from './TaskBoard';
import type { Task } from '../types';

const makeTask = (overrides: Partial<Task> = {}): Task => ({
  id: '001',
  title: 'Test task',
  description: 'A task for testing',
  status: 'backlog',
  labels: [],
  branch: '',
  commits: [],
  created: '2026-01-01T00:00:00Z',
  updated: '2026-01-01T00:00:00Z',
  priority: 0,
  ...overrides,
});

const defaultProps = {
  backlog: [] as Task[],
  inProgress: [] as Task[],
  done: [] as Task[],
  loading: false,
  onCreateTask: vi.fn().mockResolvedValue({}),
  onUpdateTask: vi.fn().mockResolvedValue({}),
  onDeleteTask: vi.fn().mockResolvedValue({}),
  onStartTask: vi.fn().mockResolvedValue({}),
};

describe('TaskBoard', () => {
  it('renders three columns', () => {
    render(<TaskBoard {...defaultProps} />);

    expect(screen.getByText('Backlog')).toBeInTheDocument();
    expect(screen.getByText('In Progress')).toBeInTheDocument();
    expect(screen.getByText('Done')).toBeInTheDocument();
  });

  it('renders tasks in correct columns', () => {
    const backlog = [makeTask({ id: '001', title: 'Plan feature', status: 'backlog' })];
    const inProgress = [makeTask({ id: '002', title: 'Build feature', status: 'in-progress' })];
    const done = [makeTask({ id: '003', title: 'Ship feature', status: 'done' })];

    render(<TaskBoard {...defaultProps} backlog={backlog} inProgress={inProgress} done={done} />);

    expect(screen.getByText('Plan feature')).toBeInTheDocument();
    expect(screen.getByText('Build feature')).toBeInTheDocument();
    expect(screen.getByText('Ship feature')).toBeInTheDocument();
  });

  it('shows task ID badge', () => {
    const backlog = [makeTask({ id: '042', title: 'My task' })];
    render(<TaskBoard {...defaultProps} backlog={backlog} />);

    expect(screen.getByText('#042')).toBeInTheDocument();
  });

  it('shows task labels', () => {
    const backlog = [makeTask({ labels: ['bug', 'P0'] })];
    render(<TaskBoard {...defaultProps} backlog={backlog} />);

    expect(screen.getByText('bug')).toBeInTheDocument();
    expect(screen.getByText('P0')).toBeInTheDocument();
  });

  it('shows branch name when set', () => {
    const backlog = [makeTask({ branch: 'feature/cool-thing' })];
    render(<TaskBoard {...defaultProps} backlog={backlog} />);

    expect(screen.getByText('feature/cool-thing')).toBeInTheDocument();
  });

  it('shows empty state for columns', () => {
    render(<TaskBoard {...defaultProps} />);

    const noTasksElements = screen.getAllByText('No tasks');
    expect(noTasksElements).toHaveLength(3);
  });

  it('toggles create form on New Task click', () => {
    render(<TaskBoard {...defaultProps} />);

    expect(screen.queryByPlaceholderText('Task title...')).not.toBeInTheDocument();

    fireEvent.click(screen.getByText('New Task'));

    expect(screen.getByPlaceholderText('Task title...')).toBeInTheDocument();
  });

  it('calls onCreateTask with form values', async () => {
    const onCreateTask = vi.fn().mockResolvedValue({});
    render(<TaskBoard {...defaultProps} onCreateTask={onCreateTask} />);

    fireEvent.click(screen.getByText('New Task'));
    fireEvent.change(screen.getByPlaceholderText('Task title...'), { target: { value: 'My new task' } });
    fireEvent.click(screen.getByText('Create'));

    await waitFor(() => {
      expect(onCreateTask).toHaveBeenCalledWith('My new task', '', [], 0);
    });
  });

  it('shows Start button for backlog tasks when expanded', () => {
    const backlog = [makeTask({ status: 'backlog' })];
    render(<TaskBoard {...defaultProps} backlog={backlog} />);

    fireEvent.click(screen.getByText('Test task'));

    expect(screen.getByText('Start')).toBeInTheDocument();
  });

  it('shows Complete button for in-progress tasks when expanded', () => {
    const inProgress = [makeTask({ id: '002', title: 'Active task', status: 'in-progress' })];
    render(<TaskBoard {...defaultProps} inProgress={inProgress} />);

    fireEvent.click(screen.getByText('Active task'));

    expect(screen.getByText('Complete')).toBeInTheDocument();
  });

  it('shows Reopen button for done tasks when expanded', () => {
    const done = [makeTask({ id: '003', title: 'Finished task', status: 'done' })];
    render(<TaskBoard {...defaultProps} done={done} />);

    fireEvent.click(screen.getByText('Finished task'));

    expect(screen.getByText('Reopen')).toBeInTheDocument();
  });

  it('calls onStartTask when Start is clicked', async () => {
    const onStartTask = vi.fn().mockResolvedValue({});
    const backlog = [makeTask({ id: '001', status: 'backlog' })];
    render(<TaskBoard {...defaultProps} backlog={backlog} onStartTask={onStartTask} />);

    fireEvent.click(screen.getByText('Test task'));
    fireEvent.click(screen.getByText('Start'));

    await waitFor(() => {
      expect(onStartTask).toHaveBeenCalledWith('001', true);
    });
  });

  it('displays column counts', () => {
    const backlog = [makeTask({ id: '001' }), makeTask({ id: '002', title: 'Second' })];
    render(<TaskBoard {...defaultProps} backlog={backlog} />);

    expect(screen.getByText('2')).toBeInTheDocument();
  });
});
