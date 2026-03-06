package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// gitCmd creates an exec.Cmd for git with process group isolation
// so that signals to the parent don't propagate to the git subprocess.
func gitCmd(args ...string) *exec.Cmd {
	bin := "git"
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

// RawCommit is the parsed output from git log before graph computation.
type RawCommit struct {
	Hash    string
	Message string
	Author  string
	Date    string           // relative date for display ("3 days ago")
	DateISO string           // ISO 8601 date for comparisons ("2025-01-15T14:30:00+00:00")
	Parents []string
	Refs    string
	Files   []FileChangeInfo // populated by ListRawCommitsWithStats only
}

// BranchInfo holds parsed branch data.
type BranchInfo struct {
	Name      string
	IsCurrent bool
	Remote    string
	Ahead     int
	Behind    int
}

// FileChangeInfo holds parsed diff stat data.
type FileChangeInfo struct {
	Path      string
	Status    string // "added", "modified", "deleted", "renamed"
	Additions int
	Deletions int
	Patch     string
}

const sep = "\x1f" // unit separator to avoid conflicts with commit messages

// CommitFilter holds optional filter parameters for listing commits.
type CommitFilter struct {
	AuthorPattern string
	GrepPattern   string
	AfterDate     string
	BeforeDate    string
	PathPattern   string
}

func appendFilterArgs(args []string, filter *CommitFilter) []string {
	if filter == nil {
		return args
	}
	if filter.AuthorPattern != "" {
		args = append(args, "--author="+filter.AuthorPattern)
	}
	if filter.GrepPattern != "" {
		args = append(args, "--grep="+filter.GrepPattern, "-i")
	}
	if filter.AfterDate != "" {
		args = append(args, "--after="+filter.AfterDate)
	}
	if filter.BeforeDate != "" {
		args = append(args, "--before="+filter.BeforeDate)
	}
	if filter.PathPattern != "" {
		args = append(args, "--", filter.PathPattern)
	}
	return args
}

// ListRawCommits runs git log and parses structured output.
func ListRawCommits(repoPath string, limit, skip int, branch string, filter *CommitFilter) ([]RawCommit, error) {
	format := strings.Join([]string{"%H", "%P", "%an", "%aI", "%cr", "%D", "%s"}, sep)
	args := []string{"-C", repoPath, "log", "--topo-order", "--format=" + format}

	if branch != "" {
		args = append(args, branch)
	} else {
		args = append(args, "--all")
	}

	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	if skip > 0 {
		args = append(args, fmt.Sprintf("--skip=%d", skip))
	}

	args = appendFilterArgs(args, filter)

	cmd := gitCmd(args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var commits []RawCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 7)
		if len(parts) < 7 {
			continue
		}
		var parents []string
		if parts[1] != "" {
			parents = strings.Split(parts[1], " ")
		}
		commits = append(commits, RawCommit{
			Hash:    parts[0],
			Parents: parents,
			Author:  parts[2],
			DateISO: parts[3],
			Date:    parts[4],
			Refs:    parts[5],
			Message: parts[6],
		})
	}
	return commits, nil
}

