import { app, BrowserWindow, shell, ipcMain, dialog, Menu, autoUpdater as electronAutoUpdater } from 'electron';
import { autoUpdater } from 'electron-updater';
import path from 'node:path';
import { startSidecar, stopSidecar } from './sidecar';
import {
  createGrpcClient,
  listCommits,
  listBranches,
  getCommitDetails,
  getFileDiff,
  getStatus,
  pull,
  listRemotes,
  listRemoteBranches,
  executeDSL,
  dslAutoComplete,
  stageFiles,
  unstageFiles,
  createCommit,
  getWorkingDiff,
  push,
  merge,
  createBranch,
  deleteBranch,
  blame,
  getConflictDetails,
  resolveConflict,
  abortMerge,
  isMerging,
  listStashes,
  createStash,
  applyStash,
  popStash,
  dropStash,
  showStash,
  cherryPick,
  revertCommit,
  listTags,
  createTag,
  deleteTag,
  pushTag,
  rebase,
  abortRebase,
  continueRebase,
  isRebasing,
  interactiveRebase,
  getRebaseTodo,
  listReflog,
  listTasks,
  createTask,
  updateTask,
  deleteTask,
  startTask,
} from './grpc-client';
import { createPty, createVexPty, writeToPty, resizePty, killPty, killAllPtys } from './pty-manager';
import { execSync } from 'child_process';
import { watch, FSWatcher, mkdirSync, readFileSync, writeFileSync, existsSync } from 'fs';
import os from 'node:os';

process.env.DIST = path.join(__dirname, '../dist');
process.env.VITE_PUBLIC = app.isPackaged
  ? process.env.DIST
  : path.join(process.env.DIST, '../public');

let mainWindow: BrowserWindow | null = null;
let currentRepoPath: string | null = null;
let repoWatcher: FSWatcher | null = null;

const JOCK_HOME = path.join(os.homedir(), '.jock');
const HISTORY_FILE = path.join(JOCK_HOME, 'history.json');
const SETTINGS_FILE = path.join(JOCK_HOME, 'settings.json');

const DEFAULT_SETTINGS = {
  general: { commitListLimit: 100, defaultShell: '' },
  editor: {
    fontSize: 13,
    fontFamily: '"JetBrains Mono", "Menlo", "Monaco", monospace',
    cursorBlink: true,
  },
};

function getSettings(): typeof DEFAULT_SETTINGS {
  try {
    if (existsSync(SETTINGS_FILE)) {
      const raw = JSON.parse(readFileSync(SETTINGS_FILE, 'utf-8'));
      return {
        general: { ...DEFAULT_SETTINGS.general, ...raw.general },
        editor: { ...DEFAULT_SETTINGS.editor, ...raw.editor },
      };
    }
  } catch {}
  return { ...DEFAULT_SETTINGS };
}

function saveSettings(settings: typeof DEFAULT_SETTINGS): void {
  mkdirSync(JOCK_HOME, { recursive: true });
  writeFileSync(SETTINGS_FILE, JSON.stringify(settings, null, 2));
}

