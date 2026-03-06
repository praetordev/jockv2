package dsl

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/daearol/jockv2/backend/internal/git"
)

// resultSet is the intermediate data flowing between pipeline stages.
type resultSet struct {
	kind           string // "commits", "branches", "files", "blame", "stashes", "count", "aggregate", "status", "remotes", "remote_branches", "reflog", "conflicts", "tasks"
	commits        []CommitResult
	branches       []BranchResult
	files          []FileResult
	blameLines     []BlameLineResult
	stashes        []StashResult
	statusEntries  []StatusResult
	remotes        []RemoteResult
	remoteBranches []RemoteBranchResult
	reflogEntries  []ReflogResult
	conflicts      []ConflictResult
	tasks          []TaskResult
	selectedFields []string // if set, only these fields are projected in output

	// Grouping / aggregation state
	groupField string                   // set by GroupByStage
	groups     map[string][]CommitResult // populated by GroupByStage
	aggregates []AggregateRow           // populated by aggregate evaluation
	aggFunc    string                   // "count", "sum", "avg", "min", "max", "median", etc.
	aggField   string                   // "additions", "deletions", ""

	// Format state
	formattedOutput string // populated by FormatStage
	formatType      string // "json", "csv", "table", "markdown", "yaml"
}

type evaluator struct {
	ctx       *EvalContext
	needStats bool // true if pipeline references additions, deletions, or files
}

// pipelineNeedsStats walks the pipeline AST to check if any stage
// references additions, deletions, or files fields.
func pipelineNeedsStats(pipeline *Pipeline) bool {
	for _, stage := range pipeline.Stages {
		switch s := stage.(type) {
		case *WhereStage:
			if exprRefsStats(s.Condition) {
				return true
			}
		case *SortStage:
			if s.Field == "additions" || s.Field == "deletions" || s.Field == "files" {
				return true
			}
		case *SelectStage:
			for _, f := range s.Fields {
				if f == "additions" || f == "deletions" || f == "files" {
					return true
				}
			}
		case *AggregateStage:
			if s.Field == "additions" || s.Field == "deletions" {
				return true
			}
		}
	}
	return false
}

func exprRefsStats(expr Expr) bool {
	switch ex := expr.(type) {
	case *FieldExpr:
		return ex.Name == "additions" || ex.Name == "deletions" || ex.Name == "files"
	case *BinaryExpr:
		return exprRefsStats(ex.Left) || exprRefsStats(ex.Right)
	case *UnaryExpr:
		return exprRefsStats(ex.Operand)
	}
	return false
}

func (e *evaluator) run(pipeline *Pipeline) (*Result, error) {
	if len(pipeline.Stages) == 0 {
		return nil, &DSLError{Message: "empty pipeline"}
	}

	e.needStats = pipelineNeedsStats(pipeline)

	var rs *resultSet

	for _, stage := range pipeline.Stages {
		var err error
		rs, err = e.evalStage(rs, stage)
		if err != nil {
			return nil, err
		}
		// Check for terminal results (count, action)
		if rs == nil {
			return nil, fmt.Errorf("internal: nil result set after stage")
		}
	}

	// Implicit count for standalone "group by" without an aggregate
	if rs.groups != nil && rs.kind != "aggregate" {
		rs, _ = e.evalCount(rs)
	}

	return e.toResult(rs), nil
}

func (e *evaluator) evalStage(rs *resultSet, stage Stage) (*resultSet, error) {
	switch s := stage.(type) {
	case *SourceStage:
		return e.evalSource(s)
	case *BlameSourceStage:
		return e.evalBlameSource(s)
	case *StashSourceStage:
		return e.evalStashSource()
	case *StatusSourceStage:
		return e.evalStatusSource()
	case *RemotesSourceStage:
		return e.evalRemotesSource()
	case *RemoteBranchesSourceStage:
		return e.evalRemoteBranchesSource(s)
	case *ReflogSourceStage:
		return e.evalReflogSource(s)
	case *ConflictsSourceStage:
		return e.evalConflictsSource()
	case *TasksSourceStage:
		return e.evalTasksSource(s)
	case *WhereStage:
		if rs == nil {
			return nil, &DSLError{Message: "'where' requires a source stage (e.g., commits | where ...)"}
		}
		return e.evalWhere(rs, s)
	case *SelectStage:
		if rs == nil {
			return nil, &DSLError{Message: "'select' requires a source stage"}
		}
		rs.selectedFields = s.Fields
		return rs, nil
	case *SortStage:
		if rs == nil {
			return nil, &DSLError{Message: "'sort' requires a source stage"}
		}
		return e.evalSort(rs, s)
	case *LimitStage:
		if rs == nil {
			return nil, &DSLError{Message: fmt.Sprintf("'%s' requires a source stage", s.Kind)}
		}
		return e.evalLimit(rs, s)
	case *UniqueStage:
		if rs == nil {
			return nil, &DSLError{Message: "'unique' requires a source stage"}
		}
		return e.evalUnique(rs)
	case *ReverseStage:
		if rs == nil {
			return nil, &DSLError{Message: "'reverse' requires a source stage"}
		}
		return e.evalReverse(rs)
	case *CountStage:
		if rs == nil {
			return nil, &DSLError{Message: "'count' requires a source stage"}
		}
		return e.evalCount(rs)
	case *GroupByStage:
		if rs == nil {
			return nil, &DSLError{Message: "'group by' requires a source stage"}
		}
		return e.evalGroupBy(rs, s)
	case *AggregateStage:
		if rs == nil {
			return nil, &DSLError{Message: fmt.Sprintf("'%s' requires a source stage", s.Func)}
		}
		return e.evalAggregate(rs, s)
	case *FormatStage:
		if rs == nil {
			return nil, &DSLError{Message: "'format' requires a source stage"}
		}
		return e.evalFormat(rs, s)
	case *SetOpStage:
		if rs == nil {
			return nil, &DSLError{Message: fmt.Sprintf("'%s' requires a source stage", s.Op)}
		}
		return e.evalSetOp(rs, s)
	case *HavingStage:
		if rs == nil || rs.kind != "aggregate" {
			return nil, &DSLError{Message: "'having' requires an aggregation stage before it"}
		}
		return e.evalHaving(rs, s)
	case *HelpStage:
		return e.evalHelp(s)
	case *ActionStage:
		return e.evalAction(rs, s)

	// Phase 1: Standalone commands
	case *BranchCreateStage:
		return e.evalBranchCreate(s)
	case *MergeStage:
		return e.evalMerge(s)
	case *StashCreateStage:
		return e.evalStashCreate(s)
	case *CommitStage:
		return e.evalCommit(s)
	case *PushStage:
		return e.evalPush(s)
	case *PullStage:
		return e.evalPull(s)
	case *AbortStage:
		return e.evalAbort(s)
	case *ContinueStage:
		return e.evalContinue(s)

	// Phase 4: Export, copy, explain
	case *ExportStage:
		if rs == nil {
			return nil, &DSLError{Message: "'export' requires a source stage"}
		}
		return e.evalExport(rs, s)
	case *CopyStage:
		if rs == nil {
			return nil, &DSLError{Message: "'copy' requires a source stage"}
		}
		return rs, nil // copy is handled at the UI layer
	case *ExplainStage:
		return e.evalExplain(s)

	// Phase 5: Window functions
	case *WindowStage:
		if rs == nil {
			return nil, &DSLError{Message: "window functions require a source stage"}
		}
		return e.evalWindow(rs, s)

	default:
		return nil, fmt.Errorf("internal: unknown stage type %T", stage)
	}
}

func (e *evaluator) evalSource(s *SourceStage) (*resultSet, error) {
	switch s.Kind {
	case "commits":
		return e.evalCommitSource(s)
	case "branches":
		return e.evalBranchSource()
	case "tags":
		return e.evalTagSource()
	case "files":
		return e.evalFileSource(s)
	default:
		return nil, &DSLError{Message: fmt.Sprintf("unknown source %q", s.Kind)}
	}
}

func (e *evaluator) evalCommitSource(s *SourceStage) (*resultSet, error) {
	limit := 500 // sensible default
	skip := 0
	branch := ""

	if s.Range != nil {
		if s.Range.LastN > 0 {
			limit = s.Range.LastN
		}
		if s.Range.Ref != "" {
			branch = s.Range.Ref
		}
		if s.Range.From != "" && s.Range.To != "" {
			// For "from..to" ranges, pass as branch arg to git log
			branch = s.Range.From + ".." + s.Range.To
		}
	}

	raw, err := e.fetchCommits(limit, skip, branch)
	if err != nil {
		return nil, fmt.Errorf("git: %w", err)
	}

	commits := make([]CommitResult, len(raw))
	for i, rc := range raw {
		branches, tags := git.ParseRefs(rc.Refs)
		cr := CommitResult{
			Hash:     rc.Hash,
			Message:  rc.Message,
			Author:   rc.Author,
			Date:     rc.Date,
			DateISO:  rc.DateISO,
			Branches: branches,
			Tags:     tags,
			Parents:  rc.Parents,
		}
		if e.needStats {
			for _, f := range rc.Files {
				cr.Additions += f.Additions
				cr.Deletions += f.Deletions
				cr.Files = append(cr.Files, f.Path)
			}
		}
		commits[i] = cr
	}

	return &resultSet{kind: "commits", commits: commits}, nil
}

