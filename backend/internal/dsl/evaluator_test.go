package dsl

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repo with predictable commits for testing.
// Returns the repo path and a cleanup function.
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

	runAs := func(author string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME="+author,
			"GIT_AUTHOR_EMAIL="+author+"@test.com",
			"GIT_COMMITTER_NAME="+author,
			"GIT_COMMITTER_EMAIL="+author+"@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "commit.gpgsign", "false")

	// Commit 1: Alice adds a Go file (3 lines)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	run("add", "main.go")
	run("commit", "-m", "initial commit")

	// Commit 2: Bob adds a test file (2 lines)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\n\nimport \"testing\"\n"), 0644)
	runAs("Bob", "add", "main_test.go")
	runAs("Bob", "commit", "-m", "add tests")

	// Commit 3: Alice modifies main.go (adds 2 lines)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)
	run("add", "main.go")
	run("commit", "-m", "fix: update main with greeting")

	// Commit 4: Bob adds a README (1 line)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644)
	runAs("Bob", "add", "README.md")
	runAs("Bob", "commit", "-m", "add readme")

	// Commit 5: Alice adds docs
	os.WriteFile(filepath.Join(dir, "docs.md"), []byte("docs\n"), 0644)
	run("add", "docs.md")
	run("commit", "-m", "add documentation")

	// Tag commit 3
	run("tag", "v1.0.0", "HEAD~2")

	return dir
}

func evalQuery(t *testing.T, repoPath, query string) *Result {
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

func TestEvalCommits(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits")
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	if len(result.Commits) != 5 {
		t.Fatalf("expected 5 commits, got %d", len(result.Commits))
	}
}

func TestEvalWhereAuthor(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where author == "Alice"`)
	if len(result.Commits) != 3 {
		t.Errorf("expected 3 Alice commits, got %d", len(result.Commits))
	}
	for _, c := range result.Commits {
		if c.Author != "Alice" {
			t.Errorf("expected author Alice, got %q", c.Author)
		}
	}
}

func TestEvalWhereMessageContains(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where message contains "fix"`)
	if len(result.Commits) != 1 {
		t.Fatalf("expected 1 commit with 'fix', got %d", len(result.Commits))
	}
	if result.Commits[0].Message != "fix: update main with greeting" {
		t.Errorf("unexpected message: %q", result.Commits[0].Message)
	}
}

func TestEvalWhereAdditions(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where additions > 0`)
	if len(result.Commits) == 0 {
		t.Fatal("expected commits with additions > 0, got none")
	}
	for _, c := range result.Commits {
		if c.Additions <= 0 {
			t.Errorf("commit %s has additions=%d, expected > 0", c.Hash[:7], c.Additions)
		}
	}
}

func TestEvalWhereFiles(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where files contains "main.go"`)
	if len(result.Commits) < 1 {
		t.Fatal("expected at least 1 commit touching main.go")
	}
	for _, c := range result.Commits {
		found := false
		for _, f := range c.Files {
			if f == "main.go" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("commit %s matched but doesn't contain main.go in files: %v", c.Hash[:7], c.Files)
		}
	}
}

func TestEvalSortDateDesc(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | sort author asc`)
	if len(result.Commits) != 5 {
		t.Fatalf("expected 5 commits, got %d", len(result.Commits))
	}
	// Alice commits should come before Bob commits
	for i := 1; i < len(result.Commits); i++ {
		if result.Commits[i-1].Author > result.Commits[i].Author {
			t.Errorf("commits not sorted by author asc: %q > %q",
				result.Commits[i-1].Author, result.Commits[i].Author)
		}
	}
}

func TestEvalFirstLastSkip(t *testing.T) {
	repo := setupTestRepo(t)

	t.Run("first", func(t *testing.T) {
		result := evalQuery(t, repo, "commits | first 2")
		if len(result.Commits) != 2 {
			t.Errorf("expected 2 commits, got %d", len(result.Commits))
		}
	})

	t.Run("last", func(t *testing.T) {
		result := evalQuery(t, repo, "commits | last 2")
		if len(result.Commits) != 2 {
			t.Errorf("expected 2 commits, got %d", len(result.Commits))
		}
	})

	t.Run("skip", func(t *testing.T) {
		result := evalQuery(t, repo, "commits | skip 3")
		if len(result.Commits) != 2 {
			t.Errorf("expected 2 commits after skip 3, got %d", len(result.Commits))
		}
	})
}

func TestEvalCount(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | count")
	if result.Kind != "count" {
		t.Fatalf("expected kind count, got %s", result.Kind)
	}
	if result.Count != 5 {
		t.Errorf("expected count 5, got %d", result.Count)
	}
}

func TestEvalUnique(t *testing.T) {
	repo := setupTestRepo(t)

	// All commits should already be unique, but this validates the stage runs
	result := evalQuery(t, repo, "commits | unique")
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 unique commits, got %d", len(result.Commits))
	}
}

func TestEvalReverse(t *testing.T) {
	repo := setupTestRepo(t)

	normal := evalQuery(t, repo, "commits")
	reversed := evalQuery(t, repo, "commits | reverse")

	if len(reversed.Commits) != len(normal.Commits) {
		t.Fatalf("lengths differ: %d vs %d", len(normal.Commits), len(reversed.Commits))
	}
	n := len(normal.Commits)
	for i := 0; i < n; i++ {
		if normal.Commits[i].Hash != reversed.Commits[n-1-i].Hash {
			t.Errorf("position %d: expected %s, got %s",
				i, normal.Commits[i].Hash[:7], reversed.Commits[n-1-i].Hash[:7])
		}
	}
}

func TestEvalSelect(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | select hash, message")
	for _, c := range result.Commits {
		if c.Hash == "" {
			t.Error("hash should be populated")
		}
		if c.Message == "" {
			t.Error("message should be populated")
		}
		if c.Author != "" {
			t.Errorf("author should be empty after select, got %q", c.Author)
		}
		if c.Date != "" {
			t.Errorf("date should be empty after select, got %q", c.Date)
		}
	}
}

func TestEvalLastNRange(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits[last 3]")
	if len(result.Commits) != 3 {
		t.Errorf("expected 3 commits from last 3 range, got %d", len(result.Commits))
	}
}

func TestEvalCompoundPipeline(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where author == "Alice" and additions > 0 | first 2`)
	if result.Kind != "commits" {
		t.Fatalf("expected commits, got %s", result.Kind)
	}
	if len(result.Commits) > 2 {
		t.Errorf("expected at most 2 commits, got %d", len(result.Commits))
	}
	for _, c := range result.Commits {
		if c.Author != "Alice" {
			t.Errorf("expected Alice, got %q", c.Author)
		}
		if c.Additions <= 0 {
			t.Errorf("expected additions > 0, got %d", c.Additions)
		}
	}
}