function getGitHubCredential(): { username: string; token: string } | null {
  try {
    const input = 'protocol=https\nhost=github.com\n';
    const output = execSync('git credential fill', {
      input,
      encoding: 'utf-8',
      timeout: 5000,
    });
    const map: Record<string, string> = {};
    for (const line of output.trim().split('\n')) {
      const idx = line.indexOf('=');
      if (idx > 0) {
        map[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
      }
    }
    if (map.password) {
      return { username: map.username || '', token: map.password };
    }
  } catch {}
  return null;
}

async function getGitHubUser(token: string): Promise<string | null> {
  try {
    const res = await fetch('https://api.github.com/user', {
      headers: { Authorization: `token ${token}`, 'User-Agent': 'Jock-Git-Client' },
    });
    if (res.ok) {
      const data = await res.json();
      return data.login || null;
    }
  } catch {}
  return null;
}

function getRepoHistory(): string[] {
  try {
    if (existsSync(HISTORY_FILE)) {
      return JSON.parse(readFileSync(HISTORY_FILE, 'utf-8'));
    }
  } catch {}
  return [];
}

function addToRepoHistory(repoPath: string) {
  mkdirSync(JOCK_HOME, { recursive: true });
  const history = getRepoHistory().filter(p => p !== repoPath);
  history.unshift(repoPath);
  if (history.length > 20) history.length = 20;
  writeFileSync(HISTORY_FILE, JSON.stringify(history));
}

function setCurrentRepo(repoPath: string) {
  currentRepoPath = repoPath;
  addToRepoHistory(repoPath);
  startRepoWatcher(repoPath);
}

let pollTimer: ReturnType<typeof setInterval> | null = null;
let lastKnownHead: string | null = null;
let lastKnownBranches: string | null = null;

function emitCurrentBranch(repoPath: string) {
  try {
    const headContent = readFileSync(path.join(repoPath, '.git', 'HEAD'), 'utf-8').trim();
    const match = headContent.match(/^ref: refs\/heads\/(.+)$/);
    const branch = match ? match[1] : null; // null = detached HEAD
    if (mainWindow && !mainWindow.isDestroyed()) {
      mainWindow.webContents.send('git:branch-changed', branch);
    }
  } catch {}
}

function startRepoWatcher(repoPath: string) {
  stopRepoWatcher();

  const notifyChanged = (() => {
    let debounceTimer: ReturnType<typeof setTimeout> | null = null;
    return () => {
      if (debounceTimer) clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        if (mainWindow && !mainWindow.isDestroyed()) {
          mainWindow.webContents.send('repo:file-changed');
        }
      }, 300);
    };
  })();

  // Emit the current branch from .git/HEAD immediately on watch start
  emitCurrentBranch(repoPath);

  // File system watcher for immediate detection
  try {
    repoWatcher = watch(repoPath, { recursive: true }, (_eventType, filename) => {
      if (!filename) return;
      const normalized = filename.replace(/\\/g, '/');
      if (normalized.startsWith('.git/') || normalized.startsWith('.git\\')) {
        // Allow through: ref changes, HEAD, packed-refs, index, merge/rebase state
        const isGitEvent =
          normalized.startsWith('.git/refs/') ||
          normalized === '.git/HEAD' ||
          normalized === '.git/packed-refs' ||
          normalized === '.git/index' ||
          normalized === '.git/MERGE_HEAD' ||
          normalized === '.git/FETCH_HEAD' ||
          normalized === '.git/REBASE_HEAD' ||
          normalized === '.git/CHERRY_PICK_HEAD';
        if (!isGitEvent) return;
        // Instantly emit branch name when HEAD changes
        if (normalized === '.git/HEAD') {
          emitCurrentBranch(repoPath);
        }
      }
      notifyChanged();
    });
  } catch {
    // Watching may fail on some platforms/filesystems
  }

  // Polling fallback: check HEAD + branches every 2s for changes fs.watch may miss
  lastKnownHead = null;
  lastKnownBranches = null;
  try {
    lastKnownHead = execSync('git rev-parse HEAD', { cwd: repoPath, encoding: 'utf-8', timeout: 5000 }).trim();
  } catch {}
  try {
    lastKnownBranches = execSync('git for-each-ref --format="%(refname:short) %(objectname:short)" refs/heads/', { cwd: repoPath, encoding: 'utf-8', timeout: 5000 }).trim();
  } catch {}
  pollTimer = setInterval(() => {
    try {
      let changed = false;
      const head = execSync('git rev-parse HEAD', { cwd: repoPath, encoding: 'utf-8', timeout: 5000 }).trim();
      if (lastKnownHead !== null && head !== lastKnownHead) changed = true;
      lastKnownHead = head;

      const branches = execSync('git for-each-ref --format="%(refname:short) %(objectname:short)" refs/heads/', { cwd: repoPath, encoding: 'utf-8', timeout: 5000 }).trim();
      if (lastKnownBranches !== null && branches !== lastKnownBranches) changed = true;
      lastKnownBranches = branches;

      if (changed) notifyChanged();
    } catch {}
  }, 2000);
}

function stopRepoWatcher() {
  if (repoWatcher) {
    repoWatcher.close();
    repoWatcher = null;
  }
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  lastKnownHead = null;
  lastKnownBranches = null;
}

const VITE_DEV_SERVER_URL = process.env['VITE_DEV_SERVER_URL'];

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 900,
    minHeight: 600,
    title: 'Jock - Git Client',
    backgroundColor: '#09090b',
    titleBarStyle: 'hiddenInset',
    trafficLightPosition: { x: 15, y: 15 },
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url);
    return { action: 'deny' };
  });

  if (VITE_DEV_SERVER_URL) {
    mainWindow.loadURL(VITE_DEV_SERVER_URL);
  } else {
    mainWindow.loadFile(path.join(process.env.DIST!, 'index.html'));
  }
}

// --- IPC Handlers ---

ipcMain.handle('git:open-repo', async () => {
  const jockHome = path.join(os.homedir(), '.jock');
  mkdirSync(jockHome, { recursive: true });
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ['openDirectory'],
    title: 'Open Git Repository',
    defaultPath: jockHome,
  });
  if (!result.canceled && result.filePaths.length > 0) {
    setCurrentRepo(result.filePaths[0]);
    return currentRepoPath;
  }
  return null;
});

