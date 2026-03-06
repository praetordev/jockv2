package dsl

import (
	"fmt"
	"strconv"
)

// Parser builds an AST from a token stream.
type Parser struct {
	tokens []Token
	pos    int
}

// NewParser creates a parser for the given tokens.
func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

// Parse parses the token stream into a Pipeline AST.
func (p *Parser) Parse() (*Pipeline, error) {
	pipeline, err := p.parsePipeline()
	if err != nil {
		return nil, err
	}
	if tok := p.current(); tok.Type != TokenEOF {
		return nil, p.errorf("unexpected %s %q, expected end of input", tok.Type, tok.Literal)
	}
	return pipeline, nil
}

func (p *Parser) parsePipeline() (*Pipeline, error) {
	pipe := &Pipeline{}

	stage, err := p.parseStage()
	if err != nil {
		return nil, err
	}
	pipe.Stages = append(pipe.Stages, stage)

	for p.current().Type == TokenPipe {
		p.advance() // consume |
		stage, err := p.parseStage()
		if err != nil {
			return nil, err
		}
		pipe.Stages = append(pipe.Stages, stage)
	}

	return pipe, nil
}

func (p *Parser) parseStage() (Stage, error) {
	tok := p.current()

	// Handle flags that might start a stage (shouldn't happen normally)
	if tok.Type == TokenFlag {
		return nil, p.errorf("unexpected flag %q, flags must follow an action", tok.Literal)
	}

	if tok.Type != TokenIdent {
		return nil, p.errorf("expected stage keyword, got %s %q", tok.Type, tok.Literal)
	}

	switch tok.Literal {
	// Sources
	case "commits", "branches", "tags", "files":
		return p.parseSource()
	case "blame":
		return p.parseBlameSource()
	case "stash":
		return p.parseStashOrCreate()
	case "status":
		p.advance()
		return &StatusSourceStage{}, nil
	case "remotes":
		p.advance()
		return &RemotesSourceStage{}, nil
	case "remote-branches":
		return p.parseRemoteBranches()
	case "reflog":
		return p.parseReflog()
	case "conflicts":
		p.advance()
		return &ConflictsSourceStage{}, nil
	case "tasks":
		p.advance()
		return &TasksSourceStage{}, nil

	// Filters
	case "where":
		return p.parseWhere()

	// Transforms
	case "select":
		return p.parseSelect()
	case "sort":
		return p.parseSort()
	case "unique":
		p.advance()
		return &UniqueStage{}, nil
	case "reverse":
		p.advance()
		return &ReverseStage{}, nil
	case "count":
		p.advance()
		return &CountStage{}, nil
	case "first", "last", "skip":
		return p.parseLimit()

	// Aggregation
	case "group":
		return p.parseGroupBy()
	case "sum", "avg", "min", "max":
		return p.parseAggregate()
	case "median", "stddev", "count_distinct":
		return p.parseAdvancedAggregate()
	case "p90", "p95", "p99":
		return p.parseAdvancedAggregate()

	// Format
	case "format":
		return p.parseFormat()

	// Set operations
	case "except", "intersect", "union":
		return p.parseSetOp()

	// Having
	case "having":
		return p.parseHaving()

	// Help
	case "help":
		return p.parseHelp()

	// Actions on result sets
	case "cherry-pick", "revert", "rebase", "tag", "log", "diff", "show":
		return p.parseAction()
	case "delete":
		return p.parseDeleteAction()
	case "push":
		return p.parsePushAction()
	case "pull":
		return p.parsePullAction()
	case "apply", "pop", "drop":
		return p.parseStashAction()
	case "stage":
		return p.parseStageAction()
	case "unstage":
		return p.parseUnstageAction()
	case "resolve":
		return p.parseResolveAction()

	// Standalone commands
	case "branch":
		return p.parseBranchCommand()
	case "merge":
		return p.parseMergeCommand()
	case "commit":
		return p.parseCommitCommand()
	case "abort":
		return p.parseAbortCommand()
	case "continue":
		return p.parseContinueCommand()

	// Phase 3: Variables
	case "let":
		return p.parseLetStage()

	// Phase 4: Export, copy, explain
	case "export":
		return p.parseExport()
	case "copy":
		p.advance()
		return &CopyStage{}, nil
	case "explain":
		return p.parseExplain()

	// Phase 5: Window functions
	case "running_sum", "running_avg", "cumulative_sum", "rank", "dense_rank", "row_number", "lag", "lead":
		return p.parseWindowFunction()
	case "moving_avg":
		return p.parseMovingAvg()

	// Phase 5: Interactive rebase
	case "squash":
		return p.parseSquashAction()
	case "reorder":
		return p.parseReorderAction()

	default:
		return nil, p.errorf("unknown stage %q", tok.Literal)
	}
}

