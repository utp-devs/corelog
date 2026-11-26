package coreql

import (
	"fmt"
	"strconv"
	"strings"
)

// SyntaxError reports the first syntax error found by Parse.
type SyntaxError struct {
	Pos      int      // byte offset of the offending token
	Len      int      // byte length of the offending token (0 at end of input)
	Message  string   // Spanish message
	Expected []string // what the parser could accept at this point
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("coreql: posición %d: %s", e.Pos, e.Message)
}

// Parse parses a CoreQL query. Empty or blank input yields an empty query.
// On failure it returns a *SyntaxError for the first error found.
//
// Grammar:
//
//	query  := [ expr ] [ "ORDER BY" order { "," order } ]
//	expr   := and { "OR" and }
//	and    := not { "AND" not }
//	not    := "NOT" not | primary
//	primary := "(" expr ")" | clause
func Parse(input string) (*Query, error) {
	p := &parser{src: input, toks: tokenize(input)}
	q, err := p.query()
	if err != nil {
		return nil, err
	}
	return q, nil
}

type parser struct {
	src  string
	toks []Token
	i    int
}

func (p *parser) cur() Token { return p.toks[p.i] }

// next consumes the current token. The EOF token is never consumed.
func (p *parser) next() Token {
	t := p.toks[p.i]
	if t.Kind != TokEOF {
		p.i++
	}
	return t
}

// lastEnd returns the end offset of the last consumed token.
func (p *parser) lastEnd() int {
	if p.i == 0 {
		return 0
	}
	t := p.toks[p.i-1]
	return t.Pos + t.Len
}

func (p *parser) raw(t Token) string {
	return p.src[t.Pos : t.Pos+t.Len]
}

// unexpected builds the error for a token that does not fit the grammar.
// Illegal tokens report their own lexical message.
func (p *parser) unexpected(t Token, expected ...string) *SyntaxError {
	want := joinExpected(expected)
	switch t.Kind {
	case TokIllegal:
		return &SyntaxError{Pos: t.Pos, Len: t.Len, Message: t.Message, Expected: expected}
	case TokEOF:
		return &SyntaxError{Pos: t.Pos, Message: "fin inesperado de la consulta; se esperaba " + want, Expected: expected}
	}
	return &SyntaxError{
		Pos:      t.Pos,
		Len:      t.Len,
		Message:  fmt.Sprintf("token inesperado '%s'; se esperaba %s", p.raw(t), want),
		Expected: expected,
	}
}

