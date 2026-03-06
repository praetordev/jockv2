import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import path from 'path';
import { app } from 'electron';

let client: any = null;

function getProtoPath(): string {
  if (app.isPackaged) {
    return path.join(process.resourcesPath, 'proto', 'jock.proto');
  }
  return path.join(__dirname, '..', 'proto', 'jock.proto');
}

export function createGrpcClient(port: number): void {
  const protoPath = getProtoPath();
  const packageDefinition = protoLoader.loadSync(protoPath, {
    keepCase: false,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  const protoDescriptor = grpc.loadPackageDefinition(packageDefinition);
  const jock = (protoDescriptor as any).jock;

  client = new jock.GitService(
    `127.0.0.1:${port}`,
    grpc.credentials.createInsecure()
  );
}

function call<T>(method: string, request: any): Promise<T> {
  return new Promise((resolve, reject) => {
    if (!client) {
      reject(new Error('gRPC client not initialized'));
      return;
    }
    client[method](request, (err: grpc.ServiceError | null, response: T) => {
      if (err) {
        reject(err);
      } else {
        resolve(response);
      }
    });
  });
}

export function listCommits(repoPath: string, limit = 100, skip = 0, branch = '', filter?: {
  authorPattern?: string;
  grepPattern?: string;
  afterDate?: string;
  beforeDate?: string;
  pathPattern?: string;
}) {
  return call<any>('listCommits', { repoPath, limit, skip, branch, ...filter });
}

export function listBranches(repoPath: string) {
  return call<any>('listBranches', { repoPath });
}

export function getCommitDetails(repoPath: string, hash: string) {
  return call<any>('getCommitDetails', { repoPath, hash });
}

export function getFileDiff(repoPath: string, hash: string, filePath: string) {
  return call<any>('getFileDiff', { repoPath, hash, filePath });
}

export function getStatus(repoPath: string) {
  return call<any>('getStatus', { repoPath });
}

export function pull(repoPath: string, remote = '', branch = '') {
  return call<any>('pull', { repoPath, remote, branch });
}

export function listRemotes(repoPath: string) {
  return call<any>('listRemotes', { repoPath });
}

export function listRemoteBranches(repoPath: string, remote: string) {
  return call<any>('listRemoteBranches', { repoPath, remote });
}

export function executeDSL(repoPath: string, query: string, dryRun = false) {
  return call<any>('executeDsl', { repoPath, query, dryRun });
}

export function dslAutoComplete(repoPath: string, partialQuery: string, cursorPosition: number) {
  return call<any>('dslAutoComplete', { repoPath, partialQuery, cursorPosition });
}

export function stageFiles(repoPath: string, paths: string[] = []) {
  return call<any>('stageFiles', { repoPath, paths });
}

export function unstageFiles(repoPath: string, paths: string[] = []) {
  return call<any>('unstageFiles', { repoPath, paths });
}

export function createCommit(repoPath: string, message: string, amend: boolean = false) {
  return call<any>('createCommit', { repoPath, message, amend });
}

export function getWorkingDiff(repoPath: string, filePath: string, staged: boolean) {
  return call<any>('getWorkingDiff', { repoPath, filePath, staged });
}

export function push(repoPath: string, remote = '', branch = '', force = false, setUpstream = false) {
  return call<any>('push', { repoPath, remote, branch, force, setUpstream });
}

export function merge(repoPath: string, branch: string, noFf = false) {
  return call<any>('merge', { repoPath, branch, noFf });
}

export function createBranch(repoPath: string, name: string, checkout = true, startPoint = '') {
  return call<any>('createBranch', { repoPath, name, checkout, startPoint });
}

export function deleteBranch(repoPath: string, name: string, force = false) {
  return call<any>('deleteBranch', { repoPath, name, force });
}

export function blame(repoPath: string, filePath: string) {
  return call<any>('blame', { repoPath, filePath });
}

export function getConflictDetails(repoPath: string, filePath: string) {
  return call<any>('getConflictDetails', { repoPath, filePath });
}

export function resolveConflict(repoPath: string, filePath: string, strategy: string) {
  return call<any>('resolveConflict', { repoPath, filePath, strategy });
}

export function abortMerge(repoPath: string) {
  return call<any>('abortMerge', { repoPath });
}

export function isMerging(repoPath: string) {
  return call<any>('isMerging', { repoPath });
}

export function listStashes(repoPath: string) {
  return call<any>('listStashes', { repoPath });
}

export function createStash(repoPath: string, message = '', includeUntracked = false) {
  return call<any>('createStash', { repoPath, message, includeUntracked });
}

export function applyStash(repoPath: string, index: number) {
  return call<any>('applyStash', { repoPath, index });
}

export function popStash(repoPath: string, index: number) {
  return call<any>('popStash', { repoPath, index });
}

export function dropStash(repoPath: string, index: number) {
  return call<any>('dropStash', { repoPath, index });
}

export function showStash(repoPath: string, index: number) {
  return call<any>('showStash', { repoPath, index });
}

export function cherryPick(repoPath: string, commitHash: string) {
  return call<any>('cherryPick', { repoPath, commitHash });
}

export function revertCommit(repoPath: string, commitHash: string) {
  return call<any>('revert', { repoPath, commitHash });
}

export function listTags(repoPath: string) {
  return call<any>('listTags', { repoPath });
}

export function createTag(repoPath: string, tagName: string, commitHash = '', message = '') {
  return call<any>('createTag', { repoPath, tagName, commitHash, message });
}

export function deleteTag(repoPath: string, tagName: string) {
  return call<any>('deleteTag', { repoPath, tagName });
}

export function pushTag(repoPath: string, remote: string, tagName: string) {
  return call<any>('pushTag', { repoPath, remote, tagName });
}

export function rebase(repoPath: string, ontoBranch: string) {
  return call<any>('rebase', { repoPath, ontoBranch });
}

export function abortRebase(repoPath: string) {
  return call<any>('abortRebase', { repoPath });
}

export function continueRebase(repoPath: string) {
  return call<any>('continueRebase', { repoPath });
}

export function isRebasing(repoPath: string) {
  return call<any>('isRebasing', { repoPath });
}

export function interactiveRebase(repoPath: string, baseCommit: string, entries: { action: string; hash: string; message: string }[]) {
  return call<any>('interactiveRebase', { repoPath, baseCommit, entries });
}

export function getRebaseTodo(repoPath: string) {
  return call<any>('getRebaseTodo', { repoPath });
}

export function listReflog(repoPath: string, limit = 50) {
  return call<any>('listReflog', { repoPath, limit });
}

// --- Tasks ---

export function listTasks(repoPath: string, statusFilter = '') {
  return call<any>('listTasks', { repoPath, statusFilter });
}

export function createTask(repoPath: string, title: string, description = '', labels: string[] = [], priority = 0) {
  return call<any>('createTask', { repoPath, title, description, labels, priority });
}

export function updateTask(repoPath: string, id: string, fields: { title?: string; description?: string; status?: string; labels?: string[]; branch?: string; priority?: number }) {
  return call<any>('updateTask', { repoPath, id, ...fields });
}

export function deleteTask(repoPath: string, id: string) {
  return call<any>('deleteTask', { repoPath, id });
}

export function startTask(repoPath: string, id: string, createBranch = true) {
  return call<any>('startTask', { repoPath, id, createBranch });
}