// ListRawCommitsWithStats runs git log with --numstat to include per-commit diffstat.
// This is more expensive than ListRawCommits but returns additions/deletions/file paths.
func ListRawCommitsWithStats(repoPath string, limit, skip int, branch string, filter *CommitFilter) ([]RawCommit, error) {
	format := strings.Join([]string{"%H", "%P", "%an", "%aI", "%cr", "%D", "%s"}, sep)
	args := []string{"-C", repoPath, "log", "--topo-order", "--format=" + format, "--numstat"}

	if branch != "" {
		args = append(args, branch)
	} else {
		args = append(args, "--all")
	}

	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	if skip > 0 {
		args = append(args, fmt.Sprintf("--skip=%d", skip))
	}

	args = appendFilterArgs(args, filter)

	cmd := gitCmd(args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var commits []RawCommit
	lines := strings.Split(string(out), "\n")

	for i := 0; i < len(lines); {
		line := lines[i]
		if line == "" {
			i++
			continue
		}

		parts := strings.SplitN(line, sep, 7)
		if len(parts) < 7 {
			i++
			continue
		}

		var parents []string
		if parts[1] != "" {
			parents = strings.Split(parts[1], " ")
		}

		rc := RawCommit{
			Hash:    parts[0],
			Parents: parents,
			Author:  parts[2],
			DateISO: parts[3],
			Date:    parts[4],
			Refs:    parts[5],
			Message: parts[6],
		}

		// Parse numstat lines following the format line
		i++
		for i < len(lines) {
			numLine := lines[i]
			if numLine == "" {
				i++
				continue
			}
			// Check if this is a new commit line (contains our separator)
			if strings.Contains(numLine, sep) {
				break
			}
			// Parse numstat: "additions\tdeletions\tfilepath"
			numParts := strings.SplitN(numLine, "\t", 3)
			if len(numParts) == 3 {
				add, _ := strconv.Atoi(numParts[0])
				del, _ := strconv.Atoi(numParts[1])
				rc.Files = append(rc.Files, FileChangeInfo{
					Path:      numParts[2],
					Additions: add,
					Deletions: del,
				})
			}
			i++
		}

		commits = append(commits, rc)
	}
	return commits, nil
}

// ListBranches returns local branches with current indicator, remote tracking info, and ahead/behind counts.
func ListBranches(repoPath string) ([]BranchInfo, error) {
	cmd := gitCmd("-C", repoPath, "branch", "--format",
		"%(refname:short)"+sep+"%(HEAD)"+sep+"%(upstream:short)"+sep+"%(upstream:track)")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}

	var branches []BranchInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 4)
		if len(parts) < 3 {
			continue
		}
		var ahead, behind int
		if len(parts) >= 4 {
			ahead, behind = parseTrackCounts(parts[3])
		}
		branches = append(branches, BranchInfo{
			Name:      parts[0],
			IsCurrent: strings.TrimSpace(parts[1]) == "*",
			Remote:    strings.TrimSpace(parts[2]),
			Ahead:     ahead,
			Behind:    behind,
		})
	}
	return branches, nil
}

// GetCommitDiffStat returns file changes with stats for a given commit.
func GetCommitDiffStat(repoPath, hash string) ([]FileChangeInfo, error) {
	// Get numstat for additions/deletions
	numstatCmd := gitCmd("-C", repoPath, "diff-tree",
		"--no-commit-id", "-r", "--numstat", hash)
	numstatOut, err := numstatCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree numstat: %w", err)
	}

	// Get diff-filter for status
	statusCmd := gitCmd("-C", repoPath, "diff-tree",
		"--no-commit-id", "-r", "--name-status", hash)
	statusOut, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree name-status: %w", err)
	}

	// Parse status map
	statusMap := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(statusOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		status := parts[0]
		path := parts[1]
		switch {
		case strings.HasPrefix(status, "A"):
			statusMap[path] = "added"
		case strings.HasPrefix(status, "D"):
			statusMap[path] = "deleted"
		case strings.HasPrefix(status, "R"):
			statusMap[path] = "renamed"
		default:
			statusMap[path] = "modified"
		}
	}

	// Parse numstat and combine
	var changes []FileChangeInfo
	for _, line := range strings.Split(strings.TrimSpace(string(numstatOut)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		add, _ := strconv.Atoi(parts[0])
		del, _ := strconv.Atoi(parts[1])
		path := parts[2]

		status := statusMap[path]
		if status == "" {
			status = "modified"
		}

		changes = append(changes, FileChangeInfo{
			Path:      path,
			Status:    status,
			Additions: add,
			Deletions: del,
		})
	}
	return changes, nil
}