func joinExpected(items []string) string {
	switch len(items) {
	case 0:
		return "un token"
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " o " + items[len(items)-1]
}

func (p *parser) query() (*Query, error) {
	q := &Query{Span: Span{Pos: 0, Len: len(p.src)}}
	if k := p.cur().Kind; k != TokEOF && k != TokOrder {
		e, err := p.or()
		if err != nil {
			return nil, err
		}
		q.Expr = e
	}
	if p.cur().Kind == TokOrder {
		ord, err := p.orderBy()
		if err != nil {
			return nil, err
		}
		q.OrderBy = ord
		if t := p.cur(); t.Kind != TokEOF {
			return nil, p.unexpected(t, "','", "fin de la consulta")
		}
		return q, nil
	}
	if t := p.cur(); t.Kind != TokEOF {
		return nil, p.unexpected(t, "AND", "OR", "ORDER BY", "fin de la consulta")
	}
	return q, nil
}

func (p *parser) or() (Expr, error) {
	left, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.cur().Kind == TokOr {
		p.next()
		right, err := p.and()
		if err != nil {
			return nil, err
		}
		left = &Or{Span: join(left.Location(), right.Location()), Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) and() (Expr, error) {
	left, err := p.not()
	if err != nil {
		return nil, err
	}
	for p.cur().Kind == TokAnd {
		p.next()
		right, err := p.not()
		if err != nil {
			return nil, err
		}
		left = &And{Span: join(left.Location(), right.Location()), Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) not() (Expr, error) {
	t := p.cur()
	if t.Kind != TokNot {
		return p.primary()
	}
	p.next()
	operand, err := p.not()
	if err != nil {
		return nil, err
	}
	return &Not{Span: Span{Pos: t.Pos, Len: p.lastEnd() - t.Pos}, Expr: operand}, nil
}

func (p *parser) primary() (Expr, error) {
	t := p.cur()
	switch t.Kind {
	case TokLParen:
		p.next()
		e, err := p.or()
		if err != nil {
			return nil, err
		}
		if rp := p.cur(); rp.Kind != TokRParen {
			return nil, p.unexpected(rp, "')'")
		}
		p.next()
		return e, nil
	case TokWord, TokCustomField:
		return p.clause()
	}
	return nil, p.unexpected(t, "un campo", "NOT", "'('")
}

func (p *parser) clause() (Expr, error) {
	ft := p.next()
	field := Field{Span: spanOf(ft), Name: p.raw(ft)}
	t := p.cur()
	switch t.Kind {
	case TokOp:
		p.next()
		val, err := p.value()
		if err != nil {
			return nil, err
		}
		return &Compare{
			Span:   Span{Pos: field.Pos, Len: p.lastEnd() - field.Pos},
			Field:  field,
			Op:     t.Op,
			OpSpan: spanOf(t),
			Value:  val,
		}, nil
	case TokIs:
		p.next()
		negated := false
		if p.cur().Kind == TokNot {
			p.next()
			negated = true
		}
		if k := p.cur().Kind; k != TokEmpty && k != TokNull {
			return nil, p.unexpected(p.cur(), "EMPTY", "NULL")
		}
		p.next()
		return &IsEmpty{Span: Span{Pos: field.Pos, Len: p.lastEnd() - field.Pos}, Field: field, Not: negated}, nil
	case TokIn:
		p.next()
		return p.inList(field, false)
	case TokNot:
		p.next()
		if p.cur().Kind != TokIn {
			return nil, p.unexpected(p.cur(), "IN")
		}
		p.next()
		return p.inList(field, true)
	}
	return nil, p.unexpected(t, "un operador", "IS", "IN", "NOT IN")
}

func (p *parser) inList(field Field, negated bool) (Expr, error) {
	if t := p.cur(); t.Kind != TokLParen {
		return nil, p.unexpected(t, "'('")
	}
	p.next()
	var vals []Value
	for {
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
		t := p.cur()
		if t.Kind == TokComma {
			p.next()
			continue
		}
		if t.Kind == TokRParen {
			p.next()
			break
		}
		return nil, p.unexpected(t, "','", "')'")
	}
	return &In{
		Span:   Span{Pos: field.Pos, Len: p.lastEnd() - field.Pos},
		Field:  field,
		Not:    negated,
		Values: vals,
	}, nil
}

func (p *parser) value() (Value, error) {
	t := p.cur()
	switch t.Kind {
	case TokString:
		p.next()
		return &String{Span: spanOf(t), Text: t.Text}, nil
	case TokNumber:
		p.next()
		return &Number{Span: spanOf(t), Text: t.Text}, nil
	case TokRelDate:
		p.next()
		amount, err := strconv.Atoi(t.Text[:len(t.Text)-1])
		if err != nil {
			return nil, &SyntaxError{Pos: t.Pos, Len: t.Len, Message: "magnitud de fecha relativa fuera de rango", Expected: []string{"fecha relativa"}}
		}
		return &RelativeDate{Span: spanOf(t), Amount: amount, Unit: t.Text[len(t.Text)-1:]}, nil
	case TokWord:
		p.next()
		if p.cur().Kind == TokLParen {
			return p.funcCall(t)
		}
		return &Word{Span: spanOf(t), Text: t.Text}, nil
	}
	return nil, p.unexpected(t, "un valor")
}

func (p *parser) funcCall(name Token) (Value, error) {
	p.next() // '('
	var args []Value
	if p.cur().Kind == TokRParen {
		p.next()
	} else {
		for {
			a, err := p.value()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			t := p.cur()
			if t.Kind == TokComma {
				p.next()
				continue
			}
			if t.Kind == TokRParen {
				p.next()
				break
			}
			return nil, p.unexpected(t, "','", "')'")
		}
	}
	return &Func{Span: Span{Pos: name.Pos, Len: p.lastEnd() - name.Pos}, Name: name.Text, Args: args}, nil
}

func (p *parser) orderBy() ([]*Order, error) {
	p.next() // ORDER
	if t := p.cur(); t.Kind != TokBy {
		return nil, p.unexpected(t, "BY")
	}
	p.next()
	var out []*Order
	for {
		ft := p.cur()
		if ft.Kind != TokWord && ft.Kind != TokCustomField {
			return nil, p.unexpected(ft, "un campo")
		}
		p.next()
		o := &Order{Field: Field{Span: spanOf(ft), Name: p.raw(ft)}}
		switch p.cur().Kind {
		case TokAsc:
			p.next()
		case TokDesc:
			p.next()
			o.Desc = true
		}
		o.Span = Span{Pos: ft.Pos, Len: p.lastEnd() - ft.Pos}
		out = append(out, o)
		if p.cur().Kind != TokComma {
			break
		}
		p.next()
	}
	return out, nil
}
