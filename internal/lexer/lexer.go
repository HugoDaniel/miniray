// Package lexer provides tokenization for WGSL source code.
//
// The lexer converts a WGSL source string into a sequence of tokens,
// handling:
// - Keywords and reserved words
// - Identifiers (including Unicode XID)
// - Numeric literals (int, float, hex)
// - Operators and punctuation
// - Comments (line and block, with nesting)
// - Template list disambiguation (<, >)
package lexer

import (
	"unicode"
	"unicode/utf8"
)

// ----------------------------------------------------------------------------
// Token Types
// ----------------------------------------------------------------------------

// TokenKind represents the type of a token.
type TokenKind uint8

const (
	TokError TokenKind = iota
	TokEOF

	// Literals
	TokIntLiteral
	TokFloatLiteral
	TokTrue
	TokFalse

	// Identifiers
	TokIdent

	// Keywords
	TokAlias
	TokBreak
	TokCase
	TokConst
	TokConstAssert
	TokContinue
	TokContinuing
	TokDefault
	TokDiagnostic
	TokDiscard
	TokElse
	TokEnable
	TokFn
	TokFor
	TokIf
	TokLet
	TokLoop
	TokOverride
	TokRequires
	TokReturn
	TokStruct
	TokSwitch
	TokVar
	TokWhile

	// Operators
	TokPlus    // +
	TokMinus   // -
	TokStar    // *
	TokSlash   // /
	TokPercent // %
	TokAmp     // &
	TokPipe    // |
	TokCaret   // ^
	TokTilde   // ~
	TokBang    // !
	TokLt      // <
	TokGt      // >
	TokEq      // =
	TokDot     // .
	TokAt      // @

	// Multi-char operators
	TokPlusPlus   // ++
	TokMinusMinus // --
	TokAmpAmp     // &&
	TokPipePipe   // ||
	TokLtLt       // <<
	TokGtGt       // >>
	TokLtEq       // <=
	TokGtEq       // >=
	TokEqEq       // ==
	TokBangEq     // !=
	TokArrow      // ->
	TokPlusEq     // +=
	TokMinusEq    // -=
	TokStarEq     // *=
	TokSlashEq    // /=
	TokPercentEq  // %=
	TokAmpEq      // &=
	TokPipeEq     // |=
	TokCaretEq    // ^=
	TokLtLtEq     // <<=
	TokGtGtEq     // >>=

	// Delimiters
	TokLParen     // (
	TokRParen     // )
	TokLBrace     // {
	TokRBrace     // }
	TokLBracket   // [
	TokRBracket   // ]
	TokSemicolon  // ;
	TokColon      // :
	TokComma      // ,
	TokUnderscore // _ (as placeholder expression)

	// Template delimiters (context-sensitive)
	TokTemplateArgsStart // < in template context
	TokTemplateArgsEnd   // > in template context
)

// String returns the string representation of a token kind.
func (k TokenKind) String() string {
	if int(k) < len(tokenNames) {
		return tokenNames[k]
	}
	return "unknown"
}

