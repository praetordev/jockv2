package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/daearol/jockv2/backend/internal/cache"
	"github.com/daearol/jockv2/backend/internal/dsl"
	"github.com/daearol/jockv2/backend/internal/git"
	"github.com/daearol/jockv2/backend/internal/repolock"
	"github.com/daearol/jockv2/backend/internal/tasks"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	cacheManager = cache.NewManager(30 * time.Second)
	writeLocks   = repolock.New()
)

// detectRepoPath finds the git repo root from the current working directory.
func detectRepoPath() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	// Fallback to cwd
	cwd, _ := os.Getwd()
	return cwd
}

func main() {
	s := server.NewMCPServer(
		"jock-mcp",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	registerReadTools(s)
	registerWriteTools(s)
	registerDSLTools(s)
	registerTaskTools(s)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("mcp server error: %v", err)
	}
}

// --- Helpers ---

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(text)},
	}
}

func jsonResult(v any) *mcp.CallToolResult {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(fmt.Sprintf("json marshal: %v", err))
	}
	return textResult(string(data))
}

func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(msg)},
		IsError: true,
	}
}

func requireString(args map[string]any, key string) (string, *mcp.CallToolResult) {
	v, ok := args[key]
	if !ok {
		return "", errResult(fmt.Sprintf("missing required argument: %s", key))
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", errResult(fmt.Sprintf("argument %s must be a non-empty string", key))
	}
	return s, nil
}

func optString(args map[string]any, key, fallback string) string {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return s
}

func optInt(args map[string]any, key string, fallback int) int {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return fallback
}

func optBool(args map[string]any, key string, fallback bool) bool {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

func optStringSlice(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []any:
		var result []string
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	case string:
		if s == "" {
			return nil
		}
		return strings.Split(s, ",")
	}
	return nil
}

// repoPathProp is the common repo_path property used by all tools.
func repoPathProp() mcp.ToolOption {
	return mcp.WithString("repo_path",
		mcp.Description("Absolute path to the git repository (defaults to current working directory's git root)"),
	)
}

// getRepoPath extracts repo_path from args, falling back to auto-detection.
func getRepoPath(args map[string]any) (string, *mcp.CallToolResult) {
	v, ok := args["repo_path"]
	if ok {
		s, ok := v.(string)
		if ok && s != "" {
			return s, nil
		}
	}
	detected := detectRepoPath()
	if detected == "" {
		return "", errResult("repo_path not provided and could not detect a git repository in the current directory")
	}
	return detected, nil
}

// --- Read-only tools ---

func registerReadTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("jock_git_status",
			mcp.WithDescription("Get the working directory status (staged, unstaged, untracked, unmerged files)"),
			repoPathProp(),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitStatus,
	)

	s.AddTool(
		mcp.NewTool("jock_git_log",
			mcp.WithDescription("List commits from git log with optional filtering by branch, author, date, message pattern, and path"),
			repoPathProp(),
			mcp.WithNumber("limit", mcp.Description("Max commits to return (default 50)")),
			mcp.WithNumber("skip", mcp.Description("Number of commits to skip")),
			mcp.WithString("branch", mcp.Description("Branch name to list commits from (default: all branches)")),
			mcp.WithString("author", mcp.Description("Filter by author pattern")),
			mcp.WithString("grep", mcp.Description("Filter by commit message pattern")),
			mcp.WithString("after", mcp.Description("Only commits after this date (e.g. '2025-01-01')")),
			mcp.WithString("before", mcp.Description("Only commits before this date")),
			mcp.WithString("path", mcp.Description("Only commits touching this file path")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitLog,
	)

	s.AddTool(
		mcp.NewTool("jock_git_diff",
			mcp.WithDescription("Get the diff (patch) for a specific commit, or the working directory diff"),
			repoPathProp(),
			mcp.WithString("hash", mcp.Description("Commit hash to diff. If omitted, shows working directory diff")),
			mcp.WithString("file_path", mcp.Description("Specific file to diff (optional)")),
			mcp.WithBoolean("staged", mcp.Description("If true and no hash, show staged diff instead of unstaged")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitDiff,
	)

	s.AddTool(
		mcp.NewTool("jock_git_blame",
			mcp.WithDescription("Get git blame for a file, showing who last modified each line"),
			repoPathProp(),
			mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to the file within the repo")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitBlame,
	)

	s.AddTool(
		mcp.NewTool("jock_git_branches",
			mcp.WithDescription("List all local branches with current branch indicator, remote tracking, and ahead/behind counts"),
			repoPathProp(),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitBranches,
	)

	s.AddTool(
		mcp.NewTool("jock_git_tags",
			mcp.WithDescription("List all tags in the repository"),
			repoPathProp(),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitTags,
	)

	s.AddTool(
		mcp.NewTool("jock_git_stashes",
			mcp.WithDescription("List all stash entries"),
			repoPathProp(),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitStashes,
	)

	s.AddTool(
		mcp.NewTool("jock_git_reflog",
			mcp.WithDescription("List recent reflog entries showing branch/HEAD history"),
			repoPathProp(),
			mcp.WithNumber("limit", mcp.Description("Max entries to return (default 50)")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitReflog,
	)

	s.AddTool(
		mcp.NewTool("jock_git_remotes",
			mcp.WithDescription("List configured git remotes"),
			repoPathProp(),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitRemotes,
	)

	s.AddTool(
		mcp.NewTool("jock_git_commit_details",
			mcp.WithDescription("Get detailed file changes and patches for a specific commit"),
			repoPathProp(),
			mcp.WithString("hash", mcp.Required(), mcp.Description("Commit hash")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitCommitDetails,
	)

	s.AddTool(
		mcp.NewTool("jock_git_show_stash",
			mcp.WithDescription("Show the diff of a stash entry"),
			repoPathProp(),
			mcp.WithNumber("index", mcp.Description("Stash index (default 0)")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleGitShowStash,
	)
}

func handleGitStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	staged, unstaged, untracked, unmerged, err := git.GetStatus(repoPath)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"staged":    staged,
		"unstaged":  unstaged,
		"untracked": untracked,
		"unmerged":  unmerged,
	}), nil
}

func handleGitLog(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	limit := optInt(args, "limit", 50)
	skip := optInt(args, "skip", 0)
	branch := optString(args, "branch", "")

	var filter *git.CommitFilter
	author := optString(args, "author", "")
	grep := optString(args, "grep", "")
	after := optString(args, "after", "")
	before := optString(args, "before", "")
	path := optString(args, "path", "")
	if author != "" || grep != "" || after != "" || before != "" || path != "" {
		filter = &git.CommitFilter{
			AuthorPattern: author,
			GrepPattern:   grep,
			AfterDate:     after,
			BeforeDate:    before,
			PathPattern:   path,
		}
	}

	commits, err := git.ListRawCommits(repoPath, limit, skip, branch, filter)
	if err != nil {
		return errResult(err.Error()), nil
	}

	type logEntry struct {
		Hash    string   `json:"hash"`
		Message string   `json:"message"`
		Author  string   `json:"author"`
		Date    string   `json:"date"`
		Parents []string `json:"parents,omitempty"`
	}
	var result []logEntry
	for _, c := range commits {
		h := c.Hash
		if len(h) > 7 {
			h = h[:7]
		}
		result = append(result, logEntry{
			Hash:    h,
			Message: c.Message,
			Author:  c.Author,
			Date:    c.Date,
			Parents: c.Parents,
		})
	}
	return jsonResult(result), nil
}

func handleGitDiff(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	hash := optString(args, "hash", "")
	filePath := optString(args, "file_path", "")
	staged := optBool(args, "staged", false)

	if hash != "" {
		if filePath != "" {
			patch, err := git.GetFilePatch(repoPath, hash, filePath)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(patch), nil
		}
		patch, err := git.GetCommitPatch(repoPath, hash)
		if err != nil {
			return errResult(err.Error()), nil
		}
		return textResult(patch), nil
	}

	patch, err := git.GetWorkingDiff(repoPath, filePath, staged)
	if err != nil {
		return errResult(err.Error()), nil
	}
	if patch == "" {
		return textResult("No changes"), nil
	}
	return textResult(patch), nil
}

func handleGitBlame(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	filePath, e := requireString(args, "file_path")
	if e != nil {
		return e, nil
	}
	lines, err := git.Blame(repoPath, filePath)
	if err != nil {
		return errResult(err.Error()), nil
	}
	var sb strings.Builder
	for _, l := range lines {
		h := l.Hash
		if len(h) > 7 {
			h = h[:7]
		}
		fmt.Fprintf(&sb, "%4d  %s  %-12s  %s  %s\n", l.LineNo, h, l.Author, l.Date, l.Content)
	}
	return textResult(sb.String()), nil
}

func handleGitBranches(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	branches, err := git.ListBranches(repoPath)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(branches), nil
}

func handleGitTags(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	tagList, err := git.ListTags(repoPath)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(tagList), nil
}

func handleGitStashes(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	stashes, err := git.ListStashes(repoPath)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(stashes), nil
}

func handleGitReflog(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	limit := optInt(args, "limit", 50)
	entries, err := git.ListReflog(repoPath, limit)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(entries), nil
}

func handleGitRemotes(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	remotes, err := git.ListRemotes(repoPath)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(remotes), nil
}

func handleGitCommitDetails(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	hash, e := requireString(args, "hash")
	if e != nil {
		return e, nil
	}
	files, err := git.GetCommitDiffStat(repoPath, hash)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(files), nil
}

func handleGitShowStash(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	index := optInt(args, "index", 0)
	patch, err := git.ShowStash(repoPath, index)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return textResult(patch), nil
}

// --- Write tools ---

func registerWriteTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("jock_git_stage",
			mcp.WithDescription("Stage files for commit. If no paths given, stages all changes"),
			repoPathProp(),
			mcp.WithArray("paths", mcp.Description("File paths to stage (empty = stage all)")),
		),
		handleGitStage,
	)

	s.AddTool(
		mcp.NewTool("jock_git_unstage",
			mcp.WithDescription("Unstage files. If no paths given, unstages everything"),
			repoPathProp(),
			mcp.WithArray("paths", mcp.Description("File paths to unstage (empty = unstage all)")),
		),
		handleGitUnstage,
	)

	s.AddTool(
		mcp.NewTool("jock_git_commit",
			mcp.WithDescription("Create a git commit with the staged changes"),
			repoPathProp(),
			mcp.WithString("message", mcp.Required(), mcp.Description("Commit message")),
			mcp.WithBoolean("amend", mcp.Description("If true, amend the previous commit")),
		),
		handleGitCommit,
	)

	s.AddTool(
		mcp.NewTool("jock_git_push",
			mcp.WithDescription("Push commits to a remote repository"),
			repoPathProp(),
			mcp.WithString("remote", mcp.Description("Remote name (default: origin)")),
			mcp.WithString("branch", mcp.Description("Branch to push")),
			mcp.WithBoolean("force", mcp.Description("Force push with lease")),
			mcp.WithBoolean("set_upstream", mcp.Description("Set upstream tracking")),
			mcp.WithDestructiveHintAnnotation(true),
		),
		handleGitPush,
	)

	s.AddTool(
		mcp.NewTool("jock_git_pull",
			mcp.WithDescription("Pull changes from a remote repository"),
			repoPathProp(),
			mcp.WithString("remote", mcp.Description("Remote name")),
			mcp.WithString("branch", mcp.Description("Branch to pull")),
		),
		handleGitPull,
	)

	s.AddTool(
		mcp.NewTool("jock_git_merge",
			mcp.WithDescription("Merge a branch into the current branch"),
			repoPathProp(),
			mcp.WithString("branch", mcp.Required(), mcp.Description("Branch to merge")),
			mcp.WithBoolean("no_ff", mcp.Description("Force a merge commit (no fast-forward)")),
		),
		handleGitMerge,
	)

	s.AddTool(
		mcp.NewTool("jock_git_create_branch",
			mcp.WithDescription("Create a new branch, optionally checking it out"),
			repoPathProp(),
			mcp.WithString("name", mcp.Required(), mcp.Description("New branch name")),
			mcp.WithString("start_point", mcp.Description("Commit or branch to start from")),
			mcp.WithBoolean("checkout", mcp.Description("Switch to the new branch after creation")),
		),
		handleGitCreateBranch,
	)

	s.AddTool(
		mcp.NewTool("jock_git_delete_branch",
			mcp.WithDescription("Delete a branch"),
			repoPathProp(),
			mcp.WithString("name", mcp.Required(), mcp.Description("Branch name to delete")),
			mcp.WithBoolean("force", mcp.Description("Force delete even if not fully merged")),
			mcp.WithDestructiveHintAnnotation(true),
		),
		handleGitDeleteBranch,
	)

	s.AddTool(
		mcp.NewTool("jock_git_create_tag",
			mcp.WithDescription("Create a new tag. If message is provided, creates an annotated tag"),
			repoPathProp(),
			mcp.WithString("tag_name", mcp.Required(), mcp.Description("Tag name")),
			mcp.WithString("commit_hash", mcp.Description("Commit to tag (default: HEAD)")),
			mcp.WithString("message", mcp.Description("Tag message (creates annotated tag)")),
		),
		handleGitCreateTag,
	)

	s.AddTool(
		mcp.NewTool("jock_git_stash_create",
			mcp.WithDescription("Stash current working directory changes"),
			repoPathProp(),
			mcp.WithString("message", mcp.Description("Stash message")),
			mcp.WithBoolean("include_untracked", mcp.Description("Include untracked files")),
		),
		handleGitStashCreate,
	)

	s.AddTool(
		mcp.NewTool("jock_git_stash_pop",
			mcp.WithDescription("Pop a stash entry (apply and remove)"),
			repoPathProp(),
			mcp.WithNumber("index", mcp.Description("Stash index (default 0)")),
		),
		handleGitStashPop,
	)

	s.AddTool(
		mcp.NewTool("jock_git_cherry_pick",
			mcp.WithDescription("Cherry-pick a commit onto the current branch"),
			repoPathProp(),
			mcp.WithString("commit_hash", mcp.Required(), mcp.Description("Commit hash to cherry-pick")),
		),
		handleGitCherryPick,
	)

	s.AddTool(
		mcp.NewTool("jock_git_revert",
			mcp.WithDescription("Revert a commit by creating a new commit that undoes its changes"),
			repoPathProp(),
			mcp.WithString("commit_hash", mcp.Required(), mcp.Description("Commit hash to revert")),
		),
		handleGitRevert,
	)

	s.AddTool(
		mcp.NewTool("jock_git_rebase",
			mcp.WithDescription("Rebase the current branch onto another branch"),
			repoPathProp(),
			mcp.WithString("onto_branch", mcp.Required(), mcp.Description("Branch to rebase onto")),
		),
		handleGitRebase,
	)

	s.AddTool(
		mcp.NewTool("jock_git_resolve_conflict",
			mcp.WithDescription("Resolve a merge conflict for a file using a strategy"),
			repoPathProp(),
			mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to the conflicted file")),
			mcp.WithString("strategy", mcp.Required(), mcp.Description("Resolution strategy: 'ours', 'theirs', or 'both'")),
		),
		handleGitResolveConflict,
	)

	s.AddTool(
		mcp.NewTool("jock_git_abort_merge",
			mcp.WithDescription("Abort an in-progress merge"),
			repoPathProp(),
			mcp.WithDestructiveHintAnnotation(true),
		),
		handleGitAbortMerge,
	)
}

func handleGitStage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	paths := optStringSlice(args, "paths")
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.StageFiles(repoPath, paths); err != nil {
		return errResult(err.Error()), nil
	}
	return textResult("Files staged successfully"), nil
}

func handleGitUnstage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	paths := optStringSlice(args, "paths")
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.UnstageFiles(repoPath, paths); err != nil {
		return errResult(err.Error()), nil
	}
	return textResult("Files unstaged successfully"), nil
}

func handleGitCommit(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	message, e := requireString(args, "message")
	if e != nil {
		return e, nil
	}
	amend := optBool(args, "amend", false)
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	result, err := git.CreateCommit(repoPath, message, amend)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]string{"hash": result.Hash}), nil
}

func handleGitPush(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	remote := optString(args, "remote", "")
	branch := optString(args, "branch", "")
	force := optBool(args, "force", false)
	setUpstream := optBool(args, "set_upstream", false)
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	result, err := git.Push(repoPath, remote, branch, force, setUpstream)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"summary": result.Summary, "success": result.Success}), nil
}

func handleGitPull(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	remote := optString(args, "remote", "")
	branch := optString(args, "branch", "")
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	result, err := git.Pull(repoPath, remote, branch)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"summary": result.Summary, "updated": result.Updated}), nil
}

func handleGitMerge(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	branch, e := requireString(args, "branch")
	if e != nil {
		return e, nil
	}
	noFF := optBool(args, "no_ff", false)
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	result, err := git.Merge(repoPath, branch, noFF)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"summary":        result.Summary,
		"success":        result.Success,
		"has_conflicts":  result.HasConflicts,
		"conflict_files": result.ConflictFiles,
	}), nil
}

