package dsl

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupExtendedTestRepo creates a repo with branches, stashes, and status entries for testing.
func setupExtendedTestRepo(t *testing.T) string {
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

	// Initial commit
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	run("add", "main.go")
	run("commit", "-m", "initial commit")

	// Second commit
	os.WriteFile(filepath.Join(dir, "util.go"), []byte("package main\n\nfunc util() {}\n"), 0644)
	run("add", "util.go")
	run("commit", "-m", "add util")

	// Create a feature branch
	run("branch", "feature/test")

	// Create a tag
	run("tag", "v0.1.0")

	// Create an untracked file for status
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("hello\n"), 0644)

	// Modify a file but don't stage it
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n"), 0644)

	return dir
}

func evalExtQuery(t *testing.T, repoPath, query string) *Result {
	t.Helper()
	pipeline, err := Parse(query)
	if err != nil {
		t.Fatalf("parse %q: %v", query, err)
	}
	ctx := &EvalContext{
		RepoPath: repoPath,
		Ctx:      context.Background(),
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("evaluate %q: %v", query, err)
	}
	return result
}

// --- Phase 1-2: New sources ---

func TestEvalStatus(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "status")
	if result.Kind != "status" {
		t.Fatalf("expected kind status, got %s", result.Kind)
	}
	if len(result.StatusEntries) == 0 {
		t.Fatal("expected status entries, got none")
	}

	// Should have at least the modified main.go and untracked file
	foundModified := false
	foundUntracked := false
	for _, s := range result.StatusEntries {
		if s.Path == "main.go" {
			foundModified = true
		}
		if s.Path == "untracked.txt" && s.Status == "untracked" {
			foundUntracked = true
		}
	}
	if !foundModified {
		t.Error("expected modified main.go in status")
	}
	if !foundUntracked {
		t.Error("expected untracked.txt in status")
	}
}

func TestEvalStatusWhereFilter(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, `status | where status == "untracked"`)
	if result.Kind != "status" {
		t.Fatalf("expected kind status, got %s", result.Kind)
	}
	for _, s := range result.StatusEntries {
		if s.Status != "untracked" {
			t.Errorf("expected status untracked, got %q for %s", s.Status, s.Path)
		}
	}
}

func TestEvalStatusCount(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "status | count")
	if result.Kind != "count" {
		t.Fatalf("expected kind count, got %s", result.Kind)
	}
	if result.Count == 0 {
		t.Fatal("expected count > 0")
	}
}

func TestEvalStatusFirstLast(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "status | first 1")
	if result.Kind != "status" {
		t.Fatalf("expected kind status, got %s", result.Kind)
	}
	if len(result.StatusEntries) != 1 {
		t.Fatalf("expected 1 status entry, got %d", len(result.StatusEntries))
	}
}

func TestEvalReflog(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "reflog")
	if result.Kind != "reflog" {
		t.Fatalf("expected kind reflog, got %s", result.Kind)
	}
	if len(result.ReflogEntries) == 0 {
		t.Fatal("expected reflog entries, got none")
	}
}

func TestEvalReflogWhereFilter(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, `reflog | where action == "commit"`)
	if result.Kind != "reflog" {
		t.Fatalf("expected kind reflog, got %s", result.Kind)
	}
	for _, r := range result.ReflogEntries {
		if r.Action != "commit" {
			t.Errorf("expected action commit, got %q", r.Action)
		}
	}
}

func TestEvalReflogCount(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "reflog | count")
	if result.Kind != "count" {
		t.Fatalf("expected kind count, got %s", result.Kind)
	}
	if result.Count == 0 {
		t.Fatal("expected count > 0")
	}
}

func TestEvalBranchesExtended(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "branches")
	if result.Kind != "branches" {
		t.Fatalf("expected kind branches, got %s", result.Kind)
	}
	if len(result.Branches) < 2 {
		t.Fatalf("expected at least 2 branches (main + feature), got %d", len(result.Branches))
	}
}

func TestEvalBranchesWhereFilter(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, `branches | where name contains "feature"`)
	if result.Kind != "branches" {
		t.Fatalf("expected kind branches, got %s", result.Kind)
	}
	if len(result.Branches) == 0 {
		t.Fatal("expected at least 1 feature branch")
	}
	for _, b := range result.Branches {
		if !strings.Contains(b.Name, "feature") {
			t.Errorf("expected branch name containing 'feature', got %q", b.Name)
		}
	}
}

