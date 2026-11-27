package usecase

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/search/application/port"
	"github.com/jhonsferg/corelog/internal/search/domain"
	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// Paging limits of issue search.
const (
	DefaultSearchPageSize = 50
	MaxSearchPageSize     = 100
	MaxQueryRunes         = 4000
)

// JQLService runs CoreQL queries on behalf of a viewer. Authorization is not decided here: the searcher
// restricts every query to the projects the viewer can browse.
type JQLService struct {
	searcher port.IssueSearcher
	custom   port.CustomFieldSource
	viewers  port.ViewerDirectory
	clock    clock.Clock
	suggest  port.SuggestSource
}

// NewJQLService builds the service. custom and viewers may be nil (no custom fields; UTC).
func NewJQLService(searcher port.IssueSearcher, custom port.CustomFieldSource, viewers port.ViewerDirectory, clk clock.Clock) *JQLService {
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &JQLService{searcher: searcher, custom: custom, viewers: viewers, clock: clk}
}

type prepared struct {
	query    *coreql.Query
	catalog  coreql.FieldCatalog
	custom   map[string]port.CustomFieldDef
	viewer   port.Viewer
	warnings []domain.Warning
}

// prepare parses and validates a query text and an optional order override.
func (s *JQLService) prepare(ctx context.Context, viewer port.Viewer, text, order string) (*prepared, error) {
	if utf8.RuneCountInString(text) > MaxQueryRunes || utf8.RuneCountInString(order) > domain.MaxFilterSortLength {
		return nil, domain.ErrCoreQLTooLong
	}
	if viewer.Timezone == "" && s.viewers != nil {
		tz, err := s.viewers.Timezone(ctx, viewer.UserID)
		if err != nil {
			return nil, err
		}
		viewer.Timezone = tz
	}
	cc := newCustomCatalog(nil)
	if s.custom != nil {
		fields, err := s.custom.CustomFields(ctx, viewer)
		if err != nil {
			return nil, err
		}
		cc = newCustomCatalog(fields)
	}
	catalog := coreql.CompositeCatalog{Top: coreql.SystemCatalog(), Base: cc}
	p := &prepared{catalog: catalog, custom: cc.refs(), viewer: viewer}

	q, err := coreql.Parse(text)
	if err != nil {
		return nil, syntaxProblem(text, err)
	}
	if problems := validationProblems(text, coreql.Validate(q, catalog, nil)); len(problems) > 0 {
		return nil, domain.NewQueryError(problems)
	}
	if strings.TrimSpace(order) != "" {
		oq, err := coreql.Parse("ORDER BY " + order)
		if err != nil || oq.Expr != nil {
			return nil, domain.NewQueryError([]domain.Problem{{Code: domain.CodeSyntaxError, Message: "el orden no es válido: usa una lista de campos con ASC o DESC"}})
		}
		if problems := validationProblems(order, coreql.Validate(oq, catalog, nil)); len(problems) > 0 {
			return nil, domain.NewQueryError(prefixProblems("orden: ", problems))
		}
		if len(q.OrderBy) > 0 {
			p.warnings = append(p.warnings, domain.Warning{Code: "search.order_overridden", Message: "el orden indicado reemplaza al ORDER BY de la consulta"})
		}
		q.OrderBy = oq.OrderBy
	}
	p.query = q
	return p, nil
}

func prefixProblems(prefix string, in []domain.Problem) []domain.Problem {
	for i := range in {
		in[i].Message = prefix + in[i].Message
	}
	return in
}

// WithSuggestions sets the source of field value suggestions.
func (s *JQLService) WithSuggestions(src port.SuggestSource) *JQLService {
	s.suggest = src
	return s
}

// Search validates the query, runs it and returns one page.
func (s *JQLService) Search(ctx context.Context, in port.SearchInput) (port.SearchOutput, error) {
	p, err := s.prepare(ctx, in.Viewer, in.Query, in.Order)
	if err != nil {
		return port.SearchOutput{}, err
	}
	size, page := in.PageSize, in.Page
	var warnings = p.warnings
	switch {
	case size <= 0:
		size = DefaultSearchPageSize
	case size > MaxSearchPageSize:
		size = MaxSearchPageSize
		warnings = append(warnings, domain.Warning{Code: "search.page_size_clamped", Message: "el tamaño de página se limitó a 100"})
	}
	if page < 1 {
		page = 1
	}
	res, err := s.run(ctx, p, in.Query, in.ProjectID, page, size, in.WithTotal, false)
	if err != nil {
		return port.SearchOutput{}, err
	}
	return port.SearchOutput{Page: res, PageNumber: page, PageSize: size, Warnings: warnings, Normalized: normalize(p.query)}, nil
}