// GetFilePatch returns the unified diff for a specific file in a commit.
func GetFilePatch(repoPath, hash, filePath string) (string, error) {
	cmd := gitCmd("-C", repoPath, "diff-tree", "-p",
		"--no-commit-id", hash, "--", filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff-tree patch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GetCommitPatch returns the full patch for a commit (all files).
func GetCommitPatch(repoPath, hash string) (string, error) {
	cmd := gitCmd("-C", repoPath, "diff-tree", "-p",
		"--no-commit-id", "-r", hash)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff-tree patch: %w", err)
	}
	return string(out), nil
}

// GetStatus returns working directory status using porcelain v2.
func GetStatus(repoPath string) (staged, unstaged []FileChangeInfo, untracked []string, unmerged []FileChangeInfo, err error) {
	cmd := gitCmd("-C", repoPath, "status", "--porcelain=v2")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("git status: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "? ") {
			// Untracked file
			untracked = append(untracked, strings.TrimPrefix(line, "? "))
			continue
		}

		if strings.HasPrefix(line, "u ") {
			// Unmerged entry: "u XY sub m1 m2 m3 h1 h2 h3 path"
			fields := strings.Fields(line)
			if len(fields) < 11 {
				continue
			}
			path := fields[10]
			unmerged = append(unmerged, FileChangeInfo{
				Path:   path,
				Status: "conflicted",
			})
			continue
		}

		if strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") {
			// Changed entry: "1 XY sub mH mI mW hH hI path" or rename "2 XY sub mH mI mW hH hI X### path\torigPath"
			fields := strings.Fields(line)
			if len(fields) < 9 {
				continue
			}
			xy := fields[1]
			path := fields[8]

			// X = staged status, Y = unstaged status
			if len(xy) >= 1 && xy[0] != '.' {
				staged = append(staged, FileChangeInfo{
					Path:   path,
					Status: porcelainStatus(xy[0]),
				})
			}
			if len(xy) >= 2 && xy[1] != '.' {
				unstaged = append(unstaged, FileChangeInfo{
					Path:   path,
					Status: porcelainStatus(xy[1]),
				})
			}
		}
	}
	return staged, unstaged, untracked, unmerged, nil
}

// parseTrackCounts extracts ahead/behind counts from git's %(upstream:track) format.
// Format is "[ahead N]", "[behind N]", "[ahead N, behind M]", or "" if in sync / no upstream.
func parseTrackCounts(track string) (ahead, behind int) {
	track = strings.TrimSpace(track)
	if track == "" || track == "[gone]" {
		return 0, 0
	}
	// Strip brackets
	track = strings.TrimPrefix(track, "[")
	track = strings.TrimSuffix(track, "]")
	for _, part := range strings.Split(track, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "ahead ") {
			ahead, _ = strconv.Atoi(strings.TrimPrefix(part, "ahead "))
		} else if strings.HasPrefix(part, "behind ") {
			behind, _ = strconv.Atoi(strings.TrimPrefix(part, "behind "))
		}
	}
	return ahead, behind
}

func porcelainStatus(c byte) string {
	switch c {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	default:
		return "modified"
	}
}

// RemoteInfo holds parsed remote data.
type RemoteInfo struct {
	Name string
	URL  string
}

// ListRemotes returns configured remotes.
func ListRemotes(repoPath string) ([]RemoteInfo, error) {
	cmd := gitCmd("-C", repoPath, "remote", "-v")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git remote: %w", err)
	}

	seen := make(map[string]bool)
	var remotes []RemoteInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		if seen[name] {
			continue
		}
		seen[name] = true
		remotes = append(remotes, RemoteInfo{Name: name, URL: parts[1]})
	}
	return remotes, nil
}

// ListRemoteBranches returns branches on a given remote.
func ListRemoteBranches(repoPath, remote string) ([]string, error) {
	cmd := gitCmd("-C", repoPath, "ls-remote", "--heads", remote)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote: %w", err)
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ref := parts[1]
		ref = strings.TrimPrefix(ref, "refs/heads/")
		branches = append(branches, ref)
	}
	return branches, nil
}

// PullResult holds the outcome of a git pull.
type PullResult struct {
	Summary string
	Updated int
}

// Pull runs git pull with optional remote and branch.
func Pull(repoPath, remote, branch string) (*PullResult, error) {
	args := []string{"-C", repoPath, "pull"}
	if remote != "" {
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
	}

	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return nil, fmt.Errorf("git pull: %s", output)
	}

	result := &PullResult{Summary: output}
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "file") && strings.Contains(line, "changed") {
			fmt.Sscanf(line, " %d file", &result.Updated)
			break
		}
	}

	return result, nil
}

// ParseRefs splits a git ref decoration string into branches and tags.
func ParseRefs(refs string) (branches []string, tags []string) {
	if refs == "" {
		return nil, nil
	}
	for _, ref := range strings.Split(refs, ", ") {
		ref = strings.TrimSpace(ref)
		ref = strings.TrimPrefix(ref, "HEAD -> ")
		if strings.HasPrefix(ref, "tag: ") {
			tags = append(tags, strings.TrimPrefix(ref, "tag: "))
		} else if ref != "" {
			branches = append(branches, ref)
		}
	}
	return
}

