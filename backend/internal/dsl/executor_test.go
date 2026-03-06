package dsl

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupCleanRepo creates a minimal repo for executor tests (must be clean workdir).
func setupCleanRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "commit.gpgsign", "false")

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line1\n"), 0644)
	run("add", "file.txt")
	run("commit", "-m", "first commit")

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line1\nline2\n"), 0644)
	run("add", "file.txt")
	run("commit", "-m", "second commit")

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line1\nline2\nline3\n"), 0644)
	run("add", "file.txt")
	run("commit", "-m", "third commit")

	return dir
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestExecutorTag(t *testing.T) {
	repo := setupCleanRepo(t)

	result := evalQuery(t, repo, `commits | first 1 | tag "test-tag"`)
	if result.Kind != "commits" {
		t.Fatalf("expected commits result, got %s", result.Kind)
	}

	// Verify tag was created
	tags := gitOutput(t, repo, "tag", "-l")
	if !strings.Contains(tags, "test-tag") {
		t.Errorf("expected test-tag in tags, got: %s", tags)
	}
}

func TestExecutorCherryPick(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "commit.gpgsign", "false")

	// Create base commit
	os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0644)
	run("add", "base.txt")
	run("commit", "-m", "base commit")

	// Create feature branch here (same base)
	run("branch", "feature")

	// Add a commit on main that touches a DIFFERENT file (no conflict)
	os.WriteFile(filepath.Join(dir, "main-only.txt"), []byte("main stuff\n"), 0644)
	run("add", "main-only.txt")
	run("commit", "-m", "main-only change")

	// Cherry-pick the latest main commit onto feature
	result := evalQuery(t, dir, `commits | first 1 | cherry-pick onto "feature"`)
	if result.Kind != "commits" {
		t.Fatalf("expected commits result, got %s", result.Kind)
	}

	// Verify we're on feature branch
	currentBranch := gitOutput(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if currentBranch != "feature" {
		t.Errorf("expected to be on feature, got %s", currentBranch)
	}

	// Verify the cherry-picked commit exists on feature
	log := gitOutput(t, dir, "log", "--oneline", "-3")
	if !strings.Contains(log, "main-only change") {
		t.Errorf("expected cherry-picked 'main-only change' in feature log:\n%s", log)
	}
}

func TestExecutorRevert(t *testing.T) {
	repo := setupCleanRepo(t)

	countBefore := gitOutput(t, repo, "rev-list", "--count", "HEAD")

	evalQuery(t, repo, `commits | first 1 | revert`)

	countAfter := gitOutput(t, repo, "rev-list", "--count", "HEAD")
	if countBefore == countAfter {
		t.Error("expected commit count to increase after revert")
	}

	log := gitOutput(t, repo, "log", "--oneline", "-1")
	if !strings.Contains(log, "Revert") {
		t.Errorf("expected revert commit message, got: %s", log)
	}
}

func TestExecutorDryRun(t *testing.T) {
	repo := setupCleanRepo(t)

	pipeline, err := Parse(`commits | first 1 | tag "dry-tag"`)
	if err != nil {
		t.Fatal(err)
	}

	ctx := &EvalContext{
		RepoPath: repo,
		DryRun:   true,
		Ctx:      context.Background(),
	}

	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatal(err)
	}

	// Tag should NOT exist
	tags := gitOutput(t, repo, "tag", "-l")
	if strings.Contains(tags, "dry-tag") {
		t.Error("dry-run should not create tags")
	}

	// Result should mention dry run
	if len(result.Commits) == 0 {
		t.Fatal("expected result from dry run")
	}
	if !strings.Contains(result.Commits[0].Message, "DRY RUN") {
		t.Errorf("expected DRY RUN in description, got: %s", result.Commits[0].Message)
	}
}

func TestExecutorDirtyWorkdir(t *testing.T) {
	repo := setupCleanRepo(t)

	// Create an untracked file to make workdir dirty
	os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty\n"), 0644)

	pipeline, err := Parse(`commits | first 1 | tag "should-fail"`)
	if err != nil {
		t.Fatal(err)
	}

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
	}

	_, err = Evaluate(ctx, pipeline)
	if err == nil {
		t.Fatal("expected error for dirty workdir, got nil")
	}
	if !strings.Contains(err.Error(), "not clean") {
		t.Errorf("expected 'not clean' error, got: %v", err)
	}
}
