package dsl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/daearol/jockv2/backend/internal/git"
)

// --- Phase 1: Branch mutations ---

func executeBranchCreate(ctx *EvalContext, s *BranchCreateStage) (*ActionReport, error) {
	if ctx.DryRun {
		desc := fmt.Sprintf("[DRY RUN] would create branch %q", s.Name)
		if s.From != "" {
			desc += fmt.Sprintf(" from %q", s.From)
		}
		return &ActionReport{Action: "branch-create", Success: true, DryRun: true, Description: desc}, nil
	}

	if err := git.CreateBranch(ctx.RepoPath, s.Name, s.From, false); err != nil {
		return &ActionReport{
			Action:      "branch-create",
			Success:     false,
			Description: fmt.Sprintf("failed to create branch %q: %v", s.Name, err),
			Errors:      []string{err.Error()},
		}, nil
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	desc := fmt.Sprintf("created branch %q", s.Name)
	if s.From != "" {
		desc += fmt.Sprintf(" from %q", s.From)
	}
	return &ActionReport{Action: "branch-create", Affected: []string{s.Name}, Success: true, Description: desc}, nil
}

func executeBranchDelete(ctx *EvalContext, rs *resultSet, flags []string) (*ActionReport, error) {
	if rs.kind != "branches" {
		return nil, &DSLError{Message: "'delete' on branches requires branch results"}
	}

	force := hasFlag(flags, "--force")

	if ctx.DryRun {
		names := branchNames(rs.branches)
		return &ActionReport{
			Action:      "branch-delete",
			Affected:    names,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would delete %d branch(es): %s", len(names), strings.Join(names, ", ")),
		}, nil
	}

	var deleted []string
	var errors []string
	for _, b := range rs.branches {
		if b.IsCurrent {
			errors = append(errors, fmt.Sprintf("cannot delete current branch %q", b.Name))
			continue
		}
		if err := git.DeleteBranch(ctx.RepoPath, b.Name, force); err != nil {
			errors = append(errors, fmt.Sprintf("delete %q: %v", b.Name, err))
		} else {
			deleted = append(deleted, b.Name)
		}
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "branch-delete",
		Affected:    deleted,
		Success:     len(errors) == 0,
		Description: fmt.Sprintf("deleted %d/%d branches", len(deleted), len(rs.branches)),
		Errors:      errors,
	}, nil
}

func branchNames(branches []BranchResult) []string {
	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = b.Name
	}
	return names
}

// --- Phase 1: Tag mutations ---

func executeTagDelete(ctx *EvalContext, rs *resultSet) (*ActionReport, error) {
	// Tags come through as commits with tag refs
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "'delete' on tags requires tagged commit results"}
	}

	if ctx.DryRun {
		var tags []string
		for _, c := range rs.commits {
			tags = append(tags, c.Tags...)
		}
		return &ActionReport{
			Action:      "tag-delete",
			Affected:    tags,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would delete %d tag(s)", len(tags)),
		}, nil
	}

	var deleted []string
	var errors []string
	for _, c := range rs.commits {
		for _, tag := range c.Tags {
			if err := git.DeleteTag(ctx.RepoPath, tag); err != nil {
				errors = append(errors, fmt.Sprintf("delete tag %q: %v", tag, err))
			} else {
				deleted = append(deleted, tag)
			}
		}
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "tag-delete",
		Affected:    deleted,
		Success:     len(errors) == 0,
		Description: fmt.Sprintf("deleted %d tag(s)", len(deleted)),
		Errors:      errors,
	}, nil
}

func executeTagPush(ctx *EvalContext, rs *resultSet) (*ActionReport, error) {
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "'push' on tags requires tagged commit results"}
	}

	if ctx.DryRun {
		var tags []string
		for _, c := range rs.commits {
			tags = append(tags, c.Tags...)
		}
		return &ActionReport{
			Action:      "tag-push",
			Affected:    tags,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would push %d tag(s)", len(tags)),
		}, nil
	}

	var pushed []string
	var errors []string
	for _, c := range rs.commits {
		for _, tag := range c.Tags {
			if err := git.PushTag(ctx.RepoPath, "origin", tag); err != nil {
				errors = append(errors, fmt.Sprintf("push tag %q: %v", tag, err))
			} else {
				pushed = append(pushed, tag)
			}
		}
	}

	return &ActionReport{
		Action:      "tag-push",
		Affected:    pushed,
		Success:     len(errors) == 0,
		Description: fmt.Sprintf("pushed %d tag(s)", len(pushed)),
		Errors:      errors,
	}, nil
}