// --- Stage / Unstage / Commit ---

// StageFiles stages the given file paths. If paths is empty, stages everything.
func StageFiles(repoPath string, paths []string) error {
	args := []string{"-C", repoPath, "add"}
	if len(paths) == 0 {
		args = append(args, "-A")
	} else {
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// UnstageFiles unstages the given file paths. If paths is empty, unstages everything.
func UnstageFiles(repoPath string, paths []string) error {
	// Check if HEAD exists (fails on initial commit)
	checkCmd := gitCmd("-C", repoPath, "rev-parse", "--verify", "HEAD")
	if err := checkCmd.Run(); err != nil {
		// No HEAD yet — use git rm --cached
		args := []string{"-C", repoPath, "rm", "--cached", "-r"}
		if len(paths) == 0 {
			args = append(args, ".")
		} else {
			args = append(args, "--")
			args = append(args, paths...)
		}
		cmd := gitCmd(args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git rm --cached: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}

	args := []string{"-C", repoPath, "restore", "--staged"}
	if len(paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, "--")
		args = append(args, paths...)
	}
	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git restore --staged: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CommitResult holds the outcome of a git commit.
type CommitResult struct {
	Hash string
}

// CreateCommit creates a new commit with the given message.
func CreateCommit(repoPath, message string, amend ...bool) (*CommitResult, error) {
	args := []string{"-C", repoPath, "commit", "-m", message}
	if len(amend) > 0 && amend[0] {
		args = append(args, "--amend")
	}
	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git commit: %s", strings.TrimSpace(string(out)))
	}

	hashCmd := gitCmd("-C", repoPath, "rev-parse", "--short", "HEAD")
	hashOut, err := hashCmd.Output()
	if err != nil {
		return &CommitResult{Hash: ""}, nil
	}
	return &CommitResult{Hash: strings.TrimSpace(string(hashOut))}, nil
}

// GetWorkingDiff returns the diff for a file in the working directory.
// If staged is true, shows the staged diff (--cached). Otherwise shows unstaged changes.
func GetWorkingDiff(repoPath, filePath string, staged bool) (string, error) {
	args := []string{"-C", repoPath, "diff"}
	if staged {
		args = append(args, "--cached")
	}
	if filePath != "" {
		args = append(args, "--", filePath)
	}
	cmd := gitCmd(args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return strings.TrimSpace(string(out)), nil
		}
		return "", fmt.Errorf("git diff: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// --- Push ---

// PushResult holds the outcome of a git push.
type PushResult struct {
	Summary string
	Success bool
}

// Push runs git push with optional remote, branch, force, and set-upstream flags.
func Push(repoPath, remote, branch string, force, setUpstream bool) (*PushResult, error) {
	args := []string{"-C", repoPath, "push"}
	if setUpstream {
		args = append(args, "-u")
	}
	if force {
		args = append(args, "--force-with-lease")
	}
	if remote != "" {
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
	}

	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return nil, fmt.Errorf("git push: %s", output)
	}
	return &PushResult{Summary: output, Success: true}, nil
}

// --- Merge ---

// MergeResult holds the outcome of a git merge.
type MergeResult struct {
	Summary       string
	Success       bool
	HasConflicts  bool
	ConflictFiles []string
}

// Merge merges the given branch into the current branch.
func Merge(repoPath, branch string, noFF bool) (*MergeResult, error) {
	args := []string{"-C", repoPath, "merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)

	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "Automatic merge failed") {
			conflicts := parseMergeConflicts(repoPath)
			return &MergeResult{
				Summary:       output,
				Success:       false,
				HasConflicts:  true,
				ConflictFiles: conflicts,
			}, nil
		}
		return nil, fmt.Errorf("git merge: %s", output)
	}
	return &MergeResult{Summary: output, Success: true}, nil
}

func parseMergeConflicts(repoPath string) []string {
	cmd := gitCmd("-C", repoPath, "diff", "--name-only", "--diff-filter=U")
	out, _ := cmd.Output()
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// --- Branch Management ---

// CreateBranch creates a new branch. If checkout is true, uses checkout -b.
func CreateBranch(repoPath, name, startPoint string, checkout bool) error {
	var args []string
	if checkout {
		args = []string{"-C", repoPath, "checkout", "-b", name}
	} else {
		args = []string{"-C", repoPath, "branch", name}
	}
	if startPoint != "" {
		args = append(args, startPoint)
	}

	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// BlameLine represents a single line in git blame output.
type BlameLine struct {
	Hash    string
	Author  string
	Date    string
	LineNo  int
	Content string
}

// Blame runs git blame --porcelain on the given file and returns per-line blame data.
func Blame(repoPath, filePath string) ([]BlameLine, error) {
	cmd := gitCmd("-C", repoPath, "blame", "--porcelain", "--", filePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git blame: %s", strings.TrimSpace(string(out)))
	}

	lines := strings.Split(string(out), "\n")
	// Parse porcelain format: commit info blocks followed by "\t<content>"
	commitAuthors := make(map[string]string)
	commitDates := make(map[string]string)
	var result []BlameLine
	var currentHash string
	var currentLineNo int

	for _, line := range lines {
		if line == "" {
			continue
		}
		if line[0] == '\t' {
			// Content line — ends a block
			result = append(result, BlameLine{
				Hash:    currentHash,
				Author:  commitAuthors[currentHash],
				Date:    commitDates[currentHash],
				LineNo:  currentLineNo,
				Content: line[1:], // strip leading tab
			})
			continue
		}
		// Check for commit header line: 40-char hex hash + orig_line + final_line [+ num_lines]
		if len(line) >= 40 && isHexString(line[:40]) && (len(line) == 40 || line[40] == ' ') {
			currentHash = line[:40]
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				currentLineNo, _ = strconv.Atoi(parts[2])
			}
			continue
		}
		// Metadata lines
		if strings.HasPrefix(line, "author ") {
			commitAuthors[currentHash] = strings.TrimPrefix(line, "author ")
		} else if strings.HasPrefix(line, "author-time ") {
			ts := strings.TrimPrefix(line, "author-time ")
			epoch, _ := strconv.ParseInt(ts, 10, 64)
			if epoch > 0 {
				commitDates[currentHash] = formatEpoch(epoch)
			}
		}
	}
	return result, nil
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func formatEpoch(epoch int64) string {
	t := time.Unix(epoch, 0)
	return t.Format("2006-01-02 15:04")
}

// DeleteBranch deletes the named branch. If force is true, uses -D instead of -d.
func DeleteBranch(repoPath, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	cmd := gitCmd("-C", repoPath, "branch", flag, name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch delete: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// --- Conflict Resolution ---

// ConflictDetail holds the ours/theirs/raw content for a conflicted file.
type ConflictDetail struct {
	Path          string
	OursContent   string
	TheirsContent string
	RawContent    string
}

// GetConflictDetails returns the ours, theirs, and raw (with markers) content for a conflicted file.
func GetConflictDetails(repoPath, filePath string) (*ConflictDetail, error) {
	// Read ours (stage 2)
	oursCmd := gitCmd("-C", repoPath, "show", ":2:"+filePath)
	oursOut, err := oursCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show :2:%s: %w", filePath, err)
	}

	// Read theirs (stage 3)
	theirsCmd := gitCmd("-C", repoPath, "show", ":3:"+filePath)
	theirsOut, err := theirsCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show :3:%s: %w", filePath, err)
	}

	// Read raw working tree file (with conflict markers)
	rawPath := filepath.Join(repoPath, filePath)
	rawContent, err := os.ReadFile(rawPath)
	if err != nil {
		return nil, fmt.Errorf("read conflict file: %w", err)
	}

	return &ConflictDetail{
		Path:          filePath,
		OursContent:   string(oursOut),
		TheirsContent: string(theirsOut),
		RawContent:    string(rawContent),
	}, nil
}

// ResolveConflict resolves a merge conflict for a single file using the given strategy.
func ResolveConflict(repoPath, filePath, strategy string) error {
	switch strategy {
	case "ours":
		cmd := gitCmd("-C", repoPath, "checkout", "--ours", "--", filePath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git checkout --ours: %s", strings.TrimSpace(string(out)))
		}
	case "theirs":
		cmd := gitCmd("-C", repoPath, "checkout", "--theirs", "--", filePath)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git checkout --theirs: %s", strings.TrimSpace(string(out)))
		}
	case "both":
		// Read ours and theirs, concatenate, write to file
		oursCmd := gitCmd("-C", repoPath, "show", ":2:"+filePath)
		oursOut, err := oursCmd.Output()
		if err != nil {
			return fmt.Errorf("git show :2:%s: %w", filePath, err)
		}
		theirsCmd := gitCmd("-C", repoPath, "show", ":3:"+filePath)
		theirsOut, err := theirsCmd.Output()
		if err != nil {
			return fmt.Errorf("git show :3:%s: %w", filePath, err)
		}
		combined := string(oursOut) + "\n" + string(theirsOut)
		fullPath := filepath.Join(repoPath, filePath)
		if err := os.WriteFile(fullPath, []byte(combined), 0644); err != nil {
			return fmt.Errorf("write combined file: %w", err)
		}
	default:
		return fmt.Errorf("unknown resolve strategy: %s", strategy)
	}

	// Stage the resolved file
	addCmd := gitCmd("-C", repoPath, "add", "--", filePath)
	out, err := addCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// AbortMerge aborts the current merge.
func AbortMerge(repoPath string) error {
	cmd := gitCmd("-C", repoPath, "merge", "--abort")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git merge --abort: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// IsMerging returns true if a merge is in progress (MERGE_HEAD exists).
func IsMerging(repoPath string) bool {
	mergeHead := filepath.Join(repoPath, ".git", "MERGE_HEAD")
	_, err := os.Stat(mergeHead)
	return err == nil
}

// --- Cherry-Pick / Revert ---

// CherryPick cherry-picks the given commit onto the current branch.
func CherryPick(repoPath, commitHash string) (*MergeResult, error) {
	cmd := gitCmd("-C", repoPath, "cherry-pick", commitHash)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "could not apply") {
			conflicts := parseMergeConflicts(repoPath)
			return &MergeResult{
				Summary:       output,
				Success:       false,
				HasConflicts:  true,
				ConflictFiles: conflicts,
			}, nil
		}
		return nil, fmt.Errorf("git cherry-pick: %s", output)
	}
	return &MergeResult{Summary: output, Success: true}, nil
}

// AbortCherryPick aborts an in-progress cherry-pick.
func AbortCherryPick(repoPath string) error {
	cmd := gitCmd("-C", repoPath, "cherry-pick", "--abort")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git cherry-pick --abort: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Revert reverts the given commit, creating a new commit.
func Revert(repoPath, commitHash string) (*MergeResult, error) {
	cmd := gitCmd("-C", repoPath, "revert", "--no-edit", commitHash)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "could not revert") {
			conflicts := parseMergeConflicts(repoPath)
			return &MergeResult{
				Summary:       output,
				Success:       false,
				HasConflicts:  true,
				ConflictFiles: conflicts,
			}, nil
		}
		return nil, fmt.Errorf("git revert: %s", output)
	}
	return &MergeResult{Summary: output, Success: true}, nil
}

// AbortRevert aborts an in-progress revert.
func AbortRevert(repoPath string) error {
	cmd := gitCmd("-C", repoPath, "revert", "--abort")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git revert --abort: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// --- Rebase ---

// Rebase rebases the current branch onto the given branch.
func Rebase(repoPath, ontoBranch string) (*MergeResult, error) {
	cmd := gitCmd("-C", repoPath, "rebase", ontoBranch)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "could not apply") {
			conflicts := parseMergeConflicts(repoPath)
			return &MergeResult{
				Summary:       output,
				Success:       false,
				HasConflicts:  true,
				ConflictFiles: conflicts,
			}, nil
		}
		return nil, fmt.Errorf("git rebase: %s", output)
	}
	return &MergeResult{Summary: output, Success: true}, nil
}

// AbortRebase aborts the current rebase.
func AbortRebase(repoPath string) error {
	cmd := gitCmd("-C", repoPath, "rebase", "--abort")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git rebase --abort: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ContinueRebase continues the current rebase after conflict resolution.
func ContinueRebase(repoPath string) (*MergeResult, error) {
	cmd := gitCmd("-C", repoPath, "rebase", "--continue")
	cmd.Env = append(os.Environ(), "GIT_EDITOR=true")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "could not apply") {
			conflicts := parseMergeConflicts(repoPath)
			return &MergeResult{
				Summary:       output,
				Success:       false,
				HasConflicts:  true,
				ConflictFiles: conflicts,
			}, nil
		}
		return nil, fmt.Errorf("git rebase --continue: %s", output)
	}
	return &MergeResult{Summary: output, Success: true}, nil
}

