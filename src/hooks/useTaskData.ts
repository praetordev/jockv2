import { useState, useCallback, useEffect } from 'react';
import { Task } from '../types';
import { isElectron } from '../lib/electron';

export function useTasks() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    if (!isElectron) return;
    setLoading(true);
    try {
      const result = await window.electronAPI.invoke('tasks:list');
      setTasks(result.tasks || []);
    } catch {
      setTasks([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const createTask = useCallback(async (title: string, description = '', labels: string[] = [], priority = 0) => {
    if (!isElectron) return null;
    const result = await window.electronAPI.invoke('tasks:create', title, description, labels, priority);
    if (result.task) {
      await refresh();
    }
    return result;
  }, [refresh]);

  const updateTask = useCallback(async (id: string, fields: { title?: string; description?: string; status?: string; labels?: string[]; branch?: string; priority?: number }) => {
    if (!isElectron) return null;
    const result = await window.electronAPI.invoke('tasks:update', id, fields);
    if (result.task) {
      await refresh();
    }
    return result;
  }, [refresh]);

  const deleteTask = useCallback(async (id: string) => {
    if (!isElectron) return null;
    const result = await window.electronAPI.invoke('tasks:delete', id);
    if (result.success) {
      await refresh();
    }
    return result;
  }, [refresh]);

  const startTask = useCallback(async (id: string, createBranch = true) => {
    if (!isElectron) return null;
    const result = await window.electronAPI.invoke('tasks:start', id, createBranch);
    if (result.task) {
      await refresh();
    }
    return result;
  }, [refresh]);

  // Group tasks by status for kanban view
  const backlog = tasks.filter(t => t.status === 'backlog');
  const inProgress = tasks.filter(t => t.status === 'in-progress');
  const done = tasks.filter(t => t.status === 'done');

  return {
    tasks,
    backlog,
    inProgress,
    done,
    loading,
    refresh,
    createTask,
    updateTask,
    deleteTask,
    startTask,
  };
}