func handleGitCreateBranch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	name, e := requireString(args, "name")
	if e != nil {
		return e, nil
	}
	startPoint := optString(args, "start_point", "")
	checkout := optBool(args, "checkout", false)
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.CreateBranch(repoPath, name, startPoint, checkout); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true, "name": name}), nil
}

func handleGitDeleteBranch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	name, e := requireString(args, "name")
	if e != nil {
		return e, nil
	}
	force := optBool(args, "force", false)
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.DeleteBranch(repoPath, name, force); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true}), nil
}

func handleGitCreateTag(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	tagName, e := requireString(args, "tag_name")
	if e != nil {
		return e, nil
	}
	commitHash := optString(args, "commit_hash", "")
	message := optString(args, "message", "")
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.CreateTag(repoPath, tagName, commitHash, message); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true, "tag": tagName}), nil
}

func handleGitStashCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	message := optString(args, "message", "")
	includeUntracked := optBool(args, "include_untracked", false)
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.CreateStash(repoPath, message, includeUntracked); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true}), nil
}

func handleGitStashPop(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	index := optInt(args, "index", 0)
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	summary, err := git.PopStash(repoPath, index)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true, "summary": summary}), nil
}

func handleGitCherryPick(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	commitHash, e := requireString(args, "commit_hash")
	if e != nil {
		return e, nil
	}
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	result, err := git.CherryPick(repoPath, commitHash)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"success":        result.Success,
		"summary":        result.Summary,
		"has_conflicts":  result.HasConflicts,
		"conflict_files": result.ConflictFiles,
	}), nil
}

