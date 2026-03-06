package dsl

import (
	"strings"

	"github.com/daearol/jockv2/backend/internal/git"
)

// Suggestion is an autocomplete suggestion for the DSL command bar.
type Suggestion struct {
	Text        string
	Kind        string // "keyword", "field", "branch", "author", "operator"
	Description string
}

var stageKeywords = []Suggestion{
	// Sources
	{Text: "commits", Kind: "keyword", Description: "list commits"},
	{Text: "branches", Kind: "keyword", Description: "list branches"},
	{Text: "tags", Kind: "keyword", Description: "list tags"},
	{Text: "files", Kind: "keyword", Description: "list changed files with stats"},
	{Text: "blame", Kind: "keyword", Description: "blame a file (blame \"path\")"},
	{Text: "stash", Kind: "keyword", Description: "list stash entries"},
	{Text: "status", Kind: "keyword", Description: "list working directory changes"},
	{Text: "remotes", Kind: "keyword", Description: "list git remotes"},
	{Text: "reflog", Kind: "keyword", Description: "list reflog entries"},
	{Text: "conflicts", Kind: "keyword", Description: "list merge conflicts"},
	{Text: "tasks", Kind: "keyword", Description: "list backlog tasks"},
	// Filters
	{Text: "where", Kind: "keyword", Description: "filter results"},
	{Text: "select", Kind: "keyword", Description: "choose fields to display"},
	{Text: "sort", Kind: "keyword", Description: "sort results"},
	{Text: "first", Kind: "keyword", Description: "take first N results"},
	{Text: "last", Kind: "keyword", Description: "take last N results"},
	{Text: "skip", Kind: "keyword", Description: "skip first N results"},
	{Text: "count", Kind: "keyword", Description: "count results"},
	{Text: "unique", Kind: "keyword", Description: "deduplicate results"},
	{Text: "reverse", Kind: "keyword", Description: "reverse order"},
	// Actions (read-only)
	{Text: "log", Kind: "keyword", Description: "display commits"},
	{Text: "diff", Kind: "keyword", Description: "show diffs"},
	{Text: "show", Kind: "keyword", Description: "show details"},
	// Actions (mutations)
	{Text: "cherry-pick", Kind: "keyword", Description: "cherry-pick commits onto branch"},
	{Text: "revert", Kind: "keyword", Description: "revert commits"},
	{Text: "rebase", Kind: "keyword", Description: "rebase commits onto branch"},
	{Text: "tag", Kind: "keyword", Description: "tag a commit"},
	{Text: "delete", Kind: "keyword", Description: "delete branches or tags"},
	{Text: "stage", Kind: "keyword", Description: "stage files"},
	{Text: "unstage", Kind: "keyword", Description: "unstage files"},
	{Text: "apply", Kind: "keyword", Description: "apply stash entries"},
	{Text: "pop", Kind: "keyword", Description: "pop stash entries"},
	{Text: "drop", Kind: "keyword", Description: "drop stash entries"},
	{Text: "resolve", Kind: "keyword", Description: "resolve conflicts (ours/theirs)"},
	{Text: "squash", Kind: "keyword", Description: "squash commits"},
	{Text: "reorder", Kind: "keyword", Description: "reorder commits"},
	// Standalone commands
	{Text: "branch", Kind: "keyword", Description: "branch operations (branch create, branch delete)"},
	{Text: "merge", Kind: "keyword", Description: "merge a branch"},
	{Text: "commit", Kind: "keyword", Description: "create a commit"},
	{Text: "push", Kind: "keyword", Description: "push to remote"},
	{Text: "pull", Kind: "keyword", Description: "pull from remote"},
	{Text: "abort", Kind: "keyword", Description: "abort merge/rebase/cherry-pick"},
	{Text: "continue", Kind: "keyword", Description: "continue rebase"},
	// Aggregation
	{Text: "group", Kind: "keyword", Description: "group results by field"},
	{Text: "sum", Kind: "keyword", Description: "sum a numeric field"},
	{Text: "avg", Kind: "keyword", Description: "average a numeric field"},
	{Text: "min", Kind: "keyword", Description: "minimum of a numeric field"},
	{Text: "max", Kind: "keyword", Description: "maximum of a numeric field"},
	{Text: "median", Kind: "keyword", Description: "median of a numeric field"},
	{Text: "stddev", Kind: "keyword", Description: "standard deviation of a numeric field"},
	{Text: "p90", Kind: "keyword", Description: "90th percentile of a numeric field"},
	{Text: "p95", Kind: "keyword", Description: "95th percentile of a numeric field"},
	{Text: "p99", Kind: "keyword", Description: "99th percentile of a numeric field"},
	{Text: "count_distinct", Kind: "keyword", Description: "count distinct values of a field"},
	{Text: "having", Kind: "keyword", Description: "filter after aggregation (having value > N)"},
	// Output
	{Text: "format", Kind: "keyword", Description: "format output (json, csv, table, markdown, yaml)"},
	{Text: "export", Kind: "keyword", Description: "export results to file"},
	{Text: "explain", Kind: "keyword", Description: "show query execution plan"},
	// Set operations
	{Text: "except", Kind: "keyword", Description: "set difference (remove commits in second source)"},
	{Text: "intersect", Kind: "keyword", Description: "set intersection (keep commits in both)"},
	{Text: "union", Kind: "keyword", Description: "set union (combine commits from both)"},
	// Aliases & variables
	{Text: "help", Kind: "keyword", Description: "show help (help, help where, help format, ...)"},
	{Text: "alias", Kind: "keyword", Description: "define a named query (alias name = query)"},
	{Text: "aliases", Kind: "keyword", Description: "list all defined aliases"},
	{Text: "unalias", Kind: "keyword", Description: "remove an alias (unalias name)"},
	{Text: "let", Kind: "keyword", Description: "define a variable (let name = value)"},
}

