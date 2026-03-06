package dsl

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/daearol/jockv2/backend/internal/git"
	"github.com/daearol/jockv2/backend/internal/tasks"
)

// --- Phase 1-2: New source evaluators ---

func (e *evaluator) evalStatusSource() (*resultSet, error) {
	staged, unstaged, untracked, unmerged, err := git.GetStatus(e.ctx.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	var results []StatusResult
	for _, f := range staged {
		results = append(results, StatusResult{
			Path:   f.Path,
			Status: f.Status,
			Staged: true,
		})
	}
	for _, f := range unstaged {
		results = append(results, StatusResult{
			Path:   f.Path,
			Status: f.Status,
			Staged: false,
		})
	}
	for _, path := range untracked {
		results = append(results, StatusResult{
			Path:   path,
			Status: "untracked",
			Staged: false,
		})
	}
	for _, f := range unmerged {
		results = append(results, StatusResult{
			Path:   f.Path,
			Status: "unmerged",
			Staged: false,
		})
	}

	return &resultSet{kind: "status", statusEntries: results}, nil
}

func (e *evaluator) evalRemotesSource() (*resultSet, error) {
	infos, err := git.ListRemotes(e.ctx.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("remotes: %w", err)
	}
	var results []RemoteResult
	for _, r := range infos {
		results = append(results, RemoteResult{
			Name:     r.Name,
			FetchURL: r.URL,
			PushURL:  r.URL,
		})
	}
	return &resultSet{kind: "remotes", remotes: results}, nil
}

func (e *evaluator) evalRemoteBranchesSource(s *RemoteBranchesSourceStage) (*resultSet, error) {
	remotes, err := git.ListRemotes(e.ctx.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("remote-branches: %w", err)
	}
	var results []RemoteBranchResult
	for _, r := range remotes {
		if s.Remote != "" && r.Name != s.Remote {
			continue
		}
		branches, err := git.ListRemoteBranches(e.ctx.RepoPath, r.Name)
		if err != nil {
			continue
		}
		for _, b := range branches {
			results = append(results, RemoteBranchResult{
				Name:   b,
				Remote: r.Name,
			})
		}
	}
	return &resultSet{kind: "remote_branches", remoteBranches: results}, nil
}

func (e *evaluator) evalReflogSource(s *ReflogSourceStage) (*resultSet, error) {
	entries, err := git.ListReflog(e.ctx.RepoPath, s.Limit)
	if err != nil {
		return nil, fmt.Errorf("reflog: %w", err)
	}
	var results []ReflogResult
	for _, entry := range entries {
		results = append(results, ReflogResult{
			Hash:    entry.Hash,
			Action:  entry.Action,
			Message: entry.Message,
			Date:    entry.Date,
		})
	}
	return &resultSet{kind: "reflog", reflogEntries: results}, nil
}

func (e *evaluator) evalConflictsSource() (*resultSet, error) {
	_, _, _, unmerged, err := git.GetStatus(e.ctx.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("conflicts: %w", err)
	}
	var results []ConflictResult
	for _, f := range unmerged {
		results = append(results, ConflictResult{Path: f.Path})
	}
	return &resultSet{kind: "conflicts", conflicts: results}, nil
}

func (e *evaluator) evalTasksSource(s *TasksSourceStage) (*resultSet, error) {
	taskList, err := tasks.List(e.ctx.RepoPath, s.StatusFilter)
	if err != nil {
		return nil, fmt.Errorf("tasks: %w", err)
	}
	var results []TaskResult
	for _, t := range taskList {
		results = append(results, TaskResult{
			ID:          t.ID,
			Title:       t.Title,
			Status:      t.Status,
			Priority:    t.Priority,
			Labels:      t.Labels,
			Branch:      t.Branch,
			Created:     t.Created,
			Updated:     t.Updated,
			Description: t.Description,
		})
	}
	return &resultSet{kind: "tasks", tasks: results}, nil
}

// --- Phase 1-2: Where filtering for new types ---

func (e *evaluator) evalWhereBranches(rs *resultSet, s *WhereStage) (*resultSet, error) {
	var filtered []BranchResult
	for _, b := range rs.branches {
		match, err := e.evalBranchCondition(s.Condition, &b)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, b)
		}
	}
	return &resultSet{kind: "branches", branches: filtered}, nil
}

