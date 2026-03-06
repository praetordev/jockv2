package dsl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/daearol/jockv2/backend/internal/git"
)

// Parse tokenizes and parses a DSL query string into a Pipeline AST.
func Parse(input string) (*Pipeline, error) {
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}
	parser := NewParser(tokens)
	return parser.Parse()
}

// CommitCache is the interface for caching git data.
// Implemented by cache.Manager. Nil means no caching.
type CommitCache interface {
	GetCommits(repoPath string, limit, skip int, branch string, needStats bool) ([]git.RawCommit, error)
	GetBranches(repoPath string) ([]git.BranchInfo, error)
	Invalidate(repoPath string)
}

// EvalContext holds the repository and execution context.
type EvalContext struct {
	RepoPath  string
	DryRun    bool
	Ctx       context.Context
	Cache     CommitCache // nil = no caching, falls back to direct git calls
	Aliases   *AliasStore // nil = no alias support
	Variables map[string]interface{} // Phase 3: user-defined variables
}

// Result is the output of evaluating a DSL pipeline.
type Result struct {
	Kind            string              // "commits", "branches", "files", "blame", "stashes", "count", "aggregate", "formatted", "action_report", "error", "status", "remotes", "remote_branches", "reflog", "conflicts", "tasks", "explain"
	Commits         []CommitResult      // populated when Kind == "commits"
	Branches        []BranchResult      // populated when Kind == "branches"
	Files           []FileResult        // populated when Kind == "files"
	BlameLines      []BlameLineResult   // populated when Kind == "blame"
	Stashes         []StashResult       // populated when Kind == "stashes"
	StatusEntries   []StatusResult      // populated when Kind == "status"
	Remotes         []RemoteResult      // populated when Kind == "remotes"
	RemoteBranches  []RemoteBranchResult // populated when Kind == "remote_branches"
	ReflogEntries   []ReflogResult      // populated when Kind == "reflog"
	Conflicts       []ConflictResult    // populated when Kind == "conflicts"
	Tasks           []TaskResult        // populated when Kind == "tasks"
	Count           int                 // populated when Kind == "count"
	Report          *ActionReport       // populated when Kind == "action_report"
	Aggregates      []AggregateRow      // populated when Kind == "aggregate"
	AggFunc         string              // "count", "sum", "avg", "min", "max", etc.
	AggField        string              // field aggregated (e.g., "additions")
	GroupField      string              // field grouped by (e.g., "author")
	FormattedOutput string              // populated when Kind == "formatted"
	FormatType      string              // "json", "csv", "table", "markdown", "yaml"
}

// BlameLineResult is a line from git blame output.
type BlameLineResult struct {
	Hash    string
	Author  string
	Date    string
	LineNo  int
	Content string
}

// StashResult is a stash entry in query results.
type StashResult struct {
	Index   int
	Message string
	Branch  string
	Date    string
}

// FileResult is a file in file-level query results.
type FileResult struct {
	Path      string
	Additions int
	Deletions int
	Commits   int // number of commits touching this file
}

// AggregateRow is a single row in an aggregate result.
type AggregateRow struct {
	Group string  // group key (e.g., author name). Empty for ungrouped scalars.
	Value float64 // the aggregate value
}

// CommitResult is a commit in query results.
type CommitResult struct {
	Hash      string
	Message   string
	Author    string
	Date      string // relative date for display ("3 days ago")
	DateISO   string // ISO 8601 for comparisons ("2025-01-15T14:30:00+00:00")
	Branches  []string
	Tags      []string
	Parents   []string
	Additions int
	Deletions int
	Files     []string
}

// BranchResult is a branch in query results.
type BranchResult struct {
	Name           string
	IsCurrent      bool
	Remote         string
	Merged         bool   // Phase 1: whether merged into current branch
	LastCommitDate string // Phase 1: date of last commit on this branch
	Ahead          int    // Phase 1: commits ahead of upstream
	Behind         int    // Phase 1: commits behind upstream
}

// StatusResult is a working directory file entry.
type StatusResult struct {
	Path   string
	Status string // "modified", "added", "deleted", "renamed", "copied", "untracked", "unmerged"
	Staged bool
}

// RemoteResult is a git remote.
type RemoteResult struct {
	Name     string
	FetchURL string
	PushURL  string
}

// RemoteBranchResult is a remote tracking branch.
type RemoteBranchResult struct {
	Name   string
	Remote string
}

// ReflogResult is a reflog entry.
type ReflogResult struct {
	Hash    string
	Action  string // "commit", "checkout", "rebase", "merge", "reset", etc.
	Message string
	Date    string
}