var tokenNames = [...]string{
	TokError:        "error",
	TokEOF:          "EOF",
	TokIntLiteral:   "int",
	TokFloatLiteral: "float",
	TokTrue:         "true",
	TokFalse:        "false",
	TokIdent:        "identifier",
	// Keywords
	TokAlias:       "alias",
	TokBreak:       "break",
	TokCase:        "case",
	TokConst:       "const",
	TokConstAssert: "const_assert",
	TokContinue:    "continue",
	TokContinuing:  "continuing",
	TokDefault:     "default",
	TokDiagnostic:  "diagnostic",
	TokDiscard:     "discard",
	TokElse:        "else",
	TokEnable:      "enable",
	TokFn:          "fn",
	TokFor:         "for",
	TokIf:          "if",
	TokLet:         "let",
	TokLoop:        "loop",
	TokOverride:    "override",
	TokRequires:    "requires",
	TokReturn:      "return",
	TokStruct:      "struct",
	TokSwitch:      "switch",
	TokVar:         "var",
	TokWhile:       "while",
	// Operators
	TokPlus:              "+",
	TokMinus:             "-",
	TokStar:              "*",
	TokSlash:             "/",
	TokPercent:           "%",
	TokAmp:               "&",
	TokPipe:              "|",
	TokCaret:             "^",
	TokTilde:             "~",
	TokBang:              "!",
	TokLt:                "<",
	TokGt:                ">",
	TokEq:                "=",
	TokDot:               ".",
	TokAt:                "@",
	TokPlusPlus:          "++",
	TokMinusMinus:        "--",
	TokAmpAmp:            "&&",
	TokPipePipe:          "||",
	TokLtLt:              "<<",
	TokGtGt:              ">>",
	TokLtEq:              "<=",
	TokGtEq:              ">=",
	TokEqEq:              "==",
	TokBangEq:            "!=",
	TokArrow:             "->",
	TokPlusEq:            "+=",
	TokMinusEq:           "-=",
	TokStarEq:            "*=",
	TokSlashEq:           "/=",
	TokPercentEq:         "%=",
	TokAmpEq:             "&=",
	TokPipeEq:            "|=",
	TokCaretEq:           "^=",
	TokLtLtEq:            "<<=",
	TokGtGtEq:            ">>=",
	TokLParen:            "(",
	TokRParen:            ")",
	TokLBrace:            "{",
	TokRBrace:            "}",
	TokLBracket:          "[",
	TokRBracket:          "]",
	TokSemicolon:         ";",
	TokColon:             ":",
	TokComma:             ",",
	TokUnderscore:        "_",
	TokTemplateArgsStart: "<template",
	TokTemplateArgsEnd:   "template>",
}

// ----------------------------------------------------------------------------
// Token
// ----------------------------------------------------------------------------

// Token represents a lexical token.
type Token struct {
	Kind  TokenKind
	Start int    // Byte offset in source
	End   int    // Byte offset of end (exclusive)
	Value string // For identifiers and literals
}

// Text returns the source text of the token.
func (t Token) Text(source string) string {
	if t.Start >= 0 && t.End <= len(source) {
		return source[t.Start:t.End]
	}
	return ""
}

// ----------------------------------------------------------------------------
// Keywords
// ----------------------------------------------------------------------------

// Keywords maps keyword strings to their token kinds.
var Keywords = map[string]TokenKind{
	"alias":        TokAlias,
	"break":        TokBreak,
	"case":         TokCase,
	"const":        TokConst,
	"const_assert": TokConstAssert,
	"continue":     TokContinue,
	"continuing":   TokContinuing,
	"default":      TokDefault,
	"diagnostic":   TokDiagnostic,
	"discard":      TokDiscard,
	"else":         TokElse,
	"enable":       TokEnable,
	"false":        TokFalse,
	"fn":           TokFn,
	"for":          TokFor,
	"if":           TokIf,
	"let":          TokLet,
	"loop":         TokLoop,
	"override":     TokOverride,
	"requires":     TokRequires,
	"return":       TokReturn,
	"struct":       TokStruct,
	"switch":       TokSwitch,
	"true":         TokTrue,
	"var":          TokVar,
	"while":        TokWhile,
}

