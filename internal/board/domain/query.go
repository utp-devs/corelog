package domain

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// Minimal card query grammar used by quick filters and query swimlanes until the search phase replaces it.
//
//	query  = [ clause { "AND" clause } ]            (empty query matches every card)
//	clause = field "=" value
//	       | field "!=" value
//	       | field "in" "(" value { "," value } ")"
//	       | field "~" string                       (case-insensitive substring)
//	       | "assignee" "is" "EMPTY"
//	field  = assignee | labels | priority | type | status | text | flagged
//	value  = currentUser() | word | "quoted string"
//
// assignee compares with the user id or name (currentUser() is the caller); labels matches when the card has
// the label; priority, type and status compare with the name or the legacy key ignoring case; text searches
// the key and the summary; flagged accepts true or false. Keywords are case-insensitive.

// QueryContext carries what a query needs besides the card.
type QueryContext struct {
	CurrentUser uuid.UUID
}

// Query is a parsed card query.
type Query struct {
	clauses []clause
}

type clause struct {
	field  string
	op     string // "=", "!=", "in", "~", "empty"
	values []string
	me     []bool // values[i] is currentUser()
}

var queryFields = map[string]bool{"assignee": true, "labels": true, "priority": true, "type": true, "status": true, "text": true, "flagged": true}

type token struct {
	text   string
	quoted bool
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	runes := []rune(input)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '"':
			j := i + 1
			var sb strings.Builder
			for ; j < len(runes) && runes[j] != '"'; j++ {
				if runes[j] == '\\' && j+1 < len(runes) {
					j++
				}
				sb.WriteRune(runes[j])
			}
			if j >= len(runes) {
				return nil, fmt.Errorf("falta cerrar las comillas")
			}
			tokens = append(tokens, token{text: sb.String(), quoted: true})
			i = j + 1
		case r == '(' || r == ')' || r == ',' || r == '=' || r == '~':
			tokens = append(tokens, token{text: string(r)})
			i++
		case r == '!':
			if i+1 < len(runes) && runes[i+1] == '=' {
				tokens = append(tokens, token{text: "!="})
				i += 2
				break
			}
			return nil, fmt.Errorf("carácter inesperado %q", r)
		default:
			j := i
			for j < len(runes) && !unicode.IsSpace(runes[j]) && !strings.ContainsRune(`(),=~!"`, runes[j]) {
				j++
			}
			// currentUser() is one word followed by "()".
			word := string(runes[i:j])
			if strings.EqualFold(word, "currentUser") && j+1 < len(runes) && runes[j] == '(' && runes[j+1] == ')' {
				word += "()"
				j += 2
			}
			tokens = append(tokens, token{text: word})
			i = j
		}
	}
	return tokens, nil
}