func handleGitRevert(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	commitHash, e := requireString(args, "commit_hash")
	if e != nil {
		return e, nil
	}
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	result, err := git.Revert(repoPath, commitHash)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"success":        result.Success,
		"summary":        result.Summary,
		"has_conflicts":  result.HasConflicts,
		"conflict_files": result.ConflictFiles,
	}), nil
}

func handleGitRebase(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	ontoBranch, e := requireString(args, "onto_branch")
	if e != nil {
		return e, nil
	}
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	result, err := git.Rebase(repoPath, ontoBranch)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{
		"success":        result.Success,
		"summary":        result.Summary,
		"has_conflicts":  result.HasConflicts,
		"conflict_files": result.ConflictFiles,
	}), nil
}

func handleGitResolveConflict(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	filePath, e := requireString(args, "file_path")
	if e != nil {
		return e, nil
	}
	strategy, e := requireString(args, "strategy")
	if e != nil {
		return e, nil
	}
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.ResolveConflict(repoPath, filePath, strategy); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true}), nil
}

func handleGitAbortMerge(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	writeLocks.Lock(repoPath)
	defer writeLocks.Unlock(repoPath)
	if err := git.AbortMerge(repoPath); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true}), nil
}

// --- DSL tool ---

func registerDSLTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("jock_dsl_query",
			mcp.WithDescription("Execute a Jock DSL query against a git repository. "+
				"The DSL supports querying commits, branches, files, blame, stashes, tasks, and more. "+
				"Examples: 'commits | where author == \"alice\" | limit 10', "+
				"'commits | where after(\"2025-01-01\") | stats additions by author', "+
				"'branches | where merged', 'blame \"src/main.go\"', "+
				"'stashes', 'tasks | where status == \"in-progress\"'"),
			repoPathProp(),
			mcp.WithString("query", mcp.Required(), mcp.Description("DSL query string")),
			mcp.WithBoolean("dry_run", mcp.Description("If true, preview destructive actions without executing")),
		),
		handleDSLQuery,
	)
}