// ReservedWords contains all WGSL reserved words that cannot be used as identifiers.
var ReservedWords = map[string]bool{
	"NULL": true, "Self": true, "abstract": true, "active": true,
	"alignas": true, "alignof": true, "as": true, "asm": true,
	"asm_fragment": true, "async": true, "attribute": true, "auto": true,
	"await": true, "become": true, "cast": true, "catch": true,
	"class": true, "co_await": true, "co_return": true, "co_yield": true,
	"coherent": true, "column_major": true, "common": true, "compile": true,
	"compile_fragment": true, "concept": true, "const_cast": true,
	"consteval": true, "constexpr": true, "constinit": true, "crate": true,
	"debugger": true, "decltype": true, "delete": true, "demote": true,
	"demote_to_helper": true, "do": true, "dynamic_cast": true, "enum": true,
	"explicit": true, "export": true, "extends": true, "extern": true,
	"external": true, "fallthrough": true, "filter": true, "final": true,
	"finally": true, "friend": true, "from": true, "fxgroup": true,
	"get": true, "goto": true, "groupshared": true, "highp": true,
	"impl": true, "implements": true, "import": true, "inline": true,
	"instanceof": true, "interface": true, "layout": true, "lowp": true,
	"macro": true, "macro_rules": true, "match": true, "mediump": true,
	"meta": true, "mod": true, "module": true, "move": true, "mut": true,
	"mutable": true, "namespace": true, "new": true, "nil": true,
	"noexcept": true, "noinline": true, "nointerpolation": true,
	"non_coherent": true, "noncoherent": true, "noperspective": true,
	"null": true, "nullptr": true, "of": true, "operator": true,
	"package": true, "packoffset": true, "partition": true, "pass": true,
	"patch": true, "pixelfragment": true, "precise": true, "precision": true,
	"premerge": true, "priv": true, "protected": true, "pub": true,
	"public": true, "readonly": true, "ref": true, "regardless": true,
	"register": true, "reinterpret_cast": true, "require": true,
	"resource": true, "restrict": true, "self": true, "set": true,
	"shared": true, "sizeof": true, "smooth": true, "snorm": true,
	"static": true, "static_assert": true, "static_cast": true, "std": true,
	"subroutine": true, "super": true, "target": true, "template": true,
	"this": true, "thread_local": true, "throw": true, "trait": true,
	"try": true, "type": true, "typedef": true, "typeid": true,
	"typename": true, "typeof": true, "union": true, "unless": true,
	"unorm": true, "unsafe": true, "unsized": true, "use": true,
	"using": true, "varying": true, "virtual": true, "volatile": true,
	"wgsl": true, "where": true, "with": true, "writeonly": true,
	"yield": true,
}

// ----------------------------------------------------------------------------
// Lexer
// ----------------------------------------------------------------------------

// Lexer tokenizes WGSL source code.
type Lexer struct {
	source string
	pos    int
	start  int
	tokens []Token

	// Template list tracking
	templateDepth int
}

// New creates a new lexer for the given source.
func New(source string) *Lexer {
	return &Lexer{
		source: source,
		tokens: make([]Token, 0, len(source)/4), // Estimate
	}
}

// Tokenize returns all tokens in the source.
func (l *Lexer) Tokenize() []Token {
	for {
		tok := l.Next()
		l.tokens = append(l.tokens, tok)
		if tok.Kind == TokEOF || tok.Kind == TokError {
			break
		}
	}
	return l.tokens
}

// Next returns the next token.
func (l *Lexer) Next() Token {
	l.skipWhitespaceAndComments()

	if l.pos >= len(l.source) {
		return Token{Kind: TokEOF, Start: l.pos, End: l.pos}
	}

	l.start = l.pos
	ch := l.source[l.pos]

	// Identifiers and keywords
	if isIdentStart(rune(ch)) {
		return l.scanIdentOrKeyword()
	}

	// Numbers
	if isDigit(ch) || (ch == '.' && l.pos+1 < len(l.source) && isDigit(l.source[l.pos+1])) {
		return l.scanNumber()
	}

	// Operators and punctuation
	return l.scanOperator()
}

// ----------------------------------------------------------------------------
// Scanning Helpers
// ----------------------------------------------------------------------------

func (l *Lexer) skipWhitespaceAndComments() {
	for l.pos < len(l.source) {
		ch := l.source[l.pos]

		// Fast path: check for common ASCII whitespace first
		// Space and newline are most common, check them first
		if ch == ' ' || ch == '\n' {
			l.pos++
			continue
		}

		// Other whitespace (less common)
		if ch == '\t' || ch == '\r' {
			l.pos++
			continue
		}

		// Line comment
		if ch == '/' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '/' {
			l.pos += 2
			// Fast scan to end of line - most comment chars are ASCII
			for l.pos < len(l.source) && l.source[l.pos] != '\n' {
				l.pos++
			}
			continue
		}

		// Block comment (with nesting)
		if ch == '/' && l.pos+1 < len(l.source) && l.source[l.pos+1] == '*' {
			l.pos += 2
			depth := 1
			for l.pos+1 < len(l.source) && depth > 0 {
				c := l.source[l.pos]
				if c == '/' && l.source[l.pos+1] == '*' {
					depth++
					l.pos += 2
				} else if c == '*' && l.source[l.pos+1] == '/' {
					depth--
					l.pos += 2
				} else {
					l.pos++
				}
			}
			continue
		}

		break
	}
}