// ParseQuery parses the text of a quick filter or swimlane query.
func ParseQuery(input string) (*Query, error) {
	tokens, err := tokenize(input)
	if err != nil {
		return nil, err
	}
	q := &Query{}
	pos := 0
	next := func() (token, bool) {
		if pos >= len(tokens) {
			return token{}, false
		}
		pos++
		return tokens[pos-1], true
	}
	value := func() (string, bool, error) {
		t, ok := next()
		if !ok || (!t.quoted && strings.ContainsAny(t.text, "(),=~") && !strings.EqualFold(t.text, "currentUser()")) {
			return "", false, fmt.Errorf("se esperaba un valor")
		}
		return t.text, !t.quoted && strings.EqualFold(t.text, "currentUser()"), nil
	}
	for pos < len(tokens) {
		if len(q.clauses) > 0 {
			t, _ := next()
			if t.quoted || !strings.EqualFold(t.text, "and") {
				return nil, fmt.Errorf("se esperaba AND antes de %q", t.text)
			}
		}
		ft, ok := next()
		field := strings.ToLower(ft.text)
		if !ok || ft.quoted || !queryFields[field] {
			return nil, fmt.Errorf("campo desconocido %q (usa assignee, labels, priority, type, status, text o flagged)", ft.text)
		}
		ot, ok := next()
		if !ok {
			return nil, fmt.Errorf("falta el operador después de %s", field)
		}
		c := clause{field: field}
		switch op := strings.ToLower(ot.text); {
		case ot.quoted:
			return nil, fmt.Errorf("operador no válido %q", ot.text)
		case op == "=" || op == "!=":
			c.op = op
			v, me, err := value()
			if err != nil {
				return nil, err
			}
			c.values, c.me = []string{v}, []bool{me}
		case op == "~":
			c.op = "~"
			t, ok := next()
			if !ok {
				return nil, fmt.Errorf("se esperaba un texto después de ~")
			}
			c.values, c.me = []string{t.text}, []bool{false}
		case op == "in":
			c.op = "in"
			if t, ok := next(); !ok || t.text != "(" {
				return nil, fmt.Errorf("se esperaba ( después de in")
			}
			for {
				v, me, err := value()
				if err != nil {
					return nil, err
				}
				c.values, c.me = append(c.values, v), append(c.me, me)
				t, ok := next()
				if !ok {
					return nil, fmt.Errorf("falta cerrar el paréntesis")
				}
				if t.text == ")" {
					break
				}
				if t.text != "," {
					return nil, fmt.Errorf("se esperaba , o )")
				}
			}
		case op == "is":
			t, ok := next()
			if !ok || !strings.EqualFold(t.text, "empty") || field != "assignee" {
				return nil, fmt.Errorf("solo se admite assignee is EMPTY")
			}
			c.op = "empty"
		default:
			return nil, fmt.Errorf("operador no válido %q", ot.text)
		}
		if field == "text" && c.op != "~" {
			return nil, fmt.Errorf("text solo admite el operador ~")
		}
		if c.op == "~" && field != "text" {
			return nil, fmt.Errorf("el operador ~ solo se admite con text")
		}
		if field == "flagged" && c.op != "=" {
			return nil, fmt.Errorf("flagged solo admite =")
		}
		q.clauses = append(q.clauses, c)
	}
	return q, nil
}

// Match reports whether the card satisfies every clause.
func (q *Query) Match(card Card, ctx QueryContext) bool {
	for _, c := range q.clauses {
		if !c.match(card, ctx) {
			return false
		}
	}
	return true
}

func (c clause) match(card Card, ctx QueryContext) bool {
	switch c.field {
	case "assignee":
		if c.op == "empty" {
			return card.AssigneeID == nil
		}
		hit := false
		for i, v := range c.values {
			if card.AssigneeID == nil {
				continue
			}
			if c.me[i] {
				hit = hit || *card.AssigneeID == ctx.CurrentUser
			} else {
				hit = hit || strings.EqualFold(card.AssigneeID.String(), v) || strings.EqualFold(card.AssigneeName, v)
			}
		}
		return c.op == "!=" != hit
	case "labels":
		hit := false
		for _, v := range c.values {
			for _, label := range card.Labels {
				hit = hit || strings.EqualFold(label, v)
			}
		}
		return c.op == "!=" != hit
	case "priority":
		return c.matchNames(card.Priority, card.PriorityKey)
	case "type":
		return c.matchNames(card.Type, card.TypeKey)
	case "status":
		return c.matchNames(card.StatusName)
	case "text":
		needle := strings.ToLower(c.values[0])
		return strings.Contains(strings.ToLower(card.Key), needle) || strings.Contains(strings.ToLower(card.Summary), needle)
	case "flagged":
		return card.Flagged == strings.EqualFold(c.values[0], "true")
	}
	return false
}

func (c clause) matchNames(names ...string) bool {
	hit := false
	for _, v := range c.values {
		for _, n := range names {
			hit = hit || (n != "" && strings.EqualFold(n, v))
		}
	}
	return c.op == "!=" != hit
}