func TestEvalBranchesCount(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "branches | count")
	if result.Kind != "count" {
		t.Fatalf("expected kind count, got %s", result.Kind)
	}
	if result.Count < 2 {
		t.Fatalf("expected count >= 2, got %d", result.Count)
	}
}

func TestEvalBranchesFirstN(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	result := evalExtQuery(t, repo, "branches | first 1")
	if result.Kind != "branches" {
		t.Fatalf("expected kind branches, got %s", result.Kind)
	}
	if len(result.Branches) != 1 {
		t.Fatalf("expected 1 branch, got %d", len(result.Branches))
	}
}

func TestEvalBranchesReverse(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	r1 := evalExtQuery(t, repo, "branches")
	r2 := evalExtQuery(t, repo, "branches | reverse")
	if len(r1.Branches) != len(r2.Branches) {
		t.Fatal("reverse should preserve count")
	}
	if len(r1.Branches) > 1 {
		if r1.Branches[0].Name == r2.Branches[0].Name {
			t.Error("reverse should change order")
		}
	}
}

// --- Phase 1: Standalone commands ---

func TestEvalBranchCreate(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
		DryRun:   true,
	}
	pipeline, err := Parse(`branch create "test-branch"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "DRY RUN") {
		t.Error("expected dry run output")
	}
	if !strings.Contains(result.FormattedOutput, "test-branch") {
		t.Error("expected branch name in output")
	}
}

func TestEvalBranchCreateActual(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
	}
	pipeline, err := Parse(`branch create "new-branch"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "created branch") {
		t.Errorf("expected 'created branch', got %q", result.FormattedOutput)
	}

	// Verify branch exists
	branches := evalExtQuery(t, repo, `branches | where name == "new-branch"`)
	if len(branches.Branches) == 0 {
		t.Error("branch 'new-branch' was not created")
	}
}

func TestEvalMergeDryRun(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
		DryRun:   true,
	}
	pipeline, err := Parse(`merge "feature/test"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "DRY RUN") {
		t.Error("expected dry run output")
	}
}

func TestEvalCommitDryRun(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
		DryRun:   true,
	}
	pipeline, err := Parse(`commit "test message"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "DRY RUN") {
		t.Error("expected dry run output")
	}
}

func TestEvalPushDryRun(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
		DryRun:   true,
	}
	pipeline, err := Parse(`push`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "DRY RUN") {
		t.Error("expected dry run output")
	}
}

func TestEvalPullDryRun(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
		DryRun:   true,
	}
	pipeline, err := Parse(`pull`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "DRY RUN") {
		t.Error("expected dry run output")
	}
}

// --- Phase 1: Piped actions ---

func TestEvalBranchDelete(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
	}
	pipeline, err := Parse(`branches | where name == "feature/test" | delete`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "deleted") {
		t.Errorf("expected 'deleted' in output, got %q", result.FormattedOutput)
	}

	// Verify branch is gone
	branches := evalExtQuery(t, repo, `branches | where name == "feature/test"`)
	if len(branches.Branches) > 0 {
		t.Error("feature/test branch should have been deleted")
	}
}