func (e *evaluator) evalBranchCondition(expr Expr, b *BranchResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalBranchCondition(ex.Left, b)
			if err != nil || !left {
				return left, err
			}
			return e.evalBranchCondition(ex.Right, b)
		}
		if ex.Op == "or" {
			left, err := e.evalBranchCondition(ex.Left, b)
			if err != nil || left {
				return left, err
			}
			return e.evalBranchCondition(ex.Right, b)
		}

		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("expected field on left side")
		}

		fieldVal := getBranchFieldString(field.Name, b)

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
			if field.Name == "merged" || field.Name == "isCurrent" {
				return strings.EqualFold(fieldVal, val), nil
			}
			return strings.EqualFold(fieldVal, val), nil
		case "!=":
			val := e.exprToString(ex.Right)
			return !strings.EqualFold(fieldVal, val), nil
		}
		return false, fmt.Errorf("unknown operator %q for branches", ex.Op)

	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalBranchCondition(ex.Operand, b)
			return !val, err
		}
	}
	return false, fmt.Errorf("unexpected expression type %T in branch condition", expr)
}

func getBranchFieldString(name string, b *BranchResult) string {
	switch name {
	case "name":
		return b.Name
	case "remote":
		return b.Remote
	case "merged":
		if b.Merged {
			return "true"
		}
		return "false"
	case "isCurrent":
		if b.IsCurrent {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func (e *evaluator) evalWhereStatus(rs *resultSet, s *WhereStage) (*resultSet, error) {
	var filtered []StatusResult
	for _, entry := range rs.statusEntries {
		match, err := e.evalStatusCondition(s.Condition, &entry)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, entry)
		}
	}
	return &resultSet{kind: "status", statusEntries: filtered}, nil
}

func (e *evaluator) evalStatusCondition(expr Expr, s *StatusResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalStatusCondition(ex.Left, s)
			if err != nil || !left {
				return left, err
			}
			return e.evalStatusCondition(ex.Right, s)
		}
		if ex.Op == "or" {
			left, err := e.evalStatusCondition(ex.Left, s)
			if err != nil || left {
				return left, err
			}
			return e.evalStatusCondition(ex.Right, s)
		}

		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("expected field on left side")
		}

		fieldVal := getStatusFieldString(field.Name, s)

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
		}
		return false, fmt.Errorf("unknown operator %q for status", ex.Op)

	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalStatusCondition(ex.Operand, s)
			return !val, err
		}
	}
	return false, fmt.Errorf("unexpected expression type %T in status condition", expr)
}

func getStatusFieldString(name string, s *StatusResult) string {
	switch name {
	case "path":
		return s.Path
	case "status":
		return s.Status
	case "staged":
		if s.Staged {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func (e *evaluator) evalWhereReflog(rs *resultSet, s *WhereStage) (*resultSet, error) {
	var filtered []ReflogResult
	for _, entry := range rs.reflogEntries {
		match, err := e.evalReflogCondition(s.Condition, &entry)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, entry)
		}
	}
	return &resultSet{kind: "reflog", reflogEntries: filtered}, nil
}

func (e *evaluator) evalReflogCondition(expr Expr, r *ReflogResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalReflogCondition(ex.Left, r)
			if err != nil || !left {
				return left, err
			}
			return e.evalReflogCondition(ex.Right, r)
		}
		if ex.Op == "or" {
			left, err := e.evalReflogCondition(ex.Left, r)
			if err != nil || left {
				return left, err
			}
			return e.evalReflogCondition(ex.Right, r)
		}

		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("expected field on left side")
		}

		fieldVal := getReflogFieldString(field.Name, r)

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
		}
		return false, fmt.Errorf("unknown operator %q for reflog", ex.Op)

	case *UnaryExpr:
		if ex.Op == "not" {
			val, err := e.evalReflogCondition(ex.Operand, r)
			return !val, err
		}
	}
	return false, fmt.Errorf("unexpected expression type %T in reflog condition", expr)
}

func getReflogFieldString(name string, r *ReflogResult) string {
	switch name {
	case "hash":
		return r.Hash
	case "action":
		return r.Action
	case "message":
		return r.Message
	case "date":
		return r.Date
	default:
		return ""
	}
}