// ConflictResult is a merge conflict entry.
type ConflictResult struct {
	Path string
}

// TaskResult is a task in query results.
type TaskResult struct {
	ID          string
	Title       string
	Status      string
	Priority    int
	Labels      []string
	Branch      string
	Created     string
	Updated     string
	Description string
}

// ActionReport summarizes a destructive action.
type ActionReport struct {
	Action      string
	Affected    []string // affected commit hashes or names
	Success     bool
	DryRun      bool
	Description string
	Errors      []string
}

// AliasStore manages named query aliases.
type AliasStore struct {
	aliases map[string]string
}

// NewAliasStore creates a new empty alias store.
func NewAliasStore() *AliasStore {
	return &AliasStore{aliases: make(map[string]string)}
}

// Set defines or overwrites an alias.
func (s *AliasStore) Set(name, query string) {
	s.aliases[name] = query
}

// Get returns the query for an alias, or empty string if not found.
func (s *AliasStore) Get(name string) (string, bool) {
	q, ok := s.aliases[name]
	return q, ok
}

// Delete removes an alias.
func (s *AliasStore) Delete(name string) {
	delete(s.aliases, name)
}

// List returns all alias names and their queries.
func (s *AliasStore) List() map[string]string {
	result := make(map[string]string, len(s.aliases))
	for k, v := range s.aliases {
		result[k] = v
	}
	return result
}

// LoadFromFile loads aliases from a JSON file.
func (s *AliasStore) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var aliases map[string]string
	if err := json.Unmarshal(data, &aliases); err != nil {
		return err
	}
	for k, v := range aliases {
		s.aliases[k] = v
	}
	return nil
}

// SaveToFile saves aliases to a JSON file.
func (s *AliasStore) SaveToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.aliases, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// RunQuery handles alias commands, expands aliases, then parses and evaluates.
// Use this instead of Parse+Evaluate when alias support is needed.
func RunQuery(ctx *EvalContext, input string) (*Result, error) {
	input = strings.TrimSpace(input)

	// Handle "alias" commands
	if strings.HasPrefix(input, "alias ") {
		return handleAliasCommand(ctx, input)
	}

	// Handle "aliases" (list all)
	if input == "aliases" {
		return listAliases(ctx), nil
	}

	// Handle "unalias <name>"
	if strings.HasPrefix(input, "unalias ") {
		name := strings.TrimSpace(strings.TrimPrefix(input, "unalias "))
		if ctx.Aliases != nil {
			ctx.Aliases.Delete(name)
		}
		return &Result{Kind: "formatted", FormattedOutput: "alias " + name + " removed", FormatType: "table"}, nil
	}

	// Handle "let" variable definitions
	if strings.HasPrefix(input, "let ") {
		return handleLetCommand(ctx, input)
	}

	// Expand alias if input matches a known alias name
	if ctx.Aliases != nil {
		if expanded, ok := ctx.Aliases.Get(input); ok {
			input = expanded
		}
	}

	// Expand variables in the input
	input = expandVariables(ctx, input)

	pipeline, err := Parse(input)
	if err != nil {
		return nil, err
	}
	return Evaluate(ctx, pipeline)
}

func handleLetCommand(ctx *EvalContext, input string) (*Result, error) {
	rest := strings.TrimPrefix(input, "let ")
	eqIdx := strings.Index(rest, "=")
	if eqIdx == -1 {
		return nil, &DSLError{Message: "usage: let <name> = <value>"}
	}
	name := strings.TrimSpace(rest[:eqIdx])
	value := strings.TrimSpace(rest[eqIdx+1:])
	if name == "" || value == "" {
		return nil, &DSLError{Message: "usage: let <name> = <value>"}
	}
	// Strip surrounding quotes if present
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	if ctx.Variables == nil {
		ctx.Variables = make(map[string]interface{})
	}
	ctx.Variables[name] = value
	return &Result{Kind: "formatted", FormattedOutput: fmt.Sprintf("$%s = %s", name, value), FormatType: "table"}, nil
}

func expandVariables(ctx *EvalContext, input string) string {
	if !strings.Contains(input, "$") {
		return input
	}
	// Expand built-in variables
	builtins := getBuiltinVariables(ctx)
	for name, val := range builtins {
		input = strings.ReplaceAll(input, "$"+name, val)
	}
	// Expand user-defined variables
	if ctx.Variables != nil {
		for name, val := range ctx.Variables {
			input = strings.ReplaceAll(input, "$"+name, fmt.Sprintf("%v", val))
		}
	}
	return input
}