func TestEvalBranches(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "branches")
	if result.Kind != "branches" {
		t.Fatalf("expected kind branches, got %s", result.Kind)
	}
	if len(result.Branches) < 1 {
		t.Fatal("expected at least 1 branch")
	}
	found := false
	for _, b := range result.Branches {
		if b.Name == "main" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find 'main' branch")
	}
}

func TestEvalTags(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "tags")
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	if len(result.Commits) < 1 {
		t.Fatal("expected at least 1 tagged commit")
	}
	found := false
	for _, c := range result.Commits {
		for _, tag := range c.Tags {
			if tag == "v1.0.0" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find tag v1.0.0")
	}
}

func TestEvalWhereCompound(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where author == "Alice" or author == "Bob"`)
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits (Alice or Bob), got %d", len(result.Commits))
	}
}

func TestEvalWhereNot(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where not author == "Bob"`)
	for _, c := range result.Commits {
		if c.Author == "Bob" {
			t.Errorf("expected no Bob commits, but found one: %s", c.Hash[:7])
		}
	}
}

func TestEvalWhereMatches(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where message matches "^(add|fix)"`)
	if len(result.Commits) == 0 {
		t.Fatal("expected commits matching regex")
	}
}

func TestEvalSortAdditions(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | sort additions desc | first 1")
	if len(result.Commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(result.Commits))
	}
	// The commit with the most additions should be first
	if result.Commits[0].Additions == 0 {
		t.Error("expected non-zero additions for top commit by additions desc")
	}
}

// --- Date Intelligence Tests ---

func TestEvalDateISO(t *testing.T) {
	repo := setupTestRepo(t)

	// Verify DateISO is populated on all commits
	result := evalQuery(t, repo, "commits")
	for _, c := range result.Commits {
		if c.DateISO == "" {
			t.Errorf("commit %s has empty DateISO", c.Hash[:7])
		}
	}
}

func TestEvalDateComparison(t *testing.T) {
	repo := setupTestRepo(t)

	// All commits were just created, so they should all be after 2020-01-01
	result := evalQuery(t, repo, `commits | where date > "2020-01-01"`)
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits after 2020-01-01, got %d", len(result.Commits))
	}

	// No commits should be before 2020-01-01
	result = evalQuery(t, repo, `commits | where date < "2020-01-01"`)
	if len(result.Commits) != 0 {
		t.Errorf("expected 0 commits before 2020-01-01, got %d", len(result.Commits))
	}
}

func TestEvalDateWithinLast(t *testing.T) {
	repo := setupTestRepo(t)

	// All commits were just created, so they should be within last 1 day
	result := evalQuery(t, repo, "commits | where date within last 1 days")
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits within last 1 day, got %d", len(result.Commits))
	}

	// Also test other time units
	result = evalQuery(t, repo, "commits | where date within last 1 hours")
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits within last 1 hour, got %d", len(result.Commits))
	}

	result = evalQuery(t, repo, "commits | where date within last 1 weeks")
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits within last 1 week, got %d", len(result.Commits))
	}

	result = evalQuery(t, repo, "commits | where date within last 1 months")
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits within last 1 month, got %d", len(result.Commits))
	}

	result = evalQuery(t, repo, "commits | where date within last 1 years")
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits within last 1 year, got %d", len(result.Commits))
	}
}

func TestEvalDateBetween(t *testing.T) {
	repo := setupTestRepo(t)

	// All commits are recent, so between 2020 and 2030 should capture all
	result := evalQuery(t, repo, `commits | where date between "2020-01-01" and "2030-12-31"`)
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits between 2020 and 2030, got %d", len(result.Commits))
	}

	// A range in the far past should capture none
	result = evalQuery(t, repo, `commits | where date between "2000-01-01" and "2010-12-31"`)
	if len(result.Commits) != 0 {
		t.Errorf("expected 0 commits between 2000 and 2010, got %d", len(result.Commits))
	}
}

func TestEvalSortDateChronological(t *testing.T) {
	repo := setupTestRepo(t)

	// Sort by date ascending should give chronological order
	result := evalQuery(t, repo, "commits | sort date asc")
	if len(result.Commits) != 5 {
		t.Fatalf("expected 5 commits, got %d", len(result.Commits))
	}
	for i := 1; i < len(result.Commits); i++ {
		// DateISO sorts lexicographically correctly
		if result.Commits[i-1].DateISO > result.Commits[i].DateISO {
			t.Errorf("commits not in chronological order at pos %d: %s > %s",
				i, result.Commits[i-1].DateISO, result.Commits[i].DateISO)
		}
	}

	// Sort desc should be reverse chronological
	result = evalQuery(t, repo, "commits | sort date desc")
	for i := 1; i < len(result.Commits); i++ {
		if result.Commits[i-1].DateISO < result.Commits[i].DateISO {
			t.Errorf("commits not in reverse chronological order at pos %d: %s < %s",
				i, result.Commits[i-1].DateISO, result.Commits[i].DateISO)
		}
	}
}

func TestEvalDateCompound(t *testing.T) {
	repo := setupTestRepo(t)

	// Combine date filter with author filter
	result := evalQuery(t, repo, `commits | where author == "Alice" and date within last 1 days`)
	if len(result.Commits) != 3 {
		t.Errorf("expected 3 Alice commits within last day, got %d", len(result.Commits))
	}
	for _, c := range result.Commits {
		if c.Author != "Alice" {
			t.Errorf("expected Alice, got %q", c.Author)
		}
	}
}

func TestEvalSelectDateISO(t *testing.T) {
	repo := setupTestRepo(t)

	// When selecting date, DateISO should also be included
	result := evalQuery(t, repo, "commits | select date, hash")
	for _, c := range result.Commits {
		if c.Date == "" {
			t.Error("date should be populated after select")
		}
		if c.DateISO == "" {
			t.Error("DateISO should be populated when date is selected")
		}
		if c.Author != "" {
			t.Errorf("author should be empty after select, got %q", c.Author)
		}
	}
}

func TestParseDateExpressions(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"date greater than", `commits | where date > "2025-01-01"`},
		{"date less than", `commits | where date < "2025-01-01"`},
		{"date gte", `commits | where date >= "2025-01-01"`},
		{"date lte", `commits | where date <= "2025-01-01"`},
		{"date within last days", `commits | where date within last 7 days`},
		{"date within last hours", `commits | where date within last 24 hours`},
		{"date within last weeks", `commits | where date within last 2 weeks`},
		{"date within last months", `commits | where date within last 3 months`},
		{"date within last years", `commits | where date within last 1 years`},
		{"date between", `commits | where date between "2025-01-01" and "2025-12-31"`},
		{"date compound", `commits | where date > "2025-01-01" and author == "Alice"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.query)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.query, err)
			}
		})
	}
}

