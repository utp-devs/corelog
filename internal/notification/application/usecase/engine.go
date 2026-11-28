package usecase

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/application/port"
	"github.com/jhonsferg/corelog/internal/notification/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
)

// Engine turns a notice into in-app notifications and emails following the scheme of the project, the
// preferences of each recipient and the rule that nobody is told about their own actions.
type Engine struct {
	schemes  port.SchemeRepository
	facts    port.FactsSource
	notes    port.Repository
	outbox   port.Outbox
	delivery port.DeliveryStore
	browser  port.ProjectBrowser
	render   port.EmailRenderer
	tx       database.TxManager
	clock    clock.Clock
	baseURL  string
}

// EngineDeps are the collaborators of the engine. Browser may be nil (recipients are not checked).
type EngineDeps struct {
	Schemes  port.SchemeRepository
	Facts    port.FactsSource
	Notes    port.Repository
	Outbox   port.Outbox
	Delivery port.DeliveryStore
	Browser  port.ProjectBrowser
	Render   port.EmailRenderer
	Tx       database.TxManager
	Clock    clock.Clock
	// BaseURL is the public address of the application, used in email links.
	BaseURL string
}

// NewEngine wires the engine.
func NewEngine(d EngineDeps) *Engine {
	return &Engine{
		schemes: d.Schemes, facts: d.Facts, notes: d.Notes, outbox: d.Outbox, delivery: d.Delivery, browser: d.Browser,
		render: d.Render, tx: d.Tx, clock: d.Clock, baseURL: strings.TrimRight(d.BaseURL, "/"),
	}
}

// Effective returns the scheme in force for a project: its own rules over the organization default over the
// built-in default.
func (e *Engine) Effective(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) (domain.Scheme, error) {
	return effectiveScheme(ctx, e.schemes, orgID, projectID)
}

func effectiveScheme(ctx context.Context, repo port.SchemeRepository, orgID, projectID uuid.UUID) (domain.Scheme, error) {
	scheme := domain.DefaultScheme()
	if orgID != uuid.Nil {
		stored, err := repo.Get(ctx, orgID, nil)
		if err != nil {
			return domain.Scheme{}, err
		}
		if stored != nil {
			scheme = stored.Scheme.Overlay(scheme)
		}
		if projectID != uuid.Nil {
			pid := projectID
			stored, err := repo.Get(ctx, orgID, &pid)
			if err != nil {
				return domain.Scheme{}, err
			}
			if stored != nil {
				scheme = stored.Scheme.Overlay(scheme)
			}
		}
	}
	return scheme, nil
}

// Deliver notifies the recipients of the notice. It is idempotent per recipient when the notice has a DedupKey.
func (e *Engine) Deliver(ctx context.Context, n port.Notice) error {
	issue, project, err := e.loadContext(ctx, &n)
	if err != nil {
		return err
	}
	scheme, err := e.Effective(ctx, n.OrgID, n.ProjectID)
	if err != nil {
		return err
	}
	rule, ok := scheme.Rule(n.Event)
	if !ok || len(rule.Recipients) == 0 || len(rule.Channels) == 0 {
		return nil
	}
	facts, err := e.buildFacts(ctx, n, rule, issue, project)
	if err != nil {
		return err
	}
	targets := rule.Resolve(facts)
	if n.Only != uuid.Nil {
		targets = onlyTarget(targets, n.Only)
	}
	if len(targets) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(targets)+1)
	for _, t := range targets {
		ids = append(ids, t.UserID)
	}
	if n.ActorID != uuid.Nil && n.ActorName == "" {
		ids = append(ids, n.ActorID)
	}
	contacts, err := e.facts.Contacts(ctx, ids)
	if err != nil {
		return err
	}
	if n.ActorName == "" {
		n.ActorName = contacts[n.ActorID].Name
	}
	key, summary := "", ""
	if issue != nil {
		key, summary = issue.Key, issue.Summary
	}
	if n.Message == "" {
		n.Message = domain.Describe(n.Event, key, summary, n.ActorName, n.Detail)
	}
	if n.Link == "" && n.IssueID != uuid.Nil {
		n.Link = "/issues/" + n.IssueID.String()
	}
	if n.Kind == "" {
		n.Kind = string(n.Event)
	}
	for _, target := range targets {
		if err := e.deliverTo(ctx, n, target, contacts[target.UserID], key, summary); err != nil {
			return err
		}
	}
	return nil
}

func onlyTarget(targets []domain.Target, user uuid.UUID) []domain.Target {
	for _, t := range targets {
		if t.UserID == user {
			return []domain.Target{t}
		}
	}
	return nil
}