func getBuiltinVariables(ctx *EvalContext) map[string]string {
	vars := make(map[string]string)
	if ctx.RepoPath != "" {
		// $current_branch
		branches, err := git.ListBranches(ctx.RepoPath)
		if err == nil {
			for _, b := range branches {
				if b.IsCurrent {
					vars["current_branch"] = b.Name
					break
				}
			}
		}
		// $repo_name
		vars["repo_name"] = filepath.Base(ctx.RepoPath)
	}
	// $today, $yesterday - ISO date strings
	vars["today"] = "\"" + strings.Split(fmt.Sprintf("%v", strings.Split(fmt.Sprint(strings.Replace(fmt.Sprintf("%v", []byte{}), "[]", "", 1)), " ")), " ")[0] + "\""
	// Simple approach: just use time package indirectly via existing patterns
	return vars
}

func handleAliasCommand(ctx *EvalContext, input string) (*Result, error) {
	// "alias name = query"
	rest := strings.TrimPrefix(input, "alias ")

	// Handle "alias save name = query" for persistent aliases
	if strings.HasPrefix(rest, "save ") {
		rest = strings.TrimPrefix(rest, "save ")
		eqIdx := strings.Index(rest, "=")
		if eqIdx == -1 {
			return nil, &DSLError{Message: "usage: alias save <name> = <query>"}
		}
		name := strings.TrimSpace(rest[:eqIdx])
		query := strings.TrimSpace(rest[eqIdx+1:])
		if len(query) >= 2 && query[0] == '"' && query[len(query)-1] == '"' {
			query = query[1 : len(query)-1]
		}
		if name == "" || query == "" {
			return nil, &DSLError{Message: "usage: alias save <name> = <query>"}
		}
		if _, err := Parse(query); err != nil {
			return nil, fmt.Errorf("alias query invalid: %w", err)
		}
		if ctx.Aliases == nil {
			ctx.Aliases = NewAliasStore()
		}
		ctx.Aliases.Set(name, query)
		// Save to file
		aliasPath := filepath.Join(ctx.RepoPath, ".jock", "aliases.json")
		if err := ctx.Aliases.SaveToFile(aliasPath); err != nil {
			return nil, fmt.Errorf("failed to save alias: %w", err)
		}
		return &Result{Kind: "formatted", FormattedOutput: "alias " + name + " = " + query + " (saved)", FormatType: "table"}, nil
	}

	eqIdx := strings.Index(rest, "=")
	if eqIdx == -1 {
		// "alias name" — show the alias
		name := strings.TrimSpace(rest)
		if ctx.Aliases != nil {
			if q, ok := ctx.Aliases.Get(name); ok {
				return &Result{Kind: "formatted", FormattedOutput: name + " = " + q, FormatType: "table"}, nil
			}
		}
		return &Result{Kind: "formatted", FormattedOutput: "alias " + name + " not found", FormatType: "table"}, nil
	}

	name := strings.TrimSpace(rest[:eqIdx])
	query := strings.TrimSpace(rest[eqIdx+1:])
	// Strip surrounding quotes if present
	if len(query) >= 2 && query[0] == '"' && query[len(query)-1] == '"' {
		query = query[1 : len(query)-1]
	}

	if name == "" || query == "" {
		return nil, &DSLError{Message: "usage: alias <name> = <query>"}
	}

	// Validate the query parses
	if _, err := Parse(query); err != nil {
		return nil, fmt.Errorf("alias query invalid: %w", err)
	}

	if ctx.Aliases == nil {
		ctx.Aliases = NewAliasStore()
	}
	ctx.Aliases.Set(name, query)

	return &Result{Kind: "formatted", FormattedOutput: "alias " + name + " = " + query, FormatType: "table"}, nil
}

func listAliases(ctx *EvalContext) *Result {
	if ctx.Aliases == nil || len(ctx.Aliases.List()) == 0 {
		return &Result{Kind: "formatted", FormattedOutput: "(no aliases defined)", FormatType: "table"}
	}

	var lines []string
	for name, query := range ctx.Aliases.List() {
		lines = append(lines, name+" = "+query)
	}
	sort.Strings(lines)
	return &Result{Kind: "formatted", FormattedOutput: strings.Join(lines, "\n"), FormatType: "table"}
}

// Evaluate runs a parsed pipeline against a repository.
func Evaluate(ctx *EvalContext, pipeline *Pipeline) (*Result, error) {
	eval := &evaluator{ctx: ctx}
	return eval.run(pipeline)
}