func handleDSLQuery(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	query, e := requireString(args, "query")
	if e != nil {
		return e, nil
	}
	dryRun := optBool(args, "dry_run", false)

	evalCtx := &dsl.EvalContext{
		RepoPath: repoPath,
		DryRun:   dryRun,
		Ctx:      context.Background(),
		Cache:    cacheManager,
	}

	result, err := dsl.RunQuery(evalCtx, query)
	if err != nil {
		return errResult(err.Error()), nil
	}

	return dslResultToMCP(result), nil
}

func dslResultToMCP(r *dsl.Result) *mcp.CallToolResult {
	switch r.Kind {
	case "formatted":
		return textResult(r.FormattedOutput)
	case "commits":
		return jsonResult(r.Commits)
	case "branches":
		return jsonResult(r.Branches)
	case "files":
		return jsonResult(r.Files)
	case "blame":
		var sb strings.Builder
		for _, l := range r.BlameLines {
			fmt.Fprintf(&sb, "%4d  %s  %-12s  %s  %s\n", l.LineNo, l.Hash, l.Author, l.Date, l.Content)
		}
		return textResult(sb.String())
	case "stashes":
		return jsonResult(r.Stashes)
	case "count":
		return jsonResult(map[string]int{"count": r.Count})
	case "aggregate":
		return jsonResult(map[string]any{
			"function":   r.AggFunc,
			"field":      r.AggField,
			"group_by":   r.GroupField,
			"aggregates": r.Aggregates,
		})
	case "action_report":
		return jsonResult(r.Report)
	case "status":
		return jsonResult(r.StatusEntries)
	case "remotes":
		return jsonResult(r.Remotes)
	case "reflog":
		return jsonResult(r.ReflogEntries)
	case "tasks":
		return jsonResult(r.Tasks)
	default:
		return jsonResult(r)
	}
}

