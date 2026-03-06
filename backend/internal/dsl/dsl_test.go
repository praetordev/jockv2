package dsl

import (
	"testing"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		input  string
		tokens []TokenType
	}{
		{
			input:  `commits`,
			tokens: []TokenType{TokenIdent, TokenEOF},
		},
		{
			input:  `commits | where author == "john"`,
			tokens: []TokenType{TokenIdent, TokenPipe, TokenIdent, TokenIdent, TokenEq, TokenString, TokenEOF},
		},
		{
			input:  `commits | first 10`,
			tokens: []TokenType{TokenIdent, TokenPipe, TokenIdent, TokenInteger, TokenEOF},
		},
		{
			input:  `commits[main..feature]`,
			tokens: []TokenType{TokenIdent, TokenLBracket, TokenIdent, TokenDotDot, TokenIdent, TokenRBracket, TokenEOF},
		},
		{
			input:  `commits | where additions >= 100 and deletions < 50`,
			tokens: []TokenType{TokenIdent, TokenPipe, TokenIdent, TokenIdent, TokenGte, TokenInteger, TokenIdent, TokenIdent, TokenLt, TokenInteger, TokenEOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			tokens, err := lexer.Tokenize()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(tokens) != len(tt.tokens) {
				t.Fatalf("expected %d tokens, got %d: %v", len(tt.tokens), len(tokens), tokens)
			}
			for i, tok := range tokens {
				if tok.Type != tt.tokens[i] {
					t.Errorf("token %d: expected %s, got %s (%q)", i, tt.tokens[i], tok.Type, tok.Literal)
				}
			}
		})
	}
}

func TestLexerErrors(t *testing.T) {
	tests := []struct {
		input string
		errAt int
	}{
		{`"unterminated`, 0},
		{`commits | = oops`, 10},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			_, err := lexer.Tokenize()
			if err == nil {
				t.Fatal("expected error, got none")
			}
			dslErr, ok := err.(*DSLError)
			if !ok {
				t.Fatalf("expected DSLError, got %T: %v", err, err)
			}
			if dslErr.Pos != tt.errAt {
				t.Errorf("expected error at position %d, got %d", tt.errAt, dslErr.Pos)
			}
		})
	}
}

func TestParseSource(t *testing.T) {
	tests := []struct {
		input string
		kind  string
	}{
		{"commits", "commits"},
		{"branches", "branches"},
		{"tags", "tags"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(p.Stages) != 1 {
				t.Fatalf("expected 1 stage, got %d", len(p.Stages))
			}
			src, ok := p.Stages[0].(*SourceStage)
			if !ok {
				t.Fatalf("expected SourceStage, got %T", p.Stages[0])
			}
			if src.Kind != tt.kind {
				t.Errorf("expected kind %q, got %q", tt.kind, src.Kind)
			}
		})
	}
}