// --- Source parsers ---

func (p *Parser) parseSource() (*SourceStage, error) {
	tok := p.current()
	stage := &SourceStage{Kind: tok.Literal}
	p.advance()

	// Optional range: commits[main], commits[abc..def], commits[last 10]
	if p.current().Type == TokenLBracket {
		p.advance() // consume [
		rangeExpr, err := p.parseRangeExpr()
		if err != nil {
			return nil, err
		}
		stage.Range = rangeExpr
		if err := p.expect(TokenRBracket); err != nil {
			return nil, err
		}
	}

	return stage, nil
}

func (p *Parser) parseRangeExpr() (*RangeExpr, error) {
	tok := p.current()

	// "last N"
	if tok.Type == TokenIdent && tok.Literal == "last" {
		p.advance()
		n, err := p.expectInteger()
		if err != nil {
			return nil, err
		}
		return &RangeExpr{LastN: n}, nil
	}

	// Identifier (branch/tag/hash) possibly followed by ".." for range
	if tok.Type == TokenIdent || tok.Type == TokenString {
		name := tok.Literal
		p.advance()

		if p.current().Type == TokenDotDot {
			p.advance() // consume ..
			toTok := p.current()
			if toTok.Type != TokenIdent && toTok.Type != TokenString {
				return nil, p.errorf("expected ref after '..', got %s", toTok.Type)
			}
			p.advance()
			return &RangeExpr{From: name, To: toTok.Literal}, nil
		}

		return &RangeExpr{Ref: name}, nil
	}

	return nil, p.errorf("expected range expression, got %s %q", tok.Type, tok.Literal)
}

func (p *Parser) parseStashOrCreate() (Stage, error) {
	p.advance() // consume "stash"

	// Check if this is "stash create"
	if p.isIdent("create") {
		p.advance() // consume "create"
		msg := ""
		if p.current().Type == TokenString {
			msg = p.current().Literal
			p.advance()
		}
		includeUntracked := false
		if p.current().Type == TokenFlag && p.current().Literal == "--include-untracked" {
			includeUntracked = true
			p.advance()
		}
		return &StashCreateStage{Message: msg, IncludeUntracked: includeUntracked}, nil
	}

	return &StashSourceStage{}, nil
}

func (p *Parser) parseRemoteBranches() (Stage, error) {
	p.advance() // consume "remote-branches"
	stage := &RemoteBranchesSourceStage{}

	// Optional remote filter in brackets
	if p.current().Type == TokenLBracket {
		p.advance()
		tok := p.current()
		if tok.Type != TokenIdent && tok.Type != TokenString {
			return nil, p.errorf("expected remote name, got %s %q", tok.Type, tok.Literal)
		}
		stage.Remote = tok.Literal
		p.advance()
		if err := p.expect(TokenRBracket); err != nil {
			return nil, err
		}
	}

	return stage, nil
}

func (p *Parser) parseReflog() (Stage, error) {
	p.advance() // consume "reflog"
	stage := &ReflogSourceStage{Limit: 100}

	// Optional limit in brackets: reflog[50]
	if p.current().Type == TokenLBracket {
		p.advance()
		n, err := p.expectInteger()
		if err != nil {
			return nil, err
		}
		stage.Limit = n
		if err := p.expect(TokenRBracket); err != nil {
			return nil, err
		}
	}

	return stage, nil
}

// --- Filter parsers ---

func (p *Parser) parseWhere() (*WhereStage, error) {
	p.advance() // consume "where"
	cond, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	return &WhereStage{Condition: cond}, nil
}

// parseOr handles "or" with lowest precedence.
func (p *Parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.isIdent("or") {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "or", Right: right}
	}

	return left, nil
}

// parseAnd handles "and" at higher precedence than "or".
func (p *Parser) parseAnd() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.isIdent("and") {
		p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "and", Right: right}
	}

	return left, nil
}

// parseUnary handles "not" prefix.
func (p *Parser) parseUnary() (Expr, error) {
	if p.isIdent("not") {
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "not", Operand: operand}, nil
	}
	return p.parseComparison()
}