func (e *evaluator) evalBranchSource() (*resultSet, error) {
	infos, err := e.fetchBranches()
	if err != nil {
		return nil, fmt.Errorf("git: %w", err)
	}

	branches := make([]BranchResult, len(infos))
	for i, bi := range infos {
		branches[i] = BranchResult{
			Name:      bi.Name,
			IsCurrent: bi.IsCurrent,
			Remote:    bi.Remote,
		}
	}

	return &resultSet{kind: "branches", branches: branches}, nil
}

func (e *evaluator) evalTagSource() (*resultSet, error) {
	// Fetch all commits and extract those with tags
	raw, err := e.fetchCommits(500, 0, "")
	if err != nil {
		return nil, fmt.Errorf("git: %w", err)
	}

	var commits []CommitResult
	for _, rc := range raw {
		_, tags := git.ParseRefs(rc.Refs)
		if len(tags) > 0 {
			cr := CommitResult{
				Hash:    rc.Hash,
				Message: rc.Message,
				Author:  rc.Author,
				Date:    rc.Date,
				DateISO: rc.DateISO,
				Tags:    tags,
				Parents: rc.Parents,
			}
			if e.needStats {
				for _, f := range rc.Files {
					cr.Additions += f.Additions
					cr.Deletions += f.Deletions
					cr.Files = append(cr.Files, f.Path)
				}
			}
			commits = append(commits, cr)
		}
	}

	return &resultSet{kind: "commits", commits: commits}, nil
}

func (e *evaluator) evalFileSource(s *SourceStage) (*resultSet, error) {
	// files always needs stats
	e.needStats = true

	limit := 500
	branch := ""
	if s.Range != nil {
		if s.Range.LastN > 0 {
			limit = s.Range.LastN
		}
		if s.Range.Ref != "" {
			branch = s.Range.Ref
		}
		if s.Range.From != "" && s.Range.To != "" {
			branch = s.Range.From + ".." + s.Range.To
		}
	}

	raw, err := e.fetchCommits(limit, 0, branch)
	if err != nil {
		return nil, fmt.Errorf("git: %w", err)
	}

	// Aggregate per-file stats across all commits
	type fileStat struct {
		additions int
		deletions int
		commits   int
	}
	fileMap := make(map[string]*fileStat)

	for _, rc := range raw {
		for _, f := range rc.Files {
			fs, ok := fileMap[f.Path]
			if !ok {
				fs = &fileStat{}
				fileMap[f.Path] = fs
			}
			fs.additions += f.Additions
			fs.deletions += f.Deletions
			fs.commits++
		}
	}

	files := make([]FileResult, 0, len(fileMap))
	for path, fs := range fileMap {
		files = append(files, FileResult{
			Path:      path,
			Additions: fs.additions,
			Deletions: fs.deletions,
			Commits:   fs.commits,
		})
	}

	// Sort by path by default for deterministic output
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return &resultSet{kind: "files", files: files}, nil
}

// fetchCommits uses cache if available, otherwise calls git directly.
func (e *evaluator) fetchCommits(limit, skip int, branch string) ([]git.RawCommit, error) {
	if e.ctx.Cache != nil {
		return e.ctx.Cache.GetCommits(e.ctx.RepoPath, limit, skip, branch, e.needStats)
	}
	if e.needStats {
		return git.ListRawCommitsWithStats(e.ctx.RepoPath, limit, skip, branch, nil)
	}
	return git.ListRawCommits(e.ctx.RepoPath, limit, skip, branch, nil)
}

// fetchBranches uses cache if available, otherwise calls git directly.
func (e *evaluator) fetchBranches() ([]git.BranchInfo, error) {
	if e.ctx.Cache != nil {
		return e.ctx.Cache.GetBranches(e.ctx.RepoPath)
	}
	return git.ListBranches(e.ctx.RepoPath)
}

func (e *evaluator) evalBlameSource(s *BlameSourceStage) (*resultSet, error) {
	lines, err := git.Blame(e.ctx.RepoPath, s.FilePath)
	if err != nil {
		return nil, fmt.Errorf("blame: %w", err)
	}

	result := make([]BlameLineResult, len(lines))
	for i, l := range lines {
		result[i] = BlameLineResult{
			Hash:    l.Hash,
			Author:  l.Author,
			Date:    l.Date,
			LineNo:  l.LineNo,
			Content: l.Content,
		}
	}
	return &resultSet{kind: "blame", blameLines: result}, nil
}

func (e *evaluator) evalStashSource() (*resultSet, error) {
	entries, err := git.ListStashes(e.ctx.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("stash: %w", err)
	}

	result := make([]StashResult, len(entries))
	for i, s := range entries {
		result[i] = StashResult{
			Index:   s.Index,
			Message: s.Message,
			Branch:  s.Branch,
			Date:    s.Date,
		}
	}
	return &resultSet{kind: "stashes", stashes: result}, nil
}

func (e *evaluator) evalHaving(rs *resultSet, s *HavingStage) (*resultSet, error) {
	var filtered []AggregateRow
	for _, row := range rs.aggregates {
		match, err := e.evalHavingCondition(s.Condition, &row)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, row)
		}
	}
	rs.aggregates = filtered
	return rs, nil
}

func (e *evaluator) evalHavingCondition(expr Expr, row *AggregateRow) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalHavingCondition(ex.Left, row)
			if err != nil {
				return false, err
			}
			if !left {
				return false, nil
			}
			return e.evalHavingCondition(ex.Right, row)
		}
		if ex.Op == "or" {
			left, err := e.evalHavingCondition(ex.Left, row)
			if err != nil {
				return false, err
			}
			if left {
				return true, nil
			}
			return e.evalHavingCondition(ex.Right, row)
		}

		// Field comparison
		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("having: expected field on left side")
		}

		var fieldVal interface{}
		switch field.Name {
		case "value":
			fieldVal = row.Value
		case "group":
			fieldVal = row.Group
		default:
			return false, fmt.Errorf("having: unknown field %q, expected 'value' or 'group'", field.Name)
		}

		switch ex.Op {
		case "==":
			return havingCompare(fieldVal, ex.Right, func(a, b float64) bool { return a == b }, func(a, b string) bool { return a == b })
		case "!=":
			return havingCompare(fieldVal, ex.Right, func(a, b float64) bool { return a != b }, func(a, b string) bool { return a != b })
		case ">":
			return havingCompare(fieldVal, ex.Right, func(a, b float64) bool { return a > b }, nil)
		case "<":
			return havingCompare(fieldVal, ex.Right, func(a, b float64) bool { return a < b }, nil)
		case ">=":
			return havingCompare(fieldVal, ex.Right, func(a, b float64) bool { return a >= b }, nil)
		case "<=":
			return havingCompare(fieldVal, ex.Right, func(a, b float64) bool { return a <= b }, nil)
		case "contains":
			if s, ok := fieldVal.(string); ok {
				if lit, ok := ex.Right.(*StringLit); ok {
					return strings.Contains(strings.ToLower(s), strings.ToLower(lit.Value)), nil
				}
			}
			return false, nil
		default:
			return false, fmt.Errorf("having: unsupported operator %q", ex.Op)
		}
	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalHavingCondition(ex.Operand, row)
			return !val, err
		}
	}
	return false, fmt.Errorf("having: unexpected expression type %T", expr)
}

func havingCompare(fieldVal interface{}, right Expr, numCmp func(float64, float64) bool, strCmp func(string, string) bool) (bool, error) {
	switch fv := fieldVal.(type) {
	case float64:
		switch rv := right.(type) {
		case *IntLit:
			return numCmp(fv, float64(rv.Value)), nil
		case *StringLit:
			return false, nil
		}
	case string:
		if strCmp == nil {
			return false, nil
		}
		if rv, ok := right.(*StringLit); ok {
			return strCmp(fv, rv.Value), nil
		}
	}
	return false, nil
}

func (e *evaluator) evalWhere(rs *resultSet, s *WhereStage) (*resultSet, error) {
	if rs.kind == "files" {
		return e.evalWhereFiles(rs, s)
	}
	if rs.kind == "blame" {
		return e.evalWhereBlame(rs, s)
	}
	if rs.kind == "stashes" {
		return e.evalWhereStash(rs, s)
	}
	if rs.kind == "branches" {
		return e.evalWhereBranches(rs, s)
	}
	if rs.kind == "status" {
		return e.evalWhereStatus(rs, s)
	}
	if rs.kind == "reflog" {
		return e.evalWhereReflog(rs, s)
	}
	if rs.kind == "remote_branches" {
		return e.evalWhereRemoteBranches(rs, s)
	}
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "'where' is not supported for result kind: " + rs.kind}
	}

	var filtered []CommitResult
	for _, c := range rs.commits {
		match, err := e.evalCondition(s.Condition, &c)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, c)
		}
	}

	return &resultSet{kind: "commits", commits: filtered}, nil
}

func (e *evaluator) evalCondition(expr Expr, c *CommitResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		return e.evalBinaryExpr(ex, c)
	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalCondition(ex.Operand, c)
			return !val, err
		}
		return false, fmt.Errorf("unknown unary op %q", ex.Op)
	case *DateWithinExpr:
		return e.evalDateWithin(ex, c)
	case *DateBetweenExpr:
		return e.evalDateBetween(ex, c)
	default:
		return false, fmt.Errorf("unexpected expression type %T in condition", expr)
	}
}