// --- Task tools ---

func registerTaskTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("jock_task_list",
			mcp.WithDescription("List tasks in the repository, optionally filtered by status"),
			repoPathProp(),
			mcp.WithString("status_filter", mcp.Description("Filter by status: 'backlog', 'in-progress', 'done'")),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithDestructiveHintAnnotation(false),
		),
		handleTaskList,
	)

	s.AddTool(
		mcp.NewTool("jock_task_create",
			mcp.WithDescription("Create a new task"),
			repoPathProp(),
			mcp.WithString("title", mcp.Required(), mcp.Description("Task title")),
			mcp.WithString("description", mcp.Description("Task description")),
			mcp.WithArray("labels", mcp.Description("Task labels")),
			mcp.WithNumber("priority", mcp.Description("Priority: 0=none, 1=low, 2=medium, 3=high")),
		),
		handleTaskCreate,
	)

	s.AddTool(
		mcp.NewTool("jock_task_update",
			mcp.WithDescription("Update an existing task"),
			repoPathProp(),
			mcp.WithString("id", mcp.Required(), mcp.Description("Task ID")),
			mcp.WithString("title", mcp.Description("New title")),
			mcp.WithString("description", mcp.Description("New description")),
			mcp.WithString("status", mcp.Description("New status: 'backlog', 'in-progress', 'done'")),
			mcp.WithArray("labels", mcp.Description("New labels")),
			mcp.WithString("branch", mcp.Description("Linked branch")),
			mcp.WithNumber("priority", mcp.Description("Priority: 0=none, 1=low, 2=medium, 3=high")),
		),
		handleTaskUpdate,
	)

	s.AddTool(
		mcp.NewTool("jock_task_delete",
			mcp.WithDescription("Delete a task"),
			repoPathProp(),
			mcp.WithString("id", mcp.Required(), mcp.Description("Task ID to delete")),
			mcp.WithDestructiveHintAnnotation(true),
		),
		handleTaskDelete,
	)

	s.AddTool(
		mcp.NewTool("jock_task_start",
			mcp.WithDescription("Start working on a task: sets status to in-progress and optionally creates a feature branch"),
			repoPathProp(),
			mcp.WithString("id", mcp.Required(), mcp.Description("Task ID to start")),
			mcp.WithBoolean("create_branch", mcp.Description("Create and checkout a feature branch for this task")),
		),
		handleTaskStart,
	)
}