// parseComparison handles field comparisons: field op value.
func (p *Parser) parseComparison() (Expr, error) {
	// Parenthesized expression
	if p.current().Type == TokenLParen {
		p.advance() // consume (
		expr, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil
	}

	// Variable reference
	if p.current().Type == TokenDollar {
		varRef := &VarRefExpr{Name: p.current().Literal}
		p.advance()
		return varRef, nil
	}

	// Must be a field name or function call
	tok := p.current()
	if tok.Type != TokenIdent {
		return nil, p.errorf("expected field name, got %s %q", tok.Type, tok.Literal)
	}

	// Check for function call: name(args...)
	if p.peek(1).Type == TokenLParen && isFuncName(tok.Literal) {
		return p.parseFuncCallExpr()
	}

	// Check for "in" subquery
	if !isFieldName(tok.Literal) && !isExtendedFieldName(tok.Literal) {
		return nil, p.errorf("unknown field %q", tok.Literal)
	}

	field := &FieldExpr{Name: tok.Literal}
	p.advance()

	// Date-specific operators: "within last N unit" and "between X and Y"
	if tok.Literal == "date" {
		if p.isIdent("within") {
			return p.parseDateWithin(field.Name)
		}
		if p.isIdent("between") {
			return p.parseDateBetween(field.Name)
		}
	}

	// "in" subquery
	if p.isIdent("in") {
		p.advance() // consume "in"
		if p.current().Type != TokenLParen {
			return nil, p.errorf("expected '(' after 'in'")
		}
		p.advance() // consume (
		subPipeline, err := p.parsePipeline()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return &InExpr{Field: field, Subquery: subPipeline}, nil
	}

	// "contains" or "matches" operators
	if p.isIdent("contains") || p.isIdent("matches") {
		op := p.current().Literal
		p.advance()
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: field, Op: op, Right: value}, nil
	}

	// Comparison operators
	op := p.current()
	switch op.Type {
	case TokenEq, TokenNeq, TokenGt, TokenLt, TokenGte, TokenLte:
		p.advance()
	default:
		return nil, p.errorf("expected comparison operator after field %q, got %s %q", field.Name, op.Type, op.Literal)
	}

	value, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}

	return &BinaryExpr{Left: field, Op: op.Literal, Right: value}, nil
}

// --- Phase 3: Arithmetic expression parsing (for computed fields) ---

func (p *Parser) parseAddSub() (Expr, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for p.current().Type == TokenPlus || p.current().Type == TokenMinus {
		op := p.current().Literal
		p.advance()
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: op, Right: right}
	}
	return left, nil
}