// IsRebasing returns true if a rebase is in progress.
func IsRebasing(repoPath string) bool {
	rebaseMerge := filepath.Join(repoPath, ".git", "rebase-merge")
	rebaseApply := filepath.Join(repoPath, ".git", "rebase-apply")
	_, err1 := os.Stat(rebaseMerge)
	_, err2 := os.Stat(rebaseApply)
	return err1 == nil || err2 == nil
}

// --- Interactive Rebase ---

// RebaseTodoEntry represents a single entry in a git rebase todo list.
type RebaseTodoEntry struct {
	Action  string
	Hash    string
	Message string
}

// InteractiveRebase performs an interactive rebase using a pre-built todo list.
func InteractiveRebase(repoPath, baseCommit string, entries []RebaseTodoEntry) (*MergeResult, error) {
	// Write the todo entries to a temp file
	tmpFile, err := os.CreateTemp("", "jock-rebase-todo-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	var todoContent strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(&todoContent, "%s %s %s\n", entry.Action, entry.Hash, entry.Message)
	}
	if _, err := tmpFile.WriteString(todoContent.String()); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("write todo file: %w", err)
	}
	tmpFile.Close()

	cmd := gitCmd("-C", repoPath, "rebase", "-i", baseCommit)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SEQUENCE_EDITOR=cp %s", tmpPath))
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		if strings.Contains(output, "CONFLICT") || strings.Contains(output, "could not apply") {
			conflicts := parseMergeConflicts(repoPath)
			return &MergeResult{
				Summary:       output,
				Success:       false,
				HasConflicts:  true,
				ConflictFiles: conflicts,
			}, nil
		}
		return nil, fmt.Errorf("git rebase -i: %s", output)
	}
	return &MergeResult{Summary: output, Success: true}, nil
}