// --- Aggregate Tests ---

func TestEvalSumAdditions(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | sum additions")
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if len(result.Aggregates) != 1 {
		t.Fatalf("expected 1 aggregate row, got %d", len(result.Aggregates))
	}
	if result.Aggregates[0].Group != "" {
		t.Errorf("expected empty group for scalar, got %q", result.Aggregates[0].Group)
	}
	if result.Aggregates[0].Value <= 0 {
		t.Errorf("expected positive sum, got %f", result.Aggregates[0].Value)
	}
	if result.AggFunc != "sum" {
		t.Errorf("expected aggFunc sum, got %q", result.AggFunc)
	}
	if result.AggField != "additions" {
		t.Errorf("expected aggField additions, got %q", result.AggField)
	}
}

func TestEvalAvgAdditions(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | avg additions")
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if len(result.Aggregates) != 1 {
		t.Fatalf("expected 1 aggregate row, got %d", len(result.Aggregates))
	}
	if result.AggFunc != "avg" {
		t.Errorf("expected aggFunc avg, got %q", result.AggFunc)
	}
	if result.Aggregates[0].Value <= 0 {
		t.Errorf("expected positive avg, got %f", result.Aggregates[0].Value)
	}
}

func TestEvalMinMax(t *testing.T) {
	repo := setupTestRepo(t)

	t.Run("min", func(t *testing.T) {
		result := evalQuery(t, repo, "commits | min additions")
		if result.Kind != "aggregate" {
			t.Fatalf("expected kind aggregate, got %s", result.Kind)
		}
		if result.AggFunc != "min" {
			t.Errorf("expected aggFunc min, got %q", result.AggFunc)
		}
	})

	t.Run("max", func(t *testing.T) {
		result := evalQuery(t, repo, "commits | max additions")
		if result.Kind != "aggregate" {
			t.Fatalf("expected kind aggregate, got %s", result.Kind)
		}
		if result.AggFunc != "max" {
			t.Errorf("expected aggFunc max, got %q", result.AggFunc)
		}
		// Max should be >= min
		minResult := evalQuery(t, repo, "commits | min additions")
		if result.Aggregates[0].Value < minResult.Aggregates[0].Value {
			t.Errorf("max (%f) < min (%f)", result.Aggregates[0].Value, minResult.Aggregates[0].Value)
		}
	})
}