func (p *Parser) parseMulDiv() (Expr, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for p.current().Type == TokenStar || p.current().Type == TokenSlash {
		op := p.current().Literal
		p.advance()
		right, err := p.parseAtom()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: op, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAtom() (Expr, error) {
	tok := p.current()

	if tok.Type == TokenLParen {
		p.advance()
		expr, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return expr, nil
	}

	if tok.Type == TokenDollar {
		varRef := &VarRefExpr{Name: tok.Literal}
		p.advance()
		return varRef, nil
	}

	if tok.Type == TokenString {
		p.advance()
		return &StringLit{Value: tok.Literal}, nil
	}

	if tok.Type == TokenInteger {
		p.advance()
		n, err := strconv.Atoi(tok.Literal)
		if err != nil {
			return nil, p.errorf("invalid integer %q", tok.Literal)
		}
		return &IntLit{Value: n}, nil
	}

	if tok.Type == TokenIdent {
		// Check for function call
		if p.peek(1).Type == TokenLParen && isFuncName(tok.Literal) {
			return p.parseFuncCallExpr()
		}
		// Otherwise it's a field reference
		p.advance()
		return &FieldExpr{Name: tok.Literal}, nil
	}

	return nil, p.errorf("expected value, got %s %q", tok.Type, tok.Literal)
}

func (p *Parser) parseFuncCallExpr() (Expr, error) {
	name := p.current().Literal
	p.advance() // consume function name
	p.advance() // consume (

	var args []Expr
	if p.current().Type != TokenRParen {
		arg, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		for p.current().Type == TokenComma {
			p.advance()
			arg, err := p.parseAddSub()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
	}
	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	return &FuncCallExpr{Name: name, Args: args}, nil
}

func (p *Parser) parseValue() (Expr, error) {
	tok := p.current()
	switch tok.Type {
	case TokenString:
		p.advance()
		return &StringLit{Value: tok.Literal}, nil
	case TokenInteger:
		p.advance()
		n, err := strconv.Atoi(tok.Literal)
		if err != nil {
			return nil, p.errorf("invalid integer %q", tok.Literal)
		}
		return &IntLit{Value: n}, nil
	case TokenDollar:
		p.advance()
		return &VarRefExpr{Name: tok.Literal}, nil
	case TokenIdent:
		// Check for function call
		if p.peek(1).Type == TokenLParen && isFuncName(tok.Literal) {
			return p.parseFuncCallExpr()
		}
		// Boolean literal or field reference
		if tok.Literal == "true" || tok.Literal == "false" {
			p.advance()
			return &StringLit{Value: tok.Literal}, nil
		}
		return nil, p.errorf("expected value (string or integer), got identifier %q", tok.Literal)
	default:
		return nil, p.errorf("expected value (string or integer), got %s %q", tok.Type, tok.Literal)
	}
}

// --- Transform parsers ---

func (p *Parser) parseSelect() (*SelectStage, error) {
	p.advance() // consume "select"
	stage := &SelectStage{}

	expr, err := p.parseSelectExpr()
	if err != nil {
		return nil, err
	}
	stage.Exprs = append(stage.Exprs, expr)
	stage.Fields = append(stage.Fields, selectExprFieldName(expr))

	for p.current().Type == TokenComma {
		p.advance() // consume ,
		expr, err := p.parseSelectExpr()
		if err != nil {
			return nil, err
		}
		stage.Exprs = append(stage.Exprs, expr)
		stage.Fields = append(stage.Fields, selectExprFieldName(expr))
	}

	return stage, nil
}

func (p *Parser) parseSelectExpr() (Expr, error) {
	expr, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}

	// Check for "as alias"
	if p.isIdent("as") {
		p.advance() // consume "as"
		aliasTok := p.current()
		if aliasTok.Type != TokenIdent {
			return nil, p.errorf("expected alias name after 'as', got %s %q", aliasTok.Type, aliasTok.Literal)
		}
		alias := aliasTok.Literal
		p.advance()
		return &AliasExpr{Expr: expr, Alias: alias}, nil
	}

	return expr, nil
}

func selectExprFieldName(expr Expr) string {
	switch e := expr.(type) {
	case *FieldExpr:
		return e.Name
	case *AliasExpr:
		return e.Alias
	case *FuncCallExpr:
		return e.Name
	default:
		return "expr"
	}
}

func (p *Parser) parseSort() (*SortStage, error) {
	p.advance() // consume "sort"

	fieldTok := p.current()
	if fieldTok.Type != TokenIdent {
		return nil, p.errorf("expected field name after 'sort', got %s %q", fieldTok.Type, fieldTok.Literal)
	}
	stage := &SortStage{Field: fieldTok.Literal}
	p.advance()

	// Optional asc/desc
	if p.isIdent("asc") {
		p.advance()
	} else if p.isIdent("desc") {
		stage.Desc = true
		p.advance()
	}

	return stage, nil
}

func (p *Parser) parseLimit() (*LimitStage, error) {
	kind := p.current().Literal
	p.advance()

	n, err := p.expectInteger()
	if err != nil {
		return nil, err
	}

	return &LimitStage{Kind: kind, Count: n}, nil
}

// --- Action parsers ---

func (p *Parser) parseAction() (*ActionStage, error) {
	tok := p.current()
	p.advance()

	stage := &ActionStage{Kind: tok.Literal}

	switch tok.Literal {
	case "cherry-pick":
		if !p.isIdent("onto") {
			return nil, p.errorf("expected 'onto' after 'cherry-pick'")
		}
		p.advance() // consume "onto"
		target, err := p.expectStringOrIdent()
		if err != nil {
			return nil, err
		}
		stage.Target = target

	case "rebase":
		// "rebase onto <branch>" or "rebase interactive <base>"
		if p.isIdent("interactive") {
			p.advance()
			base, err := p.expectStringOrIdent()
			if err != nil {
				return nil, err
			}
			return &ActionStage{Kind: "rebase"}, nil
			_ = base // interactive rebase is handled separately
		}
		if !p.isIdent("onto") {
			return nil, p.errorf("expected 'onto' after 'rebase'")
		}
		p.advance() // consume "onto"
		target, err := p.expectStringOrIdent()
		if err != nil {
			return nil, err
		}
		stage.Target = target

	case "tag":
		target, err := p.expectStringOrIdent()
		if err != nil {
			return nil, err
		}
		stage.Target = target
		// Optional "message" keyword for annotated tags
		if p.isIdent("message") {
			p.advance()
			msg, err := p.expectString()
			if err != nil {
				return nil, err
			}
			stage.Args = append(stage.Args, msg)
		}

	case "revert", "log", "diff", "show":
		// No additional arguments
	}

	// Parse trailing flags
	stage.Flags = p.parseFlags()

	return stage, nil
}

func (p *Parser) parseDeleteAction() (*ActionStage, error) {
	p.advance() // consume "delete"
	stage := &ActionStage{Kind: "delete"}
	stage.Flags = p.parseFlags()
	return stage, nil
}

func (p *Parser) parsePushAction() (Stage, error) {
	p.advance() // consume "push"

	// If next token is a pipe, this is a push action on a piped result set (e.g., tags | where ... | push)
	if p.current().Type == TokenPipe {
		return &ActionStage{Kind: "push"}, nil
	}

	// Standalone push command: push ["remote"] ["branch"] [flags]
	// Also handles bare "push" (no args) as standalone
	ps := &PushStage{}
	if p.current().Type == TokenString || (p.current().Type == TokenIdent && p.current().Literal != "as") {
		ps.Remote = p.current().Literal
		p.advance()
	}
	if p.current().Type == TokenString || (p.current().Type == TokenIdent && p.current().Literal != "as") {
		ps.Branch = p.current().Literal
		p.advance()
	}
	for p.current().Type == TokenFlag {
		switch p.current().Literal {
		case "--force":
			ps.Force = true
		case "--set-upstream":
			ps.SetUpstream = true
		}
		p.advance()
	}
	return ps, nil
}

func (p *Parser) parsePullAction() (Stage, error) {
	p.advance() // consume "pull"
	ps := &PullStage{}
	if p.current().Type == TokenString || p.current().Type == TokenIdent {
		ps.Remote = p.current().Literal
		p.advance()
	}
	if p.current().Type == TokenString || p.current().Type == TokenIdent {
		ps.Branch = p.current().Literal
		p.advance()
	}
	return ps, nil
}

func (p *Parser) parseStashAction() (Stage, error) {
	kind := p.current().Literal
	p.advance()
	return &ActionStage{Kind: kind, Flags: p.parseFlags()}, nil
}

func (p *Parser) parseStageAction() (Stage, error) {
	p.advance() // consume "stage"
	return &ActionStage{Kind: "stage", Flags: p.parseFlags()}, nil
}

func (p *Parser) parseUnstageAction() (Stage, error) {
	p.advance() // consume "unstage"
	return &ActionStage{Kind: "unstage", Flags: p.parseFlags()}, nil
}

func (p *Parser) parseResolveAction() (Stage, error) {
	p.advance() // consume "resolve"
	stage := &ActionStage{Kind: "resolve"}
	// Optional strategy: "ours", "theirs"
	if p.current().Type == TokenString || p.current().Type == TokenIdent {
		stage.Target = p.current().Literal
		p.advance()
	}
	return stage, nil
}

func (p *Parser) parseSquashAction() (Stage, error) {
	p.advance() // consume "squash"
	stage := &ActionStage{Kind: "squash"}
	if p.current().Type == TokenString {
		stage.Target = p.current().Literal
		p.advance()
	}
	return stage, nil
}

func (p *Parser) parseReorderAction() (Stage, error) {
	p.advance() // consume "reorder"
	stage := &ActionStage{Kind: "reorder"}
	// Parse comma-separated indices
	for p.current().Type == TokenInteger {
		stage.Args = append(stage.Args, p.current().Literal)
		p.advance()
		if p.current().Type == TokenComma {
			p.advance()
		}
	}
	return stage, nil
}

// --- Standalone command parsers ---

func (p *Parser) parseBranchCommand() (Stage, error) {
	p.advance() // consume "branch"

	if p.isIdent("create") {
		p.advance() // consume "create"
		name, err := p.expectStringOrIdent()
		if err != nil {
			return nil, p.errorf("expected branch name after 'branch create'")
		}
		stage := &BranchCreateStage{Name: name}
		if p.isIdent("from") {
			p.advance()
			from, err := p.expectStringOrIdent()
			if err != nil {
				return nil, err
			}
			stage.From = from
		}
		return stage, nil
	}

	// Otherwise it's the "branches" source (allows "branch" as alias)
	return p.parseSource()
}

func (p *Parser) parseMergeCommand() (Stage, error) {
	p.advance() // consume "merge"
	branch, err := p.expectStringOrIdent()
	if err != nil {
		return nil, p.errorf("expected branch name after 'merge'")
	}
	stage := &MergeStage{Branch: branch}
	for p.current().Type == TokenFlag {
		switch p.current().Literal {
		case "--no-ff":
			stage.NoFF = true
		}
		p.advance()
	}
	return stage, nil
}

func (p *Parser) parseCommitCommand() (Stage, error) {
	p.advance() // consume "commit"
	msg, err := p.expectString()
	if err != nil {
		return nil, p.errorf("expected commit message string after 'commit'")
	}
	stage := &CommitStage{Message: msg}
	for p.current().Type == TokenFlag {
		switch p.current().Literal {
		case "--amend":
			stage.Amend = true
		}
		p.advance()
	}
	return stage, nil
}

func (p *Parser) parseAbortCommand() (Stage, error) {
	p.advance() // consume "abort"
	tok := p.current()
	if tok.Type != TokenIdent {
		return nil, p.errorf("expected operation after 'abort' (merge, rebase, cherry-pick, revert)")
	}
	switch tok.Literal {
	case "merge", "rebase", "cherry-pick", "revert":
		p.advance()
		return &AbortStage{Operation: tok.Literal}, nil
	default:
		return nil, p.errorf("unknown operation %q for abort, expected merge/rebase/cherry-pick/revert", tok.Literal)
	}
}

func (p *Parser) parseContinueCommand() (Stage, error) {
	p.advance() // consume "continue"
	tok := p.current()
	if tok.Type != TokenIdent || tok.Literal != "rebase" {
		return nil, p.errorf("expected 'rebase' after 'continue'")
	}
	p.advance()
	return &ContinueStage{Operation: "rebase"}, nil
}

// --- Date parsers ---

// parseDateWithin parses: within last N days/hours/weeks/months/years
func (p *Parser) parseDateWithin(fieldName string) (*DateWithinExpr, error) {
	p.advance() // consume "within"

	if !p.isIdent("last") {
		return nil, p.errorf("expected 'last' after 'within'")
	}
	p.advance() // consume "last"

	n, err := p.expectInteger()
	if err != nil {
		return nil, err
	}

	unit := p.current()
	if unit.Type != TokenIdent || !isTimeUnit(unit.Literal) {
		return nil, p.errorf("expected time unit (days, hours, weeks, months, years), got %s %q", unit.Type, unit.Literal)
	}
	p.advance()

	return &DateWithinExpr{Field: fieldName, N: n, Unit: unit.Literal}, nil
}

// parseDateBetween parses: between "start" and "end"
func (p *Parser) parseDateBetween(fieldName string) (*DateBetweenExpr, error) {
	p.advance() // consume "between"

	start, err := p.expectString()
	if err != nil {
		return nil, err
	}

	if !p.isIdent("and") {
		return nil, p.errorf("expected 'and' after start date in 'between'")
	}
	p.advance() // consume "and"

	end, err := p.expectString()
	if err != nil {
		return nil, err
	}

	return &DateBetweenExpr{Field: fieldName, Start: start, End: end}, nil
}

// --- Aggregation parsers ---

func (p *Parser) parseGroupBy() (*GroupByStage, error) {
	p.advance() // consume "group"
	if !p.isIdent("by") {
		return nil, p.errorf("expected 'by' after 'group'")
	}
	p.advance() // consume "by"

	stage := &GroupByStage{}

	fieldTok := p.current()
	if fieldTok.Type != TokenIdent {
		return nil, p.errorf("expected field name after 'group by', got %s %q", fieldTok.Type, fieldTok.Literal)
	}

	// Check for temporal function: group by week(date), month(date), year(date)
	if isTemporalFunc(fieldTok.Literal) && p.peek(1).Type == TokenLParen {
		stage.Func = fieldTok.Literal
		p.advance() // consume function name
		p.advance() // consume (
		innerTok := p.current()
		if innerTok.Type != TokenIdent || innerTok.Literal != "date" {
			return nil, p.errorf("temporal grouping functions only work with 'date' field")
		}
		stage.Field = innerTok.Literal
		p.advance()
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	} else {
		if !isGroupableField(fieldTok.Literal) {
			return nil, p.errorf("cannot group by %q, expected one of: author, branch, tag, date", fieldTok.Literal)
		}
		stage.Field = fieldTok.Literal
		p.advance()
	}

	// Phase 5: multi-group support (group by author, branch)
	stage.Fields = append(stage.Fields, stage.Field)
	for p.current().Type == TokenComma {
		p.advance()
		nextField := p.current()
		if nextField.Type != TokenIdent {
			break
		}
		stage.Fields = append(stage.Fields, nextField.Literal)
		p.advance()
	}

	return stage, nil
}

func (p *Parser) parseAggregate() (*AggregateStage, error) {
	funcName := p.current().Literal
	p.advance()

	fieldTok := p.current()
	if fieldTok.Type != TokenIdent {
		return nil, p.errorf("expected numeric field after '%s', got %s %q", funcName, fieldTok.Type, fieldTok.Literal)
	}
	if !isNumericField(fieldTok.Literal) {
		return nil, p.errorf("'%s' requires a numeric field (additions, deletions), got %q", funcName, fieldTok.Literal)
	}
	p.advance()
	return &AggregateStage{Func: funcName, Field: fieldTok.Literal}, nil
}

func (p *Parser) parseAdvancedAggregate() (*AggregateStage, error) {
	funcName := p.current().Literal
	p.advance()

	fieldTok := p.current()
	if fieldTok.Type != TokenIdent {
		return nil, p.errorf("expected field after '%s', got %s %q", funcName, fieldTok.Type, fieldTok.Literal)
	}

	field := fieldTok.Literal
	p.advance()

	return &AggregateStage{Func: funcName, Field: field}, nil
}

// --- Format parser ---

func (p *Parser) parseFormat() (*FormatStage, error) {
	p.advance() // consume "format"

	tok := p.current()
	if tok.Type != TokenIdent {
		return nil, p.errorf("expected format type after 'format', got %s %q", tok.Type, tok.Literal)
	}
	if !isValidFormat(tok.Literal) {
		return nil, p.errorf("unknown format %q, expected one of: json, csv, table, markdown, yaml", tok.Literal)
	}
	p.advance()
	return &FormatStage{Format: tok.Literal}, nil
}

// --- Set operation parser ---

func (p *Parser) parseSetOp() (*SetOpStage, error) {
	op := p.current().Literal
	p.advance() // consume "except", "intersect", or "union"

	// Expect a source stage (commits, branches, tags)
	tok := p.current()
	if tok.Type != TokenIdent || (tok.Literal != "commits" && tok.Literal != "branches" && tok.Literal != "tags" && tok.Literal != "files") {
		return nil, p.errorf("expected source (commits, branches, tags, files) after '%s', got %s %q", op, tok.Type, tok.Literal)
	}

	source, err := p.parseSource()
	if err != nil {
		return nil, err
	}

	return &SetOpStage{Op: op, Source: source}, nil
}

// --- Having parser ---

func (p *Parser) parseHaving() (*HavingStage, error) {
	p.advance() // consume "having"
	cond, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	return &HavingStage{Condition: cond}, nil
}

// --- Help parser ---

func (p *Parser) parseHelp() (*HelpStage, error) {
	p.advance() // consume "help"

	// Optional topic
	tok := p.current()
	if tok.Type == TokenIdent && tok.Type != TokenEOF {
		topic := tok.Literal
		p.advance()
		return &HelpStage{Topic: topic}, nil
	}

	return &HelpStage{}, nil
}

// --- Blame parser ---

func (p *Parser) parseBlameSource() (*BlameSourceStage, error) {
	p.advance() // consume "blame"
	filePath, err := p.expectString()
	if err != nil {
		return nil, p.errorf("expected file path string after 'blame'")
	}
	return &BlameSourceStage{FilePath: filePath}, nil
}

// --- Phase 3: Let parser ---

func (p *Parser) parseLetStage() (*LetStage, error) {
	p.advance() // consume "let"
	nameTok := p.current()
	if nameTok.Type != TokenIdent {
		return nil, p.errorf("expected variable name after 'let'")
	}
	name := nameTok.Literal
	p.advance()

	// Expect =
	if p.current().Type != TokenEq {
		return nil, p.errorf("expected '==' after variable name in 'let' (use let name == value)")
	}
	// Actually, let's accept == as assignment in this context
	p.advance()

	expr, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	return &LetStage{Name: name, Expr: expr}, nil
}

// --- Phase 4: Export, explain ---

func (p *Parser) parseExport() (Stage, error) {
	p.advance() // consume "export"
	path, err := p.expectString()
	if err != nil {
		return nil, p.errorf("expected file path string after 'export'")
	}
	return &ExportStage{Path: path}, nil
}

func (p *Parser) parseExplain() (Stage, error) {
	p.advance() // consume "explain"
	// Parse the rest as a pipeline
	inner, err := p.parsePipeline()
	if err != nil {
		return nil, err
	}
	return &ExplainStage{Inner: inner}, nil
}

// --- Phase 5: Window functions ---

func (p *Parser) parseWindowFunction() (Stage, error) {
	funcName := p.current().Literal
	p.advance()

	// row_number doesn't require a field
	field := ""
	if funcName == "row_number" {
		// No field needed
	} else {
		fieldTok := p.current()
		if fieldTok.Type != TokenIdent || fieldTok.Literal == "as" {
			return nil, p.errorf("expected field after '%s'", funcName)
		}
		field = fieldTok.Literal
		p.advance()
	}

	alias := funcName
	if field != "" {
		alias = funcName + "_" + field
	}
	if p.isIdent("as") {
		p.advance()
		aliasTok := p.current()
		if aliasTok.Type != TokenIdent {
			return nil, p.errorf("expected alias name after 'as'")
		}
		alias = aliasTok.Literal
		p.advance()
	}

	return &WindowStage{Func: funcName, Field: field, Alias: alias}, nil
}

func (p *Parser) parseMovingAvg() (Stage, error) {
	p.advance() // consume "moving_avg"

	fieldTok := p.current()
	if fieldTok.Type != TokenIdent {
		return nil, p.errorf("expected field after 'moving_avg'")
	}
	field := fieldTok.Literal
	p.advance()

	// Window size
	window := 7 // default
	if p.current().Type == TokenInteger {
		n, _ := strconv.Atoi(p.current().Literal)
		window = n
		p.advance()
	}

	alias := "moving_avg_" + field
	if p.isIdent("as") {
		p.advance()
		aliasTok := p.current()
		if aliasTok.Type != TokenIdent {
			return nil, p.errorf("expected alias name after 'as'")
		}
		alias = aliasTok.Literal
		p.advance()
	}

	return &WindowStage{Func: "moving_avg", Field: field, Window: window, Alias: alias}, nil
}

// --- Utility functions ---

func (p *Parser) parseFlags() []string {
	var flags []string
	for p.current().Type == TokenFlag {
		flags = append(flags, p.current().Literal)
		p.advance()
	}
	return flags
}

func isValidFormat(name string) bool {
	switch name {
	case "json", "csv", "table", "markdown", "yaml":
		return true
	}
	return false
}

func isGroupableField(name string) bool {
	switch name {
	case "author", "branch", "tag", "date":
		return true
	}
	return false
}

func isNumericField(name string) bool {
	switch name {
	case "additions", "deletions":
		return true
	}
	return false
}

func isTimeUnit(s string) bool {
	switch s {
	case "days", "hours", "weeks", "months", "years":
		return true
	}
	return false
}

func isTemporalFunc(name string) bool {
	switch name {
	case "week", "month", "year", "day_of_week", "hour":
		return true
	}
	return false
}

func isFuncName(name string) bool {
	switch name {
	// String functions
	case "upper", "lower", "trim", "len", "substr", "split", "replace", "starts_with", "ends_with":
		return true
	// Date functions
	case "days_since", "hours_since", "date_add", "date_sub", "format_date", "day_of_week", "hour", "month", "year", "week":
		return true
	}
	return false
}

// isFieldName returns whether a name is a valid commit field.
func isFieldName(name string) bool {
	switch name {
	case "author", "message", "date", "hash", "branch", "tag", "files", "additions", "deletions", "path", "commits",
		"value", "group", "content", "lineno", "index":
		return true
	}
	return false
}

// isExtendedFieldName includes fields from new source types (Phase 1-2).
func isExtendedFieldName(name string) bool {
	switch name {
	// Branch fields
	case "name", "remote", "merged", "ahead", "behind", "isCurrent":
		return true
	// Status fields
	case "status", "staged":
		return true
	// Reflog fields
	case "action":
		return true
	}
	return false
}

// --- Helper methods ---

func (p *Parser) current() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokenEOF, Pos: -1}
}