ipcMain.handle('git:create-repo', async () => {
  const jockHome = path.join(os.homedir(), '.jock');
  mkdirSync(jockHome, { recursive: true });
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ['openDirectory', 'createDirectory'],
    title: 'Choose folder for new repository',
    defaultPath: jockHome,
  });
  if (!result.canceled && result.filePaths.length > 0) {
    const dir = result.filePaths[0];
    execSync('git init', { cwd: dir });
    setCurrentRepo(dir);
    return currentRepoPath;
  }
  return null;
});

ipcMain.handle('git:clone-repo', async (_event, url: string) => {
  const jockHome = path.join(os.homedir(), '.jock');
  mkdirSync(jockHome, { recursive: true });
  const result = await dialog.showOpenDialog(mainWindow!, {
    properties: ['openDirectory', 'createDirectory'],
    title: 'Choose where to clone',
    defaultPath: jockHome,
  });
  if (!result.canceled && result.filePaths.length > 0) {
    const dest = result.filePaths[0];
    execSync(`git clone ${url}`, { cwd: dest, encoding: 'utf-8', timeout: 120000 });
    const repoName = url.replace(/\.git$/, '').split('/').pop()!;
    const clonedPath = path.join(dest, repoName);
    setCurrentRepo(clonedPath);
    return currentRepoPath;
  }
  return null;
});

ipcMain.handle('git:get-user-info', async () => {
  const info: { gitUser?: string; gitEmail?: string; ghUser?: string } = {};
  try {
    info.gitUser = execSync('git config user.name', { encoding: 'utf-8' }).trim();
  } catch {}
  try {
    info.gitEmail = execSync('git config user.email', { encoding: 'utf-8' }).trim();
  } catch {}
  const cred = getGitHubCredential();
  if (cred) {
    const login = await getGitHubUser(cred.token);
    if (login) info.ghUser = login;
  }
  return info;
});

ipcMain.handle('git:create-github-repo', async (_event, repoName: string, isPrivate: boolean) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  const opts = { cwd: currentRepoPath, encoding: 'utf-8' as const, timeout: 30000 };

  try {
    // Step 1: Ensure there's at least one commit
    let hasCommits = false;
    try {
      execSync('git rev-parse HEAD', opts);
      hasCommits = true;
    } catch {}

    if (!hasCommits) {
      execSync('git add -A', opts);
      execSync('git commit -m "Initial commit"', opts);
    }

    // Step 2: Remove existing 'origin' remote if present (cleanup from previous failed attempt)
    try {
      execSync('git remote remove origin', opts);
    } catch {}

    // Step 3: Get GitHub credentials and username
    const cred = getGitHubCredential();
    if (!cred) {
      return { success: false, error: 'No GitHub credentials found. Configure git credentials for github.com (e.g., store a Personal Access Token via your git credential helper).' };
    }
    const ghUser = await getGitHubUser(cred.token);
    if (!ghUser) {
      return { success: false, error: 'Failed to authenticate with GitHub. Your stored credentials may be expired.' };
    }

    // Step 4: Create the repo on GitHub via REST API
    const createRes = await fetch('https://api.github.com/user/repos', {
      method: 'POST',
      headers: {
        Authorization: `token ${cred.token}`,
        'Content-Type': 'application/json',
        'User-Agent': 'Jock-Git-Client',
      },
      body: JSON.stringify({ name: repoName, private: isPrivate }),
    });
    if (!createRes.ok) {
      const body = await createRes.json().catch(() => ({}));
      const msg = (body as any).message || `HTTP ${createRes.status}`;
      if (!msg.includes('already exists')) {
        return { success: false, error: `Failed to create GitHub repo: ${msg}` };
      }
    }

    // Step 5: Add the remote
    const repoUrl = `https://github.com/${ghUser}/${repoName}.git`;
    try {
      execSync(`git remote add origin ${repoUrl}`, opts);
    } catch (e: any) {
      const msg = (e.stderr || e.message || '').toString();
      if (!msg.includes('already exists')) {
        return { success: false, error: `Failed to add remote: ${msg}` };
      }
    }

    // Step 6: Push to remote
    const branch = execSync('git branch --show-current', opts).trim() || 'main';
    execSync(`git push -u origin ${branch}`, { ...opts, timeout: 60000 });

    return { success: true };
  } catch (err: any) {
    const errorMsg = (err.stderr || err.stdout || err.message || '').toString();
    console.error('[create-github-repo] error:', errorMsg);
    return { success: false, error: errorMsg || 'Unknown error creating GitHub repository' };
  }
});