func (e *evaluator) evalWhereRemoteBranches(rs *resultSet, s *WhereStage) (*resultSet, error) {
	var filtered []RemoteBranchResult
	for _, rb := range rs.remoteBranches {
		match, err := e.evalRemoteBranchCondition(s.Condition, &rb)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, rb)
		}
	}
	return &resultSet{kind: "remote_branches", remoteBranches: filtered}, nil
}

func (e *evaluator) evalRemoteBranchCondition(expr Expr, rb *RemoteBranchResult) (bool, error) {
	switch ex := expr.(type) {
	case *BinaryExpr:
		if ex.Op == "and" {
			left, err := e.evalRemoteBranchCondition(ex.Left, rb)
			if err != nil || !left {
				return left, err
			}
			return e.evalRemoteBranchCondition(ex.Right, rb)
		}
		if ex.Op == "or" {
			left, err := e.evalRemoteBranchCondition(ex.Left, rb)
			if err != nil || left {
				return left, err
			}
			return e.evalRemoteBranchCondition(ex.Right, rb)
		}

		field, ok := ex.Left.(*FieldExpr)
		if !ok {
			return false, fmt.Errorf("expected field on left side")
		}

		var fieldVal string
		switch field.Name {
		case "name":
			fieldVal = rb.Name
		case "remote":
			fieldVal = rb.Remote
		default:
			return false, fmt.Errorf("unknown field %q for remote-branches", field.Name)
		}

		switch ex.Op {
		case "contains":
			val, ok := ex.Right.(*StringLit)
			if !ok {
				return false, nil
			}
			return strings.Contains(strings.ToLower(fieldVal), strings.ToLower(val.Value)), nil
		case "==":
			return strings.EqualFold(fieldVal, e.exprToString(ex.Right)), nil
		case "!=":
			return !strings.EqualFold(fieldVal, e.exprToString(ex.Right)), nil
		}
	}
	return false, nil
}

// --- Phase 3: String functions ---

func evalStringFunc(name string, args []interface{}) (interface{}, error) {
	switch name {
	case "upper":
		if len(args) < 1 {
			return nil, fmt.Errorf("upper() requires 1 argument")
		}
		return strings.ToUpper(fmt.Sprintf("%v", args[0])), nil
	case "lower":
		if len(args) < 1 {
			return nil, fmt.Errorf("lower() requires 1 argument")
		}
		return strings.ToLower(fmt.Sprintf("%v", args[0])), nil
	case "trim":
		if len(args) < 1 {
			return nil, fmt.Errorf("trim() requires 1 argument")
		}
		return strings.TrimSpace(fmt.Sprintf("%v", args[0])), nil
	case "len":
		if len(args) < 1 {
			return nil, fmt.Errorf("len() requires 1 argument")
		}
		return float64(len(fmt.Sprintf("%v", args[0]))), nil
	case "substr":
		if len(args) < 3 {
			return nil, fmt.Errorf("substr() requires 3 arguments: string, start, length")
		}
		s := fmt.Sprintf("%v", args[0])
		start := int(toFloat64(args[1]))
		length := int(toFloat64(args[2]))
		if start < 0 || start >= len(s) {
			return "", nil
		}
		end := start + length
		if end > len(s) {
			end = len(s)
		}
		return s[start:end], nil
	case "split":
		if len(args) < 3 {
			return nil, fmt.Errorf("split() requires 3 arguments: string, separator, index")
		}
		s := fmt.Sprintf("%v", args[0])
		sep := fmt.Sprintf("%v", args[1])
		idx := int(toFloat64(args[2]))
		parts := strings.Split(s, sep)
		if idx < 0 || idx >= len(parts) {
			return "", nil
		}
		return parts[idx], nil
	case "replace":
		if len(args) < 3 {
			return nil, fmt.Errorf("replace() requires 3 arguments: string, old, new")
		}
		return strings.ReplaceAll(fmt.Sprintf("%v", args[0]), fmt.Sprintf("%v", args[1]), fmt.Sprintf("%v", args[2])), nil
	case "starts_with":
		if len(args) < 2 {
			return nil, fmt.Errorf("starts_with() requires 2 arguments")
		}
		return strings.HasPrefix(fmt.Sprintf("%v", args[0]), fmt.Sprintf("%v", args[1])), nil
	case "ends_with":
		if len(args) < 2 {
			return nil, fmt.Errorf("ends_with() requires 2 arguments")
		}
		return strings.HasSuffix(fmt.Sprintf("%v", args[0]), fmt.Sprintf("%v", args[1])), nil
	}
	return nil, fmt.Errorf("unknown string function %q", name)
}