func TestParseSourceWithRange(t *testing.T) {
	t.Run("branch ref", func(t *testing.T) {
		p, err := Parse("commits[main]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		src := p.Stages[0].(*SourceStage)
		if src.Range == nil {
			t.Fatal("expected range, got nil")
		}
		if src.Range.Ref != "main" {
			t.Errorf("expected ref 'main', got %q", src.Range.Ref)
		}
	})

	t.Run("dotdot range", func(t *testing.T) {
		p, err := Parse("commits[main..feature]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		src := p.Stages[0].(*SourceStage)
		if src.Range.From != "main" || src.Range.To != "feature" {
			t.Errorf("expected main..feature, got %s..%s", src.Range.From, src.Range.To)
		}
	})

	t.Run("last N", func(t *testing.T) {
		p, err := Parse("commits[last 10]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		src := p.Stages[0].(*SourceStage)
		if src.Range.LastN != 10 {
			t.Errorf("expected lastN=10, got %d", src.Range.LastN)
		}
	})
}

func TestParseWhere(t *testing.T) {
	tests := []struct {
		input string
		field string
		op    string
		value string
	}{
		{`commits | where author == "john"`, "author", "==", "john"},
		{`commits | where message contains "fix"`, "message", "contains", "fix"},
		{`commits | where hash != "abc123"`, "hash", "!=", "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(p.Stages) != 2 {
				t.Fatalf("expected 2 stages, got %d", len(p.Stages))
			}
			where, ok := p.Stages[1].(*WhereStage)
			if !ok {
				t.Fatalf("expected WhereStage, got %T", p.Stages[1])
			}
			bin, ok := where.Condition.(*BinaryExpr)
			if !ok {
				t.Fatalf("expected BinaryExpr, got %T", where.Condition)
			}
			field := bin.Left.(*FieldExpr)
			if field.Name != tt.field {
				t.Errorf("expected field %q, got %q", tt.field, field.Name)
			}
			if bin.Op != tt.op {
				t.Errorf("expected op %q, got %q", tt.op, bin.Op)
			}
			str := bin.Right.(*StringLit)
			if str.Value != tt.value {
				t.Errorf("expected value %q, got %q", tt.value, str.Value)
			}
		})
	}
}

func TestParseWhereCompound(t *testing.T) {
	p, err := Parse(`commits | where author == "john" and message contains "fix"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	where := p.Stages[1].(*WhereStage)
	and, ok := where.Condition.(*BinaryExpr)
	if !ok || and.Op != "and" {
		t.Fatalf("expected 'and' BinaryExpr, got %T with op %q", where.Condition, and.Op)
	}

	left := and.Left.(*BinaryExpr)
	if left.Left.(*FieldExpr).Name != "author" {
		t.Error("expected left field 'author'")
	}

	right := and.Right.(*BinaryExpr)
	if right.Left.(*FieldExpr).Name != "message" {
		t.Error("expected right field 'message'")
	}
}

func TestParseWhereOr(t *testing.T) {
	p, err := Parse(`commits | where author == "john" or author == "jane"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	where := p.Stages[1].(*WhereStage)
	or, ok := where.Condition.(*BinaryExpr)
	if !ok || or.Op != "or" {
		t.Fatalf("expected 'or' BinaryExpr")
	}
}

func TestParseWhereNot(t *testing.T) {
	p, err := Parse(`commits | where not author == "bot"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	where := p.Stages[1].(*WhereStage)
	un, ok := where.Condition.(*UnaryExpr)
	if !ok || un.Op != "not" {
		t.Fatalf("expected UnaryExpr with 'not'")
	}
}

func TestParsePipeline(t *testing.T) {
	p, err := Parse(`commits | where author == "john" | sort date desc | first 5`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(p.Stages))
	}

	if _, ok := p.Stages[0].(*SourceStage); !ok {
		t.Error("stage 0: expected SourceStage")
	}
	if _, ok := p.Stages[1].(*WhereStage); !ok {
		t.Error("stage 1: expected WhereStage")
	}
	if s, ok := p.Stages[2].(*SortStage); !ok || s.Field != "date" || !s.Desc {
		t.Error("stage 2: expected SortStage with date desc")
	}
	if l, ok := p.Stages[3].(*LimitStage); !ok || l.Kind != "first" || l.Count != 5 {
		t.Error("stage 3: expected LimitStage first 5")
	}
}

func TestParseAction(t *testing.T) {
	tests := []struct {
		input  string
		action string
		target string
	}{
		{`commits | first 1 | cherry-pick onto "main"`, "cherry-pick", "main"},
		{`commits | first 1 | revert`, "revert", ""},
		{`commits | first 1 | rebase onto "develop"`, "rebase", "develop"},
		{`commits | first 1 | tag "v1.0.0"`, "tag", "v1.0.0"},
		{`commits | log`, "log", ""},
		{`commits | diff`, "diff", ""},
		{`commits | show`, "show", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			last := p.Stages[len(p.Stages)-1]
			action, ok := last.(*ActionStage)
			if !ok {
				t.Fatalf("expected ActionStage, got %T", last)
			}
			if action.Kind != tt.action {
				t.Errorf("expected action %q, got %q", tt.action, action.Kind)
			}
			if action.Target != tt.target {
				t.Errorf("expected target %q, got %q", tt.target, action.Target)
			}
		})
	}
}

func TestParseTransforms(t *testing.T) {
	t.Run("select", func(t *testing.T) {
		p, err := Parse("commits | select hash, message, author")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sel := p.Stages[1].(*SelectStage)
		if len(sel.Fields) != 3 {
			t.Fatalf("expected 3 fields, got %d", len(sel.Fields))
		}
		expected := []string{"hash", "message", "author"}
		for i, f := range sel.Fields {
			if f != expected[i] {
				t.Errorf("field %d: expected %q, got %q", i, expected[i], f)
			}
		}
	})

	t.Run("count", func(t *testing.T) {
		p, err := Parse("commits | count")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := p.Stages[1].(*CountStage); !ok {
			t.Error("expected CountStage")
		}
	})

	t.Run("unique", func(t *testing.T) {
		p, err := Parse("commits | unique")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := p.Stages[1].(*UniqueStage); !ok {
			t.Error("expected UniqueStage")
		}
	})

	t.Run("reverse", func(t *testing.T) {
		p, err := Parse("commits | reverse")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := p.Stages[1].(*ReverseStage); !ok {
			t.Error("expected ReverseStage")
		}
	})
}

func TestParseNumericComparison(t *testing.T) {
	p, err := Parse(`commits | where additions > 100`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	where := p.Stages[1].(*WhereStage)
	bin := where.Condition.(*BinaryExpr)
	if bin.Op != ">" {
		t.Errorf("expected op '>', got %q", bin.Op)
	}
	intLit := bin.Right.(*IntLit)
	if intLit.Value != 100 {
		t.Errorf("expected 100, got %d", intLit.Value)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		input string
		desc  string
	}{
		{``, "empty input"},
		{`| where`, "pipe at start"},
		{`commits | where`, "where without condition"},
		{`commits | where foo == "bar"`, "unknown field"},
		{`commits | cherry-pick`, "cherry-pick without onto"},
		{`commits | sort`, "sort without field"},
		{`commits | first`, "first without count"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			_, err := Parse(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParseFileSource(t *testing.T) {
	p, err := Parse("files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	src := p.Stages[0].(*SourceStage)
	if src.Kind != "files" {
		t.Errorf("expected kind 'files', got %q", src.Kind)
	}
}

func TestParseHelp(t *testing.T) {
	tests := []struct {
		input string
		topic string
	}{
		{"help", ""},
		{"help where", "where"},
		{"help format", "format"},
		{"help sort", "sort"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			help, ok := p.Stages[0].(*HelpStage)
			if !ok {
				t.Fatalf("expected HelpStage, got %T", p.Stages[0])
			}
			if help.Topic != tt.topic {
				t.Errorf("expected topic %q, got %q", tt.topic, help.Topic)
			}
		})
	}
}

func TestParseSetOp(t *testing.T) {
	tests := []struct {
		input string
		op    string
	}{
		{"commits | except commits", "except"},
		{"commits | intersect commits", "intersect"},
		{"commits | union commits", "union"},
		{"commits[main] | except commits[develop]", "except"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// SetOpStage should be the second stage
			setOp, ok := p.Stages[1].(*SetOpStage)
			if !ok {
				t.Fatalf("expected SetOpStage, got %T", p.Stages[1])
			}
			if setOp.Op != tt.op {
				t.Errorf("expected op %q, got %q", tt.op, setOp.Op)
			}
			if setOp.Source == nil {
				t.Fatal("expected non-nil source on SetOpStage")
			}
		})
	}
}

func TestParseFilePipeline(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"files with where", `files | where path contains "main"`},
		{"files with sort", "files | sort additions desc"},
		{"files with first", "files | first 5"},
		{"files with count", "files | count"},
		{"files with format", "files | format json"},
		{"files full pipeline", `files | where additions > 0 | sort additions desc | first 3`},
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

func TestParseBlameSource(t *testing.T) {
	p, err := Parse(`blame "main.go"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(p.Stages))
	}
	blame, ok := p.Stages[0].(*BlameSourceStage)
	if !ok {
		t.Fatalf("expected BlameSourceStage, got %T", p.Stages[0])
	}
	if blame.FilePath != "main.go" {
		t.Fatalf("expected file path %q, got %q", "main.go", blame.FilePath)
	}
}

func TestParseBlameWithWhere(t *testing.T) {
	p, err := Parse(`blame "main.go" | where author == "alice"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(p.Stages))
	}
	_, ok := p.Stages[0].(*BlameSourceStage)
	if !ok {
		t.Fatalf("expected BlameSourceStage, got %T", p.Stages[0])
	}
	_, ok = p.Stages[1].(*WhereStage)
	if !ok {
		t.Fatalf("expected WhereStage, got %T", p.Stages[1])
	}
}

func TestParseStashSource(t *testing.T) {
	p, err := Parse(`stash`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(p.Stages))
	}
	_, ok := p.Stages[0].(*StashSourceStage)
	if !ok {
		t.Fatalf("expected StashSourceStage, got %T", p.Stages[0])
	}
}

func TestParseStashWithWhere(t *testing.T) {
	p, err := Parse(`stash | where message contains "wip"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(p.Stages))
	}
}

