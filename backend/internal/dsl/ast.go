package dsl

// Pipeline is the root AST node: a sequence of stages separated by |.
type Pipeline struct {
	Stages []Stage
}

// Stage is one segment of the pipeline.
type Stage interface {
	stageNode()
}

// --- Source stages ---

// SourceStage: commits, branches, tags.
type SourceStage struct {
	Kind  string     // "commits", "branches", "tags"
	Range *RangeExpr // optional: commits[main], commits[abc..def], commits[last 10]
}

func (*SourceStage) stageNode() {}

// RangeExpr for commit ranges in source brackets.
type RangeExpr struct {
	From  string // start ref (hash, branch, tag) for ".." ranges
	To    string // end ref for ".." ranges
	LastN int    // if > 0, means "last N"
	Ref   string // single ref like a branch name
}

// --- Filter stages ---

// WhereStage: filtering with a condition expression.
type WhereStage struct {
	Condition Expr
}

func (*WhereStage) stageNode() {}

// --- Expression nodes (used in where conditions) ---

// Expr is the interface for condition expression nodes.
type Expr interface {
	exprNode()
}

// BinaryExpr: field == "value", a and b, a or b, etc.
type BinaryExpr struct {
	Left  Expr
	Op    string // "==", "!=", ">", "<", ">=", "<=", "contains", "matches", "and", "or", "+", "-", "*", "/"
	Right Expr
}

func (*BinaryExpr) exprNode() {}

// UnaryExpr: not condition.
type UnaryExpr struct {
	Op      string // "not"
	Operand Expr
}

func (*UnaryExpr) exprNode() {}

// FieldExpr: reference to a commit field.
type FieldExpr struct {
	Name string // "author", "message", "date", "hash", "branch", "tag", "files", "additions", "deletions", etc.
}

func (*FieldExpr) exprNode() {}

// StringLit is a quoted string value.
type StringLit struct {
	Value string
}

func (*StringLit) exprNode() {}

// IntLit is an integer value.
type IntLit struct {
	Value int
}

func (*IntLit) exprNode() {}

// FloatLit is a floating-point value.
type FloatLit struct {
	Value float64
}

func (*FloatLit) exprNode() {}

// DateWithinExpr: date within last N days/weeks/months/years/hours.
type DateWithinExpr struct {
	Field string // "date"
	N     int    // number of units
	Unit  string // "days", "hours", "weeks", "months", "years"
}

func (*DateWithinExpr) exprNode() {}

// DateBetweenExpr: date between "start" and "end".
type DateBetweenExpr struct {
	Field string // "date"
	Start string // start date string
	End   string // end date string
}

func (*DateBetweenExpr) exprNode() {}

// --- Phase 3: Computed fields & functions ---

// FuncCallExpr: upper(field), days_since(date), etc.
type FuncCallExpr struct {
	Name string // "upper", "lower", "trim", "len", "substr", "split", "replace", etc.
	Args []Expr
}

func (*FuncCallExpr) exprNode() {}

// AliasExpr: <expr> as <name> in select projections.
type AliasExpr struct {
	Expr  Expr
	Alias string
}

func (*AliasExpr) exprNode() {}

// VarRefExpr: $current_branch, $head, $user, $today, or user-defined $var.
type VarRefExpr struct {
	Name string // without $ prefix
}

func (*VarRefExpr) exprNode() {}

// InExpr: field in (subquery).
type InExpr struct {
	Field    Expr
	Subquery *Pipeline
}

func (*InExpr) exprNode() {}

// --- Transform stages ---

// SelectStage: field projection.
type SelectStage struct {
	Fields []string
	Exprs  []Expr // Phase 3: computed field expressions (may include AliasExpr, FuncCallExpr, BinaryExpr)
}

func (*SelectStage) stageNode() {}

// SortStage: sort by a field.
type SortStage struct {
	Field string
	Desc  bool
}

func (*SortStage) stageNode() {}

// UniqueStage: deduplicate results.
type UniqueStage struct{}

func (*UniqueStage) stageNode() {}

// ReverseStage: reverse result order.
type ReverseStage struct{}

func (*ReverseStage) stageNode() {}

// CountStage: count results.
type CountStage struct{}

func (*CountStage) stageNode() {}

// --- Aggregate stages ---

// GroupByStage: group by <field> or group by <func(field)>.
type GroupByStage struct {
	Field  string // "author", "branch", "tag", "date"
	Fields []string // Phase 5: multi-group support
	Func   string // Phase 5: temporal grouping function "week", "month", "year"
}

func (*GroupByStage) stageNode() {}

// AggregateStage: sum/avg/min/max <field>.
type AggregateStage struct {
	Func  string // "sum", "avg", "min", "max", "median", "p90", "p95", "p99", "count_distinct", "stddev"
	Field string // "additions", "deletions"
}

func (*AggregateStage) stageNode() {}

// --- Limit stages ---

// LimitStage: first N, last N, skip N.
type LimitStage struct {
	Kind  string // "first", "last", "skip"
	Count int
}

func (*LimitStage) stageNode() {}

// --- Format stages ---

// FormatStage: format json, format csv, format table, format markdown, format yaml.
type FormatStage struct {
	Format string // "json", "csv", "table", "markdown", "yaml"
}

func (*FormatStage) stageNode() {}

// --- Set operation stages ---