func TestEvalGroupByAuthorCount(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | group by author | count")
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if result.AggFunc != "count" {
		t.Errorf("expected aggFunc count, got %q", result.AggFunc)
	}
	if result.GroupField != "author" {
		t.Errorf("expected groupField author, got %q", result.GroupField)
	}
	if len(result.Aggregates) != 2 {
		t.Fatalf("expected 2 groups (Alice, Bob), got %d", len(result.Aggregates))
	}

	// Rows are sorted by key, so Alice comes first
	alice := result.Aggregates[0]
	bob := result.Aggregates[1]
	if alice.Group != "Alice" || bob.Group != "Bob" {
		t.Errorf("expected Alice and Bob groups, got %q and %q", alice.Group, bob.Group)
	}
	if alice.Value != 3 {
		t.Errorf("expected Alice count 3, got %f", alice.Value)
	}
	if bob.Value != 2 {
		t.Errorf("expected Bob count 2, got %f", bob.Value)
	}
}

func TestEvalGroupByAuthorSum(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | group by author | sum additions")
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if len(result.Aggregates) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result.Aggregates))
	}
	for _, row := range result.Aggregates {
		if row.Group == "" {
			t.Error("expected non-empty group key")
		}
	}
}

func TestEvalGroupByStandalone(t *testing.T) {
	repo := setupTestRepo(t)

	// Standalone group by should produce implicit count
	result := evalQuery(t, repo, "commits | group by author")
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if result.AggFunc != "count" {
		t.Errorf("expected implicit count, got aggFunc %q", result.AggFunc)
	}
	if len(result.Aggregates) != 2 {
		t.Errorf("expected 2 groups, got %d", len(result.Aggregates))
	}
}

func TestEvalGroupByWithSort(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "commits | group by author | sum additions | sort value desc | first 1")
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if len(result.Aggregates) != 1 {
		t.Fatalf("expected 1 row after first 1, got %d", len(result.Aggregates))
	}
}

func TestEvalGroupByWithFilter(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | where author == "Alice" | group by author | sum additions`)
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind aggregate, got %s", result.Kind)
	}
	if len(result.Aggregates) != 1 {
		t.Fatalf("expected 1 group (Alice only), got %d", len(result.Aggregates))
	}
	if result.Aggregates[0].Group != "Alice" {
		t.Errorf("expected Alice, got %q", result.Aggregates[0].Group)
	}
}

func TestParseAggregateStages(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"sum", "commits | sum additions"},
		{"avg", "commits | avg deletions"},
		{"min", "commits | min additions"},
		{"max", "commits | max additions"},
		{"group by", "commits | group by author"},
		{"group by count", "commits | group by author | count"},
		{"group by sum", "commits | group by author | sum additions"},
		{"full pipeline", "commits | group by author | sum additions | sort value desc | first 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.query)
			if err != nil {
				t.Errorf("failed to parse %q: %v", tt.query, err)
			}
		})
	}
}

func TestParseDateHelper(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"2025-01-15", true},
		{"2025-01-15T14:30:00", true},
		{"2025-01-15T14:30:00Z", true},
		{"2025-01-15T14:30:00+00:00", true},
		{"not a date", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseDate(tt.input)
			if tt.valid && err != nil {
				t.Errorf("expected valid date, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected error for invalid date %q", tt.input)
			}
		})
	}
}

// --- Format tests ---

func TestFormatJSON_Commits(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | first 2 | format json`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}
	if result.FormatType != "json" {
		t.Fatalf("expected format type json, got %s", result.FormatType)
	}

	// Validate it's valid JSON
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result.FormattedOutput), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, result.FormattedOutput)
	}
	if len(parsed) != 2 {
		t.Errorf("expected 2 commits in JSON, got %d", len(parsed))
	}
	// Each commit should have hash, message, author, date
	for _, c := range parsed {
		if _, ok := c["hash"]; !ok {
			t.Error("JSON commit missing 'hash' field")
		}
		if _, ok := c["author"]; !ok {
			t.Error("JSON commit missing 'author' field")
		}
	}
}

func TestFormatCSV_Commits(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | first 2 | format csv`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}
	if result.FormatType != "csv" {
		t.Fatalf("expected format type csv, got %s", result.FormatType)
	}

	lines := strings.Split(result.FormattedOutput, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 CSV lines (header + 2 rows), got %d", len(lines))
	}
	if lines[0] != "hash,author,date,message" {
		t.Errorf("unexpected CSV header: %q", lines[0])
	}
}

func TestFormatTable_Commits(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | first 2 | format table`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}
	if result.FormatType != "table" {
		t.Fatalf("expected format type table, got %s", result.FormatType)
	}

	lines := strings.Split(result.FormattedOutput, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 table lines (header + separator + 2 rows), got %d", len(lines))
	}
	// Header should contain column names
	if !strings.Contains(lines[0], "HASH") || !strings.Contains(lines[0], "AUTHOR") {
		t.Errorf("table header missing expected columns: %q", lines[0])
	}
	// Second line should be separator
	if !strings.Contains(lines[1], "---") {
		t.Errorf("expected separator row, got: %q", lines[1])
	}
}