func (e *evaluator) evalBinaryExpr(ex *BinaryExpr, c *CommitResult) (bool, error) {
	// Logical operators
	if ex.Op == "and" {
		left, err := e.evalCondition(ex.Left, c)
		if err != nil {
			return false, err
		}
		if !left {
			return false, nil // short-circuit
		}
		return e.evalCondition(ex.Right, c)
	}
	if ex.Op == "or" {
		left, err := e.evalCondition(ex.Left, c)
		if err != nil {
			return false, err
		}
		if left {
			return true, nil // short-circuit
		}
		return e.evalCondition(ex.Right, c)
	}

	// Field comparisons
	field, ok := ex.Left.(*FieldExpr)
	if !ok {
		return false, fmt.Errorf("expected field on left side of comparison")
	}

	fieldVal := e.getFieldString(field.Name, c)

	switch ex.Op {
	case "contains":
		val, ok := ex.Right.(*StringLit)
		if !ok {
			return false, fmt.Errorf("'contains' requires a string value")
		}
		return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(val.Value)), nil

	case "matches":
		val, ok := ex.Right.(*StringLit)
		if !ok {
			return false, fmt.Errorf("'matches' requires a string pattern")
		}
		re, err := regexp.Compile(val.Value)
		if err != nil {
			return false, fmt.Errorf("invalid regex %q: %w", val.Value, err)
		}
		return re.MatchString(fieldVal), nil

	case "==":
		val := e.exprToString(ex.Right)
		return strings.EqualFold(fieldVal, val), nil

	case "!=":
		val := e.exprToString(ex.Right)
		return !strings.EqualFold(fieldVal, val), nil

	case ">", "<", ">=", "<=":
		if field.Name == "date" {
			return e.evalDateComparison(c, ex.Op, ex.Right)
		}
		return e.evalNumericComparison(field.Name, c, ex.Op, ex.Right)
	}

	return false, fmt.Errorf("unknown operator %q", ex.Op)
}

func (e *evaluator) getFieldString(name string, c *CommitResult) string {
	switch name {
	case "author":
		return c.Author
	case "message":
		return c.Message
	case "date":
		return c.Date
	case "hash":
		return c.Hash
	case "branch":
		return strings.Join(c.Branches, ",")
	case "tag":
		return strings.Join(c.Tags, ",")
	case "files":
		return strings.Join(c.Files, ",")
	default:
		return ""
	}
}

func (e *evaluator) getFieldNumeric(name string, c *CommitResult) (int, bool) {
	switch name {
	case "additions":
		return c.Additions, true
	case "deletions":
		return c.Deletions, true
	default:
		return 0, false
	}
}

func (e *evaluator) evalNumericComparison(fieldName string, c *CommitResult, op string, right Expr) (bool, error) {
	fieldNum, isNumeric := e.getFieldNumeric(fieldName, c)
	if !isNumeric {
		return false, fmt.Errorf("field %q does not support numeric comparison", fieldName)
	}

	rightInt, ok := right.(*IntLit)
	if !ok {
		return false, fmt.Errorf("numeric comparison requires an integer value")
	}

	switch op {
	case ">":
		return fieldNum > rightInt.Value, nil
	case "<":
		return fieldNum < rightInt.Value, nil
	case ">=":
		return fieldNum >= rightInt.Value, nil
	case "<=":
		return fieldNum <= rightInt.Value, nil
	default:
		return false, fmt.Errorf("unknown numeric operator %q", op)
	}
}

func (e *evaluator) exprToString(expr Expr) string {
	switch v := expr.(type) {
	case *StringLit:
		return v.Value
	case *IntLit:
		return fmt.Sprintf("%d", v.Value)
	default:
		return ""
	}
}

func (e *evaluator) evalSort(rs *resultSet, s *SortStage) (*resultSet, error) {
	if rs.kind == "aggregate" {
		return e.evalSortAggregate(rs, s)
	}
	if rs.kind == "files" {
		return e.evalSortFiles(rs, s)
	}
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "'sort' currently only supports commits and files"}
	}

	sorted := make([]CommitResult, len(rs.commits))
	copy(sorted, rs.commits)

	sort.SliceStable(sorted, func(i, j int) bool {
		a := e.sortKey(s.Field, &sorted[i])
		b := e.sortKey(s.Field, &sorted[j])
		if s.Desc {
			return a > b
		}
		return a < b
	})

	return &resultSet{kind: "commits", commits: sorted}, nil
}

func (e *evaluator) evalSortAggregate(rs *resultSet, s *SortStage) (*resultSet, error) {
	sorted := make([]AggregateRow, len(rs.aggregates))
	copy(sorted, rs.aggregates)

	sort.SliceStable(sorted, func(i, j int) bool {
		switch s.Field {
		case "value":
			if s.Desc {
				return sorted[i].Value > sorted[j].Value
			}
			return sorted[i].Value < sorted[j].Value
		default: // "group" or any other
			if s.Desc {
				return sorted[i].Group > sorted[j].Group
			}
			return sorted[i].Group < sorted[j].Group
		}
	})

	rs.aggregates = sorted
	return rs, nil
}

func (e *evaluator) evalSortFiles(rs *resultSet, s *SortStage) (*resultSet, error) {
	sorted := make([]FileResult, len(rs.files))
	copy(sorted, rs.files)

	sort.SliceStable(sorted, func(i, j int) bool {
		a := e.fileSortKey(s.Field, &sorted[i])
		b := e.fileSortKey(s.Field, &sorted[j])
		if s.Desc {
			return a > b
		}
		return a < b
	})

	return &resultSet{kind: "files", files: sorted}, nil
}

func (e *evaluator) fileSortKey(field string, f *FileResult) string {
	switch field {
	case "path", "files":
		return strings.ToLower(f.Path)
	case "additions":
		return fmt.Sprintf("%010d", f.Additions)
	case "deletions":
		return fmt.Sprintf("%010d", f.Deletions)
	case "commits":
		return fmt.Sprintf("%010d", f.Commits)
	default:
		return ""
	}
}

func (e *evaluator) sortKey(field string, c *CommitResult) string {
	switch field {
	case "author":
		return strings.ToLower(c.Author)
	case "message":
		return strings.ToLower(c.Message)
	case "date":
		return c.DateISO // ISO 8601 sorts lexicographically correct
	case "hash":
		return c.Hash
	case "additions":
		return fmt.Sprintf("%010d", c.Additions)
	case "deletions":
		return fmt.Sprintf("%010d", c.Deletions)
	default:
		return ""
	}
}

func (e *evaluator) evalLimit(rs *resultSet, s *LimitStage) (*resultSet, error) {
	if rs.kind == "aggregate" {
		rows := rs.aggregates
		switch s.Kind {
		case "first":
			if s.Count < len(rows) {
				rows = rows[:s.Count]
			}
		case "last":
			if s.Count < len(rows) {
				rows = rows[len(rows)-s.Count:]
			}
		case "skip":
			if s.Count < len(rows) {
				rows = rows[s.Count:]
			} else {
				rows = nil
			}
		}
		rs.aggregates = rows
		return rs, nil
	}

	if rs.kind == "files" {
		files := rs.files
		switch s.Kind {
		case "first":
			if s.Count < len(files) {
				files = files[:s.Count]
			}
		case "last":
			if s.Count < len(files) {
				files = files[len(files)-s.Count:]
			}
		case "skip":
			if s.Count < len(files) {
				files = files[s.Count:]
			} else {
				files = nil
			}
		}
		return &resultSet{kind: "files", files: files}, nil
	}

	if rs.kind == "blame" {
		lines := rs.blameLines
		switch s.Kind {
		case "first":
			if s.Count < len(lines) {
				lines = lines[:s.Count]
			}
		case "last":
			if s.Count < len(lines) {
				lines = lines[len(lines)-s.Count:]
			}
		case "skip":
			if s.Count < len(lines) {
				lines = lines[s.Count:]
			} else {
				lines = nil
			}
		}
		return &resultSet{kind: "blame", blameLines: lines}, nil
	}

	if rs.kind == "stashes" {
		stashes := rs.stashes
		switch s.Kind {
		case "first":
			if s.Count < len(stashes) {
				stashes = stashes[:s.Count]
			}
		case "last":
			if s.Count < len(stashes) {
				stashes = stashes[len(stashes)-s.Count:]
			}
		case "skip":
			if s.Count < len(stashes) {
				stashes = stashes[s.Count:]
			} else {
				stashes = nil
			}
		}
		return &resultSet{kind: "stashes", stashes: stashes}, nil
	}

	if rs.kind == "branches" {
		items := rs.branches
		switch s.Kind {
		case "first":
			if s.Count < len(items) {
				items = items[:s.Count]
			}
		case "last":
			if s.Count < len(items) {
				items = items[len(items)-s.Count:]
			}
		case "skip":
			if s.Count < len(items) {
				items = items[s.Count:]
			} else {
				items = nil
			}
		}
		return &resultSet{kind: "branches", branches: items}, nil
	}

	if rs.kind == "status" {
		items := rs.statusEntries
		switch s.Kind {
		case "first":
			if s.Count < len(items) {
				items = items[:s.Count]
			}
		case "last":
			if s.Count < len(items) {
				items = items[len(items)-s.Count:]
			}
		case "skip":
			if s.Count < len(items) {
				items = items[s.Count:]
			} else {
				items = nil
			}
		}
		return &resultSet{kind: "status", statusEntries: items}, nil
	}

	if rs.kind == "remotes" {
		items := rs.remotes
		switch s.Kind {
		case "first":
			if s.Count < len(items) {
				items = items[:s.Count]
			}
		case "last":
			if s.Count < len(items) {
				items = items[len(items)-s.Count:]
			}
		case "skip":
			if s.Count < len(items) {
				items = items[s.Count:]
			} else {
				items = nil
			}
		}
		return &resultSet{kind: "remotes", remotes: items}, nil
	}

	if rs.kind == "remote_branches" {
		items := rs.remoteBranches
		switch s.Kind {
		case "first":
			if s.Count < len(items) {
				items = items[:s.Count]
			}
		case "last":
			if s.Count < len(items) {
				items = items[len(items)-s.Count:]
			}
		case "skip":
			if s.Count < len(items) {
				items = items[s.Count:]
			} else {
				items = nil
			}
		}
		return &resultSet{kind: "remote_branches", remoteBranches: items}, nil
	}

	if rs.kind == "reflog" {
		items := rs.reflogEntries
		switch s.Kind {
		case "first":
			if s.Count < len(items) {
				items = items[:s.Count]
			}
		case "last":
			if s.Count < len(items) {
				items = items[len(items)-s.Count:]
			}
		case "skip":
			if s.Count < len(items) {
				items = items[s.Count:]
			} else {
				items = nil
			}
		}
		return &resultSet{kind: "reflog", reflogEntries: items}, nil
	}

	if rs.kind == "conflicts" {
		items := rs.conflicts
		switch s.Kind {
		case "first":
			if s.Count < len(items) {
				items = items[:s.Count]
			}
		case "last":
			if s.Count < len(items) {
				items = items[len(items)-s.Count:]
			}
		case "skip":
			if s.Count < len(items) {
				items = items[s.Count:]
			} else {
				items = nil
			}
		}
		return &resultSet{kind: "conflicts", conflicts: items}, nil
	}

	if rs.kind != "commits" {
		return nil, &DSLError{Message: fmt.Sprintf("'%s' is not supported for result kind: %s", s.Kind, rs.kind)}
	}

	commits := rs.commits
	switch s.Kind {
	case "first":
		if s.Count < len(commits) {
			commits = commits[:s.Count]
		}
	case "last":
		if s.Count < len(commits) {
			commits = commits[len(commits)-s.Count:]
		}
	case "skip":
		if s.Count < len(commits) {
			commits = commits[s.Count:]
		} else {
			commits = nil
		}
	}

	return &resultSet{kind: "commits", commits: commits}, nil
}