ipcMain.handle('git:add-remote', (_event, url: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    execSync(`git remote add origin ${url}`, {
      cwd: currentRepoPath,
      encoding: 'utf-8',
    });
    return { success: true };
  } catch (err: any) {
    return { success: false, error: err.stderr || err.message };
  }
});

ipcMain.handle('git:stash-and-switch', async (_event, targetBranch: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  // Validate branch name to prevent command injection
  if (!/^[\w./-]+$/.test(targetBranch)) {
    return { success: false, error: 'Invalid branch name' };
  }
  const opts = { cwd: currentRepoPath, encoding: 'utf-8' as const, timeout: 15000 };
  try {
    // Abort any in-progress merge/cherry-pick/rebase that would block checkout
    const gitDir = path.join(currentRepoPath, '.git');
    if (existsSync(path.join(gitDir, 'MERGE_HEAD'))) {
      execSync('git merge --abort', opts);
    } else if (existsSync(path.join(gitDir, 'CHERRY_PICK_HEAD'))) {
      execSync('git cherry-pick --abort', opts);
    } else if (existsSync(path.join(gitDir, 'rebase-merge')) || existsSync(path.join(gitDir, 'rebase-apply'))) {
      execSync('git rebase --abort', opts);
    }

    const status = execSync('git status --porcelain', opts).trim();
    const hasChanges = status.length > 0;

    // Use direct git commands for stash to avoid gRPC round-trip issues
    if (hasChanges) {
      execSync('git stash push -m "jock-auto-stash"', opts);
    }

    execSync(`git checkout ${targetBranch}`, opts);

    if (hasChanges) {
      try {
        execSync('git stash pop', opts);
      } catch (e: any) {
        return { success: true, warning: 'Switched branch but stash pop had conflicts. Resolve manually.' };
      }
    }

    return { success: true };
  } catch (err: any) {
    return { success: false, error: err.stderr?.toString() || err.message };
  }
});

// --- Stash IPC Handlers ---

ipcMain.handle('git:list-stashes', async () => {
  if (!currentRepoPath) return { stashes: [] };
  try {
    return await listStashes(currentRepoPath);
  } catch {
    return { stashes: [] };
  }
});