// GetRebaseTodo reads the current rebase todo list from an in-progress rebase.
func GetRebaseTodo(repoPath string) ([]RebaseTodoEntry, error) {
	todoPath := filepath.Join(repoPath, ".git", "rebase-merge", "git-rebase-todo")
	data, err := os.ReadFile(todoPath)
	if err != nil {
		return nil, fmt.Errorf("read rebase todo: %w", err)
	}

	var entries []RebaseTodoEntry
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}
		entry := RebaseTodoEntry{
			Action: parts[0],
			Hash:   parts[1],
		}
		if len(parts) >= 3 {
			entry.Message = parts[2]
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// --- Reflog ---

// ReflogEntry represents a single entry in the git reflog.
type ReflogEntry struct {
	Hash    string
	Action  string
	Message string
	Date    string
}

// ListReflog returns recent reflog entries.
func ListReflog(repoPath string, limit int) ([]ReflogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	format := strings.Join([]string{"%H", "%gs", "%cr"}, sep)
	cmd := gitCmd("-C", repoPath, "reflog", "--format="+format, fmt.Sprintf("-n%d", limit))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git reflog: %w", err)
	}

	var entries []ReflogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 3)
		if len(parts) < 3 {
			continue
		}
		// Parse the action type from the reflog subject (e.g., "commit: message" → "commit")
		action := parts[1]
		message := parts[1]
		if idx := strings.Index(parts[1], ": "); idx >= 0 {
			action = parts[1][:idx]
			message = parts[1][idx+2:]
		}
		entries = append(entries, ReflogEntry{
			Hash:    parts[0],
			Action:  action,
			Message: message,
			Date:    parts[2],
		})
	}
	return entries, nil
}