func (s *JQLService) run(ctx context.Context, p *prepared, text string, project *uuid.UUID, page, size int, withTotal, dryRun bool) (port.IssuePage, error) {
	res, err := s.searcher.SearchIssues(ctx, port.IssueQuery{
		Viewer: p.viewer, Query: p.query, Custom: p.custom, ProjectID: project, Now: s.clock.Now(),
		Page: page, PageSize: size, WithTotal: withTotal, DryRun: dryRun,
	})
	if err != nil {
		var be *port.BuildError
		if errors.As(err, &be) {
			return port.IssuePage{}, buildProblem(text, be)
		}
		return port.IssuePage{}, err
	}
	return res, nil
}

// Check validates a query text and order fully, including the build-time rules (unavailable fields, invalid
// values), without running it. It returns the normalized text.
func (s *JQLService) Check(ctx context.Context, viewer port.Viewer, text, order string, project *uuid.UUID) (string, error) {
	p, err := s.prepare(ctx, viewer, text, order)
	if err != nil {
		return "", err
	}
	if _, err := s.run(ctx, p, text, project, 1, 1, false, true); err != nil {
		return "", err
	}
	return normalize(p.query), nil
}

func normalize(q *coreql.Query) string {
	if q == nil {
		return ""
	}
	return q.String()
}

// Validate reports every problem of a query text without running it. Problems that make the request itself
// wrong (a too long text) are returned as errors.
func (s *JQLService) Validate(ctx context.Context, viewer port.Viewer, text string, project *uuid.UUID) (port.ValidationReport, error) {
	norm, err := s.Check(ctx, viewer, text, "", project)
	if err == nil {
		return port.ValidationReport{Valid: true, Normalized: norm}, nil
	}
	var qe *domain.QueryError
	if errors.As(err, &qe) {
		return port.ValidationReport{Problems: qe.Problems}, nil
	}
	return port.ValidationReport{}, err
}

// Suggest returns autocomplete data. With Field set it lists candidate values of the field; otherwise it
// completes the query at the cursor.
func (s *JQLService) Suggest(ctx context.Context, in port.SuggestInput) (port.SuggestOutput, error) {
	values := s.suggest
	limit := in.Limit
	if limit <= 0 || limit > 25 {
		limit = 10
	}
	var cc *customCatalog
	if s.custom != nil {
		fields, err := s.custom.CustomFields(ctx, in.Viewer)
		if err != nil {
			return port.SuggestOutput{}, err
		}
		cc = newCustomCatalog(fields)
	} else {
		cc = newCustomCatalog(nil)
	}
	catalog := coreql.CompositeCatalog{Top: coreql.SystemCatalog(), Base: cc}
	if in.Field == "" {
		if utf8.RuneCountInString(in.Query) > MaxQueryRunes {
			return port.SuggestOutput{}, domain.ErrCoreQLTooLong
		}
		cursor := in.Cursor
		if cursor < 0 || cursor > utf8.RuneCountInString(in.Query) {
			cursor = utf8.RuneCountInString(in.Query)
		}
		out := coreql.Suggest(in.Query, cursor, catalog)
		if len(out) > 50 {
			out = out[:50]
		}
		return port.SuggestOutput{Completions: out}, nil
	}
	if values == nil {
		return port.SuggestOutput{}, nil
	}
	name := strings.ToLower(in.Field)
	if _, ok := coreql.SystemCatalog().Lookup(in.Field); !ok {
		if _, ok := cc.Lookup(in.Field); !ok {
			return port.SuggestOutput{}, apperror.WithCode(apperror.KindValidation, domain.CodeUnknownField, "campo desconocido '"+in.Field+"'")
		}
		// Values of custom fields are not suggested.
		return port.SuggestOutput{}, nil
	}
	vals, err := values.SuggestValues(ctx, in.Viewer, name, strings.TrimSpace(in.Prefix), limit)
	if err != nil {
		return port.SuggestOutput{}, err
	}
	return port.SuggestOutput{Values: vals}, nil
}