func (p *Parser) peek(offset int) Token {
	idx := p.pos + offset
	if idx < len(p.tokens) {
		return p.tokens[idx]
	}
	return Token{Type: TokenEOF, Pos: -1}
}

func (p *Parser) advance() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

func (p *Parser) isIdent(name string) bool {
	tok := p.current()
	return tok.Type == TokenIdent && tok.Literal == name
}

func (p *Parser) expect(tt TokenType) error {
	tok := p.current()
	if tok.Type != tt {
		return p.errorf("expected %s, got %s %q", tt, tok.Type, tok.Literal)
	}
	p.advance()
	return nil
}

func (p *Parser) expectInteger() (int, error) {
	tok := p.current()
	if tok.Type != TokenInteger {
		return 0, p.errorf("expected integer, got %s %q", tok.Type, tok.Literal)
	}
	n, err := strconv.Atoi(tok.Literal)
	if err != nil {
		return 0, p.errorf("invalid integer %q", tok.Literal)
	}
	p.advance()
	return n, nil
}

func (p *Parser) expectString() (string, error) {
	tok := p.current()
	if tok.Type != TokenString {
		return "", p.errorf("expected string, got %s %q", tok.Type, tok.Literal)
	}
	p.advance()
	return tok.Literal, nil
}

func (p *Parser) expectStringOrIdent() (string, error) {
	tok := p.current()
	if tok.Type == TokenString || tok.Type == TokenIdent {
		p.advance()
		return tok.Literal, nil
	}
	return "", p.errorf("expected string or identifier, got %s %q", tok.Type, tok.Literal)
}

func (p *Parser) errorf(format string, args ...any) error {
	pos := 0
	if p.pos < len(p.tokens) {
		pos = p.tokens[p.pos].Pos
	}
	return &DSLError{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	}
}