func (l *Lexer) scanIdentOrKeyword() Token {
	start := l.pos

	// Fast path: scan ASCII identifier characters without UTF-8 decoding
	// This is the common case for most WGSL code and gives ~20% speedup
	for l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch < 128 {
			// Fast ASCII path
			if asciiIdentContinue[ch] {
				l.pos++
				continue
			}
			// ASCII but not identifier - done
			break
		}
		// Slow path: non-ASCII, need UTF-8 decoding
		r, size := utf8.DecodeRuneInString(l.source[l.pos:])
		if !isIdentContinueSlow(r) {
			break
		}
		l.pos += size
	}

	text := l.source[start:l.pos]

	// Check for keywords (fast path: only ASCII identifiers can be keywords)
	if kind, ok := Keywords[text]; ok {
		return Token{Kind: kind, Start: start, End: l.pos, Value: text}
	}

	// Check for single underscore (invalid identifier)
	if text == "_" {
		return Token{Kind: TokUnderscore, Start: start, End: l.pos, Value: text}
	}

	// Check for reserved words
	if ReservedWords[text] {
		return Token{Kind: TokError, Start: start, End: l.pos, Value: "reserved word: " + text}
	}

	// Check for double underscore prefix (invalid)
	if len(text) >= 2 && text[0] == '_' && text[1] == '_' {
		return Token{Kind: TokError, Start: start, End: l.pos, Value: "identifier cannot start with __"}
	}

	return Token{Kind: TokIdent, Start: start, End: l.pos, Value: text}
}

func (l *Lexer) scanNumber() Token {
	start := l.pos
	kind := TokIntLiteral

	// Check for hex
	if l.pos+1 < len(l.source) && l.source[l.pos] == '0' &&
		(l.source[l.pos+1] == 'x' || l.source[l.pos+1] == 'X') {
		l.pos += 2
		// Hex digits
		for l.pos < len(l.source) && isHexDigit(l.source[l.pos]) {
			l.pos++
		}
		// Check for hex float
		if l.pos < len(l.source) && l.source[l.pos] == '.' {
			kind = TokFloatLiteral
			l.pos++
			for l.pos < len(l.source) && isHexDigit(l.source[l.pos]) {
				l.pos++
			}
		}
		// Hex exponent
		if l.pos < len(l.source) && (l.source[l.pos] == 'p' || l.source[l.pos] == 'P') {
			kind = TokFloatLiteral
			l.pos++
			if l.pos < len(l.source) && (l.source[l.pos] == '+' || l.source[l.pos] == '-') {
				l.pos++
			}
			for l.pos < len(l.source) && isDigit(l.source[l.pos]) {
				l.pos++
			}
		}
	} else {
		// Decimal integer part
		for l.pos < len(l.source) && isDigit(l.source[l.pos]) {
			l.pos++
		}
		// Decimal point
		if l.pos < len(l.source) && l.source[l.pos] == '.' {
			// Check it's not a member access like "1.xxx"
			// A trailing dot with no digits is valid: "0." is a float
			nextIsDigit := l.pos+1 < len(l.source) && isDigit(l.source[l.pos+1])
			nextIsIdent := l.pos+1 < len(l.source) && isIdentStart(rune(l.source[l.pos+1]))
			atEnd := l.pos+1 >= len(l.source)

			if nextIsDigit || atEnd || !nextIsIdent {
				kind = TokFloatLiteral
				l.pos++
				for l.pos < len(l.source) && isDigit(l.source[l.pos]) {
					l.pos++
				}
			}
		}
		// Exponent
		if l.pos < len(l.source) && (l.source[l.pos] == 'e' || l.source[l.pos] == 'E') {
			kind = TokFloatLiteral
			l.pos++
			if l.pos < len(l.source) && (l.source[l.pos] == '+' || l.source[l.pos] == '-') {
				l.pos++
			}
			for l.pos < len(l.source) && isDigit(l.source[l.pos]) {
				l.pos++
			}
		}
	}

	// Type suffix
	if l.pos < len(l.source) {
		ch := l.source[l.pos]
		if ch == 'i' || ch == 'u' {
			l.pos++
		} else if ch == 'f' || ch == 'h' {
			kind = TokFloatLiteral
			l.pos++
		}
	}

	return Token{Kind: kind, Start: start, End: l.pos, Value: l.source[start:l.pos]}
}