// --- Tag Management ---

// TagInfo holds parsed tag data.
type TagInfo struct {
	Name        string
	Hash        string
	Date        string
	Message     string
	IsAnnotated bool
}

// ListTags returns all tags in the repository.
func ListTags(repoPath string) ([]TagInfo, error) {
	cmd := gitCmd("-C", repoPath, "tag", "-l",
		"--format=%(refname:short)"+sep+"%(objectname:short)"+sep+"%(creatordate:relative)"+sep+"%(contents:subject)"+sep+"%(objecttype)")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git tag list: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	var tags []TagInfo
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 5)
		if len(parts) < 5 {
			continue
		}
		tags = append(tags, TagInfo{
			Name:        parts[0],
			Hash:        parts[1],
			Date:        parts[2],
			Message:     parts[3],
			IsAnnotated: parts[4] == "tag",
		})
	}
	return tags, nil
}

// CreateTag creates a new tag. If message is non-empty, creates an annotated tag.
func CreateTag(repoPath, tagName, commitHash, message string) error {
	var args []string
	if message != "" {
		args = []string{"-C", repoPath, "tag", "-a", tagName, "-m", message}
	} else {
		args = []string{"-C", repoPath, "tag", tagName}
	}
	if commitHash != "" {
		args = append(args, commitHash)
	}

	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git tag: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteTag deletes a local tag.
func DeleteTag(repoPath, tagName string) error {
	cmd := gitCmd("-C", repoPath, "tag", "-d", tagName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git tag -d: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// PushTag pushes a tag to the specified remote.
func PushTag(repoPath, remote, tagName string) error {
	if remote == "" {
		remote = "origin"
	}
	cmd := gitCmd("-C", repoPath, "push", remote, tagName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push tag: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// --- Stash ---

// StashEntry holds parsed stash data.
type StashEntry struct {
	Index   int
	Message string
	Branch  string
	Date    string
}

// ListStashes returns the stash list.
func ListStashes(repoPath string) ([]StashEntry, error) {
	cmd := gitCmd("-C", repoPath, "stash", "list",
		"--format=%gd"+sep+"%gs"+sep+"%cr")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git stash list: %w", err)
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return nil, nil
	}

	var stashes []StashEntry
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, sep, 3)
		if len(parts) < 3 {
			continue
		}

		// Parse index from "stash@{N}"
		idx := 0
		ref := parts[0]
		if i := strings.Index(ref, "{"); i != -1 {
			if j := strings.Index(ref, "}"); j > i {
				idx, _ = strconv.Atoi(ref[i+1 : j])
			}
		}

		// Extract branch from subject (e.g. "WIP on main: abc1234 msg" or "On main: abc1234 msg")
		subject := parts[1]
		branch := ""
		if strings.HasPrefix(subject, "WIP on ") || strings.HasPrefix(subject, "On ") {
			rest := subject
			if strings.HasPrefix(rest, "WIP on ") {
				rest = rest[7:]
			} else {
				rest = rest[3:]
			}
			if colonIdx := strings.Index(rest, ":"); colonIdx != -1 {
				branch = rest[:colonIdx]
			}
		}

		stashes = append(stashes, StashEntry{
			Index:   idx,
			Message: subject,
			Branch:  branch,
			Date:    strings.TrimSpace(parts[2]),
		})
	}
	return stashes, nil
}

// CreateStash creates a new stash with an optional message.
func CreateStash(repoPath, message string, includeUntracked bool) error {
	args := []string{"-C", repoPath, "stash", "push"}
	if message != "" {
		args = append(args, "-m", message)
	}
	if includeUntracked {
		args = append(args, "-u")
	}
	cmd := gitCmd(args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash push: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ApplyStash applies a stash by index without removing it.
func ApplyStash(repoPath string, index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	cmd := gitCmd("-C", repoPath, "stash", "apply", ref)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("git stash apply: %s", output)
	}
	return output, nil
}

// PopStash pops a stash by index (applies and removes it).
func PopStash(repoPath string, index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	cmd := gitCmd("-C", repoPath, "stash", "pop", ref)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("git stash pop: %s", output)
	}
	return output, nil
}

// DropStash removes a stash by index.
func DropStash(repoPath string, index int) error {
	ref := fmt.Sprintf("stash@{%d}", index)
	cmd := gitCmd("-C", repoPath, "stash", "drop", ref)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git stash drop: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ShowStash returns the unified diff for a stash by index.
func ShowStash(repoPath string, index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	cmd := gitCmd("-C", repoPath, "stash", "show", "-p", ref)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git stash show: %w", err)
	}
	return string(out), nil
}