// loadContext fills the organization and project of the notice from its issue or project.
func (e *Engine) loadContext(ctx context.Context, n *port.Notice) (*port.IssueFacts, *port.ProjectFacts, error) {
	if n.IssueID != uuid.Nil {
		issue, err := e.facts.Issue(ctx, n.IssueID)
		if err != nil {
			return nil, nil, err
		}
		if issue != nil {
			n.OrgID, n.ProjectID = issue.OrgID, issue.ProjectID
			return issue, nil, nil
		}
	}
	if n.ProjectID != uuid.Nil {
		project, err := e.facts.Project(ctx, n.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		if project != nil {
			n.OrgID = project.OrgID
			return nil, project, nil
		}
	}
	return nil, nil, nil
}

func (e *Engine) buildFacts(ctx context.Context, n port.Notice, rule domain.Rule, issue *port.IssueFacts, project *port.ProjectFacts) (domain.Facts, error) {
	var f domain.Facts
	if issue != nil {
		f.ReporterID, f.AssigneeID, f.WatcherIDs = issue.ReporterID, issue.AssigneeID, issue.WatcherIDs
		f.ComponentLeadIDs, f.ProjectLeadID = issue.ComponentLeadIDs, issue.ProjectLeadID
	}
	if project != nil {
		f.ProjectLeadID = project.LeadID
	}
	if ids := n.Explicit[domain.RecipientAssignee]; len(ids) > 0 {
		f.AssigneeID = ids[0]
	}
	if ids := n.Explicit[domain.RecipientReplyTarget]; len(ids) > 0 {
		f.ReplyTargetID = ids[0]
	}
	f.MentionedIDs = n.Explicit[domain.RecipientMentioned]

	var teams []uuid.UUID
	var roles []string
	for _, r := range rule.Recipients {
		switch r.Type {
		case domain.RecipientTeam:
			teams = append(teams, r.ID)
		case domain.RecipientRole:
			roles = append(roles, r.Role)
		}
	}
	var err error
	if len(teams) > 0 {
		if f.TeamMembers, err = e.facts.TeamMembers(ctx, teams); err != nil {
			return f, err
		}
	}
	if len(roles) > 0 && n.ProjectID != uuid.Nil {
		if f.RoleMembers, err = e.facts.RoleMembers(ctx, n.ProjectID, roles); err != nil {
			return f, err
		}
	}
	return f, nil
}

// deliverTo applies the preferences of one recipient and writes the notification, the email or the digest item
// in one transaction.
func (e *Engine) deliverTo(ctx context.Context, n port.Notice, target domain.Target, contact port.Contact, key, summary string) error {
	if contact.ID == uuid.Nil || !contact.Active {
		return nil
	}
	prefs, err := e.notes.GetPreferences(ctx, target.UserID)
	if err != nil {
		return err
	}
	if n.ActorID == target.UserID && !prefs.NotifyOwnActions {
		return nil
	}
	if n.ProjectID != uuid.Nil && e.browser != nil && !n.SkipVisibility {
		browse, err := e.browser.CanBrowse(ctx, target.UserID, n.ProjectID)
		if err != nil {
			return err
		}
		if !browse {
			return nil
		}
	}
	wantInApp := target.Has(domain.ChannelInApp) && prefs.InApp
	mode := prefs.Email
	if mode == "" {
		mode = domain.EmailImmediate
	}
	wantEmail := target.Has(domain.ChannelEmail) && mode != domain.EmailOff && contact.Email != ""
	if !wantInApp && !wantEmail {
		return nil
	}
	return e.tx.InTx(ctx, func(ctx context.Context) error {
		now := e.clock.Now()
		if n.DedupKey != "" {
			first, err := e.delivery.Claim(ctx, n.DedupKey, target.UserID, now)
			if err != nil {
				return err
			}
			if !first {
				return nil
			}
		}
		if wantInApp {
			note, err := domain.New(target.UserID, n.Kind, n.Message, n.Link)
			if err != nil {
				return err
			}
			note.CreatedAt = now
			if err := e.notes.Save(ctx, note); err != nil {
				return err
			}
		}
		if !wantEmail {
			return nil
		}
		if mode == domain.EmailDaily {
			return e.delivery.AddDigestItem(ctx, port.DigestItem{
				UserID: target.UserID, Event: n.Event, IssueKey: key, Summary: domain.Truncate(summary, 300),
				ActorName: n.ActorName, Link: n.Link, CreatedAt: now,
			})
		}
		rendered := e.render.Notice(port.Email{
			Event: n.Event, Greeting: contact.Name, IssueKey: key, Summary: summary, ActorName: n.ActorName, Detail: n.Detail,
			Link: e.absoluteLink(key, n.Link), Preference: e.baseURL + "/profile/notifications",
		})
		idem := ""
		if n.DedupKey != "" {
			idem = "notice:" + n.DedupKey + ":" + target.UserID.String()
		}
		_, err := e.outbox.Enqueue(ctx, port.EmailMessage{
			UserID: target.UserID, To: contact.Email, Kind: string(n.Event), Subject: rendered.Subject,
			Text: rendered.Text, HTML: rendered.HTML, IdempotencyKey: idem,
		}, now)
		return err
	})
}

// absoluteLink is the deep link of an email: the issue page when the notice has an issue key.
func (e *Engine) absoluteLink(key, link string) string {
	if key != "" {
		return e.baseURL + "/browse/" + key
	}
	if strings.HasPrefix(link, "/") {
		return e.baseURL + link
	}
	return e.baseURL
}