ipcMain.handle('git:create-stash', async (_event, message?: string, includeUntracked?: boolean) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await createStash(currentRepoPath, message || '', includeUntracked || false);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:apply-stash', async (_event, index: number) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await applyStash(currentRepoPath, index);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:pop-stash', async (_event, index: number) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await popStash(currentRepoPath, index);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:drop-stash', async (_event, index: number) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await dropStash(currentRepoPath, index);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:show-stash', async (_event, index: number) => {
  if (!currentRepoPath) return { patch: '' };
  try {
    return await showStash(currentRepoPath, index);
  } catch (err: any) {
    return { patch: '', error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:get-repo-path', () => {
  return currentRepoPath;
});

ipcMain.handle('git:get-repo-history', () => {
  return getRepoHistory();
});

ipcMain.handle('git:switch-repo', (_event, repoPath: string) => {
  setCurrentRepo(repoPath);
  return currentRepoPath;
});

ipcMain.handle('git:has-remote', () => {
  if (!currentRepoPath) return false;
  try {
    const output = execSync('git remote', { cwd: currentRepoPath, encoding: 'utf-8' });
    return output.trim().length > 0;
  } catch {
    return false;
  }
});

ipcMain.handle('git:list-commits', async (_event, opts?: {
  limit?: number; skip?: number; branch?: string;
  authorPattern?: string; grepPattern?: string; afterDate?: string; beforeDate?: string; pathPattern?: string;
}) => {
  if (!currentRepoPath) return [];
  const filter = (opts?.authorPattern || opts?.grepPattern || opts?.afterDate || opts?.beforeDate || opts?.pathPattern)
    ? { authorPattern: opts?.authorPattern, grepPattern: opts?.grepPattern, afterDate: opts?.afterDate, beforeDate: opts?.beforeDate, pathPattern: opts?.pathPattern }
    : undefined;
  const response = await listCommits(currentRepoPath, opts?.limit, opts?.skip, opts?.branch, filter);
  return response.commits;
});

ipcMain.handle('git:list-branches', async () => {
  if (!currentRepoPath) return [];
  const response = await listBranches(currentRepoPath);
  return response.branches;
});

ipcMain.handle('git:get-commit-details', async (_event, hash: string) => {
  if (!currentRepoPath) return { commit: null, fileChanges: [] };
  const response = await getCommitDetails(currentRepoPath, hash);
  return { commit: response.commit, fileChanges: response.fileChanges };
});

ipcMain.handle('git:get-file-diff', async (_event, hash: string, filePath: string) => {
  if (!currentRepoPath) return '';
  const response = await getFileDiff(currentRepoPath, hash, filePath);
  return response.patch;
});

ipcMain.handle('git:get-status', async () => {
  if (!currentRepoPath) return { files: [] };
  return await getStatus(currentRepoPath);
});

ipcMain.handle('git:pull', async (_event, remote?: string, branch?: string) => {
  if (!currentRepoPath) return { summary: '', updated: 0, error: 'No repository open' };
  try {
    return await pull(currentRepoPath, remote || '', branch || '');
  } catch (err: any) {
    return { summary: '', updated: 0, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:list-remotes', async () => {
  if (!currentRepoPath) return { remotes: [] };
  try {
    return await listRemotes(currentRepoPath);
  } catch {
    return { remotes: [] };
  }
});

ipcMain.handle('git:list-remote-branches', async (_event, remote: string) => {
  if (!currentRepoPath) return { branches: [] };
  try {
    return await listRemoteBranches(currentRepoPath, remote);
  } catch {
    return { branches: [] };
  }
});

// --- Stage / Unstage / Commit IPC Handlers ---

ipcMain.handle('git:stage-files', async (_event, paths?: string[]) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await stageFiles(currentRepoPath, paths || []);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:unstage-files', async (_event, paths?: string[]) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await unstageFiles(currentRepoPath, paths || []);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:create-commit', async (_event, message: string, amend: boolean = false) => {
  if (!currentRepoPath) return { hash: '', error: 'No repository open' };
  try {
    return await createCommit(currentRepoPath, message, amend);
  } catch (err: any) {
    return { hash: '', error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:get-working-diff', async (_event, filePath: string, staged: boolean) => {
  if (!currentRepoPath) return { patch: '' };
  try {
    return await getWorkingDiff(currentRepoPath, filePath, staged);
  } catch (err: any) {
    return { patch: '', error: err.details || err.message || String(err) };
  }
});

// --- Push IPC Handler ---

ipcMain.handle('git:push', async (_event, remote?: string, branch?: string, force?: boolean, setUpstream?: boolean) => {
  if (!currentRepoPath) return { success: false, summary: '', error: 'No repository open' };
  try {
    return await push(currentRepoPath, remote || '', branch || '', force || false, setUpstream || false);
  } catch (err: any) {
    return { success: false, summary: '', error: err.details || err.message || String(err) };
  }
});

// --- Merge IPC Handler ---

ipcMain.handle('git:merge', async (_event, branch: string, noFf?: boolean) => {
  if (!currentRepoPath) return { success: false, summary: '', hasConflicts: false, conflictFiles: [] };
  try {
    return await merge(currentRepoPath, branch, noFf || false);
  } catch (err: any) {
    return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.details || err.message || String(err) };
  }
});

// --- Conflict Resolution IPC Handlers ---

ipcMain.handle('git:get-conflict-details', async (_event, filePath: string) => {
  if (!currentRepoPath) return { oursContent: '', theirsContent: '', rawContent: '' };
  try {
    return await getConflictDetails(currentRepoPath, filePath);
  } catch (err: any) {
    return { oursContent: '', theirsContent: '', rawContent: '', error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:resolve-conflict', async (_event, filePath: string, strategy: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await resolveConflict(currentRepoPath, filePath, strategy);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:abort-merge', async () => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await abortMerge(currentRepoPath);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:is-merging', async () => {
  if (!currentRepoPath) return { isMerging: false };
  try {
    return await isMerging(currentRepoPath);
  } catch {
    return { isMerging: false };
  }
});

// --- Cherry-Pick / Revert IPC Handlers ---

ipcMain.handle('git:cherry-pick', async (_event, commitHash: string) => {
  if (!currentRepoPath) return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: 'No repository open' };
  try {
    return await cherryPick(currentRepoPath, commitHash);
  } catch (err: any) {
    return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:revert', async (_event, commitHash: string) => {
  if (!currentRepoPath) return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: 'No repository open' };
  try {
    return await revertCommit(currentRepoPath, commitHash);
  } catch (err: any) {
    return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.details || err.message || String(err) };
  }
});

// --- Tag Management IPC Handlers ---

ipcMain.handle('git:list-tags', async () => {
  if (!currentRepoPath) return { tags: [] };
  try {
    return await listTags(currentRepoPath);
  } catch {
    return { tags: [] };
  }
});

ipcMain.handle('git:create-tag', async (_event, tagName: string, commitHash?: string, message?: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await createTag(currentRepoPath, tagName, commitHash || '', message || '');
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:delete-tag', async (_event, tagName: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await deleteTag(currentRepoPath, tagName);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:push-tag', async (_event, tagName: string, remote?: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await pushTag(currentRepoPath, remote || 'origin', tagName);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

// --- Rebase IPC Handlers ---

ipcMain.handle('git:rebase', async (_event, ontoBranch: string) => {
  if (!currentRepoPath) return { success: false, summary: '', hasConflicts: false, conflictFiles: [] };
  try {
    return await rebase(currentRepoPath, ontoBranch);
  } catch (err: any) {
    return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:abort-rebase', async () => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await abortRebase(currentRepoPath);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:continue-rebase', async () => {
  if (!currentRepoPath) return { success: false, summary: '', hasConflicts: false, conflictFiles: [] };
  try {
    return await continueRebase(currentRepoPath);
  } catch (err: any) {
    return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:is-rebasing', async () => {
  if (!currentRepoPath) return { isRebasing: false };
  try {
    return await isRebasing(currentRepoPath);
  } catch {
    return { isRebasing: false };
  }
});

ipcMain.handle('git:interactive-rebase', async (_event, baseCommit: string, entries: { action: string; hash: string; message: string }[]) => {
  if (!currentRepoPath) return { success: false, summary: '', hasConflicts: false, conflictFiles: [] };
  try {
    return await interactiveRebase(currentRepoPath, baseCommit, entries);
  } catch (err: any) {
    return { success: false, summary: '', hasConflicts: false, conflictFiles: [], error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:get-rebase-todo', async () => {
  if (!currentRepoPath) return { entries: [] };
  try {
    return await getRebaseTodo(currentRepoPath);
  } catch {
    return { entries: [] };
  }
});

ipcMain.handle('git:list-reflog', async (_event, limit?: number) => {
  if (!currentRepoPath) return { entries: [] };
  try {
    return await listReflog(currentRepoPath, limit || 50);
  } catch {
    return { entries: [] };
  }
});

// --- Task IPC Handlers ---

ipcMain.handle('tasks:list', async (_event, statusFilter?: string) => {
  if (!currentRepoPath) return { tasks: [] };
  try {
    return await listTasks(currentRepoPath, statusFilter || '');
  } catch {
    return { tasks: [] };
  }
});

ipcMain.handle('tasks:create', async (_event, title: string, description?: string, labels?: string[], priority?: number) => {
  if (!currentRepoPath) return { task: null, error: 'No repository open' };
  try {
    const result = await createTask(currentRepoPath, title, description || '', labels || [], priority || 0);
    return result;
  } catch (err: any) {
    return { task: null, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('tasks:update', async (_event, id: string, fields: { title?: string; description?: string; status?: string; labels?: string[]; branch?: string; priority?: number }) => {
  if (!currentRepoPath) return { task: null, error: 'No repository open' };
  try {
    const result = await updateTask(currentRepoPath, id, fields);
    return result;
  } catch (err: any) {
    return { task: null, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('tasks:delete', async (_event, id: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await deleteTask(currentRepoPath, id);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('tasks:start', async (_event, id: string, createBranchFlag?: boolean) => {
  if (!currentRepoPath) return { task: null, error: 'No repository open' };
  try {
    return await startTask(currentRepoPath, id, createBranchFlag ?? true);
  } catch (err: any) {
    return { task: null, error: err.details || err.message || String(err) };
  }
});

// --- Branch Management IPC Handlers ---

ipcMain.handle('git:create-branch', async (_event, name: string, checkout?: boolean, startPoint?: string) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await createBranch(currentRepoPath, name, checkout !== false, startPoint || '');
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:delete-branch', async (_event, name: string, force?: boolean) => {
  if (!currentRepoPath) return { success: false, error: 'No repository open' };
  try {
    return await deleteBranch(currentRepoPath, name, force || false);
  } catch (err: any) {
    return { success: false, error: err.details || err.message || String(err) };
  }
});

ipcMain.handle('git:blame', async (_event, filePath: string) => {
  if (!currentRepoPath) return { lines: [] };
  try {
    const response = await blame(currentRepoPath, filePath);
    return { lines: response.lines || [] };
  } catch (err: any) {
    return { lines: [], error: err.details || err.message || String(err) };
  }
});

// --- Git Config IPC Handler ---

ipcMain.handle('git:get-config', async () => {
  try {
    const userName = execSync('git config --global user.name', { encoding: 'utf-8' }).trim();
    const userEmail = execSync('git config --global user.email', { encoding: 'utf-8' }).trim();
    let gpgSign = false;
    try {
      gpgSign = execSync('git config --global commit.gpgsign', { encoding: 'utf-8' }).trim() === 'true';
    } catch { /* not set */ }
    return { userName, userEmail, gpgSign };
  } catch {
    return { userName: '', userEmail: '', gpgSign: false };
  }
});

// --- File Listing IPC Handlers ---

ipcMain.handle('files:list-tree', async () => {
  if (!currentRepoPath) return { repoPath: null, files: [] };
  try {
    const output = execSync('git ls-files --others --cached --exclude-standard', {
      cwd: currentRepoPath,
      encoding: 'utf-8',
      maxBuffer: 10 * 1024 * 1024,
    });
    const files = output.trim().split('\n').filter(Boolean);
    return { repoPath: currentRepoPath, files };
  } catch {
    return { repoPath: currentRepoPath, files: [] };
  }
});

ipcMain.handle('files:git-status-map', async () => {
  if (!currentRepoPath) return {};
  try {
    const output = execSync('git status --porcelain', {
      cwd: currentRepoPath,
      encoding: 'utf-8',
    });
    const statusMap: Record<string, string> = {};
    for (const line of output.trim().split('\n')) {
      if (!line) continue;
      const xy = line.substring(0, 2);
      const filePath = line.substring(3);
      statusMap[filePath] = xy.trim();
    }
    return statusMap;
  } catch {
    return {};
  }
});

// --- Shell Command Execution ---

ipcMain.handle('shell:exec', async (_event, command: string) => {
  if (!currentRepoPath) return { stdout: '', stderr: 'No repository open', code: 1 };
  try {
    const stdout = execSync(command, {
      cwd: currentRepoPath,
      encoding: 'utf-8',
      timeout: 30000,
      maxBuffer: 5 * 1024 * 1024,
    });
    return { stdout, stderr: '', code: 0 };
  } catch (err: any) {
    return {
      stdout: err.stdout || '',
      stderr: err.stderr || err.message,
      code: err.status ?? 1,
    };
  }
});

// --- DSL IPC Handlers ---

ipcMain.handle('dsl:execute', async (_event, query: string, dryRun: boolean = false) => {
  if (!currentRepoPath) return { resultKind: 'error', error: 'No repository open' };
  try {
    return await executeDSL(currentRepoPath, query, dryRun);
  } catch (err: any) {
    return { resultKind: 'error', error: err.message || String(err) };
  }
});

ipcMain.handle('dsl:autocomplete', async (_event, partialQuery: string, cursorPosition: number) => {
  if (!currentRepoPath) return { suggestions: [] };
  try {
    return await dslAutoComplete(currentRepoPath, partialQuery, cursorPosition);
  } catch {
    return { suggestions: [] };
  }
});

// --- PTY IPC Listeners ---

ipcMain.on('pty:create', (_event, payload: { id: string; cols: number; rows: number; cwd?: string }) => {
  if (!mainWindow) return;
  const settings = getSettings();
  const shell = settings.general.defaultShell || undefined;
  createPty(mainWindow, payload.id, payload.cols, payload.rows, payload.cwd || currentRepoPath, shell);
});

ipcMain.on('pty:create-vex', (_event, payload: { id: string; cols: number; rows: number; filePath: string }) => {
  if (!mainWindow) return;
  createVexPty(mainWindow, payload.id, payload.cols, payload.rows, payload.filePath, currentRepoPath);
});

ipcMain.on('pty:data', (_event, payload: { id: string; data: string }) => {
  writeToPty(payload.id, payload.data);
});

ipcMain.on('pty:resize', (_event, payload: { id: string; cols: number; rows: number }) => {
  resizePty(payload.id, payload.cols, payload.rows);
});

ipcMain.on('pty:kill', (_event, payload: { id: string }) => {
  killPty(payload.id);
});

// --- Settings IPC ---

ipcMain.handle('settings:get', () => {
  return getSettings();
});

ipcMain.handle('settings:set', (_event, newSettings: typeof DEFAULT_SETTINGS) => {
  saveSettings(newSettings);
  if (mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.webContents.send('settings:changed', newSettings);
  }
  return newSettings;
});

// --- Lifecycle ---

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit();
    mainWindow = null;
  }
});

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow();
  }
});

app.on('before-quit', () => {
  stopRepoWatcher();
  killAllPtys();
  stopSidecar();
});

let backendError: string | null = null;

ipcMain.handle('backend:status', () => {
  return { ok: backendError === null, error: backendError };
});

async function startBackendWithRetry(retries = 3): Promise<void> {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      const port = await startSidecar();
      console.log(`Go sidecar started on port ${port}`);
      createGrpcClient(port);
      backendError = null;
      return;
    } catch (err: any) {
      console.error(`Backend startup attempt ${attempt}/${retries} failed:`, err);
      if (attempt === retries) {
        backendError = err.message || String(err);
      }
    }
  }
}

app.whenReady().then(async () => {
  await startBackendWithRetry();
  createWindow();

  if (backendError && mainWindow && !mainWindow.isDestroyed()) {
    mainWindow.webContents.once('did-finish-load', () => {
      mainWindow!.webContents.send('backend:error', backendError);
    });
  }

  // --- Query History Persistence ---
  const historyPath = path.join(app.getPath('userData'), 'query-history.json');

  ipcMain.handle('dsl:get-history', () => {
    try {
      if (existsSync(historyPath)) {
        return JSON.parse(readFileSync(historyPath, 'utf-8'));
      }
    } catch { /* ignore */ }
    return [];
  });

  ipcMain.handle('dsl:add-history', (_event, query: string) => {
    let history: string[] = [];
    try {
      if (existsSync(historyPath)) {
        history = JSON.parse(readFileSync(historyPath, 'utf-8'));
      }
    } catch { /* ignore */ }
    // Deduplicate: remove existing occurrence, then prepend
    history = history.filter(h => h !== query);
    history.unshift(query);
    // Cap at 100 entries
    if (history.length > 100) history = history.slice(0, 100);
    writeFileSync(historyPath, JSON.stringify(history), 'utf-8');
    return history;
  });

  // Build macOS application menu with Preferences
  const isMac = process.platform === 'darwin';
  const template: Electron.MenuItemConstructorOptions[] = [
    ...(isMac ? [{
      label: app.name,
      submenu: [
        { role: 'about' as const },
        { type: 'separator' as const },
        {
          label: 'Preferences...',
          accelerator: 'CmdOrCtrl+,' as const,
          click: () => {
            if (mainWindow && !mainWindow.isDestroyed()) {
              mainWindow.webContents.send('menu:open-settings');
            }
          },
        },
        { type: 'separator' as const },
        { role: 'services' as const },
        { type: 'separator' as const },
        { role: 'hide' as const },
        { role: 'hideOthers' as const },
        { role: 'unhide' as const },
        { type: 'separator' as const },
        { role: 'quit' as const },
      ],
    }] : []),
    {
      label: 'Edit',
      submenu: [
        { role: 'undo' as const },
        { role: 'redo' as const },
        { type: 'separator' as const },
        { role: 'cut' as const },
        { role: 'copy' as const },
        { role: 'paste' as const },
        { role: 'selectAll' as const },
      ],
    },
    {
      label: 'View',
      submenu: [
        { role: 'reload' as const },
        { role: 'forceReload' as const },
        { role: 'toggleDevTools' as const },
        { type: 'separator' as const },
        { role: 'resetZoom' as const },
        { role: 'zoomIn' as const },
        { role: 'zoomOut' as const },
        { type: 'separator' as const },
        { role: 'togglefullscreen' as const },
      ],
    },
    {
      label: 'Window',
      submenu: [
        { role: 'minimize' as const },
        { role: 'zoom' as const },
        ...(isMac ? [
          { type: 'separator' as const },
          { role: 'front' as const },
        ] : [
          { role: 'close' as const },
        ]),
      ],
    },
  ];
  Menu.setApplicationMenu(Menu.buildFromTemplate(template));

  // --- Auto-update ---
  if (app.isPackaged) {
    autoUpdater.autoDownload = false;
    autoUpdater.autoInstallOnAppQuit = true;

    autoUpdater.on('update-available', (info) => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send('updater:update-available', {
          version: info.version,
          releaseNotes: typeof info.releaseNotes === 'string' ? info.releaseNotes : '',
        });
      }
    });

    autoUpdater.on('update-not-available', () => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send('updater:up-to-date');
      }
    });

    autoUpdater.on('download-progress', (progress) => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send('updater:download-progress', {
          percent: Math.round(progress.percent),
        });
      }
    });

    autoUpdater.on('update-downloaded', () => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send('updater:update-downloaded');
      }
    });

    autoUpdater.on('error', (err) => {
      if (mainWindow && !mainWindow.isDestroyed()) {
        mainWindow.webContents.send('updater:error', err.message);
      }
    });

    ipcMain.handle('updater:check', async () => {
      try {
        const result = await autoUpdater.checkForUpdates();
        return { success: true, version: result?.updateInfo?.version };
      } catch (err: any) {
        return { success: false, error: err.message };
      }
    });

    ipcMain.handle('updater:download', async () => {
      try {
        await autoUpdater.downloadUpdate();
        return { success: true };
      } catch (err: any) {
        return { success: false, error: err.message };
      }
    });

    ipcMain.handle('updater:install', () => {
      autoUpdater.quitAndInstall(false, true);
    });

    // Check for updates 5 seconds after launch
    setTimeout(() => autoUpdater.checkForUpdates().catch(() => {}), 5000);
  }
});