func handleTaskList(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	statusFilter := optString(args, "status_filter", "")
	taskList, err := tasks.List(repoPath, statusFilter)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(taskList), nil
}

func handleTaskCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	title, e := requireString(args, "title")
	if e != nil {
		return e, nil
	}
	description := optString(args, "description", "")
	labels := optStringSlice(args, "labels")
	priority := optInt(args, "priority", 0)
	t, err := tasks.Create(repoPath, title, description, labels, priority)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(t), nil
}

func handleTaskUpdate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	id, e := requireString(args, "id")
	if e != nil {
		return e, nil
	}
	title := optString(args, "title", "")
	description := optString(args, "description", "")
	status := optString(args, "status", "")
	labels := optStringSlice(args, "labels")
	branch := optString(args, "branch", "")
	priority := optInt(args, "priority", -1)
	t, err := tasks.Update(repoPath, id, title, description, status, labels, branch, priority)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(t), nil
}

func handleTaskDelete(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	id, e := requireString(args, "id")
	if e != nil {
		return e, nil
	}
	if err := tasks.Delete(repoPath, id); err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"success": true}), nil
}

func handleTaskStart(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoPath, e := getRepoPath(args)
	if e != nil {
		return e, nil
	}
	id, e := requireString(args, "id")
	if e != nil {
		return e, nil
	}
	createBranch := optBool(args, "create_branch", false)

	t, err := tasks.Get(repoPath, id)
	if err != nil {
		return errResult(err.Error()), nil
	}

	branchName := ""
	if createBranch {
		branchName = fmt.Sprintf("task/%s-%s", t.ID, slugify(t.Title))
		writeLocks.Lock(repoPath)
		if err := git.CreateBranch(repoPath, branchName, "", true); err != nil {
			writeLocks.Unlock(repoPath)
			return errResult(err.Error()), nil
		}
		writeLocks.Unlock(repoPath)
	}

	t, err = tasks.Update(repoPath, id, "", "", "in-progress", nil, branchName, -1)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return jsonResult(map[string]any{"task": t, "branch": branchName}), nil
}

func slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > 30 {
		s = s[:30]
		s = strings.TrimRight(s, "-")
	}
	return s
}
