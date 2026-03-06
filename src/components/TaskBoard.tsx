import React, { useState } from 'react';
import { Plus, Play, Trash2, ChevronRight, Tag, GitBranch } from 'lucide-react';
import type { Task } from '../types';

interface TaskBoardProps {
  backlog: Task[];
  inProgress: Task[];
  done: Task[];
  loading: boolean;
  onCreateTask: (title: string, description?: string, labels?: string[], priority?: number) => Promise<any>;
  onUpdateTask: (id: string, fields: { title?: string; description?: string; status?: string; labels?: string[]; branch?: string; priority?: number }) => Promise<any>;
  onDeleteTask: (id: string) => Promise<any>;
  onStartTask: (id: string, createBranch?: boolean) => Promise<any>;
}

const PRIORITY_COLORS: Record<number, string> = {
  0: '',
  1: '#3b82f6',  // blue
  2: '#eab308',  // yellow
  3: '#ef4444',  // red
};

const PRIORITY_LABELS: Record<number, string> = {
  0: '',
  1: 'Low',
  2: 'Med',
  3: 'High',
};

function TaskCard({ task, onUpdate, onDelete, onStart }: {
  task: Task;
  onUpdate: TaskBoardProps['onUpdateTask'];
  onDelete: TaskBoardProps['onDeleteTask'];
  onStart: TaskBoardProps['onStartTask'];
}) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div
      style={{
        background: 'var(--bg-tertiary)',
        borderRadius: 6,
        padding: '8px 10px',
        marginBottom: 6,
        cursor: 'pointer',
        border: '1px solid var(--border)',
      }}
      onClick={() => setExpanded(!expanded)}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
        {task.priority > 0 && (
          <span style={{
            width: 6, height: 6, borderRadius: '50%',
            background: PRIORITY_COLORS[task.priority],
            flexShrink: 0,
          }} title={PRIORITY_LABELS[task.priority]} />
        )}
        <span style={{ color: 'var(--text-muted)', fontSize: 11, flexShrink: 0 }}>
          #{task.id}
        </span>
        <span style={{
          color: 'var(--text-primary)',
          fontSize: 12,
          fontWeight: 500,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
          flex: 1,
        }}>
          {task.title}
        </span>
      </div>

      {task.labels.length > 0 && (
        <div style={{ display: 'flex', gap: 4, marginTop: 4, flexWrap: 'wrap' }}>
          {task.labels.map(label => (
            <span key={label} style={{
              fontSize: 10,
              padding: '1px 5px',
              borderRadius: 3,
              background: 'var(--bg-hover)',
              color: 'var(--text-secondary)',
            }}>
              {label}
            </span>
          ))}
        </div>
      )}

      {task.branch && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 4, marginTop: 4, color: 'var(--text-muted)', fontSize: 11 }}>
          <GitBranch size={10} />
          <span>{task.branch}</span>
        </div>
      )}

      {expanded && (
        <div style={{ marginTop: 8, borderTop: '1px solid var(--border)', paddingTop: 8 }}>
          {task.description && (
            <p style={{ fontSize: 11, color: 'var(--text-secondary)', margin: '0 0 8px 0', whiteSpace: 'pre-wrap' }}>
              {task.description}
            </p>
          )}
          <div style={{ display: 'flex', gap: 4 }}>
            {task.status === 'backlog' && (
              <button
                onClick={(e) => { e.stopPropagation(); onStart(task.id, true); }}
                style={{
                  fontSize: 11, padding: '3px 8px', borderRadius: 4,
                  background: '#22c55e', color: '#fff', border: 'none', cursor: 'pointer',
                  display: 'flex', alignItems: 'center', gap: 3,
                }}
              >
                <Play size={10} /> Start
              </button>
            )}
            {task.status === 'in-progress' && (
              <button
                onClick={(e) => { e.stopPropagation(); onUpdate(task.id, { status: 'done' }); }}
                style={{
                  fontSize: 11, padding: '3px 8px', borderRadius: 4,
                  background: '#3b82f6', color: '#fff', border: 'none', cursor: 'pointer',
                }}
              >
                Complete
              </button>
            )}
            {task.status === 'done' && (
              <button
                onClick={(e) => { e.stopPropagation(); onUpdate(task.id, { status: 'backlog' }); }}
                style={{
                  fontSize: 11, padding: '3px 8px', borderRadius: 4,
                  background: 'var(--bg-hover)', color: 'var(--text-secondary)', border: '1px solid var(--border)', cursor: 'pointer',
                }}
              >
                Reopen
              </button>
            )}
            <button
              onClick={(e) => { e.stopPropagation(); onDelete(task.id); }}
              style={{
                fontSize: 11, padding: '3px 8px', borderRadius: 4,
                background: 'transparent', color: 'var(--text-muted)', border: '1px solid var(--border)', cursor: 'pointer',
                display: 'flex', alignItems: 'center', gap: 3,
              }}
            >
              <Trash2 size={10} />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function Column({ title, tasks, count, onUpdate, onDelete, onStart }: {
  title: string;
  tasks: Task[];
  count: number;
  onUpdate: TaskBoardProps['onUpdateTask'];
  onDelete: TaskBoardProps['onDeleteTask'];
  onStart: TaskBoardProps['onStartTask'];
}) {
  return (
    <div style={{ flex: 1, minWidth: 200 }}>
      <div style={{
        display: 'flex', alignItems: 'center', gap: 6,
        marginBottom: 8, padding: '0 4px',
      }}>
        <span style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-primary)' }}>
          {title}
        </span>
        <span style={{
          fontSize: 10, padding: '1px 6px', borderRadius: 8,
          background: 'var(--bg-hover)', color: 'var(--text-muted)',
        }}>
          {count}
        </span>
      </div>
      <div style={{
        background: 'var(--bg-secondary)',
        borderRadius: 8,
        padding: 6,
        minHeight: 100,
        border: '1px solid var(--border)',
      }}>
        {tasks.map(task => (
          <TaskCard
            key={task.id}
            task={task}
            onUpdate={onUpdate}
            onDelete={onDelete}
            onStart={onStart}
          />
        ))}
        {tasks.length === 0 && (
          <div style={{
            color: 'var(--text-muted)', fontSize: 11,
            textAlign: 'center', padding: 20,
          }}>
            No tasks
          </div>
        )}
      </div>
    </div>
  );
}

export default function TaskBoard({
  backlog, inProgress, done, loading,
  onCreateTask, onUpdateTask, onDeleteTask, onStartTask,
}: TaskBoardProps) {
  const [showCreate, setShowCreate] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const [newDescription, setNewDescription] = useState('');
  const [newPriority, setNewPriority] = useState(0);
  const [newLabels, setNewLabels] = useState('');

  const handleCreate = async () => {
    if (!newTitle.trim()) return;
    const labels = newLabels.trim() ? newLabels.split(',').map(l => l.trim()).filter(Boolean) : [];
    await onCreateTask(newTitle.trim(), newDescription.trim(), labels, newPriority);
    setNewTitle('');
    setNewDescription('');
    setNewPriority(0);
    setNewLabels('');
    setShowCreate(false);
  };

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      {/* Header */}
      <div style={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        padding: '8px 12px',
        borderBottom: '1px solid var(--border)',
      }}>
        <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)' }}>
          Tasks
        </span>
        <button
          onClick={() => setShowCreate(!showCreate)}
          style={{
            fontSize: 11, padding: '3px 10px', borderRadius: 4,
            background: showCreate ? 'var(--bg-tertiary)' : '#3b82f6',
            color: showCreate ? 'var(--text-secondary)' : '#fff',
            border: 'none', cursor: 'pointer',
            display: 'flex', alignItems: 'center', gap: 4,
          }}
        >
          <Plus size={12} /> New Task
        </button>
      </div>

      {/* Create form */}
      {showCreate && (
        <div style={{
          padding: '8px 12px',
          borderBottom: '1px solid var(--border)',
          background: 'var(--bg-secondary)',
        }}>
          <input
            value={newTitle}
            onChange={e => setNewTitle(e.target.value)}
            placeholder="Task title..."
            autoFocus
            onKeyDown={e => { if (e.key === 'Enter') handleCreate(); if (e.key === 'Escape') setShowCreate(false); }}
            style={{
              width: '100%', fontSize: 12, padding: '5px 8px',
              background: 'var(--bg-primary)', color: 'var(--text-primary)',
              border: '1px solid var(--border)', borderRadius: 4,
              outline: 'none', marginBottom: 4, boxSizing: 'border-box',
            }}
          />
          <textarea
            value={newDescription}
            onChange={e => setNewDescription(e.target.value)}
            placeholder="Description (optional)..."
            rows={2}
            style={{
              width: '100%', fontSize: 11, padding: '5px 8px',
              background: 'var(--bg-primary)', color: 'var(--text-primary)',
              border: '1px solid var(--border)', borderRadius: 4,
              outline: 'none', resize: 'vertical', marginBottom: 4, boxSizing: 'border-box',
              fontFamily: 'inherit',
            }}
          />
          <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
            <input
              value={newLabels}
              onChange={e => setNewLabels(e.target.value)}
              placeholder="Labels (comma-separated)"
              style={{
                flex: 1, fontSize: 11, padding: '4px 8px',
                background: 'var(--bg-primary)', color: 'var(--text-primary)',
                border: '1px solid var(--border)', borderRadius: 4,
                outline: 'none',
              }}
            />
            <select
              value={newPriority}
              onChange={e => setNewPriority(Number(e.target.value))}
              style={{
                fontSize: 11, padding: '4px 6px',
                background: 'var(--bg-primary)', color: 'var(--text-primary)',
                border: '1px solid var(--border)', borderRadius: 4,
              }}
            >
              <option value={0}>No priority</option>
              <option value={1}>Low</option>
              <option value={2}>Medium</option>
              <option value={3}>High</option>
            </select>
            <button
              onClick={handleCreate}
              disabled={!newTitle.trim()}
              style={{
                fontSize: 11, padding: '4px 12px', borderRadius: 4,
                background: newTitle.trim() ? '#22c55e' : 'var(--bg-tertiary)',
                color: newTitle.trim() ? '#fff' : 'var(--text-muted)',
                border: 'none', cursor: newTitle.trim() ? 'pointer' : 'default',
              }}
            >
              Create
            </button>
          </div>
        </div>
      )}

      {/* Kanban columns */}
      <div style={{
        flex: 1, display: 'flex', gap: 8, padding: 12,
        overflow: 'auto',
      }}>
        <Column title="Backlog" tasks={backlog} count={backlog.length} onUpdate={onUpdateTask} onDelete={onDeleteTask} onStart={onStartTask} />
        <Column title="In Progress" tasks={inProgress} count={inProgress.length} onUpdate={onUpdateTask} onDelete={onDeleteTask} onStart={onStartTask} />
        <Column title="Done" tasks={done} count={done.length} onUpdate={onUpdateTask} onDelete={onDeleteTask} onStart={onStartTask} />
      </div>
    </div>
  );
}