// --- Phase 3: Date functions ---

func evalDateFunc(name string, args []interface{}) (interface{}, error) {
	switch name {
	case "days_since":
		if len(args) < 1 {
			return nil, fmt.Errorf("days_since() requires 1 argument")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return float64(0), nil
		}
		return math.Floor(time.Since(t).Hours() / 24), nil
	case "hours_since":
		if len(args) < 1 {
			return nil, fmt.Errorf("hours_since() requires 1 argument")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return float64(0), nil
		}
		return math.Floor(time.Since(t).Hours()), nil
	case "day_of_week":
		if len(args) < 1 {
			return nil, fmt.Errorf("day_of_week() requires 1 argument")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return "", nil
		}
		return t.Weekday().String(), nil
	case "hour":
		if len(args) < 1 {
			return nil, fmt.Errorf("hour() requires 1 argument")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return float64(0), nil
		}
		return float64(t.Hour()), nil
	case "month":
		if len(args) < 1 {
			return nil, fmt.Errorf("month() requires 1 argument")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return float64(0), nil
		}
		return float64(t.Month()), nil
	case "year":
		if len(args) < 1 {
			return nil, fmt.Errorf("year() requires 1 argument")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return float64(0), nil
		}
		return float64(t.Year()), nil
	case "week":
		if len(args) < 1 {
			return nil, fmt.Errorf("week() requires 1 argument")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return "", nil
		}
		y, w := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", y, w), nil
	case "format_date":
		if len(args) < 2 {
			return nil, fmt.Errorf("format_date() requires 2 arguments: date, format")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return dateStr, nil
		}
		fmtStr := fmt.Sprintf("%v", args[1])
		// Convert common format tokens to Go layout
		fmtStr = strings.ReplaceAll(fmtStr, "YYYY", "2006")
		fmtStr = strings.ReplaceAll(fmtStr, "MM", "01")
		fmtStr = strings.ReplaceAll(fmtStr, "DD", "02")
		fmtStr = strings.ReplaceAll(fmtStr, "HH", "15")
		fmtStr = strings.ReplaceAll(fmtStr, "mm", "04")
		fmtStr = strings.ReplaceAll(fmtStr, "ss", "05")
		return t.Format(fmtStr), nil
	case "date_add":
		if len(args) < 3 {
			return nil, fmt.Errorf("date_add() requires 3 arguments: date, amount, unit")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return dateStr, nil
		}
		n := int(toFloat64(args[1]))
		unit := fmt.Sprintf("%v", args[2])
		t = addDateUnit(t, n, unit)
		return t.Format("2006-01-02"), nil
	case "date_sub":
		if len(args) < 3 {
			return nil, fmt.Errorf("date_sub() requires 3 arguments: date, amount, unit")
		}
		dateStr := fmt.Sprintf("%v", args[0])
		t, err := parseDate(dateStr)
		if err != nil {
			return dateStr, nil
		}
		n := int(toFloat64(args[1]))
		unit := fmt.Sprintf("%v", args[2])
		t = addDateUnit(t, -n, unit)
		return t.Format("2006-01-02"), nil
	}
	return nil, fmt.Errorf("unknown date function %q", name)
}

