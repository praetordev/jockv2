package dsl

import (
	"fmt"
	"unicode"
)

// Lexer tokenizes a DSL input string.
type Lexer struct {
	input []rune
	pos   int
}

// NewLexer creates a lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: []rune(input)}
}

// Tokenize scans the entire input and returns all tokens.
func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return tokens, nil
}

func (l *Lexer) next() (Token, error) {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Pos: l.pos}, nil
	}

	ch := l.input[l.pos]
	start := l.pos

	switch ch {
	case '|':
		l.pos++
		return Token{Type: TokenPipe, Literal: "|", Pos: start}, nil
	case ',':
		l.pos++
		return Token{Type: TokenComma, Literal: ",", Pos: start}, nil
	case '(':
		l.pos++
		return Token{Type: TokenLParen, Literal: "(", Pos: start}, nil
	case ')':
		l.pos++
		return Token{Type: TokenRParen, Literal: ")", Pos: start}, nil
	case '[':
		l.pos++
		return Token{Type: TokenLBracket, Literal: "[", Pos: start}, nil
	case ']':
		l.pos++
		return Token{Type: TokenRBracket, Literal: "]", Pos: start}, nil
	case '{':
		l.pos++
		return Token{Type: TokenLBrace, Literal: "{", Pos: start}, nil
	case '}':
		l.pos++
		return Token{Type: TokenRBrace, Literal: "}", Pos: start}, nil
	case '+':
		l.pos++
		return Token{Type: TokenPlus, Literal: "+", Pos: start}, nil
	case '*':
		l.pos++
		return Token{Type: TokenStar, Literal: "*", Pos: start}, nil
	case '/':
		l.pos++
		return Token{Type: TokenSlash, Literal: "/", Pos: start}, nil

	case '$':
		return l.readVariable()

	case '-':
		// Check for -- flag
		if l.peek(1) == '-' {
			return l.readFlag()
		}
		l.pos++
		return Token{Type: TokenMinus, Literal: "-", Pos: start}, nil

	case '=':
		if l.peek(1) == '=' {
			l.pos += 2
			return Token{Type: TokenEq, Literal: "==", Pos: start}, nil
		}
		return Token{}, l.errorf(start, "unexpected '=', did you mean '=='?")

	case '!':
		if l.peek(1) == '=' {
			l.pos += 2
			return Token{Type: TokenNeq, Literal: "!=", Pos: start}, nil
		}
		return Token{}, l.errorf(start, "unexpected '!', did you mean '!='?")

	case '>':
		if l.peek(1) == '=' {
			l.pos += 2
			return Token{Type: TokenGte, Literal: ">=", Pos: start}, nil
		}
		l.pos++
		return Token{Type: TokenGt, Literal: ">", Pos: start}, nil

	case '<':
		if l.peek(1) == '=' {
			l.pos += 2
			return Token{Type: TokenLte, Literal: "<=", Pos: start}, nil
		}
		l.pos++
		return Token{Type: TokenLt, Literal: "<", Pos: start}, nil

	case '.':
		if l.peek(1) == '.' {
			l.pos += 2
			return Token{Type: TokenDotDot, Literal: "..", Pos: start}, nil
		}
		return Token{}, l.errorf(start, "unexpected '.', did you mean '..'?")

	case '"':
		return l.readString()
	}

	if unicode.IsDigit(ch) {
		return l.readInteger()
	}

	if isIdentStart(ch) {
		return l.readIdent()
	}

	return Token{}, l.errorf(start, "unexpected character %q", ch)
}

func (l *Lexer) readString() (Token, error) {
	start := l.pos
	l.pos++ // skip opening quote
	var result []rune
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) {
			next := l.input[l.pos+1]
			switch next {
			case '"', '\\':
				result = append(result, next)
				l.pos += 2
				continue
			}
		}
		if ch == '"' {
			l.pos++ // skip closing quote
			return Token{Type: TokenString, Literal: string(result), Pos: start}, nil
		}
		result = append(result, ch)
		l.pos++
	}
	return Token{}, l.errorf(start, "unterminated string")
}

func (l *Lexer) readInteger() (Token, error) {
	start := l.pos
	for l.pos < len(l.input) && unicode.IsDigit(l.input[l.pos]) {
		l.pos++
	}
	return Token{Type: TokenInteger, Literal: string(l.input[start:l.pos]), Pos: start}, nil
}

func (l *Lexer) readIdent() (Token, error) {
	start := l.pos
	for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
		l.pos++
	}
	return Token{Type: TokenIdent, Literal: string(l.input[start:l.pos]), Pos: start}, nil
}

func (l *Lexer) readFlag() (Token, error) {
	start := l.pos
	l.pos += 2 // skip --
	for l.pos < len(l.input) && (unicode.IsLetter(l.input[l.pos]) || l.input[l.pos] == '-') {
		l.pos++
	}
	flag := string(l.input[start:l.pos])
	if flag == "--" {
		return Token{}, l.errorf(start, "empty flag '--'")
	}
	return Token{Type: TokenFlag, Literal: flag, Pos: start}, nil
}

func (l *Lexer) readVariable() (Token, error) {
	start := l.pos
	l.pos++ // skip $
	if l.pos >= len(l.input) || !isIdentStart(l.input[l.pos]) {
		return Token{}, l.errorf(start, "expected variable name after '$'")
	}
	nameStart := l.pos
	for l.pos < len(l.input) && isIdentChar(l.input[l.pos]) {
		l.pos++
	}
	return Token{Type: TokenDollar, Literal: string(l.input[nameStart:l.pos]), Pos: start}, nil
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
}

func (l *Lexer) peek(offset int) rune {
	idx := l.pos + offset
	if idx < len(l.input) {
		return l.input[idx]
	}
	return 0
}

func (l *Lexer) errorf(pos int, format string, args ...any) error {
	return &DSLError{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	}
}

func isIdentStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_' || ch == '-' || ch == '~'
}

func isIdentChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-' || ch == '~' || ch == '/'
}