func TestEvalStashCreateAndApply(t *testing.T) {
	repo := setupExtendedTestRepo(t)

	// Stage main.go first (it was modified)
	cmd := exec.Command("git", "-C", repo, "add", "main.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s", out)
	}

	// Create stash
	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
	}
	pipeline, err := Parse(`stash create "test stash"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "stashed") {
		t.Errorf("expected 'stashed' in output, got %q", result.FormattedOutput)
	}

	// Verify stash exists
	stashes := evalExtQuery(t, repo, "stash")
	if len(stashes.Stashes) == 0 {
		t.Error("expected at least 1 stash entry")
	}
}

// --- Phase 3: Advanced aggregates ---

func TestEvalAdvancedAggregates(t *testing.T) {
	repo := setupTestRepo(t)

	tests := []struct {
		query string
		kind  string
	}{
		{"commits | median additions", "aggregate"},
		{"commits | stddev additions", "aggregate"},
		{"commits | p90 additions", "aggregate"},
		{"commits | p95 additions", "aggregate"},
		{"commits | count_distinct author", "aggregate"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := evalExtQuery(t, repo, tt.query)
			if result.Kind != tt.kind {
				t.Fatalf("expected kind %s, got %s", tt.kind, result.Kind)
			}
			if len(result.Aggregates) == 0 {
				t.Fatal("expected aggregate rows")
			}
		})
	}
}

func TestEvalGroupedAdvancedAggregates(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalExtQuery(t, repo, "commits | group by author | median additions")
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if len(result.Aggregates) < 2 {
		t.Fatalf("expected at least 2 groups (Alice, Bob), got %d", len(result.Aggregates))
	}
}

// --- Phase 4: Format markdown/yaml ---

func TestEvalFormatMarkdown(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalExtQuery(t, repo, "commits | first 2 | format markdown")
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}
	if !strings.Contains(result.FormattedOutput, "|") {
		t.Error("expected markdown table with pipe characters")
	}
	if !strings.Contains(result.FormattedOutput, "Hash") {
		t.Error("expected markdown table header")
	}
}

func TestEvalFormatYAML(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalExtQuery(t, repo, "commits | first 2 | format yaml")
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}
	if !strings.Contains(result.FormattedOutput, "hash:") {
		t.Error("expected YAML output with hash field")
	}
	if !strings.Contains(result.FormattedOutput, "- hash:") {
		t.Error("expected YAML list items")
	}
}

// --- Phase 4: Explain ---

func TestEvalExplain(t *testing.T) {
	pipeline, err := Parse(`explain commits | where author == "Alice" | first 5`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	ctx := &EvalContext{
		RepoPath: "/tmp/fake",
		Ctx:      context.Background(),
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}
	if !strings.Contains(result.FormattedOutput, "QUERY PLAN") {
		t.Error("expected QUERY PLAN in output")
	}
	if !strings.Contains(result.FormattedOutput, "Source: commits") {
		t.Error("expected Source: commits in plan")
	}
	if !strings.Contains(result.FormattedOutput, "Filter:") {
		t.Error("expected Filter in plan")
	}
}

// --- Phase 4: Export ---

func TestEvalExport(t *testing.T) {
	repo := setupTestRepo(t)
	exportPath := filepath.Join(t.TempDir(), "output.json")

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
	}
	pipeline, err := Parse(`commits | first 2 | export "` + exportPath + `"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	result, err := Evaluate(ctx, pipeline)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "exported") {
		t.Error("expected 'exported' in output")
	}

	// Verify file exists
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}
	if len(data) == 0 {
		t.Error("exported file is empty")
	}
}

// --- Phase 5: Window functions ---

func TestEvalWindowRunningSum(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalExtQuery(t, repo, "commits | running_sum additions as total")
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	// Window functions append to message
	for _, c := range result.Commits {
		if !strings.Contains(c.Message, "total=") {
			t.Errorf("expected 'total=' in message, got %q", c.Message)
		}
	}
}

func TestEvalWindowRowNumber(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalExtQuery(t, repo, "commits | row_number as rn")
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	for i, c := range result.Commits {
		expected := "rn=" + strings.TrimLeft(strings.Repeat("0", 0), "0")
		_ = expected
		if !strings.Contains(c.Message, "rn=") {
			t.Errorf("commit %d: expected 'rn=' in message, got %q", i, c.Message)
		}
	}
}

// --- Alias and variable tests ---

func TestRunQueryAliasExtended(t *testing.T) {
	repo := setupTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
		Aliases:  NewAliasStore(),
	}

	// Define alias
	result, err := RunQuery(ctx, `alias recent = commits | first 3`)
	if err != nil {
		t.Fatalf("alias define error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "recent =") {
		t.Errorf("expected alias confirmation, got %q", result.FormattedOutput)
	}

	// Use alias
	result, err = RunQuery(ctx, "recent")
	if err != nil {
		t.Fatalf("alias use error: %v", err)
	}
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	if len(result.Commits) != 3 {
		t.Fatalf("expected 3 commits from alias, got %d", len(result.Commits))
	}
}