var numericFields = []Suggestion{
	{Text: "additions", Kind: "field", Description: "lines added"},
	{Text: "deletions", Kind: "field", Description: "lines deleted"},
}

var groupableFields = []Suggestion{
	{Text: "author", Kind: "field", Description: "commit author"},
	{Text: "branch", Kind: "field", Description: "branch refs"},
	{Text: "tag", Kind: "field", Description: "tag refs"},
	{Text: "date", Kind: "field", Description: "commit date"},
}

var fieldNames = []Suggestion{
	{Text: "author", Kind: "field", Description: "commit author"},
	{Text: "message", Kind: "field", Description: "commit message"},
	{Text: "date", Kind: "field", Description: "commit date"},
	{Text: "hash", Kind: "field", Description: "commit hash"},
	{Text: "branch", Kind: "field", Description: "branch refs"},
	{Text: "tag", Kind: "field", Description: "tag refs"},
	{Text: "files", Kind: "field", Description: "changed files"},
	{Text: "additions", Kind: "field", Description: "lines added"},
	{Text: "deletions", Kind: "field", Description: "lines deleted"},
}

var operators = []Suggestion{
	{Text: "==", Kind: "operator", Description: "equals"},
	{Text: "!=", Kind: "operator", Description: "not equals"},
	{Text: ">", Kind: "operator", Description: "greater than"},
	{Text: "<", Kind: "operator", Description: "less than"},
	{Text: ">=", Kind: "operator", Description: "greater or equal"},
	{Text: "<=", Kind: "operator", Description: "less or equal"},
	{Text: "contains", Kind: "operator", Description: "substring match"},
	{Text: "matches", Kind: "operator", Description: "regex match"},
}

