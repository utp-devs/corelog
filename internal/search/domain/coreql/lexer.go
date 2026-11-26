package coreql

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokenize splits src into tokens. The last token is always TokEOF. Invalid
// input produces TokIllegal tokens that carry their own message; the parser
// reports them when it reaches them, so the first error in the input wins.
func tokenize(src string) []Token {
	var toks []Token
	i := 0
	for {
		i = skipSpace(src, i)
		if i >= len(src) {
			return append(toks, Token{Kind: TokEOF, Pos: len(src)})
		}
		t, next := scanToken(src, i)
		toks = append(toks, t)
		i = next
	}
}

// RuneOffset converts a byte offset into input to a rune offset. Use it to
// report error positions to the UI, which counts characters, not bytes.
func RuneOffset(input string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset > len(input) {
		byteOffset = len(input)
	}
	return utf8.RuneCountInString(input[:byteOffset])
}

// ByteOffset converts a rune offset into input to a byte offset. Offsets past
// the end are clamped to len(input).
func ByteOffset(input string, runeOffset int) int {
	if runeOffset <= 0 {
		return 0
	}
	n := 0
	for i := range input {
		if n == runeOffset {
			return i
		}
		n++
	}
	return len(input)
}

func skipSpace(src string, i int) int {
	for i < len(src) {
		r, w := utf8.DecodeRuneInString(src[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += w
	}
	return i
}

func scanToken(src string, i int) (Token, int) {
	switch c := src[i]; c {
	case '"', '\'':
		return scanString(src, i)
	case '(':
		return Token{Kind: TokLParen, Text: "(", Pos: i, Len: 1}, i + 1
	case ')':
		return Token{Kind: TokRParen, Text: ")", Pos: i, Len: 1}, i + 1
	case ',':
		return Token{Kind: TokComma, Text: ",", Pos: i, Len: 1}, i + 1
	case ';':
		return Token{Kind: TokSemicolon, Text: ";", Pos: i, Len: 1}, i + 1
	case '=':
		return opToken(i, 1, OpEq), i + 1
	case '~':
		return opToken(i, 1, OpContains), i + 1
	case '>':
		if i+1 < len(src) && src[i+1] == '=' {
			return opToken(i, 2, OpGe), i + 2
		}
		return opToken(i, 1, OpGt), i + 1
	case '<':
		if i+1 < len(src) && src[i+1] == '=' {
			return opToken(i, 2, OpLe), i + 2
		}
		return opToken(i, 1, OpLt), i + 1
	case '!':
		if i+1 < len(src) {
			switch src[i+1] {
			case '=':
				return opToken(i, 2, OpNe), i + 2
			case '~':
				return opToken(i, 2, OpNotContains), i + 2
			}
		}
		return illegal(i, 1, "operador no válido: se esperaba '=' o '~' después de '!'"), i + 1
	}
	if isDigit(src[i]) || ((src[i] == '+' || src[i] == '-') && i+1 < len(src) && isDigit(src[i+1])) {
		return scanNumeric(src, i)
	}
	r, w := utf8.DecodeRuneInString(src[i:])
	if isWordStart(r) {
		return scanWord(src, i)
	}
	return illegal(i, w, fmt.Sprintf("carácter no válido '%s'", src[i:i+w])), i + w
}

func opToken(pos, n int, op Op) Token {
	return Token{Kind: TokOp, Text: opText(op), Op: op, Pos: pos, Len: n}
}

func illegal(pos, n int, msg string) Token {
	return Token{Kind: TokIllegal, Pos: pos, Len: n, Message: msg}
}

// scanString reads a quoted string. A backslash escapes the next character.
func scanString(src string, i int) (Token, int) {
	quote := src[i]
	var b strings.Builder
	j := i + 1
	for j < len(src) {
		c := src[j]
		if c == '\\' {
			if j+1 >= len(src) {
				break
			}
			_, w := utf8.DecodeRuneInString(src[j+1:])
			b.WriteString(src[j+1 : j+1+w])
			j += 1 + w
			continue
		}
		if c == quote {
			return Token{Kind: TokString, Text: b.String(), Pos: i, Len: j + 1 - i}, j + 1
		}
		_, w := utf8.DecodeRuneInString(src[j:])
		b.WriteString(src[j : j+w])
		j += w
	}
	return illegal(i, len(src)-i, "cadena sin cerrar"), len(src)
}

// scanNumeric reads an integer, a decimal, a relative date or, when digits
// are followed by letters or dashes (for example 5days or 1d-), a word.
func scanNumeric(src string, i int) (Token, int) {
	j := i
	if src[j] == '+' || src[j] == '-' {
		j++
	}
	for j < len(src) && isDigit(src[j]) {
		j++
	}
	decimal := false
	if j+1 < len(src) && src[j] == '.' && isDigit(src[j+1]) {
		decimal = true
		j++
		for j < len(src) && isDigit(src[j]) {
			j++
		}
	}
	if !decimal && j < len(src) && isUnit(src[j]) && !wordRuneAt(src, j+1) {
		return Token{Kind: TokRelDate, Text: src[i : j+1], Pos: i, Len: j + 1 - i}, j + 1
	}
	if wordRuneAt(src, j) {
		k := scanWordRun(src, j)
		return Token{Kind: TokWord, Text: src[i:k], Pos: i, Len: k - i}, k
	}
	return Token{Kind: TokNumber, Text: src[i:j], Pos: i, Len: j - i}, j
}

// scanWord reads a bare word or keyword, or a cf[<key>] custom field reference.
func scanWord(src string, i int) (Token, int) {
	if len(src)-i >= 3 && strings.EqualFold(src[i:i+2], "cf") && src[i+2] == '[' {
		return scanCustomField(src, i)
	}
	k := scanWordRun(src, i)
	raw := src[i:k]
	if kind, ok := keywords[strings.ToLower(raw)]; ok {
		return Token{Kind: kind, Text: raw, Pos: i, Len: k - i}, k
	}
	return Token{Kind: TokWord, Text: raw, Pos: i, Len: k - i}, k
}

func scanCustomField(src string, i int) (Token, int) {
	start := i + 3
	end := strings.IndexByte(src[start:], ']')
	if end < 0 {
		return illegal(i, len(src)-i, "campo personalizado sin cerrar: se esperaba ']'"), len(src)
	}
	k := start + end + 1
	if strings.TrimSpace(src[start:start+end]) == "" {
		return illegal(i, k-i, "clave de campo personalizado vacía"), k
	}
	return Token{Kind: TokCustomField, Text: src[i:k], Pos: i, Len: k - i}, k
}

// scanWordRun returns the end offset of the run of word runes starting at i.
func scanWordRun(src string, i int) int {
	k := i
	for k < len(src) {
		r, w := utf8.DecodeRuneInString(src[k:])
		if !isWordRune(r) {
			break
		}
		k += w
	}
	return k
}

func wordRuneAt(src string, k int) bool {
	if k >= len(src) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(src[k:])
	return isWordRune(r)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// isUnit reports whether c is a relative date unit: m (minutes), h, d, w,
// M (months), y. Units are case-sensitive, so 1m and 1M differ.
func isUnit(c byte) bool {
	switch c {
	case 'm', 'h', 'd', 'w', 'M', 'y':
		return true
	}
	return false
}

func isWordStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-'
}