func TestRunQueryVariable(t *testing.T) {
	repo := setupTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
	}

	// Define variable
	result, err := RunQuery(ctx, `let team = Alice`)
	if err != nil {
		t.Fatalf("let error: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "$team = Alice") {
		t.Errorf("expected variable confirmation, got %q", result.FormattedOutput)
	}

	// Use variable
	result, err = RunQuery(ctx, `commits | where author == "$team"`)
	if err != nil {
		t.Fatalf("variable use error: %v", err)
	}
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	for _, c := range result.Commits {
		if c.Author != "Alice" {
			t.Errorf("expected author Alice, got %q", c.Author)
		}
	}
}

// --- Parser tests for new syntax ---

func TestParseNewSources(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"status"},
		{"remotes"},
		{"reflog"},
		{"conflicts"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseNewActions(t *testing.T) {
	tests := []struct {
		input string
	}{
		{`branch create "test"`},
		{`branch create "test" from "main"`},
		{`merge "feature"`},
		{`commit "message"`},
		{`commit "message" --amend`},
		{`push`},
		{`push origin main`},
		{`push --force`},
		{`pull`},
		{`pull origin main`},
		{`abort merge`},
		{`abort rebase`},
		{`continue rebase`},
		{`stash create "wip"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParsePipedActions(t *testing.T) {
	tests := []struct {
		input string
	}{
		{`branches | where name contains "feature" | delete`},
		{`branches | where merged == "true" | delete --force`},
		{`stash | first 1 | apply`},
		{`stash | first 1 | pop`},
		{`stash | first 1 | drop`},
		{`status | where status == "modified" | stage`},
		{`status | where staged == "true" | unstage`},
		{`conflicts | resolve ours`},
		{`conflicts | resolve theirs`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseAdvancedAggregates(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"commits | median additions"},
		{"commits | stddev deletions"},
		{"commits | p90 additions"},
		{"commits | p95 additions"},
		{"commits | p99 additions"},
		{"commits | count_distinct author"},
		{"commits | group by author | median additions"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseOutputFeatures(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"commits | format markdown"},
		{"commits | format yaml"},
		{`explain commits | where author == "Alice"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse %q: %v", tt.input, err)
			}
		})
	}
}

func TestParseWindowFunctions(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"commits | running_sum additions as total"},
		{"commits | running_avg deletions as avg_del"},
		{"commits | row_number as rn"},
		{"commits | rank additions as r"},
		{"commits | dense_rank additions as dr"},
		{"commits | lag additions as prev"},
		{"commits | lead additions as next"},
		{"commits | cumulative_sum additions as cum"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("parse %q: %v", tt.input, err)
			}
		})
	}
}

// --- Help system tests ---

func TestHelpTopics(t *testing.T) {
	topics := []string{
		"", "where", "sort", "group", "format", "date", "set", "select",
		"blame", "stash", "having",
		// New topics
		"status", "branches", "remotes", "reflog", "conflicts", "actions", "variables",
	}

	for _, topic := range topics {
		t.Run("help_"+topic, func(t *testing.T) {
			text := getHelpText(topic)
			if text == "" {
				t.Errorf("empty help for topic %q", topic)
			}
			if strings.HasPrefix(text, "No help available") {
				t.Errorf("no help for topic %q", topic)
			}
		})
	}
}

// --- Autocomplete tests ---

func TestAutocompleteNewKeywords(t *testing.T) {
	// Verify new keywords appear in suggestions
	suggestions := AutoComplete("", "", 0)
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for empty input")
	}

	// After pipe, should include new keywords
	suggestions = AutoComplete("", "commits |", 9)
	found := make(map[string]bool)
	for _, s := range suggestions {
		found[s.Text] = true
	}

	expected := []string{"status", "remotes", "reflog", "conflicts", "merge", "push", "pull", "delete", "stage", "explain", "export"}
	for _, kw := range expected {
		if !found[kw] {
			t.Errorf("expected keyword %q in pipe suggestions", kw)
		}
	}
}

func TestAutocompleteFormatTypes(t *testing.T) {
	suggestions := AutoComplete("", "commits | format", 16)
	found := make(map[string]bool)
	for _, s := range suggestions {
		found[s.Text] = true
	}

	if !found["markdown"] {
		t.Error("expected 'markdown' in format suggestions")
	}
	if !found["yaml"] {
		t.Error("expected 'yaml' in format suggestions")
	}
}