func TestParseHaving(t *testing.T) {
	p, err := Parse(`commits | group by author | count | having value > 2`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Stages) != 4 {
		t.Fatalf("expected 4 stages, got %d", len(p.Stages))
	}
	having, ok := p.Stages[3].(*HavingStage)
	if !ok {
		t.Fatalf("expected HavingStage, got %T", p.Stages[3])
	}
	bin := having.Condition.(*BinaryExpr)
	if bin.Op != ">" {
		t.Fatalf("expected '>', got %q", bin.Op)
	}
	field := bin.Left.(*FieldExpr)
	if field.Name != "value" {
		t.Fatalf("expected 'value', got %q", field.Name)
	}
}

func TestParseHavingGroupContains(t *testing.T) {
	p, err := Parse(`commits | group by author | count | having group contains "alice"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	having := p.Stages[3].(*HavingStage)
	bin := having.Condition.(*BinaryExpr)
	if bin.Op != "contains" {
		t.Fatalf("expected 'contains', got %q", bin.Op)
	}
	field := bin.Left.(*FieldExpr)
	if field.Name != "group" {
		t.Fatalf("expected 'group', got %q", field.Name)
	}
}

func TestParseWhereParentheses(t *testing.T) {
	p, err := Parse(`commits | where (author == "john" or author == "jane") and message contains "fix"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	where := p.Stages[1].(*WhereStage)
	and := where.Condition.(*BinaryExpr)
	if and.Op != "and" {
		t.Fatalf("expected 'and', got %q", and.Op)
	}

	// Left side should be an 'or' from the parentheses
	or := and.Left.(*BinaryExpr)
	if or.Op != "or" {
		t.Fatalf("expected 'or' in parens, got %q", or.Op)
	}
}