// --- Phase 1: Stash mutations ---

func executeStashCreate(ctx *EvalContext, s *StashCreateStage) (*ActionReport, error) {
	if ctx.DryRun {
		return &ActionReport{
			Action:      "stash-create",
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would create stash %q", s.Message),
		}, nil
	}

	if err := git.CreateStash(ctx.RepoPath, s.Message, s.IncludeUntracked); err != nil {
		return nil, fmt.Errorf("stash create: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "stash-create",
		Success:     true,
		Description: fmt.Sprintf("stashed changes: %s", s.Message),
	}, nil
}

func executeStashAction(ctx *EvalContext, rs *resultSet, action string) (*ActionReport, error) {
	if rs.kind != "stashes" {
		return nil, &DSLError{Message: fmt.Sprintf("'%s' requires stash results", action)}
	}

	if len(rs.stashes) == 0 {
		return nil, &DSLError{Message: "no stash entries to operate on"}
	}

	if ctx.DryRun {
		return &ActionReport{
			Action:      "stash-" + action,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would %s %d stash(es)", action, len(rs.stashes)),
		}, nil
	}

	var affected []string
	var errors []string

	// Process in reverse order (highest index first) to avoid index shifting
	for i := len(rs.stashes) - 1; i >= 0; i-- {
		s := rs.stashes[i]
		var err error
		switch action {
		case "apply":
			_, err = git.ApplyStash(ctx.RepoPath, s.Index)
		case "pop":
			_, err = git.PopStash(ctx.RepoPath, s.Index)
		case "drop":
			err = git.DropStash(ctx.RepoPath, s.Index)
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s stash@{%d}: %v", action, s.Index, err))
		} else {
			affected = append(affected, fmt.Sprintf("stash@{%d}", s.Index))
		}
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "stash-" + action,
		Affected:    affected,
		Success:     len(errors) == 0,
		Description: fmt.Sprintf("%s %d/%d stash(es)", action, len(affected), len(rs.stashes)),
		Errors:      errors,
	}, nil
}

// --- Phase 1: Merge ---

func executeMerge(ctx *EvalContext, s *MergeStage) (*ActionReport, error) {
	if ctx.DryRun {
		return &ActionReport{
			Action:      "merge",
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would merge %q", s.Branch),
		}, nil
	}

	result, err := git.Merge(ctx.RepoPath, s.Branch, s.NoFF)
	if err != nil {
		return nil, fmt.Errorf("merge: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	report := &ActionReport{
		Action:      "merge",
		Success:     result.Success,
		Description: result.Summary,
	}
	if result.HasConflicts {
		report.Errors = []string{fmt.Sprintf("conflicts in: %s", strings.Join(result.ConflictFiles, ", "))}
	}
	return report, nil
}

// --- Phase 2: Commit ---

func executeCommit(ctx *EvalContext, s *CommitStage) (*ActionReport, error) {
	if ctx.DryRun {
		desc := fmt.Sprintf("[DRY RUN] would commit with message %q", s.Message)
		if s.Amend {
			desc += " (amend)"
		}
		return &ActionReport{Action: "commit", Success: true, DryRun: true, Description: desc}, nil
	}

	result, err := git.CreateCommit(ctx.RepoPath, s.Message, s.Amend)
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "commit",
		Affected:    []string{result.Hash},
		Success:     true,
		Description: fmt.Sprintf("committed %s", result.Hash),
	}, nil
}

// --- Phase 2: Push ---

func executePush(ctx *EvalContext, s *PushStage) (*ActionReport, error) {
	remote := s.Remote
	if remote == "" {
		remote = "origin"
	}

	if ctx.DryRun {
		return &ActionReport{
			Action:      "push",
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would push to %s %s", remote, s.Branch),
		}, nil
	}

	result, err := git.Push(ctx.RepoPath, remote, s.Branch, s.Force, s.SetUpstream)
	if err != nil {
		return nil, fmt.Errorf("push: %w", err)
	}

	return &ActionReport{
		Action:      "push",
		Success:     result.Success,
		Description: result.Summary,
	}, nil
}

// --- Phase 2: Pull ---

func executePull(ctx *EvalContext, s *PullStage) (*ActionReport, error) {
	if ctx.DryRun {
		return &ActionReport{
			Action:      "pull",
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would pull from %s %s", s.Remote, s.Branch),
		}, nil
	}

	result, err := git.Pull(ctx.RepoPath, s.Remote, s.Branch)
	if err != nil {
		return nil, fmt.Errorf("pull: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "pull",
		Success:     true,
		Description: result.Summary,
	}, nil
}

// --- Phase 2: Stage/Unstage ---

func executeStage(ctx *EvalContext, rs *resultSet) (*ActionReport, error) {
	if rs.kind != "status" {
		return nil, &DSLError{Message: "'stage' requires status results"}
	}

	var paths []string
	for _, s := range rs.statusEntries {
		paths = append(paths, s.Path)
	}

	if len(paths) == 0 {
		return &ActionReport{Action: "stage", Success: true, Description: "no files to stage"}, nil
	}

	if ctx.DryRun {
		return &ActionReport{
			Action:      "stage",
			Affected:    paths,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would stage %d file(s)", len(paths)),
		}, nil
	}

	if err := git.StageFiles(ctx.RepoPath, paths); err != nil {
		return nil, fmt.Errorf("stage: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "stage",
		Affected:    paths,
		Success:     true,
		Description: fmt.Sprintf("staged %d file(s)", len(paths)),
	}, nil
}

func executeUnstage(ctx *EvalContext, rs *resultSet) (*ActionReport, error) {
	if rs.kind != "status" {
		return nil, &DSLError{Message: "'unstage' requires status results"}
	}

	var paths []string
	for _, s := range rs.statusEntries {
		if s.Staged {
			paths = append(paths, s.Path)
		}
	}

	if len(paths) == 0 {
		return &ActionReport{Action: "unstage", Success: true, Description: "no staged files to unstage"}, nil
	}

	if ctx.DryRun {
		return &ActionReport{
			Action:      "unstage",
			Affected:    paths,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would unstage %d file(s)", len(paths)),
		}, nil
	}

	if err := git.UnstageFiles(ctx.RepoPath, paths); err != nil {
		return nil, fmt.Errorf("unstage: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "unstage",
		Affected:    paths,
		Success:     true,
		Description: fmt.Sprintf("unstaged %d file(s)", len(paths)),
	}, nil
}

// --- Phase 2: Resolve conflicts ---

func executeResolve(ctx *EvalContext, rs *resultSet, strategy string) (*ActionReport, error) {
	if rs.kind != "conflicts" {
		return nil, &DSLError{Message: "'resolve' requires conflicts results"}
	}

	if strategy == "" {
		strategy = "ours"
	}

	if ctx.DryRun {
		var paths []string
		for _, c := range rs.conflicts {
			paths = append(paths, c.Path)
		}
		return &ActionReport{
			Action:      "resolve",
			Affected:    paths,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would resolve %d conflict(s) with strategy %q", len(paths), strategy),
		}, nil
	}

	var resolved []string
	var errors []string
	for _, c := range rs.conflicts {
		if err := git.ResolveConflict(ctx.RepoPath, c.Path, strategy); err != nil {
			errors = append(errors, fmt.Sprintf("resolve %q: %v", c.Path, err))
		} else {
			resolved = append(resolved, c.Path)
		}
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "resolve",
		Affected:    resolved,
		Success:     len(errors) == 0,
		Description: fmt.Sprintf("resolved %d/%d conflict(s) with %q", len(resolved), len(rs.conflicts), strategy),
		Errors:      errors,
	}, nil
}

// --- Phase 2: Abort / Continue ---

func executeAbort(ctx *EvalContext, s *AbortStage) (*ActionReport, error) {
	if ctx.DryRun {
		return &ActionReport{
			Action:      "abort",
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would abort %s", s.Operation),
		}, nil
	}

	var err error
	switch s.Operation {
	case "merge":
		err = git.AbortMerge(ctx.RepoPath)
	case "rebase":
		err = git.AbortRebase(ctx.RepoPath)
	case "cherry-pick":
		err = git.AbortCherryPick(ctx.RepoPath)
	case "revert":
		err = git.AbortRevert(ctx.RepoPath)
	default:
		return nil, fmt.Errorf("unknown operation to abort: %s", s.Operation)
	}

	if err != nil {
		return nil, fmt.Errorf("abort %s: %w", s.Operation, err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "abort",
		Success:     true,
		Description: fmt.Sprintf("aborted %s", s.Operation),
	}, nil
}

func executeContinue(ctx *EvalContext, s *ContinueStage) (*ActionReport, error) {
	if ctx.DryRun {
		return &ActionReport{
			Action:      "continue",
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would continue %s", s.Operation),
		}, nil
	}

	switch s.Operation {
	case "rebase":
		result, err := git.ContinueRebase(ctx.RepoPath)
		if err != nil {
			return nil, fmt.Errorf("continue rebase: %w", err)
		}
		if ctx.Cache != nil {
			ctx.Cache.Invalidate(ctx.RepoPath)
		}
		return &ActionReport{
			Action:      "continue",
			Success:     result.Success,
			Description: result.Summary,
		}, nil
	default:
		return nil, fmt.Errorf("cannot continue %s", s.Operation)
	}
}

// --- Phase 5: Squash (interactive rebase) ---

func executeSquash(ctx *EvalContext, rs *resultSet, message string) (*ActionReport, error) {
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "'squash' requires commit results"}
	}
	hashes := collectHashes(rs)
	if len(hashes) < 2 {
		return nil, &DSLError{Message: "squash requires at least 2 commits"}
	}

	if ctx.DryRun {
		return &ActionReport{
			Action:      "squash",
			Affected:    hashes,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would squash %d commits", len(hashes)),
		}, nil
	}

	// Build rebase todo: first commit is "pick", rest are "squash"
	entries := make([]git.RebaseTodoEntry, len(hashes))
	for i := len(hashes) - 1; i >= 0; i-- {
		action := "squash"
		if i == len(hashes)-1 {
			action = "pick"
		}
		entries[len(hashes)-1-i] = git.RebaseTodoEntry{
			Action: action,
			Hash:   hashes[i],
		}
	}

	oldest := hashes[len(hashes)-1]
	result, err := git.InteractiveRebase(ctx.RepoPath, oldest+"^", entries)
	if err != nil {
		return nil, fmt.Errorf("squash: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "squash",
		Affected:    hashes,
		Success:     result.Success,
		Description: fmt.Sprintf("squashed %d commits", len(hashes)),
	}, nil
}

// --- Phase 5: Reorder (interactive rebase) ---

func executeReorder(ctx *EvalContext, rs *resultSet, indices []string) (*ActionReport, error) {
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "'reorder' requires commit results"}
	}
	hashes := collectHashes(rs)
	if len(indices) != len(hashes) {
		return nil, &DSLError{Message: fmt.Sprintf("reorder requires %d indices, got %d", len(hashes), len(indices))}
	}

	if ctx.DryRun {
		return &ActionReport{
			Action:      "reorder",
			Affected:    hashes,
			Success:     true,
			DryRun:      true,
			Description: fmt.Sprintf("[DRY RUN] would reorder %d commits", len(hashes)),
		}, nil
	}

	// Build reordered entries
	entries := make([]git.RebaseTodoEntry, len(indices))
	for i, idxStr := range indices {
		idx, err := strconv.Atoi(idxStr)
		if err != nil || idx < 1 || idx > len(hashes) {
			return nil, &DSLError{Message: fmt.Sprintf("invalid reorder index %q", idxStr)}
		}
		entries[i] = git.RebaseTodoEntry{
			Action: "pick",
			Hash:   hashes[idx-1],
		}
	}

	oldest := hashes[len(hashes)-1]
	result, err := git.InteractiveRebase(ctx.RepoPath, oldest+"^", entries)
	if err != nil {
		return nil, fmt.Errorf("reorder: %w", err)
	}

	if ctx.Cache != nil {
		ctx.Cache.Invalidate(ctx.RepoPath)
	}

	return &ActionReport{
		Action:      "reorder",
		Affected:    hashes,
		Success:     result.Success,
		Description: fmt.Sprintf("reordered %d commits", len(hashes)),
	}, nil
}

// --- Helpers ---

func hasFlag(flags []string, flag string) bool {
	for _, f := range flags {
		if f == flag {
			return true
		}
	}
	return false
}
