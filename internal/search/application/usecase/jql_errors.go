package usecase

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// runeSpan converts a byte span of src to a rune position and length.
func runeSpan(src string, pos, length int) (int, int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(src) {
		pos = len(src)
	}
	end := pos + length
	if end > len(src) {
		end = len(src)
	}
	return coreql.RuneOffset(src, pos), utf8.RuneCountInString(src[pos:end])
}

// syntaxProblem converts a parse failure to a query error. WAS and CHANGED are recognized so that the message
// says they are not available instead of reporting a generic syntax error.
func syntaxProblem(src string, err error) error {
	var se *coreql.SyntaxError
	if !errors.As(err, &se) {
		return domain.NewQueryError([]domain.Problem{{Code: domain.CodeSyntaxError, Message: err.Error()}})
	}
	pos, length := runeSpan(src, se.Pos, se.Len)
	p := domain.Problem{Pos: pos, Len: length, Code: domain.CodeSyntaxError, Message: se.Message, Expected: se.Expected}
	if se.Pos <= len(src) {
		end := se.Pos + se.Len
		if end > len(src) {
			end = len(src)
		}
		switch strings.ToLower(src[se.Pos:end]) {
		case "was", "changed":
			p.Code = domain.CodeOperatorUnavailable
			p.Message = "los operadores WAS y CHANGED todavía no están disponibles"
		}
	}
	return domain.NewQueryError([]domain.Problem{p})
}

func validationProblems(src string, errs []coreql.ValidationError) []domain.Problem {
	out := make([]domain.Problem, 0, len(errs))
	for _, e := range errs {
		pos, length := runeSpan(src, e.Pos, e.Len)
		code := domain.CodeInvalidQuery
		if strings.HasPrefix(e.Message, "campo desconocido") {
			code = domain.CodeUnknownField
		}
		out = append(out, domain.Problem{Pos: pos, Len: length, Code: code, Message: e.Message, Suggestions: e.Suggestions})
	}
	return out
}

func buildProblem(src string, be *port.BuildError) error {
	pos, length := runeSpan(src, be.Pos, be.Len)
	return domain.NewQueryError([]domain.Problem{{Pos: pos, Len: length, Code: be.Code, Message: be.Message}})
}
