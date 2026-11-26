package coreql

// TokenKind classifies a lexical token.
type TokenKind int

const (
	TokEOF TokenKind = iota
	TokIllegal
	TokWord        // identifier or bare value, e.g. status, Done, CORE-12
	TokCustomField // cf[<key>], raw text kept in Token.Text
	TokString      // "..." or '...', Token.Text holds the unescaped content
	TokNumber      // 12, -3, 1.5
	TokRelDate     // -7d, +3d, 1M (raw text kept in Token.Text)
	TokOp          // = != > >= < <= ~ !~
	TokLParen
	TokRParen
	TokComma
	TokSemicolon
	TokAnd
	TokOr
	TokNot
	TokIn
	TokIs
	TokEmpty
	TokNull
	TokOrder
	TokBy
	TokAsc
	TokDesc
)

var tokenKindNames = map[TokenKind]string{
	TokEOF:         "fin de la consulta",
	TokIllegal:     "carácter no válido",
	TokWord:        "identificador",
	TokCustomField: "campo personalizado",
	TokString:      "cadena",
	TokNumber:      "número",
	TokRelDate:     "fecha relativa",
	TokOp:          "operador",
	TokLParen:      "'('",
	TokRParen:      "')'",
	TokComma:       "','",
	TokSemicolon:   "';'",
	TokAnd:         "AND",
	TokOr:          "OR",
	TokNot:         "NOT",
	TokIn:          "IN",
	TokIs:          "IS",
	TokEmpty:       "EMPTY",
	TokNull:        "NULL",
	TokOrder:       "ORDER",
	TokBy:          "BY",
	TokAsc:         "ASC",
	TokDesc:        "DESC",
}

// String returns a short Spanish description of the token kind.
func (k TokenKind) String() string {
	if s, ok := tokenKindNames[k]; ok {
		return s
	}
	return "token"
}

// keywords maps lower-case keyword spellings to their token kinds.
var keywords = map[string]TokenKind{
	"and":   TokAnd,
	"or":    TokOr,
	"not":   TokNot,
	"in":    TokIn,
	"is":    TokIs,
	"empty": TokEmpty,
	"null":  TokNull,
	"order": TokOrder,
	"by":    TokBy,
	"asc":   TokAsc,
	"desc":  TokDesc,
}

// Token is a lexical token. Pos and Len are byte offsets into the input.
type Token struct {
	Kind    TokenKind
	Text    string
	Op      Op     // set for TokOp only
	Pos     int    // byte offset of the first byte
	Len     int    // byte length of the raw source text
	Message string // set for TokIllegal only: Spanish error message
}