func (e *evaluator) evalUnique(rs *resultSet) (*resultSet, error) {
	if rs.kind != "commits" {
		return rs, nil
	}

	seen := make(map[string]bool)
	var unique []CommitResult
	for _, c := range rs.commits {
		if !seen[c.Hash] {
			seen[c.Hash] = true
			unique = append(unique, c)
		}
	}

	return &resultSet{kind: "commits", commits: unique}, nil
}

func (e *evaluator) evalReverse(rs *resultSet) (*resultSet, error) {
	if rs.kind == "aggregate" {
		reversed := make([]AggregateRow, len(rs.aggregates))
		for i, r := range rs.aggregates {
			reversed[len(rs.aggregates)-1-i] = r
		}
		rs.aggregates = reversed
		return rs, nil
	}

	if rs.kind == "blame" {
		reversed := make([]BlameLineResult, len(rs.blameLines))
		for i, l := range rs.blameLines {
			reversed[len(rs.blameLines)-1-i] = l
		}
		return &resultSet{kind: "blame", blameLines: reversed}, nil
	}

	if rs.kind == "stashes" {
		reversed := make([]StashResult, len(rs.stashes))
		for i, s := range rs.stashes {
			reversed[len(rs.stashes)-1-i] = s
		}
		return &resultSet{kind: "stashes", stashes: reversed}, nil
	}

	if rs.kind == "branches" {
		reversed := make([]BranchResult, len(rs.branches))
		for i, b := range rs.branches {
			reversed[len(rs.branches)-1-i] = b
		}
		return &resultSet{kind: "branches", branches: reversed}, nil
	}

	if rs.kind == "status" {
		reversed := make([]StatusResult, len(rs.statusEntries))
		for i, s := range rs.statusEntries {
			reversed[len(rs.statusEntries)-1-i] = s
		}
		return &resultSet{kind: "status", statusEntries: reversed}, nil
	}

	if rs.kind == "reflog" {
		reversed := make([]ReflogResult, len(rs.reflogEntries))
		for i, r := range rs.reflogEntries {
			reversed[len(rs.reflogEntries)-1-i] = r
		}
		return &resultSet{kind: "reflog", reflogEntries: reversed}, nil
	}

	if rs.kind == "remote_branches" {
		reversed := make([]RemoteBranchResult, len(rs.remoteBranches))
		for i, r := range rs.remoteBranches {
			reversed[len(rs.remoteBranches)-1-i] = r
		}
		return &resultSet{kind: "remote_branches", remoteBranches: reversed}, nil
	}

	if rs.kind != "commits" {
		return rs, nil
	}

	reversed := make([]CommitResult, len(rs.commits))
	for i, c := range rs.commits {
		reversed[len(rs.commits)-1-i] = c
	}

	return &resultSet{kind: "commits", commits: reversed}, nil
}

func (e *evaluator) evalCount(rs *resultSet) (*resultSet, error) {
	// If groups exist, produce per-group counts
	if rs.groups != nil {
		keys := sortedGroupKeys(rs.groups)
		rows := make([]AggregateRow, len(keys))
		for i, key := range keys {
			rows[i] = AggregateRow{Group: key, Value: float64(len(rs.groups[key]))}
		}
		return &resultSet{
			kind:       "aggregate",
			aggregates: rows,
			aggFunc:    "count",
			groupField: rs.groupField,
		}, nil
	}

	// Scalar count (original behavior)
	count := 0
	switch rs.kind {
	case "commits":
		count = len(rs.commits)
	case "branches":
		count = len(rs.branches)
	case "files":
		count = len(rs.files)
	case "blame":
		count = len(rs.blameLines)
	case "stashes":
		count = len(rs.stashes)
	case "status":
		count = len(rs.statusEntries)
	case "remotes":
		count = len(rs.remotes)
	case "remote_branches":
		count = len(rs.remoteBranches)
	case "reflog":
		count = len(rs.reflogEntries)
	case "conflicts":
		count = len(rs.conflicts)
	}

	return &resultSet{
		kind: "count",
		commits: []CommitResult{{
			Hash:    fmt.Sprintf("%d", count),
			Message: fmt.Sprintf("count: %d", count),
		}},
	}, nil
}

func (e *evaluator) evalAction(rs *resultSet, s *ActionStage) (*resultSet, error) {
	// Read-only actions
	switch s.Kind {
	case "log", "diff", "show":
		if rs == nil {
			return nil, &DSLError{Message: fmt.Sprintf("'%s' requires a source stage", s.Kind)}
		}
		return rs, nil // these are display hints, the result is the current set
	}

	if rs == nil {
		return nil, &DSLError{Message: fmt.Sprintf("'%s' requires a source stage", s.Kind)}
	}

	// Actions that work on non-commit result types
	switch s.Kind {
	case "delete":
		if rs.kind == "branches" {
			report, err := executeBranchDelete(e.ctx, rs, s.Flags)
			if err != nil {
				return nil, err
			}
			return actionReportResult(report), nil
		}
		if rs.kind == "commits" {
			// tags | where ... | delete — tags come through as commits
			report, err := executeTagDelete(e.ctx, rs)
			if err != nil {
				return nil, err
			}
			return actionReportResult(report), nil
		}
		return nil, &DSLError{Message: "'delete' requires branch or tag results"}

	case "push":
		if rs.kind == "commits" {
			// tags | where ... | push
			report, err := executeTagPush(e.ctx, rs)
			if err != nil {
				return nil, err
			}
			return actionReportResult(report), nil
		}
		return nil, &DSLError{Message: "'push' on results requires tag results"}

	case "apply":
		report, err := executeStashAction(e.ctx, rs, "apply")
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil

	case "pop":
		report, err := executeStashAction(e.ctx, rs, "pop")
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil

	case "drop":
		report, err := executeStashAction(e.ctx, rs, "drop")
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil

	case "stage":
		report, err := executeStage(e.ctx, rs)
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil

	case "unstage":
		report, err := executeUnstage(e.ctx, rs)
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil

	case "resolve":
		strategy := s.Target // target holds the strategy (ours/theirs)
		report, err := executeResolve(e.ctx, rs, strategy)
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil

	case "squash":
		report, err := executeSquash(e.ctx, rs, s.Target)
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil

	case "reorder":
		report, err := executeReorder(e.ctx, rs, s.Args)
		if err != nil {
			return nil, err
		}
		return actionReportResult(report), nil
	}

	// Original commit-based destructive actions
	if rs.kind != "commits" {
		return nil, &DSLError{Message: fmt.Sprintf("'%s' requires commits", s.Kind)}
	}

	report, err := executeAction(e.ctx, rs, s)
	if err != nil {
		return nil, err
	}

	return &resultSet{
		kind: "action_report",
		commits: []CommitResult{{
			Hash:    report.Action,
			Message: report.Description,
		}},
	}, nil
}