var dateOperators = []Suggestion{
	{Text: "within", Kind: "keyword", Description: "within last N days/weeks/months"},
	{Text: "between", Kind: "keyword", Description: "between two dates"},
	{Text: "==", Kind: "operator", Description: "equals"},
	{Text: "!=", Kind: "operator", Description: "not equals"},
	{Text: ">", Kind: "operator", Description: "after date"},
	{Text: "<", Kind: "operator", Description: "before date"},
	{Text: ">=", Kind: "operator", Description: "on or after date"},
	{Text: "<=", Kind: "operator", Description: "on or before date"},
	{Text: "contains", Kind: "operator", Description: "substring match"},
	{Text: "matches", Kind: "operator", Description: "regex match"},
}

var timeUnits = []Suggestion{
	{Text: "hours", Kind: "keyword", Description: "hours"},
	{Text: "days", Kind: "keyword", Description: "days"},
	{Text: "weeks", Kind: "keyword", Description: "weeks"},
	{Text: "months", Kind: "keyword", Description: "months"},
	{Text: "years", Kind: "keyword", Description: "years"},
}

var logicalKeywords = []Suggestion{
	{Text: "and", Kind: "keyword", Description: "logical AND"},
	{Text: "or", Kind: "keyword", Description: "logical OR"},
	{Text: "not", Kind: "keyword", Description: "logical NOT"},
}