func TestFormatJSON_Count(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | count | format json`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.FormattedOutput), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, result.FormattedOutput)
	}
	count, ok := parsed["count"]
	if !ok {
		t.Fatal("JSON missing 'count' field")
	}
	if count.(float64) != 5 {
		t.Errorf("expected count 5, got %v", count)
	}
}

func TestFormatJSON_Aggregate(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `commits | group by author | count | format json`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}

	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result.FormattedOutput), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, result.FormattedOutput)
	}
	if len(parsed) != 2 {
		t.Errorf("expected 2 aggregate rows (Alice, Bob), got %d", len(parsed))
	}
}

func TestFormatCSV_Branches(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `branches | format csv`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}

	lines := strings.Split(result.FormattedOutput, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 CSV lines, got %d", len(lines))
	}
	if lines[0] != "name,is_current,remote" {
		t.Errorf("unexpected branch CSV header: %q", lines[0])
	}
}

func TestFormatWithSelect(t *testing.T) {
	repo := setupTestRepo(t)

	// format should work after select (select projects fields, format serializes)
	result := evalQuery(t, repo, `commits | first 1 | select hash, author | format json`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}

	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result.FormattedOutput), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, result.FormattedOutput)
	}
	if len(parsed) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(parsed))
	}
	// Should have hash and author, message should be empty/missing
	if parsed[0]["hash"] == "" {
		t.Error("expected hash to be populated")
	}
	if parsed[0]["author"] == "" {
		t.Error("expected author to be populated")
	}
}

func TestParseFormatStage(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"commits | format json", true},
		{"commits | format csv", true},
		{"commits | format table", true},
		{"commits | format xml", false},
		{"commits | format", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Parse(tt.input)
			if tt.valid && err != nil {
				t.Errorf("expected valid parse, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected parse error for %q", tt.input)
			}
		})
	}
}

// --- Help System Tests ---

func TestEvalHelp(t *testing.T) {
	repo := setupTestRepo(t)

	t.Run("general help", func(t *testing.T) {
		result := evalQuery(t, repo, "help")
		if result.Kind != "formatted" {
			t.Fatalf("expected kind formatted, got %s", result.Kind)
		}
		if result.FormattedOutput == "" {
			t.Fatal("expected non-empty help output")
		}
		if !strings.Contains(result.FormattedOutput, "commits") {
			t.Error("general help should mention 'commits'")
		}
	})

	t.Run("help where", func(t *testing.T) {
		result := evalQuery(t, repo, "help where")
		if result.Kind != "formatted" {
			t.Fatalf("expected kind formatted, got %s", result.Kind)
		}
		if !strings.Contains(result.FormattedOutput, "where") {
			t.Error("help where should mention 'where'")
		}
	})

	t.Run("help format", func(t *testing.T) {
		result := evalQuery(t, repo, "help format")
		if result.Kind != "formatted" {
			t.Fatalf("expected kind formatted, got %s", result.Kind)
		}
		if !strings.Contains(result.FormattedOutput, "json") {
			t.Error("help format should mention 'json'")
		}
	})

	t.Run("help sort", func(t *testing.T) {
		result := evalQuery(t, repo, "help sort")
		if result.Kind != "formatted" {
			t.Fatalf("expected kind formatted, got %s", result.Kind)
		}
		if !strings.Contains(result.FormattedOutput, "sort") {
			t.Error("help sort should mention 'sort'")
		}
	})

	t.Run("help date", func(t *testing.T) {
		result := evalQuery(t, repo, "help date")
		if result.Kind != "formatted" {
			t.Fatalf("expected kind formatted, got %s", result.Kind)
		}
		if !strings.Contains(result.FormattedOutput, "date") {
			t.Error("help date should mention 'date'")
		}
	})

	t.Run("help unknown topic", func(t *testing.T) {
		result := evalQuery(t, repo, "help nonexistent")
		if result.Kind != "formatted" {
			t.Fatalf("expected kind formatted, got %s", result.Kind)
		}
		// Should still return something (general help or unknown topic message)
		if result.FormattedOutput == "" {
			t.Fatal("expected non-empty output for unknown help topic")
		}
	})
}

// --- Set Operations Tests ---

func TestEvalSetExcept(t *testing.T) {
	repo := setupTestRepo(t)

	// Create a feature branch with an extra commit
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
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

	run("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(repo, "feature.go"), []byte("package feature\n"), 0644)
	run("add", "feature.go")
	run("commit", "-m", "feature commit")
	run("checkout", "main")

	// commits[feature] except commits[main] should return only the feature-only commit
	result := evalQuery(t, repo, "commits[feature] | except commits[main]")
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("expected 1 commit (feature only), got %d", len(result.Commits))
	}
	if result.Commits[0].Message != "feature commit" {
		t.Errorf("expected 'feature commit', got %q", result.Commits[0].Message)
	}
}

func TestEvalSetIntersect(t *testing.T) {
	repo := setupTestRepo(t)

	// Intersect main with itself should return all commits
	result := evalQuery(t, repo, "commits[main] | intersect commits[main]")
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits (self-intersection), got %d", len(result.Commits))
	}
}

func TestEvalSetUnion(t *testing.T) {
	repo := setupTestRepo(t)

	// Union of main with itself should still be 5 (unique hashes)
	result := evalQuery(t, repo, "commits[main] | union commits[main]")
	if result.Kind != "commits" {
		t.Fatalf("expected kind commits, got %s", result.Kind)
	}
	if len(result.Commits) != 5 {
		t.Errorf("expected 5 commits (self-union deduped), got %d", len(result.Commits))
	}
}

// --- File-level Query Tests ---

func TestEvalFileSource(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "files")
	if result.Kind != "files" {
		t.Fatalf("expected kind files, got %s", result.Kind)
	}
	if len(result.Files) == 0 {
		t.Fatal("expected at least 1 file")
	}

	// main.go should appear (touched in multiple commits)
	found := false
	for _, f := range result.Files {
		if f.Path == "main.go" {
			found = true
			if f.Commits < 2 {
				t.Errorf("main.go should have at least 2 commits, got %d", f.Commits)
			}
			if f.Additions <= 0 {
				t.Errorf("main.go should have positive additions, got %d", f.Additions)
			}
		}
	}
	if !found {
		t.Error("expected to find main.go in file results")
	}
}

func TestEvalFileWherePathContains(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, `files | where path contains "main"`)
	if result.Kind != "files" {
		t.Fatalf("expected kind files, got %s", result.Kind)
	}
	for _, f := range result.Files {
		if !strings.Contains(f.Path, "main") {
			t.Errorf("expected path containing 'main', got %q", f.Path)
		}
	}
	if len(result.Files) == 0 {
		t.Error("expected at least 1 file matching 'main'")
	}
}

func TestEvalFileWhereAdditions(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "files | where additions > 1")
	if result.Kind != "files" {
		t.Fatalf("expected kind files, got %s", result.Kind)
	}
	for _, f := range result.Files {
		if f.Additions <= 1 {
			t.Errorf("expected additions > 1, got %d for %s", f.Additions, f.Path)
		}
	}
}

func TestEvalFileSortAdditions(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "files | sort additions desc")
	if result.Kind != "files" {
		t.Fatalf("expected kind files, got %s", result.Kind)
	}
	if len(result.Files) < 2 {
		t.Skip("need at least 2 files to test sorting")
	}
	for i := 1; i < len(result.Files); i++ {
		if result.Files[i-1].Additions < result.Files[i].Additions {
			t.Errorf("files not sorted by additions desc: %d < %d",
				result.Files[i-1].Additions, result.Files[i].Additions)
		}
	}
}

func TestEvalFileCount(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "files | count")
	if result.Kind != "count" {
		t.Fatalf("expected kind count, got %s", result.Kind)
	}
	if result.Count == 0 {
		t.Error("expected non-zero file count")
	}
}

func TestEvalFileFirst(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "files | first 2")
	if result.Kind != "files" {
		t.Fatalf("expected kind files, got %s", result.Kind)
	}
	if len(result.Files) > 2 {
		t.Errorf("expected at most 2 files, got %d", len(result.Files))
	}
}

func TestEvalFileFormatJSON(t *testing.T) {
	repo := setupTestRepo(t)

	result := evalQuery(t, repo, "files | format json")
	if result.Kind != "formatted" {
		t.Fatalf("expected kind formatted, got %s", result.Kind)
	}
	if result.FormatType != "json" {
		t.Fatalf("expected format type json, got %s", result.FormatType)
	}

	var parsed []map[string]any
	if err := json.Unmarshal([]byte(result.FormattedOutput), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, result.FormattedOutput)
	}
	if len(parsed) == 0 {
		t.Error("expected non-empty JSON array")
	}
	for _, f := range parsed {
		if _, ok := f["path"]; !ok {
			t.Error("JSON file result missing 'path' field")
		}
	}
}

// --- Alias Tests ---

func TestRunQueryAlias(t *testing.T) {
	repo := setupTestRepo(t)

	ctx := &EvalContext{
		RepoPath: repo,
		Ctx:      context.Background(),
		Aliases:  NewAliasStore(),
	}

	// Define an alias
	result, err := RunQuery(ctx, `alias recent = commits | first 2`)
	if err != nil {
		t.Fatalf("alias define failed: %v", err)
	}
	if result.Kind != "formatted" {
		t.Fatalf("expected formatted result, got %s", result.Kind)
	}
	if !strings.Contains(result.FormattedOutput, "recent") {
		t.Errorf("expected alias confirmation, got %q", result.FormattedOutput)
	}

	// Use the alias
	result, err = RunQuery(ctx, "recent")
	if err != nil {
		t.Fatalf("alias expand failed: %v", err)
	}
	if result.Kind != "commits" {
		t.Fatalf("expected commits from alias, got %s", result.Kind)
	}
	if len(result.Commits) != 2 {
		t.Errorf("expected 2 commits from alias, got %d", len(result.Commits))
	}
}

func TestRunQueryAliasList(t *testing.T) {
	ctx := &EvalContext{
		RepoPath: ".",
		Ctx:      context.Background(),
		Aliases:  NewAliasStore(),
	}

	// No aliases yet
	result, err := RunQuery(ctx, "aliases")
	if err != nil {
		t.Fatalf("aliases list failed: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "no aliases") {
		t.Errorf("expected 'no aliases' message, got %q", result.FormattedOutput)
	}

	// Add one, then list
	ctx.Aliases.Set("foo", "commits | first 1")
	result, err = RunQuery(ctx, "aliases")
	if err != nil {
		t.Fatalf("aliases list failed: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "foo") {
		t.Errorf("expected 'foo' in aliases list, got %q", result.FormattedOutput)
	}
}

func TestRunQueryUnalias(t *testing.T) {
	ctx := &EvalContext{
		RepoPath: ".",
		Ctx:      context.Background(),
		Aliases:  NewAliasStore(),
	}

	ctx.Aliases.Set("bar", "commits")
	result, err := RunQuery(ctx, "unalias bar")
	if err != nil {
		t.Fatalf("unalias failed: %v", err)
	}
	if !strings.Contains(result.FormattedOutput, "removed") {
		t.Errorf("expected 'removed' message, got %q", result.FormattedOutput)
	}

	// Verify it's gone
	if _, ok := ctx.Aliases.Get("bar"); ok {
		t.Error("alias 'bar' should have been removed")
	}
}

// --- Blame tests ---

func TestEvalBlameSource(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `blame "main.go"`)
	if result.Kind != "blame" {
		t.Fatalf("expected kind 'blame', got %q", result.Kind)
	}
	if len(result.BlameLines) == 0 {
		t.Fatal("expected blame lines, got none")
	}
	// main.go has 7 lines after the last commit
	if len(result.BlameLines) != 7 {
		t.Fatalf("expected 7 blame lines, got %d", len(result.BlameLines))
	}
}

func TestEvalBlameWhereAuthor(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `blame "main.go" | where author == "Alice"`)
	if result.Kind != "blame" {
		t.Fatalf("expected kind 'blame', got %q", result.Kind)
	}
	for _, line := range result.BlameLines {
		if line.Author != "Alice" {
			t.Errorf("expected author 'Alice', got %q", line.Author)
		}
	}
}

func TestEvalBlameWhereContent(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `blame "main.go" | where content contains "fmt"`)
	if result.Kind != "blame" {
		t.Fatalf("expected kind 'blame', got %q", result.Kind)
	}
	if len(result.BlameLines) == 0 {
		t.Fatal("expected at least one blame line matching 'fmt'")
	}
	for _, line := range result.BlameLines {
		if !strings.Contains(strings.ToLower(line.Content), "fmt") {
			t.Errorf("expected content containing 'fmt', got %q", line.Content)
		}
	}
}

func TestEvalBlameCount(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `blame "main.go" | count`)
	if result.Kind != "count" {
		t.Fatalf("expected kind 'count', got %q", result.Kind)
	}
	if result.Count != 7 {
		t.Fatalf("expected count 7, got %d", result.Count)
	}
}

func TestEvalBlameFirst(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `blame "main.go" | first 3`)
	if result.Kind != "blame" {
		t.Fatalf("expected kind 'blame', got %q", result.Kind)
	}
	if len(result.BlameLines) != 3 {
		t.Fatalf("expected 3 blame lines, got %d", len(result.BlameLines))
	}
}

func TestEvalBlameWhereLineno(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `blame "main.go" | where lineno <= 3`)
	if result.Kind != "blame" {
		t.Fatalf("expected kind 'blame', got %q", result.Kind)
	}
	if len(result.BlameLines) != 3 {
		t.Fatalf("expected 3 blame lines, got %d", len(result.BlameLines))
	}
	for _, line := range result.BlameLines {
		if line.LineNo > 3 {
			t.Errorf("expected lineno <= 3, got %d", line.LineNo)
		}
	}
}

// --- Stash tests ---

func TestEvalStashSourceEmpty(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `stash`)
	if result.Kind != "stashes" {
		t.Fatalf("expected kind 'stashes', got %q", result.Kind)
	}
	// No stashes in the test repo
	if len(result.Stashes) != 0 {
		t.Fatalf("expected 0 stashes, got %d", len(result.Stashes))
	}
}

func TestEvalStashCountEmpty(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `stash | count`)
	if result.Kind != "count" {
		t.Fatalf("expected kind 'count', got %q", result.Kind)
	}
	if result.Count != 0 {
		t.Fatalf("expected count 0, got %d", result.Count)
	}
}

func TestEvalStashWithEntries(t *testing.T) {
	dir := setupTestRepo(t)

	// Create a stash
	os.WriteFile(filepath.Join(dir, "stash_test.txt"), []byte("stash content\n"), 0644)
	cmd := exec.Command("git", "-C", dir, "add", "stash_test.txt")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Alice",
		"GIT_AUTHOR_EMAIL=alice@test.com",
		"GIT_COMMITTER_NAME=Alice",
		"GIT_COMMITTER_EMAIL=alice@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %s\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", dir, "stash", "push", "-m", "wip stash")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Alice",
		"GIT_AUTHOR_EMAIL=alice@test.com",
		"GIT_COMMITTER_NAME=Alice",
		"GIT_COMMITTER_EMAIL=alice@test.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git stash failed: %s\n%s", err, out)
	}

	result := evalQuery(t, dir, `stash`)
	if result.Kind != "stashes" {
		t.Fatalf("expected kind 'stashes', got %q", result.Kind)
	}
	if len(result.Stashes) != 1 {
		t.Fatalf("expected 1 stash, got %d", len(result.Stashes))
	}
	if !strings.Contains(result.Stashes[0].Message, "wip stash") {
		t.Errorf("expected message containing 'wip stash', got %q", result.Stashes[0].Message)
	}
}

func TestEvalStashWhereMessage(t *testing.T) {
	dir := setupTestRepo(t)

	// Create two stashes
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644)
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Alice",
			"GIT_AUTHOR_EMAIL=alice@test.com",
			"GIT_COMMITTER_NAME=Alice",
			"GIT_COMMITTER_EMAIL=alice@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}
	run("add", "a.txt")
	run("stash", "push", "-m", "feature work")

	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0644)
	run("add", "b.txt")
	run("stash", "push", "-m", "wip bugfix")

	result := evalQuery(t, dir, `stash | where message contains "wip"`)
	if result.Kind != "stashes" {
		t.Fatalf("expected kind 'stashes', got %q", result.Kind)
	}
	if len(result.Stashes) != 1 {
		t.Fatalf("expected 1 stash matching 'wip', got %d", len(result.Stashes))
	}
}

// --- Having tests ---

func TestEvalHaving(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `commits | group by author | count | having value > 2`)
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind 'aggregate', got %q", result.Kind)
	}
	// Alice has 3 commits, Bob has 2 — only Alice should remain
	if len(result.Aggregates) != 1 {
		t.Fatalf("expected 1 aggregate row, got %d", len(result.Aggregates))
	}
	if result.Aggregates[0].Group != "Alice" {
		t.Errorf("expected group 'Alice', got %q", result.Aggregates[0].Group)
	}
}

func TestEvalHavingGroupContains(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `commits | group by author | count | having group contains "bob"`)
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind 'aggregate', got %q", result.Kind)
	}
	if len(result.Aggregates) != 1 {
		t.Fatalf("expected 1 aggregate row, got %d", len(result.Aggregates))
	}
	if result.Aggregates[0].Group != "Bob" {
		t.Errorf("expected group 'Bob', got %q", result.Aggregates[0].Group)
	}
}

func TestEvalHavingValueEquals(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `commits | group by author | count | having value == 2`)
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind 'aggregate', got %q", result.Kind)
	}
	// Bob has exactly 2 commits
	if len(result.Aggregates) != 1 {
		t.Fatalf("expected 1 aggregate row, got %d", len(result.Aggregates))
	}
	if result.Aggregates[0].Group != "Bob" {
		t.Errorf("expected group 'Bob', got %q", result.Aggregates[0].Group)
	}
}

func TestEvalHavingNoResults(t *testing.T) {
	dir := setupTestRepo(t)
	result := evalQuery(t, dir, `commits | group by author | count | having value > 100`)
	if result.Kind != "aggregate" {
		t.Fatalf("expected kind 'aggregate', got %q", result.Kind)
	}
	if len(result.Aggregates) != 0 {
		t.Fatalf("expected 0 aggregate rows, got %d", len(result.Aggregates))
	}
}

// --- Help tests for new topics ---

func TestEvalHelpBlame(t *testing.T) {
	result := evalQuery(t, ".", `help blame`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind 'formatted', got %q", result.Kind)
	}
	if !strings.Contains(result.FormattedOutput, "BLAME") {
		t.Error("expected help text to contain 'BLAME'")
	}
}

func TestEvalHelpStash(t *testing.T) {
	result := evalQuery(t, ".", `help stash`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind 'formatted', got %q", result.Kind)
	}
	if !strings.Contains(result.FormattedOutput, "STASH") {
		t.Error("expected help text to contain 'STASH'")
	}
}

func TestEvalHelpHaving(t *testing.T) {
	result := evalQuery(t, ".", `help having`)
	if result.Kind != "formatted" {
		t.Fatalf("expected kind 'formatted', got %q", result.Kind)
	}
	if !strings.Contains(result.FormattedOutput, "HAVING") {
		t.Error("expected help text to contain 'HAVING'")
	}
}

func TestRunQueryAliasInvalidQuery(t *testing.T) {
	ctx := &EvalContext{
		RepoPath: ".",
		Ctx:      context.Background(),
		Aliases:  NewAliasStore(),
	}

	// Invalid query should fail
	_, err := RunQuery(ctx, `alias bad = | invalid`)
	if err == nil {
		t.Fatal("expected error for invalid alias query")
	}
}
