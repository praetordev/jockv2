package dsl

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/daearol/jockv2/backend/internal/git"
)

// repoMutex prevents concurrent destructive operations on the same repo.
var repoMutexes sync.Map

func getRepoMutex(repoPath string) *sync.Mutex {
	val, _ := repoMutexes.LoadOrStore(repoPath, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// executeAction performs a git mutation on the result set.
func executeAction(ctx *EvalContext, rs *resultSet, stage *ActionStage) (*ActionReport, error) {
	mu := getRepoMutex(ctx.RepoPath)
	mu.Lock()
	defer mu.Unlock()

	// Safety: check for dirty working directory
	if err := checkCleanWorkdir(ctx.RepoPath); err != nil {
		return nil, err
	}

	if ctx.DryRun {
		return dryRunAction(rs, stage)
	}

	var report *ActionReport
	var err error

	switch stage.Kind {
	case "cherry-pick":
		report, err = executeCherryPick(ctx, rs, stage.Target)
	case "revert":
		report, err = executeRevert(ctx, rs)
	case "rebase":
		report, err = executeRebase(ctx, rs, stage.Target)
	case "tag":
		report, err = executeTag(ctx, rs, stage.Target)
	default:
		return nil, fmt.Errorf("unknown action: %s", stage.Kind)
	}

	// Invalidate cache after successful mutation
	if err == nil && ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return report, err
}

func checkCleanWorkdir(repoPath string) error {
	staged, unstaged, untracked, unmerged, err := git.GetStatus(repoPath)
	if err != nil {
		return fmt.Errorf("cannot check working directory: %w", err)
	}
	if len(staged) > 0 || len(unstaged) > 0 || len(untracked) > 0 || len(unmerged) > 0 {
		return &DSLError{
			Message: "working directory is not clean; commit, stash, or discard changes before running actions",
		}
	}
	return nil
}

func dryRunAction(rs *resultSet, stage *ActionStage) (*ActionReport, error) {
	hashes := collectHashes(rs)
	desc := fmt.Sprintf("[DRY RUN] would %s %d commit(s)", stage.Kind, len(hashes))
	if stage.Target != "" {
		desc += fmt.Sprintf(" onto %q", stage.Target)
	}

	return &ActionReport{
		Action:      stage.Kind,
		Affected:    hashes,
		Success:     true,
		DryRun:      true,
		Description: desc,
	}, nil
}

func executeCherryPick(ctx *EvalContext, rs *resultSet, ontoBranch string) (*ActionReport, error) {
	hashes := collectHashes(rs)
	if len(hashes) == 0 {
		return nil, &DSLError{Message: "no commits to cherry-pick"}
	}

	// Switch to target branch
	if err := gitCmd(ctx.RepoPath, "checkout", ontoBranch); err != nil {
		return nil, fmt.Errorf("failed to checkout %q: %w", ontoBranch, err)
	}

	var applied []string
	var errors []string

	// Apply in reverse order (oldest first) for correct topological ordering
	for i := len(hashes) - 1; i >= 0; i-- {
		h := hashes[i]
		if err := gitCmd(ctx.RepoPath, "cherry-pick", h); err != nil {
			errors = append(errors, fmt.Sprintf("cherry-pick %s failed: %v", shortHash(h), err))
			// Abort the cherry-pick to leave repo in clean state
			_ = gitCmd(ctx.RepoPath, "cherry-pick", "--abort")
			break
		}
		applied = append(applied, h)
	}

	return &ActionReport{
		Action:      "cherry-pick",
		Affected:    applied,
		Success:     len(errors) == 0,
		Description: fmt.Sprintf("cherry-picked %d/%d commits onto %q", len(applied), len(hashes), ontoBranch),
		Errors:      errors,
	}, nil
}

func executeRevert(ctx *EvalContext, rs *resultSet) (*ActionReport, error) {
	hashes := collectHashes(rs)
	if len(hashes) == 0 {
		return nil, &DSLError{Message: "no commits to revert"}
	}

	var reverted []string
	var errors []string

	for _, h := range hashes {
		if err := gitCmd(ctx.RepoPath, "revert", "--no-edit", h); err != nil {
			errors = append(errors, fmt.Sprintf("revert %s failed: %v", shortHash(h), err))
			_ = gitCmd(ctx.RepoPath, "revert", "--abort")
			break
		}
		reverted = append(reverted, h)
	}

	return &ActionReport{
		Action:      "revert",
		Affected:    reverted,
		Success:     len(errors) == 0,
		Description: fmt.Sprintf("reverted %d/%d commits", len(reverted), len(hashes)),
		Errors:      errors,
	}, nil
}

func executeRebase(ctx *EvalContext, rs *resultSet, ontoBranch string) (*ActionReport, error) {
	hashes := collectHashes(rs)
	if len(hashes) == 0 {
		return nil, &DSLError{Message: "no commits to rebase"}
	}

	// Rebase from the oldest commit in the set onto the target branch
	oldest := hashes[len(hashes)-1]
	if err := gitCmd(ctx.RepoPath, "rebase", "--onto", ontoBranch, oldest+"^"); err != nil {
		_ = gitCmd(ctx.RepoPath, "rebase", "--abort")
		return &ActionReport{
			Action:      "rebase",
			Affected:    nil,
			Success:     false,
			Description: fmt.Sprintf("rebase onto %q failed", ontoBranch),
			Errors:      []string{fmt.Sprintf("rebase failed: %v", err)},
		}, nil
	}

	return &ActionReport{
		Action:      "rebase",
		Affected:    hashes,
		Success:     true,
		Description: fmt.Sprintf("rebased %d commits onto %q", len(hashes), ontoBranch),
	}, nil
}

func executeTag(ctx *EvalContext, rs *resultSet, tagName string) (*ActionReport, error) {
	hashes := collectHashes(rs)
	if len(hashes) == 0 {
		return nil, &DSLError{Message: "no commits to tag"}
	}

	// Tag the first (most recent) commit
	target := hashes[0]
	if err := gitCmd(ctx.RepoPath, "tag", tagName, target); err != nil {
		return &ActionReport{
			Action:      "tag",
			Affected:    nil,
			Success:     false,
			Description: fmt.Sprintf("failed to tag %s as %q", shortHash(target), tagName),
			Errors:      []string{err.Error()},
		}, nil
	}

	return &ActionReport{
		Action:      "tag",
		Affected:    []string{target},
		Success:     true,
		Description: fmt.Sprintf("tagged %s as %q", shortHash(target), tagName),
	}, nil
}

// --- Helpers ---

func collectHashes(rs *resultSet) []string {
	var hashes []string
	for _, c := range rs.commits {
		if c.Hash != "" {
			hashes = append(hashes, c.Hash)
		}
	}
	return hashes
}

func gitCmd(repoPath string, args ...string) error {
	fullArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