// AutoComplete provides context-sensitive suggestions for partial DSL queries.
func AutoComplete(repoPath, partial string, cursorPos int) []Suggestion {
	if cursorPos > len(partial) {
		cursorPos = len(partial)
	}
	input := partial[:cursorPos]
	input = strings.TrimSpace(input)

	// Empty input: suggest sources
	if input == "" {
		return stageKeywords[:4] // commits, branches, tags
	}

	// Tokenize what we have so far (ignore errors for incomplete input)
	lexer := NewLexer(input)
	tokens, _ := lexer.Tokenize()
	if len(tokens) == 0 {
		return stageKeywords[:4]
	}

	// Remove EOF token
	if len(tokens) > 0 && tokens[len(tokens)-1].Type == TokenEOF {
		tokens = tokens[:len(tokens)-1]
	}

	if len(tokens) == 0 {
		return stageKeywords[:4]
	}

	lastTok := tokens[len(tokens)-1]

	// After a pipe: suggest stage keywords
	if lastTok.Type == TokenPipe {
		return stageKeywords
	}

	// After "where": suggest field names
	if lastTok.Type == TokenIdent && lastTok.Literal == "where" {
		return fieldNames
	}

	// After a field name in a where clause: suggest operators
	if lastTok.Type == TokenIdent && isFieldName(lastTok.Literal) {
		// Check if preceded by "where" or logical operator
		if isPrecededByWhereContext(tokens) {
			if lastTok.Literal == "date" {
				return dateOperators
			}
			return operators
		}
	}

	// After "group": suggest "by"
	if lastTok.Type == TokenIdent && lastTok.Literal == "group" {
		return []Suggestion{{Text: "by", Kind: "keyword", Description: "group by field"}}
	}

	// After "group by" (last token is "by", preceded by "group"): suggest groupable fields
	if lastTok.Type == TokenIdent && lastTok.Literal == "by" {
		if len(tokens) >= 2 && tokens[len(tokens)-2].Type == TokenIdent && tokens[len(tokens)-2].Literal == "group" {
			return groupableFields
		}
	}

	// After aggregate functions: suggest numeric fields
	if lastTok.Type == TokenIdent && (lastTok.Literal == "sum" || lastTok.Literal == "avg" || lastTok.Literal == "min" || lastTok.Literal == "max" || lastTok.Literal == "median" || lastTok.Literal == "stddev" || lastTok.Literal == "p90" || lastTok.Literal == "p95" || lastTok.Literal == "p99" || lastTok.Literal == "count_distinct") {
		return numericFields
	}

	// After "except", "intersect", "union": suggest sources
	if lastTok.Type == TokenIdent && (lastTok.Literal == "except" || lastTok.Literal == "intersect" || lastTok.Literal == "union") {
		return stageKeywords[:4] // commits, branches, tags
	}

	// After "help": suggest help topics
	if lastTok.Type == TokenIdent && lastTok.Literal == "help" {
		return []Suggestion{
			{Text: "where", Kind: "keyword", Description: "filtering syntax"},
			{Text: "sort", Kind: "keyword", Description: "sorting syntax"},
			{Text: "group", Kind: "keyword", Description: "grouping and aggregates"},
			{Text: "format", Kind: "keyword", Description: "output formatting"},
			{Text: "date", Kind: "keyword", Description: "date filtering"},
			{Text: "set", Kind: "keyword", Description: "set operations (except, intersect, union)"},
			{Text: "select", Kind: "keyword", Description: "field projection"},
			{Text: "status", Kind: "keyword", Description: "working directory status"},
			{Text: "branches", Kind: "keyword", Description: "branch querying and mutations"},
			{Text: "remotes", Kind: "keyword", Description: "remote and remote branch queries"},
			{Text: "reflog", Kind: "keyword", Description: "reflog queries"},
			{Text: "conflicts", Kind: "keyword", Description: "merge conflict resolution"},
			{Text: "tasks", Kind: "keyword", Description: "backlog task management"},
			{Text: "actions", Kind: "keyword", Description: "mutation operations"},
			{Text: "variables", Kind: "keyword", Description: "user-defined and built-in variables"},
		}
	}

	// After "having": suggest value and group fields
	if lastTok.Type == TokenIdent && lastTok.Literal == "having" {
		return []Suggestion{
			{Text: "value", Kind: "field", Description: "aggregate value"},
			{Text: "group", Kind: "field", Description: "group key"},
		}
	}

	// After "blame": prompt for file path
	if lastTok.Type == TokenIdent && lastTok.Literal == "blame" {
		return []Suggestion{
			{Text: "\"", Kind: "keyword", Description: "file path (e.g. \"main.go\")"},
		}
	}

	// After "format": suggest format types
	if lastTok.Type == TokenIdent && lastTok.Literal == "format" {
		return []Suggestion{
			{Text: "json", Kind: "keyword", Description: "JSON output"},
			{Text: "csv", Kind: "keyword", Description: "CSV output"},
			{Text: "table", Kind: "keyword", Description: "ASCII table output"},
			{Text: "markdown", Kind: "keyword", Description: "Markdown table output"},
			{Text: "yaml", Kind: "keyword", Description: "YAML output"},
		}
	}

	// After "within": suggest "last"
	if lastTok.Type == TokenIdent && lastTok.Literal == "within" {
		return []Suggestion{{Text: "last", Kind: "keyword", Description: "last N units"}}
	}

	// After an integer following "last" in date context: suggest time units
	if lastTok.Type == TokenInteger && isInDateWithinContext(tokens) {
		return timeUnits
	}

	// After a comparison value: suggest logical operators or pipe
	if lastTok.Type == TokenString || lastTok.Type == TokenInteger {
		if isInWhereContext(tokens) {
			result := append([]Suggestion{{Text: "|", Kind: "operator", Description: "pipe to next stage"}}, logicalKeywords...)
			return result
		}
		return []Suggestion{{Text: "|", Kind: "operator", Description: "pipe to next stage"}}
	}

	// After "onto": suggest branch names from repo
	if lastTok.Type == TokenIdent && lastTok.Literal == "onto" {
		return getBranchSuggestions(repoPath)
	}

	// After "stash" keyword: suggest pipe or create
	if lastTok.Type == TokenIdent && lastTok.Literal == "stash" {
		return []Suggestion{
			{Text: "|", Kind: "operator", Description: "pipe to next stage"},
			{Text: "create", Kind: "keyword", Description: "create a new stash"},
		}
	}

	// After "branch" keyword: suggest create
	if lastTok.Type == TokenIdent && lastTok.Literal == "branch" {
		return []Suggestion{
			{Text: "create", Kind: "keyword", Description: "create a new branch"},
		}
	}

	// After "abort": suggest operation type
	if lastTok.Type == TokenIdent && lastTok.Literal == "abort" {
		return []Suggestion{
			{Text: "merge", Kind: "keyword", Description: "abort merge"},
			{Text: "rebase", Kind: "keyword", Description: "abort rebase"},
			{Text: "cherry-pick", Kind: "keyword", Description: "abort cherry-pick"},
			{Text: "revert", Kind: "keyword", Description: "abort revert"},
		}
	}

	// After "continue": suggest rebase
	if lastTok.Type == TokenIdent && lastTok.Literal == "continue" {
		return []Suggestion{
			{Text: "rebase", Kind: "keyword", Description: "continue rebase"},
		}
	}

	// After a source keyword without pipe: suggest pipe
	if lastTok.Type == TokenIdent && (lastTok.Literal == "commits" || lastTok.Literal == "branches" || lastTok.Literal == "tags" || lastTok.Literal == "files" || lastTok.Literal == "status" || lastTok.Literal == "remotes" || lastTok.Literal == "reflog" || lastTok.Literal == "conflicts" || lastTok.Literal == "tasks") {
		return []Suggestion{
			{Text: "|", Kind: "operator", Description: "pipe to next stage"},
			{Text: "[", Kind: "operator", Description: "specify range"},
		}
	}

	// After "sort": suggest field names
	if lastTok.Type == TokenIdent && lastTok.Literal == "sort" {
		return fieldNames
	}

	// After "select": suggest field names
	if lastTok.Type == TokenIdent && lastTok.Literal == "select" {
		return fieldNames
	}

	// Default: try prefix matching against all keywords
	if lastTok.Type == TokenIdent {
		return prefixMatch(lastTok.Literal, stageKeywords)
	}

	return nil
}