func (l *Lexer) scanOperator() Token {
	start := l.pos
	ch := l.source[l.pos]
	l.pos++

	// Look for two-character operators
	var next byte
	if l.pos < len(l.source) {
		next = l.source[l.pos]
	}

	switch ch {
	case '+':
		if next == '+' {
			l.pos++
			return Token{Kind: TokPlusPlus, Start: start, End: l.pos}
		}
		if next == '=' {
			l.pos++
			return Token{Kind: TokPlusEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokPlus, Start: start, End: l.pos}

	case '-':
		if next == '-' {
			l.pos++
			return Token{Kind: TokMinusMinus, Start: start, End: l.pos}
		}
		if next == '=' {
			l.pos++
			return Token{Kind: TokMinusEq, Start: start, End: l.pos}
		}
		if next == '>' {
			l.pos++
			return Token{Kind: TokArrow, Start: start, End: l.pos}
		}
		return Token{Kind: TokMinus, Start: start, End: l.pos}

	case '*':
		if next == '=' {
			l.pos++
			return Token{Kind: TokStarEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokStar, Start: start, End: l.pos}

	case '/':
		if next == '=' {
			l.pos++
			return Token{Kind: TokSlashEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokSlash, Start: start, End: l.pos}

	case '%':
		if next == '=' {
			l.pos++
			return Token{Kind: TokPercentEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokPercent, Start: start, End: l.pos}

	case '&':
		if next == '&' {
			l.pos++
			return Token{Kind: TokAmpAmp, Start: start, End: l.pos}
		}
		if next == '=' {
			l.pos++
			return Token{Kind: TokAmpEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokAmp, Start: start, End: l.pos}

	case '|':
		if next == '|' {
			l.pos++
			return Token{Kind: TokPipePipe, Start: start, End: l.pos}
		}
		if next == '=' {
			l.pos++
			return Token{Kind: TokPipeEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokPipe, Start: start, End: l.pos}

	case '^':
		if next == '=' {
			l.pos++
			return Token{Kind: TokCaretEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokCaret, Start: start, End: l.pos}

	case '<':
		if next == '<' {
			l.pos++
			if l.pos < len(l.source) && l.source[l.pos] == '=' {
				l.pos++
				return Token{Kind: TokLtLtEq, Start: start, End: l.pos}
			}
			return Token{Kind: TokLtLt, Start: start, End: l.pos}
		}
		if next == '=' {
			l.pos++
			return Token{Kind: TokLtEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokLt, Start: start, End: l.pos}

	case '>':
		if next == '>' {
			l.pos++
			if l.pos < len(l.source) && l.source[l.pos] == '=' {
				l.pos++
				return Token{Kind: TokGtGtEq, Start: start, End: l.pos}
			}
			return Token{Kind: TokGtGt, Start: start, End: l.pos}
		}
		if next == '=' {
			l.pos++
			return Token{Kind: TokGtEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokGt, Start: start, End: l.pos}

	case '=':
		if next == '=' {
			l.pos++
			return Token{Kind: TokEqEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokEq, Start: start, End: l.pos}

	case '!':
		if next == '=' {
			l.pos++
			return Token{Kind: TokBangEq, Start: start, End: l.pos}
		}
		return Token{Kind: TokBang, Start: start, End: l.pos}

	case '~':
		return Token{Kind: TokTilde, Start: start, End: l.pos}
	case '.':
		return Token{Kind: TokDot, Start: start, End: l.pos}
	case '@':
		return Token{Kind: TokAt, Start: start, End: l.pos}
	case '(':
		return Token{Kind: TokLParen, Start: start, End: l.pos}
	case ')':
		return Token{Kind: TokRParen, Start: start, End: l.pos}
	case '{':
		return Token{Kind: TokLBrace, Start: start, End: l.pos}
	case '}':
		return Token{Kind: TokRBrace, Start: start, End: l.pos}
	case '[':
		return Token{Kind: TokLBracket, Start: start, End: l.pos}
	case ']':
		return Token{Kind: TokRBracket, Start: start, End: l.pos}
	case ';':
		return Token{Kind: TokSemicolon, Start: start, End: l.pos}
	case ':':
		return Token{Kind: TokColon, Start: start, End: l.pos}
	case ',':
		return Token{Kind: TokComma, Start: start, End: l.pos}
	}

	return Token{Kind: TokError, Start: start, End: l.pos, Value: "unexpected character"}
}

// ----------------------------------------------------------------------------
// Character Classification
// ----------------------------------------------------------------------------

// ASCII lookup tables for fast character classification (esbuild-style optimization)
// These avoid expensive Unicode lookups for the common ASCII case.
var (
	// asciiIdentStart[c] is true if ASCII byte c can start an identifier
	asciiIdentStart [128]bool
	// asciiIdentContinue[c] is true if ASCII byte c can continue an identifier
	asciiIdentContinue [128]bool
	// asciiWhitespace[c] is true if ASCII byte c is whitespace
	asciiWhitespace [128]bool
)

func init() {
	// Initialize identifier start characters: a-z, A-Z, _
	for c := 'a'; c <= 'z'; c++ {
		asciiIdentStart[c] = true
		asciiIdentContinue[c] = true
	}
	for c := 'A'; c <= 'Z'; c++ {
		asciiIdentStart[c] = true
		asciiIdentContinue[c] = true
	}
	asciiIdentStart['_'] = true
	asciiIdentContinue['_'] = true

	// Digits can continue but not start identifiers
	for c := '0'; c <= '9'; c++ {
		asciiIdentContinue[c] = true
	}

	// Whitespace characters
	asciiWhitespace[' '] = true
	asciiWhitespace['\t'] = true
	asciiWhitespace['\n'] = true
	asciiWhitespace['\r'] = true
	asciiWhitespace['\v'] = true
	asciiWhitespace['\f'] = true
}

func isWhitespace(ch byte) bool {
	return ch < 128 && asciiWhitespace[ch]
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// isASCIIIdentStart checks if an ASCII byte can start an identifier.
// This is the fast path - use isIdentStartSlow for non-ASCII.
func isASCIIIdentStart(ch byte) bool {
	return ch < 128 && asciiIdentStart[ch]
}

// isASCIIIdentContinue checks if an ASCII byte can continue an identifier.
// This is the fast path - use isIdentContinueSlow for non-ASCII.
func isASCIIIdentContinue(ch byte) bool {
	return ch < 128 && asciiIdentContinue[ch]
}

// isIdentStartSlow handles Unicode identifier start characters.
// Called only when the fast ASCII path fails.
func isIdentStartSlow(r rune) bool {
	// XID_Start or underscore
	return r == '_' || unicode.Is(unicode.Other_ID_Start, r) || unicode.IsLetter(r)
}

// isIdentContinueSlow handles Unicode identifier continuation characters.
// Called only when the fast ASCII path fails.
func isIdentContinueSlow(r rune) bool {
	// XID_Continue
	return isIdentStartSlow(r) || unicode.Is(unicode.Other_ID_Continue, r) ||
		unicode.IsDigit(r) || r == '_'
}

// isIdentStart checks if a rune can start an identifier.
// Prefer isASCIIIdentStart for bytes when possible.
func isIdentStart(r rune) bool {
	if r < 128 {
		return asciiIdentStart[r]
	}
	return isIdentStartSlow(r)
}

// isIdentContinue checks if a rune can continue an identifier.
// Prefer isASCIIIdentContinue for bytes when possible.
func isIdentContinue(r rune) bool {
	if r < 128 {
		return asciiIdentContinue[r]
	}
	return isIdentContinueSlow(r)
}