func addDateUnit(t time.Time, n int, unit string) time.Time {
	switch unit {
	case "days":
		return t.AddDate(0, 0, n)
	case "weeks":
		return t.AddDate(0, 0, n*7)
	case "months":
		return t.AddDate(0, n, 0)
	case "years":
		return t.AddDate(n, 0, 0)
	case "hours":
		return t.Add(time.Duration(n) * time.Hour)
	default:
		return t
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}

// --- Phase 3: Advanced aggregates ---

func computeAdvancedAggregate(fn, field string, commits []CommitResult) float64 {
	if len(commits) == 0 {
		return 0
	}

	e := &evaluator{}
	vals := make([]float64, len(commits))
	for i := range commits {
		v, _ := e.getFieldNumeric(field, &commits[i])
		vals[i] = float64(v)
	}
	sort.Float64s(vals)

	switch fn {
	case "median":
		n := len(vals)
		if n%2 == 0 {
			return (vals[n/2-1] + vals[n/2]) / 2
		}
		return vals[n/2]
	case "p90":
		return percentile(vals, 90)
	case "p95":
		return percentile(vals, 95)
	case "p99":
		return percentile(vals, 99)
	case "stddev":
		mean := 0.0
		for _, v := range vals {
			mean += v
		}
		mean /= float64(len(vals))
		variance := 0.0
		for _, v := range vals {
			diff := v - mean
			variance += diff * diff
		}
		variance /= float64(len(vals))
		return math.Sqrt(variance)
	case "count_distinct":
		seen := make(map[string]bool)
		for i := range commits {
			key := e.getFieldString(field, &commits[i])
			seen[key] = true
		}
		return float64(len(seen))
	}
	return 0
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	k := (p / 100.0) * float64(len(sorted)-1)
	f := math.Floor(k)
	c := math.Ceil(k)
	if f == c {
		return sorted[int(k)]
	}
	return sorted[int(f)]*(c-k) + sorted[int(c)]*(k-f)
}

// --- Phase 4: Explain ---

func explainPipeline(pipeline *Pipeline) string {
	var lines []string
	lines = append(lines, "QUERY PLAN:")
	for i, stage := range pipeline.Stages {
		lines = append(lines, fmt.Sprintf("  %d. %s", i+1, describeStage(stage)))
	}
	return strings.Join(lines, "\n")
}

func describeStage(stage Stage) string {
	switch s := stage.(type) {
	case *SourceStage:
		desc := fmt.Sprintf("Source: %s", s.Kind)
		if s.Range != nil {
			if s.Range.LastN > 0 {
				desc += fmt.Sprintf(" [last %d]", s.Range.LastN)
			} else if s.Range.Ref != "" {
				desc += fmt.Sprintf(" [%s]", s.Range.Ref)
			} else if s.Range.From != "" {
				desc += fmt.Sprintf(" [%s..%s]", s.Range.From, s.Range.To)
			}
		}
		return desc
	case *WhereStage:
		return "Filter: where <condition>"
	case *SelectStage:
		return fmt.Sprintf("Project: select %s", strings.Join(s.Fields, ", "))
	case *SortStage:
		dir := "asc"
		if s.Desc {
			dir = "desc"
		}
		return fmt.Sprintf("Sort: %s %s", s.Field, dir)
	case *LimitStage:
		return fmt.Sprintf("Limit: %s %d", s.Kind, s.Count)
	case *GroupByStage:
		return fmt.Sprintf("Group: by %s", s.Field)
	case *AggregateStage:
		return fmt.Sprintf("Aggregate: %s %s", s.Func, s.Field)
	case *CountStage:
		return "Aggregate: count"
	case *FormatStage:
		return fmt.Sprintf("Format: %s", s.Format)
	case *UniqueStage:
		return "Transform: unique"
	case *ReverseStage:
		return "Transform: reverse"
	case *SetOpStage:
		return fmt.Sprintf("Set operation: %s", s.Op)
	case *ActionStage:
		desc := fmt.Sprintf("Action: %s", s.Kind)
		if s.Target != "" {
			desc += fmt.Sprintf(" → %s", s.Target)
		}
		return desc
	case *StatusSourceStage:
		return "Source: status"
	case *RemotesSourceStage:
		return "Source: remotes"
	case *ReflogSourceStage:
		return fmt.Sprintf("Source: reflog [limit %d]", s.Limit)
	case *ConflictsSourceStage:
		return "Source: conflicts"
	case *BlameSourceStage:
		return fmt.Sprintf("Source: blame %q", s.FilePath)
	case *StashSourceStage:
		return "Source: stash"
	case *ExportStage:
		return fmt.Sprintf("Export: %s", s.Path)
	case *WindowStage:
		return fmt.Sprintf("Window: %s(%s) as %s", s.Func, s.Field, s.Alias)
	default:
		return fmt.Sprintf("Stage: %T", stage)
	}
}

// --- Phase 4: Export ---

func exportToFile(output, path string) error {
	return os.WriteFile(path, []byte(output), 0o644)
}

// --- Phase 4: Format markdown and yaml ---

func formatMarkdown(r *Result) (string, error) {
	var buf strings.Builder

	switch r.Kind {
	case "commits":
		buf.WriteString("| Hash | Author | Date | Message |\n")
		buf.WriteString("|------|--------|------|---------|\n")
		for _, c := range r.Commits {
			hash := c.Hash
			if len(hash) > 7 {
				hash = hash[:7]
			}
			buf.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", hash, c.Author, c.Date, c.Message))
		}
	case "branches":
		buf.WriteString("| Name | Current | Remote |\n")
		buf.WriteString("|------|---------|--------|\n")
		for _, b := range r.Branches {
			current := " "
			if b.IsCurrent {
				current = "*"
			}
			buf.WriteString(fmt.Sprintf("| %s | %s | %s |\n", b.Name, current, b.Remote))
		}
	case "aggregate":
		buf.WriteString("| Group | Value |\n")
		buf.WriteString("|-------|-------|\n")
		for _, a := range r.Aggregates {
			buf.WriteString(fmt.Sprintf("| %s | %.0f |\n", a.Group, a.Value))
		}
	default:
		return fmt.Sprintf("Unsupported format 'markdown' for result kind %q", r.Kind), nil
	}

	return buf.String(), nil
}

func formatYAML(r *Result) (string, error) {
	var buf strings.Builder

	switch r.Kind {
	case "commits":
		for _, c := range r.Commits {
			buf.WriteString(fmt.Sprintf("- hash: %s\n", c.Hash))
			buf.WriteString(fmt.Sprintf("  author: %s\n", c.Author))
			buf.WriteString(fmt.Sprintf("  date: %s\n", c.Date))
			buf.WriteString(fmt.Sprintf("  message: %s\n", c.Message))
		}
	case "branches":
		for _, b := range r.Branches {
			buf.WriteString(fmt.Sprintf("- name: %s\n", b.Name))
			buf.WriteString(fmt.Sprintf("  current: %v\n", b.IsCurrent))
			buf.WriteString(fmt.Sprintf("  remote: %s\n", b.Remote))
		}
	case "aggregate":
		for _, a := range r.Aggregates {
			buf.WriteString(fmt.Sprintf("- group: %s\n", a.Group))
			buf.WriteString(fmt.Sprintf("  value: %.0f\n", a.Value))
		}
	default:
		return fmt.Sprintf("# Unsupported result kind: %s", r.Kind), nil
	}

	return buf.String(), nil
}

// --- Phase 1-2: Standalone command evaluators ---

func (e *evaluator) evalBranchCreate(s *BranchCreateStage) (*resultSet, error) {
	report, err := executeBranchCreate(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

func (e *evaluator) evalMerge(s *MergeStage) (*resultSet, error) {
	report, err := executeMerge(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

func (e *evaluator) evalStashCreate(s *StashCreateStage) (*resultSet, error) {
	report, err := executeStashCreate(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

func (e *evaluator) evalCommit(s *CommitStage) (*resultSet, error) {
	report, err := executeCommit(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

func (e *evaluator) evalPush(s *PushStage) (*resultSet, error) {
	report, err := executePush(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

func (e *evaluator) evalPull(s *PullStage) (*resultSet, error) {
	report, err := executePull(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

func (e *evaluator) evalAbort(s *AbortStage) (*resultSet, error) {
	report, err := executeAbort(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

func (e *evaluator) evalContinue(s *ContinueStage) (*resultSet, error) {
	report, err := executeContinue(e.ctx, s)
	if err != nil {
		return nil, err
	}
	return actionReportResult(report), nil
}

// --- Phase 4: Export, Explain ---

func (e *evaluator) evalExport(rs *resultSet, s *ExportStage) (*resultSet, error) {
	// First format the result
	result := e.toResult(rs)
	output, err := formatJSON(result)
	if err != nil {
		return nil, err
	}
	if rs.formatType != "" {
		output = rs.formattedOutput
	}
	if err := exportToFile(output, s.Path); err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}
	return &resultSet{
		kind:            "formatted",
		formattedOutput: fmt.Sprintf("exported to %s", s.Path),
		formatType:      "table",
	}, nil
}

func (e *evaluator) evalExplain(s *ExplainStage) (*resultSet, error) {
	text := explainPipeline(s.Inner)
	return &resultSet{
		kind:            "formatted",
		formattedOutput: text,
		formatType:      "table",
	}, nil
}

func actionReportResult(report *ActionReport) *resultSet {
	return &resultSet{
		kind: "formatted",
		formattedOutput: report.Description,
		formatType:      "table",
	}
}

// --- Phase 5: Window functions ---

func (e *evaluator) evalWindow(rs *resultSet, s *WindowStage) (*resultSet, error) {
	if rs.kind != "commits" {
		return nil, &DSLError{Message: "window functions require commits"}
	}

	// Window functions add computed values but we return the same result type.
	// Since CommitResult doesn't have dynamic fields, we'll store the result
	// in the Message field as a formatted suffix (pragmatic approach for v1).
	// A better approach would be a dynamic field map — marked as future improvement.

	commits := make([]CommitResult, len(rs.commits))
	copy(commits, rs.commits)

	switch s.Func {
	case "running_sum", "cumulative_sum":
		var running float64
		for i := range commits {
			v, _ := e.getFieldNumeric(s.Field, &commits[i])
			running += float64(v)
			commits[i].Message = fmt.Sprintf("%s | %s=%.0f", commits[i].Message, s.Alias, running)
		}
	case "running_avg":
		var running float64
		for i := range commits {
			v, _ := e.getFieldNumeric(s.Field, &commits[i])
			running += float64(v)
			avg := running / float64(i+1)
			commits[i].Message = fmt.Sprintf("%s | %s=%.1f", commits[i].Message, s.Alias, avg)
		}
	case "moving_avg":
		window := s.Window
		if window <= 0 {
			window = 7
		}
		for i := range commits {
			start := i - window + 1
			if start < 0 {
				start = 0
			}
			var sum float64
			count := 0
			for j := start; j <= i; j++ {
				v, _ := e.getFieldNumeric(s.Field, &commits[j])
				sum += float64(v)
				count++
			}
			avg := sum / float64(count)
			commits[i].Message = fmt.Sprintf("%s | %s=%.1f", commits[i].Message, s.Alias, avg)
		}
	case "rank":
		type indexed struct {
			idx int
			val int
		}
		var items []indexed
		for i := range commits {
			v, _ := e.getFieldNumeric(s.Field, &commits[i])
			items = append(items, indexed{i, v})
		}
		sort.Slice(items, func(a, b int) bool { return items[a].val > items[b].val })
		for rank, item := range items {
			commits[item.idx].Message = fmt.Sprintf("%s | %s=%d", commits[item.idx].Message, s.Alias, rank+1)
		}
	case "dense_rank":
		type indexed struct {
			idx int
			val int
		}
		var items []indexed
		for i := range commits {
			v, _ := e.getFieldNumeric(s.Field, &commits[i])
			items = append(items, indexed{i, v})
		}
		sort.Slice(items, func(a, b int) bool { return items[a].val > items[b].val })
		rank := 1
		for i, item := range items {
			if i > 0 && items[i].val != items[i-1].val {
				rank++
			}
			commits[item.idx].Message = fmt.Sprintf("%s | %s=%d", commits[item.idx].Message, s.Alias, rank)
		}
	case "row_number":
		for i := range commits {
			commits[i].Message = fmt.Sprintf("%s | %s=%d", commits[i].Message, s.Alias, i+1)
		}
	case "lag":
		for i := range commits {
			var lagVal int
			if i > 0 {
				lagVal, _ = e.getFieldNumeric(s.Field, &commits[i-1])
			}
			commits[i].Message = fmt.Sprintf("%s | %s=%d", commits[i].Message, s.Alias, lagVal)
		}
	case "lead":
		for i := range commits {
			var leadVal int
			if i < len(commits)-1 {
				leadVal, _ = e.getFieldNumeric(s.Field, &commits[i+1])
			}
			commits[i].Message = fmt.Sprintf("%s | %s=%d", commits[i].Message, s.Alias, leadVal)
		}
	}

	return &resultSet{kind: "commits", commits: commits}, nil
}