func isPrecededByWhereContext(tokens []Token) bool {
	for i := len(tokens) - 2; i >= 0; i-- {
		if tokens[i].Type == TokenIdent && tokens[i].Literal == "where" {
			return true
		}
		if tokens[i].Type == TokenIdent && (tokens[i].Literal == "and" || tokens[i].Literal == "or") {
			return true
		}
		if tokens[i].Type == TokenPipe {
			return false
		}
	}
	return false
}

func isInWhereContext(tokens []Token) bool {
	for i := len(tokens) - 1; i >= 0; i-- {
		if tokens[i].Type == TokenIdent && tokens[i].Literal == "where" {
			return true
		}
		if tokens[i].Type == TokenPipe {
			return false
		}
	}
	return false
}

func getBranchSuggestions(repoPath string) []Suggestion {
	if repoPath == "" {
		return nil
	}
	branches, err := git.ListBranches(repoPath)
	if err != nil {
		return nil
	}

	var suggestions []Suggestion
	for _, b := range branches {
		desc := "branch"
		if b.IsCurrent {
			desc = "current branch"
		}
		suggestions = append(suggestions, Suggestion{
			Text:        b.Name,
			Kind:        "branch",
			Description: desc,
		})
	}
	return suggestions
}

func isInDateWithinContext(tokens []Token) bool {
	// Look backwards for pattern: date within last <integer>
	for i := len(tokens) - 2; i >= 0; i-- {
		if tokens[i].Type == TokenIdent && tokens[i].Literal == "last" {
			if i-1 >= 0 && tokens[i-1].Type == TokenIdent && tokens[i-1].Literal == "within" {
				return true
			}
		}
		if tokens[i].Type == TokenPipe {
			return false
		}
	}
	return false
}

func prefixMatch(prefix string, candidates []Suggestion) []Suggestion {
	prefix = strings.ToLower(prefix)
	var matches []Suggestion
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c.Text), prefix) && strings.ToLower(c.Text) != prefix {
			matches = append(matches, c)
		}
	}
	return matches
}
