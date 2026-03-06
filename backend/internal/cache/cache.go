package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/daearol/jockv2/backend/internal/git"
)

// cacheEntry holds cached data with a timestamp.
type cacheEntry[T any] struct {
	data      T
	fetchedAt time.Time
}

// RepoCache holds cached git data for a single repository.
type RepoCache struct {
	mu          sync.RWMutex
	commits     map[string]cacheEntry[[]git.RawCommit]
	branches    *cacheEntry[[]git.BranchInfo]
	invalidated time.Time
}

// Manager manages per-repo caches with TTL-based expiration.
type Manager struct {
	mu    sync.RWMutex
	repos map[string]*RepoCache
	ttl   time.Duration
}

// NewManager creates a cache manager with the given TTL.
func NewManager(ttl time.Duration) *Manager {
	return &Manager{
		repos: make(map[string]*RepoCache),
		ttl:   ttl,
	}
}

func (m *Manager) getRepo(repoPath string) *RepoCache {
	m.mu.RLock()
	rc, ok := m.repos[repoPath]
	m.mu.RUnlock()
	if ok {
		return rc
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check after write lock
	if rc, ok = m.repos[repoPath]; ok {
		return rc
	}
	rc = &RepoCache{
		commits: make(map[string]cacheEntry[[]git.RawCommit]),
	}
	m.repos[repoPath] = rc
	return rc
}

func commitKey(branch string, limit, skip int, needStats bool) string {
	if branch == "" {
		branch = "all"
	}
	return fmt.Sprintf("%s|%d|%d|%t", branch, limit, skip, needStats)
}

func (rc *RepoCache) isValid(fetchedAt time.Time, ttl time.Duration) bool {
	if fetchedAt.Before(rc.invalidated) {
		return false
	}
	return time.Since(fetchedAt) < ttl
}

// GetCommits returns cached commits or fetches from git on cache miss.
func (m *Manager) GetCommits(repoPath string, limit, skip int, branch string, needStats bool) ([]git.RawCommit, error) {
	rc := m.getRepo(repoPath)
	key := commitKey(branch, limit, skip, needStats)

	// Check cache
	rc.mu.RLock()
	entry, ok := rc.commits[key]
	if ok && rc.isValid(entry.fetchedAt, m.ttl) {
		rc.mu.RUnlock()
		return entry.data, nil
	}
	rc.mu.RUnlock()

	// Cache miss — fetch from git
	var raw []git.RawCommit
	var err error
	if needStats {
		raw, err = git.ListRawCommitsWithStats(repoPath, limit, skip, branch, nil)
	} else {
		raw, err = git.ListRawCommits(repoPath, limit, skip, branch, nil)
	}
	if err != nil {
		return nil, err
	}

	// Store in cache
	rc.mu.Lock()
	rc.commits[key] = cacheEntry[[]git.RawCommit]{
		data:      raw,
		fetchedAt: time.Now(),
	}
	rc.mu.Unlock()

	return raw, nil
}

// GetBranches returns cached branches or fetches from git on cache miss.
func (m *Manager) GetBranches(repoPath string) ([]git.BranchInfo, error) {
	rc := m.getRepo(repoPath)

	rc.mu.RLock()
	if rc.branches != nil && rc.isValid(rc.branches.fetchedAt, m.ttl) {
		data := rc.branches.data
		rc.mu.RUnlock()
		return data, nil
	}
	rc.mu.RUnlock()

	branches, err := git.ListBranches(repoPath)
	if err != nil {
		return nil, err
	}

	rc.mu.Lock()
	rc.branches = &cacheEntry[[]git.BranchInfo]{
		data:      branches,
		fetchedAt: time.Now(),
	}
	rc.mu.Unlock()

	return branches, nil
}

// Invalidate clears all cached data for a repository.
func (m *Manager) Invalidate(repoPath string) {
	rc := m.getRepo(repoPath)
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.invalidated = time.Now()
	rc.commits = make(map[string]cacheEntry[[]git.RawCommit])
	rc.branches = nil
}