func (e *evaluator) toResult(rs *resultSet) *Result {
	switch rs.kind {
	case "commits":
		commits := rs.commits
		if len(rs.selectedFields) > 0 {
			commits = projectCommits(commits, rs.selectedFields)
		}
		return &Result{Kind: "commits", Commits: commits}
	case "branches":
		return &Result{Kind: "branches", Branches: rs.branches}
	case "files":
		return &Result{Kind: "files", Files: rs.files}
	case "blame":
		return &Result{Kind: "blame", BlameLines: rs.blameLines}
	case "stashes":
		return &Result{Kind: "stashes", Stashes: rs.stashes}
	case "status":
		return &Result{Kind: "status", StatusEntries: rs.statusEntries}
	case "remotes":
		return &Result{Kind: "remotes", Remotes: rs.remotes}
	case "remote_branches":
		return &Result{Kind: "remote_branches", RemoteBranches: rs.remoteBranches}
	case "reflog":
		return &Result{Kind: "reflog", ReflogEntries: rs.reflogEntries}
	case "conflicts":
		return &Result{Kind: "conflicts", Conflicts: rs.conflicts}
	case "tasks":
		return &Result{Kind: "tasks", Tasks: rs.tasks}
	case "count":
		count := 0
		if len(rs.commits) > 0 {
			fmt.Sscanf(rs.commits[0].Hash, "%d", &count)
		}
		return &Result{Kind: "count", Count: count}
	case "aggregate":
		return &Result{
			Kind:       "aggregate",
			Aggregates: rs.aggregates,
			AggFunc:    rs.aggFunc,
			AggField:   rs.aggField,
			GroupField: rs.groupField,
		}
	case "formatted":
		return &Result{
			Kind:            "formatted",
			FormattedOutput: rs.formattedOutput,
			FormatType:      rs.formatType,
		}
	case "action_report":
		// The report was already stored during evalAction
		return &Result{Kind: "commits", Commits: rs.commits}
	default:
		return &Result{Kind: "commits", Commits: rs.commits}
	}
}

// projectCommits zeros out fields not in the selected set.
func projectCommits(commits []CommitResult, fields []string) []CommitResult {
	allowed := make(map[string]bool, len(fields))
	for _, f := range fields {
		allowed[f] = true
	}

	projected := make([]CommitResult, len(commits))
	for i, c := range commits {
		var p CommitResult
		if allowed["hash"] {
			p.Hash = c.Hash
		}
		if allowed["message"] {
			p.Message = c.Message
		}
		if allowed["author"] {
			p.Author = c.Author
		}
		if allowed["date"] {
			p.Date = c.Date
			p.DateISO = c.DateISO
		}
		if allowed["branch"] {
			p.Branches = c.Branches
		}
		if allowed["tag"] {
			p.Tags = c.Tags
		}
		if allowed["additions"] {
			p.Additions = c.Additions
		}
		if allowed["deletions"] {
			p.Deletions = c.Deletions
		}
		if allowed["files"] {
			p.Files = c.Files
		}
		projected[i] = p
	}
	return projected
}

// --- Aggregate evaluation ---

func (e *evaluator) evalGroupBy(rs *resultSet, s *GroupByStage) (*resultSet, error) {
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "'group by' requires commits"}
	}

	groups := make(map[string][]CommitResult)
	for _, c := range rs.commits {
		key := e.getFieldString(s.Field, &c)
		groups[key] = append(groups[key], c)
	}

	return &resultSet{
		kind:       "commits",
		commits:    rs.commits,
		groupField: s.Field,
		groups:     groups,
	}, nil
}

func (e *evaluator) evalAggregate(rs *resultSet, s *AggregateStage) (*resultSet, error) {
	if rs.kind != "commits" {
		return nil, &DSLError{Message: fmt.Sprintf("'%s' requires commits", s.Func)}
	}

	// Check if this is an advanced aggregate function
	isAdvanced := false
	switch s.Func {
	case "median", "p90", "p95", "p99", "stddev", "count_distinct":
		isAdvanced = true
	}

	if rs.groups != nil {
		// Grouped aggregation
		keys := sortedGroupKeys(rs.groups)
		rows := make([]AggregateRow, len(keys))
		for i, key := range keys {
			if isAdvanced {
				rows[i] = AggregateRow{Group: key, Value: computeAdvancedAggregate(s.Func, s.Field, rs.groups[key])}
			} else {
				rows[i] = AggregateRow{Group: key, Value: computeAggregate(s.Func, s.Field, rs.groups[key])}
			}
		}
		return &resultSet{
			kind:       "aggregate",
			aggregates: rows,
			aggFunc:    s.Func,
			aggField:   s.Field,
			groupField: rs.groupField,
		}, nil
	}

	// Ungrouped scalar
	var val float64
	if isAdvanced {
		val = computeAdvancedAggregate(s.Func, s.Field, rs.commits)
	} else {
		val = computeAggregate(s.Func, s.Field, rs.commits)
	}
	return &resultSet{
		kind:       "aggregate",
		aggregates: []AggregateRow{{Value: val}},
		aggFunc:    s.Func,
		aggField:   s.Field,
	}, nil
}

func computeAggregate(fn, field string, commits []CommitResult) float64 {
	if len(commits) == 0 {
		return 0
	}

	e := &evaluator{}
	vals := make([]int, len(commits))
	for i := range commits {
		v, _ := e.getFieldNumeric(field, &commits[i])
		vals[i] = v
	}

	switch fn {
	case "sum":
		total := 0
		for _, v := range vals {
			total += v
		}
		return float64(total)
	case "avg":
		total := 0
		for _, v := range vals {
			total += v
		}
		return float64(total) / float64(len(vals))
	case "min":
		m := vals[0]
		for _, v := range vals[1:] {
			if v < m {
				m = v
			}
		}
		return float64(m)
	case "max":
		m := vals[0]
		for _, v := range vals[1:] {
			if v > m {
				m = v
			}
		}
		return float64(m)
	}
	return 0
}

