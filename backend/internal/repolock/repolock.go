package repolock

import "sync"

// Manager provides per-repo mutexes to serialize git write operations.
// Read operations don't need locking — git handles concurrent reads fine.
// Write operations (commit, merge, rebase, push, etc.) can conflict via
// .git/index.lock, so we serialize them per-repo.
type Manager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// New creates a new lock manager.
func New() *Manager {
	return &Manager{locks: make(map[string]*sync.Mutex)}
}

// Lock acquires the write lock for the given repo path.
func (m *Manager) Lock(repoPath string) {
	m.mu.Lock()
	l, ok := m.locks[repoPath]
	if !ok {
		l = &sync.Mutex{}
		m.locks[repoPath] = l
	}
	m.mu.Unlock()
	l.Lock()
}

// Unlock releases the write lock for the given repo path.
func (m *Manager) Unlock(repoPath string) {
	m.mu.Lock()
	l, ok := m.locks[repoPath]
	m.mu.Unlock()
	if ok {
		l.Unlock()
	}
}