// SetOpStage: set operations (except, intersect, union) against another source.
type SetOpStage struct {
	Op     string       // "except", "intersect", "union"
	Source *SourceStage // the second source to compare against
}

func (*SetOpStage) stageNode() {}

// --- Help stage ---

// HelpStage: display help for a topic or general usage.
type HelpStage struct {
	Topic string // empty for general help, or a keyword like "where", "format", "sort"
}

func (*HelpStage) stageNode() {}

// --- Blame source ---

// BlameSourceStage: blame "file.go".
type BlameSourceStage struct {
	FilePath string
}

func (*BlameSourceStage) stageNode() {}

// --- Stash source ---

// StashSourceStage: stash.
type StashSourceStage struct{}

func (*StashSourceStage) stageNode() {}

// --- Having stage (post-aggregation filter) ---

// HavingStage: having value > 2, having group contains "foo".
type HavingStage struct {
	Condition Expr
}

func (*HavingStage) stageNode() {}

// --- Action stages ---

// ActionStage: terminal operations that read or mutate.
type ActionStage struct {
	Kind   string   // "cherry-pick", "revert", "rebase", "tag", "log", "diff", "show", "delete", "push", "apply", "pop", "drop", "stage", "unstage", "resolve", "squash", "edit", "reorder"
	Target string   // branch name for cherry-pick/rebase, tag name for tag, strategy for resolve
	Flags  []string // --force, --no-ff, --include-untracked, --set-upstream, --confirm, --amend
	Args   []string // additional arguments (e.g., reorder indices)
}

func (*ActionStage) stageNode() {}

// --- Phase 1-2: Standalone command stages ---

// BranchCreateStage: branch create "name" [from "ref"].
type BranchCreateStage struct {
	Name string
	From string // optional start point
}

func (*BranchCreateStage) stageNode() {}

// MergeStage: merge "branch" [--no-ff] [--dry-run].
type MergeStage struct {
	Branch string
	NoFF   bool
}

func (*MergeStage) stageNode() {}

// StashCreateStage: stash create "message" [--include-untracked].
type StashCreateStage struct {
	Message          string
	IncludeUntracked bool
}

func (*StashCreateStage) stageNode() {}

// CommitStage: commit "message" [--amend].
type CommitStage struct {
	Message string
	Amend   bool
}

func (*CommitStage) stageNode() {}

// PushStage: push ["remote"] ["branch"] [--force] [--set-upstream].
type PushStage struct {
	Remote      string
	Branch      string
	Force       bool
	SetUpstream bool
}

func (*PushStage) stageNode() {}

// PullStage: pull ["remote"] ["branch"].
type PullStage struct {
	Remote string
	Branch string
}

func (*PullStage) stageNode() {}

// AbortStage: abort merge|rebase|cherry-pick|revert.
type AbortStage struct {
	Operation string // "merge", "rebase", "cherry-pick", "revert"
}

func (*AbortStage) stageNode() {}

// ContinueStage: continue rebase.
type ContinueStage struct {
	Operation string // "rebase"
}

func (*ContinueStage) stageNode() {}

// --- Phase 2: New source stages ---

// StatusSourceStage: status.
type StatusSourceStage struct{}

func (*StatusSourceStage) stageNode() {}

// RemotesSourceStage: remotes.
type RemotesSourceStage struct{}

func (*RemotesSourceStage) stageNode() {}

// RemoteBranchesSourceStage: remote-branches.
type RemoteBranchesSourceStage struct {
	Remote string // optional filter by remote
}

func (*RemoteBranchesSourceStage) stageNode() {}

// ReflogSourceStage: reflog.
type ReflogSourceStage struct {
	Limit int // optional limit
}

func (*ReflogSourceStage) stageNode() {}

// ConflictsSourceStage: conflicts.
type ConflictsSourceStage struct{}

func (*ConflictsSourceStage) stageNode() {}

// --- Phase 3: Variable definition ---

// LetStage: let name = expr.
type LetStage struct {
	Name string
	Expr Expr
}

func (*LetStage) stageNode() {}

// --- Phase 4: Export stage ---

// ExportStage: export "path".
type ExportStage struct {
	Path string
}

func (*ExportStage) stageNode() {}

// CopyStage: copy to clipboard.
type CopyStage struct{}

func (*CopyStage) stageNode() {}

// ExplainStage: explain <pipeline>.
type ExplainStage struct {
	Inner *Pipeline
}

func (*ExplainStage) stageNode() {}

// --- Phase 5: Window function stage ---

// WindowStage: running_sum, moving_avg, rank, etc.
type WindowStage struct {
	Func   string // "running_sum", "running_avg", "moving_avg", "rank", "dense_rank", "row_number", "lag", "lead", "cumulative_sum"
	Field  string // field to compute over
	Window int    // window size for moving_avg
	Alias  string // output field name
}

func (*WindowStage) stageNode() {}

// InteractiveRebaseStage: rebase interactive "base".
type InteractiveRebaseStage struct {
	Base string // base commit/branch for interactive rebase
}

func (*InteractiveRebaseStage) stageNode() {}

// --- Tasks source ---

// TasksSourceStage: tasks.
type TasksSourceStage struct {
	StatusFilter string // optional: "backlog", "in-progress", "done"
}

func (*TasksSourceStage) stageNode() {}
