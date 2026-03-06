package dsl

// TokenType classifies a lexer token.
type TokenType int

const (
	// Literals
	TokenString  TokenType = iota // "quoted string"
	TokenInteger                  // 42
	TokenIdent                    // unquoted identifier (keywords, field names)

	// Operators
	TokenPipe   // |
	TokenEq     // ==
	TokenNeq    // !=
	TokenGt     // >
	TokenLt     // <
	TokenGte    // >=
	TokenLte    // <=
	TokenDotDot // ..
	TokenPlus   // +
	TokenMinus  // -
	TokenStar   // *
	TokenSlash  // /

	// Delimiters
	TokenLBracket // [
	TokenRBracket // ]
	TokenLParen   // (
	TokenRParen   // )
	TokenComma    // ,
	TokenLBrace   // {
	TokenRBrace   // }

	// Prefix
	TokenFlag   // --flag
	TokenDollar // $variable

	// Special
	TokenEOF
)

// Token is a single lexical unit from the input.
type Token struct {
	Type    TokenType
	Literal string
	Pos     int // byte offset in input
}

// String names for token types, used in error messages.
var tokenNames = map[TokenType]string{
	TokenString:   "string",
	TokenInteger:  "integer",
	TokenIdent:    "identifier",
	TokenPipe:     "|",
	TokenEq:       "==",
	TokenNeq:      "!=",
	TokenGt:       ">",
	TokenLt:       "<",
	TokenGte:      ">=",
	TokenLte:      "<=",
	TokenDotDot:   "..",
	TokenPlus:     "+",
	TokenMinus:    "-",
	TokenStar:     "*",
	TokenSlash:    "/",
	TokenLBracket: "[",
	TokenRBracket: "]",
	TokenLParen:   "(",
	TokenRParen:   ")",
	TokenComma:    ",",
	TokenLBrace:   "{",
	TokenRBrace:   "}",
	TokenFlag:     "flag",
	TokenDollar:   "$",
	TokenEOF:      "EOF",
}

func (t TokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	return "unknown"
}
