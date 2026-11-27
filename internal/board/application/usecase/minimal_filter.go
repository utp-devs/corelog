package usecase

import (
	"github.com/jhonsferg/corelog/internal/board/application/port"
	"github.com/jhonsferg/corelog/internal/board/domain"
)

// MinimalQueryFilter compiles quick filter queries with the minimal grammar of the domain (see domain.ParseQuery).
type MinimalQueryFilter struct{}

var _ port.QueryFilter = MinimalQueryFilter{}

// Compile parses the text; an empty text matches every card.
func (MinimalQueryFilter) Compile(text string) (domain.Matcher, error) {
	q, err := domain.ParseQuery(text)
	if err != nil {
		return nil, domain.ErrInvalidQuery
	}
	return q, nil
}