func sortedGroupKeys(groups map[string][]CommitResult) []string {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- Format evaluation ---

func (e *evaluator) evalFormat(rs *resultSet, s *FormatStage) (*resultSet, error) {
	// Convert to Result first to get the finalized data
	result := e.toResult(rs)

	var output string
	var err error

	switch s.Format {
	case "json":
		output, err = formatJSON(result)
	case "csv":
		output, err = formatCSV(result)
	case "table":
		output, err = formatTable(result)
	case "markdown":
		output, err = formatMarkdown(result)
	case "yaml":
		output, err = formatYAML(result)
	default:
		return nil, fmt.Errorf("unknown format %q", s.Format)
	}
	if err != nil {
		return nil, err
	}

	return &resultSet{
		kind:            "formatted",
		formattedOutput: output,
		formatType:      s.Format,
	}, nil
}

func formatJSON(r *Result) (string, error) {
	var data any

	switch r.Kind {
	case "commits":
		type jsonCommit struct {
			Hash      string   `json:"hash"`
			Message   string   `json:"message,omitempty"`
			Author    string   `json:"author,omitempty"`
			Date      string   `json:"date,omitempty"`
			Branches  []string `json:"branches,omitempty"`
			Tags      []string `json:"tags,omitempty"`
			Additions int      `json:"additions,omitempty"`
			Deletions int      `json:"deletions,omitempty"`
			Files     []string `json:"files,omitempty"`
		}
		commits := make([]jsonCommit, len(r.Commits))
		for i, c := range r.Commits {
			commits[i] = jsonCommit{
				Hash:      c.Hash,
				Message:   c.Message,
				Author:    c.Author,
				Date:      c.Date,
				Branches:  c.Branches,
				Tags:      c.Tags,
				Additions: c.Additions,
				Deletions: c.Deletions,
				Files:     c.Files,
			}
		}
		data = commits

	case "branches":
		type jsonBranch struct {
			Name      string `json:"name"`
			IsCurrent bool   `json:"isCurrent"`
			Remote    string `json:"remote,omitempty"`
		}
		branches := make([]jsonBranch, len(r.Branches))
		for i, b := range r.Branches {
			branches[i] = jsonBranch{
				Name:      b.Name,
				IsCurrent: b.IsCurrent,
				Remote:    b.Remote,
			}
		}
		data = branches

	case "files":
		type jsonFile struct {
			Path      string `json:"path"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
			Commits   int    `json:"commits"`
		}
		files := make([]jsonFile, len(r.Files))
		for i, f := range r.Files {
			files[i] = jsonFile{
				Path:      f.Path,
				Additions: f.Additions,
				Deletions: f.Deletions,
				Commits:   f.Commits,
			}
		}
		data = files

	case "count":
		data = map[string]int{"count": r.Count}

	case "aggregate":
		if len(r.Aggregates) == 1 && r.Aggregates[0].Group == "" {
			data = map[string]float64{"value": r.Aggregates[0].Value}
		} else {
			type jsonAgg struct {
				Group string  `json:"group"`
				Value float64 `json:"value"`
			}
			rows := make([]jsonAgg, len(r.Aggregates))
			for i, a := range r.Aggregates {
				rows[i] = jsonAgg{Group: a.Group, Value: a.Value}
			}
			data = rows
		}

	default:
		data = map[string]string{"result": "unknown"}
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("json marshal: %w", err)
	}
	return string(b), nil
}

func formatCSV(r *Result) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	switch r.Kind {
	case "commits":
		// Determine which fields have data (respect select projection)
		w.Write([]string{"hash", "author", "date", "message"})
		for _, c := range r.Commits {
			w.Write([]string{c.Hash, c.Author, c.Date, c.Message})
		}

	case "files":
		w.Write([]string{"path", "additions", "deletions", "commits"})
		for _, f := range r.Files {
			w.Write([]string{f.Path, fmt.Sprintf("%d", f.Additions), fmt.Sprintf("%d", f.Deletions), fmt.Sprintf("%d", f.Commits)})
		}

	case "branches":
		w.Write([]string{"name", "is_current", "remote"})
		for _, b := range r.Branches {
			current := "false"
			if b.IsCurrent {
				current = "true"
			}
			w.Write([]string{b.Name, current, b.Remote})
		}

	case "count":
		w.Write([]string{"count"})
		w.Write([]string{fmt.Sprintf("%d", r.Count)})

	case "aggregate":
		if len(r.Aggregates) == 1 && r.Aggregates[0].Group == "" {
			w.Write([]string{"value"})
			w.Write([]string{formatAggValue(r.AggFunc, r.Aggregates[0].Value)})
		} else {
			w.Write([]string{"group", "value"})
			for _, a := range r.Aggregates {
				w.Write([]string{a.Group, formatAggValue(r.AggFunc, a.Value)})
			}
		}
	}

	w.Flush()
	return strings.TrimRight(buf.String(), "\n"), w.Error()
}

func formatTable(r *Result) (string, error) {
	var headers []string
	var rows [][]string

	switch r.Kind {
	case "commits":
		headers = []string{"HASH", "AUTHOR", "DATE", "MESSAGE"}
		for _, c := range r.Commits {
			rows = append(rows, []string{c.Hash, c.Author, c.Date, c.Message})
		}

	case "files":
		headers = []string{"PATH", "ADDITIONS", "DELETIONS", "COMMITS"}
		for _, f := range r.Files {
			rows = append(rows, []string{f.Path, fmt.Sprintf("%d", f.Additions), fmt.Sprintf("%d", f.Deletions), fmt.Sprintf("%d", f.Commits)})
		}

	case "branches":
		headers = []string{"NAME", "CURRENT", "REMOTE"}
		for _, b := range r.Branches {
			current := " "
			if b.IsCurrent {
				current = "*"
			}
			rows = append(rows, []string{b.Name, current, b.Remote})
		}

	case "count":
		headers = []string{"COUNT"}
		rows = [][]string{{fmt.Sprintf("%d", r.Count)}}

	case "aggregate":
		if len(r.Aggregates) == 1 && r.Aggregates[0].Group == "" {
			headers = []string{"VALUE"}
			rows = [][]string{{formatAggValue(r.AggFunc, r.Aggregates[0].Value)}}
		} else {
			headers = []string{"GROUP", "VALUE"}
			for _, a := range r.Aggregates {
				rows = append(rows, []string{a.Group, formatAggValue(r.AggFunc, a.Value)})
			}
		}
	}

	if len(headers) == 0 {
		return "", nil
	}

	// Compute column widths
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var buf strings.Builder

	// Header row
	for i, h := range headers {
		if i > 0 {
			buf.WriteString(" | ")
		}
		buf.WriteString(padRight(h, widths[i]))
	}
	buf.WriteString("\n")

	// Separator row
	for i, w := range widths {
		if i > 0 {
			buf.WriteString("-+-")
		}
		buf.WriteString(strings.Repeat("-", w))
	}
	buf.WriteString("\n")

	// Data rows
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				buf.WriteString(" | ")
			}
			if i < len(widths) {
				buf.WriteString(padRight(cell, widths[i]))
			} else {
				buf.WriteString(cell)
			}
		}
		buf.WriteString("\n")
	}

	return strings.TrimRight(buf.String(), "\n"), nil
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatAggValue(aggFunc string, v float64) string {
	if aggFunc == "avg" {
		return fmt.Sprintf("%.1f", v)
	}
	return fmt.Sprintf("%.0f", v)
}

// --- File evaluation helpers ---

func (e *evaluator) evalWhereFiles(rs *resultSet, s *WhereStage) (*resultSet, error) {
	var filtered []FileResult
	for _, f := range rs.files {
		match, err := e.evalFileCondition(s.Condition, &f)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, f)
		}
	}
	return &resultSet{kind: "files", files: filtered}, nil
}

func (e *evaluator) evalFileCondition(expr Expr, f *FileResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalFileCondition(ex.Left, f)
			if err != nil || !left {
				return left, err
			}
			return e.evalFileCondition(ex.Right, f)
		}
		if ex.Op == "or" {
			left, err := e.evalFileCondition(ex.Left, f)
			if err != nil || left {
				return left, err
			}
			return e.evalFileCondition(ex.Right, f)
		}

		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("expected field on left side")
		}

		fieldVal := e.getFileFieldString(field.Name, f)

		switch ex.Op {
		case "contains":
			val, ok := ex.Right.(*StringLit)
			if !ok {
				return false, fmt.Errorf("'contains' requires a string value")
			}
			return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(val.Value)), nil
		case "matches":
			val, ok := ex.Right.(*StringLit)
			if !ok {
				return false, fmt.Errorf("'matches' requires a string pattern")
			}
			re, err := regexp.Compile(val.Value)
			if err != nil {
				return false, fmt.Errorf("invalid regex %q: %w", val.Value, err)
			}
			return re.MatchString(fieldVal), nil
		case "==":
			val := e.exprToString(ex.Right)
			return strings.EqualFold(fieldVal, val), nil
		case "!=":
			val := e.exprToString(ex.Right)
			return !strings.EqualFold(fieldVal, val), nil
		case ">", "<", ">=", "<=":
			return e.evalFileNumericComparison(field.Name, f, ex.Op, ex.Right)
		}
		return false, fmt.Errorf("unknown operator %q", ex.Op)

	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalFileCondition(ex.Operand, f)
			return !val, err
		}
		return false, fmt.Errorf("unknown unary op %q", ex.Op)
	default:
		return false, fmt.Errorf("unexpected expression type %T in file condition", expr)
	}
}

func (e *evaluator) getFileFieldString(name string, f *FileResult) string {
	switch name {
	case "path", "files":
		return f.Path
	default:
		return ""
	}
}

func (e *evaluator) getFileFieldNumeric(name string, f *FileResult) (int, bool) {
	switch name {
	case "additions":
		return f.Additions, true
	case "deletions":
		return f.Deletions, true
	case "commits":
		return f.Commits, true
	default:
		return 0, false
	}
}

func (e *evaluator) evalFileNumericComparison(fieldName string, f *FileResult, op string, right Expr) (bool, error) {
	num, ok := e.getFileFieldNumeric(fieldName, f)
	if !ok {
		return false, fmt.Errorf("field %q does not support numeric comparison on files", fieldName)
	}
	rightInt, ok := right.(*IntLit)
	if !ok {
		return false, fmt.Errorf("numeric comparison requires an integer value")
	}
	switch op {
	case ">":
		return num > rightInt.Value, nil
	case "<":
		return num < rightInt.Value, nil
	case ">=":
		return num >= rightInt.Value, nil
	case "<=":
		return num <= rightInt.Value, nil
	}
	return false, fmt.Errorf("unknown numeric operator %q", op)
}

// --- Blame where filtering ---

func (e *evaluator) evalWhereBlame(rs *resultSet, s *WhereStage) (*resultSet, error) {
	var filtered []BlameLineResult
	for _, line := range rs.blameLines {
		match, err := e.evalBlameCondition(s.Condition, &line)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, line)
		}
	}
	return &resultSet{kind: "blame", blameLines: filtered}, nil
}

func (e *evaluator) evalBlameCondition(expr Expr, line *BlameLineResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalBlameCondition(ex.Left, line)
			if err != nil || !left {
				return left, err
			}
			return e.evalBlameCondition(ex.Right, line)
		}
		if ex.Op == "or" {
			left, err := e.evalBlameCondition(ex.Left, line)
			if err != nil || left {
				return left, err
			}
			return e.evalBlameCondition(ex.Right, line)
		}

		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("expected field on left side")
		}

		fieldVal := e.getBlameFieldString(field.Name, line)

		switch ex.Op {
		case "contains":
			val, ok := ex.Right.(*StringLit)
			if !ok {
				return false, fmt.Errorf("'contains' requires a string value")
			}
			return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(val.Value)), nil
		case "matches":
			val, ok := ex.Right.(*StringLit)
			if !ok {
				return false, fmt.Errorf("'matches' requires a string pattern")
			}
			re, err := regexp.Compile(val.Value)
			if err != nil {
				return false, fmt.Errorf("invalid regex %q: %w", val.Value, err)
			}
			return re.MatchString(fieldVal), nil
		case "==":
			val := e.exprToString(ex.Right)
			return strings.EqualFold(fieldVal, val), nil
		case "!=":
			val := e.exprToString(ex.Right)
			return !strings.EqualFold(fieldVal, val), nil
		case ">", "<", ">=", "<=":
			if field.Name == "lineno" {
				rightInt, ok := ex.Right.(*IntLit)
				if !ok {
					return false, fmt.Errorf("numeric comparison requires an integer value")
				}
				switch ex.Op {
				case ">":
					return line.LineNo > rightInt.Value, nil
				case "<":
					return line.LineNo < rightInt.Value, nil
				case ">=":
					return line.LineNo >= rightInt.Value, nil
				case "<=":
					return line.LineNo <= rightInt.Value, nil
				}
			}
			return false, fmt.Errorf("field %q does not support numeric comparison in blame", field.Name)
		}
		return false, fmt.Errorf("unknown operator %q", ex.Op)

	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalBlameCondition(ex.Operand, line)
			return !val, err
		}
		return false, fmt.Errorf("unknown unary op %q", ex.Op)
	default:
		return false, fmt.Errorf("unexpected expression type %T in blame condition", expr)
	}
}

func (e *evaluator) getBlameFieldString(name string, line *BlameLineResult) string {
	switch name {
	case "author":
		return line.Author
	case "hash":
		return line.Hash
	case "content", "message":
		return line.Content
	case "date":
		return line.Date
	case "lineno":
		return fmt.Sprintf("%d", line.LineNo)
	default:
		return ""
	}
}

// --- Stash where filtering ---

func (e *evaluator) evalWhereStash(rs *resultSet, s *WhereStage) (*resultSet, error) {
	var filtered []StashResult
	for _, stash := range rs.stashes {
		match, err := e.evalStashCondition(s.Condition, &stash)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, stash)
		}
	}
	return &resultSet{kind: "stashes", stashes: filtered}, nil
}

func (e *evaluator) evalStashCondition(expr Expr, stash *StashResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalStashCondition(ex.Left, stash)
			if err != nil || !left {
				return left, err
			}
			return e.evalStashCondition(ex.Right, stash)
		}
		if ex.Op == "or" {
			left, err := e.evalStashCondition(ex.Left, stash)
			if err != nil || left {
				return left, err
			}
			return e.evalStashCondition(ex.Right, stash)
		}

		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("expected field on left side")
		}

		fieldVal := e.getStashFieldString(field.Name, stash)

		switch ex.Op {
		case "contains":
			val, ok := ex.Right.(*StringLit)
			if !ok {
				return false, fmt.Errorf("'contains' requires a string value")
			}
			return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(val.Value)), nil
		case "matches":
			val, ok := ex.Right.(*StringLit)
			if !ok {
				return false, fmt.Errorf("'matches' requires a string pattern")
			}
			re, err := regexp.Compile(val.Value)
			if err != nil {
				return false, fmt.Errorf("invalid regex %q: %w", val.Value, err)
			}
			return re.MatchString(fieldVal), nil
		case "==":
			val := e.exprToString(ex.Right)
			return strings.EqualFold(fieldVal, val), nil
		case "!=":
			val := e.exprToString(ex.Right)
			return !strings.EqualFold(fieldVal, val), nil
		case ">", "<", ">=", "<=":
			if field.Name == "index" {
				rightInt, ok := ex.Right.(*IntLit)
				if !ok {
					return false, fmt.Errorf("numeric comparison requires an integer value")
				}
				switch ex.Op {
				case ">":
					return stash.Index > rightInt.Value, nil
				case "<":
					return stash.Index < rightInt.Value, nil
				case ">=":
					return stash.Index >= rightInt.Value, nil
				case "<=":
					return stash.Index <= rightInt.Value, nil
				}
			}
			return false, fmt.Errorf("field %q does not support numeric comparison in stash", field.Name)
		}
		return false, fmt.Errorf("unknown operator %q", ex.Op)

	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalStashCondition(ex.Operand, stash)
			return !val, err
		}
		return false, fmt.Errorf("unknown unary op %q", ex.Op)
	default:
		return false, fmt.Errorf("unexpected expression type %T in stash condition", expr)
	}
}

func (e *evaluator) getStashFieldString(name string, stash *StashResult) string {
	switch name {
	case "message":
		return stash.Message
	case "branch":
		return stash.Branch
	case "date":
		return stash.Date
	case "index":
		return fmt.Sprintf("%d", stash.Index)
	default:
		return ""
	}
}

// --- Set operation evaluation ---

func (e *evaluator) evalSetOp(rs *resultSet, s *SetOpStage) (*resultSet, error) {
	if rs.kind != "commits" {
		return nil, &DSLError{Message: fmt.Sprintf("'%s' currently only supports commits", s.Op)}
	}

	// Evaluate the second source
	other, err := e.evalSource(s.Source)
	if err != nil {
		return nil, fmt.Errorf("%s source: %w", s.Op, err)
	}
	if other.kind != "commits" {
		return nil, &DSLError{Message: fmt.Sprintf("'%s' requires commits on both sides", s.Op)}
	}

	otherSet := make(map[string]bool, len(other.commits))
	for _, c := range other.commits {
		otherSet[c.Hash] = true
	}

	var result []CommitResult

	switch s.Op {
	case "except":
		for _, c := range rs.commits {
			if !otherSet[c.Hash] {
				result = append(result, c)
			}
		}
	case "intersect":
		for _, c := range rs.commits {
			if otherSet[c.Hash] {
				result = append(result, c)
			}
		}
	case "union":
		seen := make(map[string]bool, len(rs.commits))
		for _, c := range rs.commits {
			seen[c.Hash] = true
			result = append(result, c)
		}
		for _, c := range other.commits {
			if !seen[c.Hash] {
				result = append(result, c)
			}
		}
	}

	return &resultSet{kind: "commits", commits: result}, nil
}

// --- Help evaluation ---

func (e *evaluator) evalHelp(s *HelpStage) (*resultSet, error) {
	text := getHelpText(s.Topic)
	return &resultSet{
		kind:            "formatted",
		formattedOutput: text,
		formatType:      "table",
	}, nil
}

func getHelpText(topic string) string {
	switch topic {
	case "":
		return `Jock DSL — Git Query Language

SOURCES
  commits              list commits (default last 500)
  commits[main]        commits on a branch
  commits[a..b]        commits in a range
  commits[last 10]     last N commits
  branches             list branches
  tags                 list tagged commits
  files                list changed files with stats
  blame "file.go"      per-line blame for a file
  stash                list stash entries
  status               working directory changes
  remotes              list git remotes
  remote branches      list remote tracking branches
  reflog               list reflog entries
  conflicts            list merge conflicts

FILTERS
  where <condition>    filter results
  unique               deduplicate by hash
  except <source>      set difference
  intersect <source>   set intersection
  union <source>       set union

TRANSFORMS
  sort <field> [asc|desc]   sort results
  select <fields>           choose fields to display
  first N / last N / skip N limit results
  reverse                   reverse order
  group by <field>          group results

AGGREGATES
  count                count results
  sum / avg / min / max <field>     basic aggregates
  median / stddev / p90 / p95 / p99 advanced aggregates
  count_distinct <field>            count unique values
  having <condition>                filter after aggregation

ACTIONS (standalone)
  branch create "name" [from "ref"]   create a branch
  merge "branch" [--no-ff]            merge a branch
  commit "message" [--amend]          create a commit
  push [remote] [branch] [--force]    push to remote
  pull [remote] [branch]              pull from remote
  abort merge|rebase|cherry-pick      abort operation
  continue rebase                     continue rebase
  stash create "message"              create a stash

ACTIONS (piped)
  | delete [--force]    delete branches or tags
  | stage               stage files (from status)
  | unstage             unstage files (from status)
  | apply / pop / drop  stash operations
  | resolve [ours|theirs]  resolve conflicts
  | squash "message"    squash commits
  | reorder 3,1,2       reorder commits

OUTPUT
  format json|csv|table|markdown|yaml   format output
  export "path"                         export to file
  explain <query>                       show query plan

VARIABLES
  let name = value                set a variable
  $current_branch, $repo_name    built-in variables

ALIASES
  alias name = query              define alias
  alias save name = query         persistent alias
  aliases                         list all aliases

EXAMPLES
  commits | where author == "Alice" | first 10
  commits | where date within last 7 days | group by author | count
  commits | sort additions desc | first 5 | format json
  branches | where merged == "true" | delete
  status | where status == "modified" | stage
  stash | first 1 | apply
  conflicts | resolve ours
  commits | last 3 | squash "combined"

Type "help <topic>" for details: where, sort, group, format, date, set, blame,
  stash, having, status, branches, remotes, reflog, conflicts, actions, variables`

	case "where":
		return `WHERE — Filter results by condition

SYNTAX
  commits | where <condition>

OPERATORS
  ==, !=, >, <, >=, <=    comparison
  contains "text"          substring match (case-insensitive)
  matches "regex"          regex match

LOGICAL
  and, or, not             combine conditions
  ( ... )                  grouping

DATE OPERATORS
  date within last N days|hours|weeks|months|years
  date between "start" and "end"

EXAMPLES
  commits | where author == "Alice"
  commits | where message contains "fix" and additions > 10
  commits | where (author == "Alice" or author == "Bob") and date within last 7 days
  commits | where files contains "main.go"`

	case "sort":
		return `SORT — Order results by field

SYNTAX
  commits | sort <field> [asc|desc]

FIELDS
  author, message, date, hash, additions, deletions

For aggregate results:
  ... | sort value desc    sort by aggregate value
  ... | sort group asc     sort by group key

EXAMPLES
  commits | sort date desc
  commits | sort additions desc | first 5
  commits | group by author | sum additions | sort value desc`

	case "group":
		return `GROUP BY — Group and aggregate results

SYNTAX
  commits | group by <field>
  commits | group by <field> | <aggregate>

GROUPABLE FIELDS
  author, branch, tag, date

AGGREGATES
  count                  count per group
  sum <field>            sum a numeric field
  avg <field>            average a numeric field
  min <field>            minimum value
  max <field>            maximum value

NUMERIC FIELDS
  additions, deletions

EXAMPLES
  commits | group by author | count
  commits | group by author | sum additions | sort value desc
  commits | where date within last 30 days | group by author | avg additions`

	case "format":
		return `FORMAT — Control output format

SYNTAX
  ... | format json       JSON output
  ... | format csv        CSV with header row
  ... | format table      ASCII table with borders
  ... | format markdown   Markdown table
  ... | format yaml       YAML output
  ... | export "path"     export to file

EXAMPLES
  commits | first 5 | format json
  commits | group by author | count | format csv
  branches | format markdown
  commits | first 10 | format json | export "output.json"`

	case "date":
		return `DATE — Date filtering and comparison

OPERATORS
  date > "2025-01-01"                after a date
  date < "2025-06-15"                before a date
  date >= "2025-01-01"               on or after
  date <= "2025-06-15"               on or before
  date within last N days            within recent period
  date within last N hours|weeks|months|years
  date between "start" and "end"     within a range

DATE FORMATS (ISO 8601)
  "2025-01-15"
  "2025-01-15T14:30:00"
  "2025-01-15T14:30:00Z"
  "2025-01-15T14:30:00+00:00"

EXAMPLES
  commits | where date within last 7 days
  commits | where date > "2025-01-01" and date < "2025-06-01"
  commits | where date between "2025-01-01" and "2025-03-31"`

	case "set", "except", "intersect", "union":
		return `SET OPERATIONS — Combine commit sets

SYNTAX
  <source> | except <source>      commits in left but not right
  <source> | intersect <source>   commits in both
  <source> | union <source>       commits in either (deduplicated)

The right-hand source supports the same syntax as a regular source:
  commits[branch], commits[a..b], commits[last N]

EXAMPLES
  commits[main] | except commits[develop]
  commits[main] | intersect commits[feature]
  commits[main] | union commits[develop] | sort date desc`

	case "select":
		return `SELECT — Project specific fields

SYNTAX
  commits | select <field1>, <field2>, ...

FIELDS
  hash, message, author, date, branch, tag
  files, additions, deletions

EXAMPLES
  commits | select hash, author, message
  commits | first 5 | select hash, additions, deletions | format csv`

	case "blame":
		return `BLAME — Per-line blame for a file

SYNTAX
  blame "path/to/file.go"

FIELDS
  author     line's last author
  hash       commit hash for the line
  content    line content
  date       author date
  lineno     line number

EXAMPLES
  blame "main.go"
  blame "main.go" | where author == "Alice"
  blame "main.go" | where content contains "TODO"
  blame "main.go" | where lineno > 10 and lineno < 50
  blame "main.go" | count`

	case "stash":
		return `STASH — Query stash entries

SYNTAX
  stash

FIELDS
  message    stash message
  branch     branch the stash was created on
  date       stash date
  index      stash index number

EXAMPLES
  stash
  stash | where message contains "wip"
  stash | where branch == "main"
  stash | count`

	case "having":
		return `HAVING — Filter aggregation results

SYNTAX
  ... | group by <field> | count | having <condition>

FIELDS
  value      the aggregate value (count, sum, etc.)
  group      the group key

OPERATORS
  ==, !=, >, <, >=, <=    comparison
  contains "text"          substring match on group key

EXAMPLES
  commits | group by author | count | having value > 5
  commits | group by author | sum additions | having value >= 100
  commits | group by author | count | having group contains "Alice"`

	case "status":
		return `STATUS — Working directory changes

SYNTAX
  status

FIELDS
  path       file path
  status     modified, added, deleted, renamed, untracked, unmerged
  staged     true/false

EXAMPLES
  status
  status | where status == "modified"
  status | where staged == "false" | stage
  status | where path contains ".go" | stage`

	case "branches":
		return `BRANCHES — Query and manage branches

SYNTAX
  branches                      list all branches
  branch create "name"          create a new branch
  branch create "name" from "ref"   create from specific ref

FIELDS
  name        branch name
  isCurrent   true if current branch
  remote      remote tracking branch
  merged      true if merged into current

PIPE ACTIONS
  ... | delete [--force]   delete matching branches

EXAMPLES
  branches | where merged == "true" | delete
  branches | where name contains "feature"
  branch create "feature/new" from "main"`

	case "remotes", "remote":
		return `REMOTES — Query remote information

SYNTAX
  remotes                       list all remotes
  remote branches               list remote tracking branches
  remote branches "origin"      for a specific remote

FIELDS (remotes)
  name        remote name
  fetchURL    fetch URL
  pushURL     push URL

FIELDS (remote branches)
  name        branch name
  remote      remote name

EXAMPLES
  remotes
  remote branches | where name contains "feature"
  remote branches "origin" | count`

	case "reflog":
		return `REFLOG — Query reflog entries

SYNTAX
  reflog              list reflog (default 100 entries)
  reflog[50]          list last 50 entries

FIELDS
  hash       commit hash
  action     commit, checkout, rebase, merge, reset, etc.
  message    reflog message
  date       entry date

EXAMPLES
  reflog | where action == "checkout"
  reflog | where message contains "feature"
  reflog | first 20`

	case "conflicts":
		return `CONFLICTS — Query and resolve merge conflicts

SYNTAX
  conflicts

FIELDS
  path    file path with conflict

PIPE ACTIONS
  ... | resolve [ours|theirs]   resolve with strategy

EXAMPLES
  conflicts
  conflicts | resolve ours
  conflicts | where path contains ".go" | resolve theirs`

	case "actions":
		return `ACTIONS — Mutation operations

STANDALONE ACTIONS
  branch create "name" [from "ref"]   create a branch
  merge "branch" [--no-ff]            merge a branch
  commit "message" [--amend]          create a commit
  push [remote] [branch] [--force] [--set-upstream]
  pull [remote] [branch]              pull from remote
  abort merge|rebase|cherry-pick      abort operation
  continue rebase                     continue rebase
  stash create "message" [--include-untracked]

PIPED ACTIONS
  branches | ... | delete [--force]       delete branches
  tags | ... | delete                     delete tags
  tags | ... | push                       push tags
  stash | ... | apply / pop / drop        stash operations
  status | ... | stage / unstage          staging operations
  conflicts | ... | resolve [ours|theirs] resolve conflicts
  commits | ... | squash "message"        squash commits
  commits | ... | reorder 3,1,2           reorder commits

SAFETY
  All mutations check for clean working directory
  Use --dry-run context for preview`

	case "variables", "let":
		return `VARIABLES — User-defined and built-in variables

SYNTAX
  let name = value        define a variable
  $name                   use a variable in any query

BUILT-IN VARIABLES
  $current_branch    current branch name
  $repo_name         repository directory name

EXAMPLES
  let team = "Alice"
  commits | where author == $team
  commits[$current_branch] | first 10`

	default:
		return fmt.Sprintf("No help available for %q. Try: help where, help sort, help group, help format, help date, help set, help blame, help stash, help having, help status, help branches, help remotes, help reflog, help conflicts, help actions, help variables", topic)
	}
}

// --- Date evaluation helpers ---

// dateFormats lists the formats we accept for user-supplied date strings, in priority order.
var dateFormats = []string{
	time.RFC3339,                // 2025-01-15T14:30:00+00:00
	"2006-01-02T15:04:05Z",     // with Z timezone
	"2006-01-02T15:04:05",      // date + time, no tz
	"2006-01-02",               // date only
}

// parseDate parses a date string trying multiple formats.
func parseDate(s string) (time.Time, error) {
	for _, layout := range dateFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q (expected ISO 8601 format like 2025-01-15)", s)
}

// getCommitDate parses the commit's DateISO field.
func getCommitDate(c *CommitResult) (time.Time, error) {
	if c.DateISO == "" {
		return time.Time{}, fmt.Errorf("commit %s has no ISO date", c.Hash)
	}
	return parseDate(c.DateISO)
}

// evalDateComparison handles date > "2025-01-01", date < "2025-06-15", etc.
func (e *evaluator) evalDateComparison(c *CommitResult, op string, right Expr) (bool, error) {
	commitDate, err := getCommitDate(c)
	if err != nil {
		return false, err
	}

	val, ok := right.(*StringLit)
	if !ok {
		return false, fmt.Errorf("date comparison requires a string value (e.g., \"2025-01-15\")")
	}

	target, err := parseDate(val.Value)
	if err != nil {
		return false, err
	}

	switch op {
	case ">":
		return commitDate.After(target), nil
	case "<":
		return commitDate.Before(target), nil
	case ">=":
		return !commitDate.Before(target), nil
	case "<=":
		return !commitDate.After(target), nil
	default:
		return false, fmt.Errorf("unknown date operator %q", op)
	}
}

// evalDateWithin handles: date within last N days/hours/weeks/months/years
func (e *evaluator) evalDateWithin(ex *DateWithinExpr, c *CommitResult) (bool, error) {
	commitDate, err := getCommitDate(c)
	if err != nil {
		return false, err
	}

	now := time.Now()
	var cutoff time.Time

	switch ex.Unit {
	case "hours":
		cutoff = now.Add(-time.Duration(ex.N) * time.Hour)
	case "days":
		cutoff = now.AddDate(0, 0, -ex.N)
	case "weeks":
		cutoff = now.AddDate(0, 0, -ex.N*7)
	case "months":
		cutoff = now.AddDate(0, -ex.N, 0)
	case "years":
		cutoff = now.AddDate(-ex.N, 0, 0)
	default:
		return false, fmt.Errorf("unknown time unit %q", ex.Unit)
	}

	return commitDate.After(cutoff), nil
}

// evalDateBetween handles: date between "start" and "end"
func (e *evaluator) evalDateBetween(ex *DateBetweenExpr, c *CommitResult) (bool, error) {
	commitDate, err := getCommitDate(c)
	if err != nil {
		return false, err
	}

	start, err := parseDate(ex.Start)
	if err != nil {
		return false, fmt.Errorf("invalid start date: %w", err)
	}

	end, err := parseDate(ex.End)
	if err != nil {
		return false, fmt.Errorf("invalid end date: %w", err)
	}

	return !commitDate.Before(start) && !commitDate.After(end), nil
}
