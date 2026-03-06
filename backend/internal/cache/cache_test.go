package cache

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Alice",
			"GIT_AUTHOR_EMAIL=alice@test.com",
			"GIT_COMMITTER_NAME=Alice",
			"GIT_COMMITTER_EMAIL=alice@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "commit.gpgsign", "false")

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	run("add", "main.go")
	run("commit", "-m", "initial commit")

	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\n"), 0644)
	run("add", "test.go")
	run("commit", "-m", "add test")

	return dir
}

func TestCacheHit(t *testing.T) {
	repo := setupTestRepo(t)
	m := NewManager(30 * time.Second)

	// First call — cache miss, fetches from git
	commits1, err := m.GetCommits(repo, 100, 0, "", false)
	if err != nil {
		t.Fatalf("first GetCommits: %v", err)
	}
	if len(commits1) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits1))
	}

	// Second call — cache hit, should return same data
	commits2, err := m.GetCommits(repo, 100, 0, "", false)
	if err != nil {
		t.Fatalf("second GetCommits: %v", err)
	}
	if len(commits2) != len(commits1) {
		t.Errorf("cache hit returned different count: %d vs %d", len(commits2), len(commits1))
	}
	if commits2[0].Hash != commits1[0].Hash {
		t.Error("cache hit returned different data")
	}
}

func TestCacheMiss_Expired(t *testing.T) {
	repo := setupTestRepo(t)
	m := NewManager(1 * time.Millisecond) // very short TTL

	commits1, err := m.GetCommits(repo, 100, 0, "", false)
	if err != nil {
		t.Fatalf("first GetCommits: %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(5 * time.Millisecond)

	// Should re-fetch (cache expired)
	commits2, err := m.GetCommits(repo, 100, 0, "", false)
	if err != nil {
		t.Fatalf("second GetCommits: %v", err)
	}
	if len(commits2) != len(commits1) {
		t.Errorf("re-fetch returned different count: %d vs %d", len(commits2), len(commits1))
	}
}

func TestInvalidate(t *testing.T) {
	repo := setupTestRepo(t)
	m := NewManager(30 * time.Second)

	// Populate cache
	_, err := m.GetCommits(repo, 100, 0, "", false)
	if err != nil {
		t.Fatalf("GetCommits: %v", err)
	}

	// Verify cache is populated
	rc := m.getRepo(repo)
	rc.mu.RLock()
	cachedBefore := len(rc.commits)
	rc.mu.RUnlock()
	if cachedBefore == 0 {
		t.Fatal("expected cache to be populated")
	}

	// Invalidate
	m.Invalidate(repo)

	// Cache should be cleared
	rc.mu.RLock()
	cachedAfter := len(rc.commits)
	rc.mu.RUnlock()
	if cachedAfter != 0 {
		t.Errorf("expected empty cache after invalidation, got %d entries", cachedAfter)
	}
}

func TestCacheBranches(t *testing.T) {
	repo := setupTestRepo(t)
	m := NewManager(30 * time.Second)

	branches1, err := m.GetBranches(repo)
	if err != nil {
		t.Fatalf("first GetBranches: %v", err)
	}
	if len(branches1) == 0 {
		t.Fatal("expected at least one branch")
	}

	// Second call — cache hit
	branches2, err := m.GetBranches(repo)
	if err != nil {
		t.Fatalf("second GetBranches: %v", err)
	}
	if len(branches2) != len(branches1) {
		t.Errorf("cache hit returned different count: %d vs %d", len(branches2), len(branches1))
	}
}

func TestConcurrentAccess(t *testing.T) {
	repo := setupTestRepo(t)
	m := NewManager(30 * time.Second)

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.GetCommits(repo, 100, 0, "", false)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestDifferentCacheKeys(t *testing.T) {
	repo := setupTestRepo(t)
	m := NewManager(30 * time.Second)

	// Fetch without stats
	noStats, err := m.GetCommits(repo, 100, 0, "", false)
	if err != nil {
		t.Fatalf("GetCommits without stats: %v", err)
	}

	// Fetch with stats — different cache key
	withStats, err := m.GetCommits(repo, 100, 0, "", true)
	if err != nil {
		t.Fatalf("GetCommits with stats: %v", err)
	}

	// Both should return same number of commits
	if len(noStats) != len(withStats) {
		t.Errorf("expected same count, got %d vs %d", len(noStats), len(withStats))
	}

	// Verify two cache entries exist
	rc := m.getRepo(repo)
	rc.mu.RLock()
	entries := len(rc.commits)
	rc.mu.RUnlock()
	if entries != 2 {
		t.Errorf("expected 2 cache entries (stats/no-stats), got %d", entries)
	}
}
