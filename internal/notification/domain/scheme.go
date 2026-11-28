package domain

import (
	"regexp"
	"sort"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Event is something that can notify people. Schemes map each event to recipients and channels.
type Event string

// Events a scheme configures.
const (
	EventIssueCreated       Event = "issue_created"
	EventIssueAssigned      Event = "issue_assigned"
	EventIssueCommented     Event = "issue_commented"
	EventIssueMentioned     Event = "issue_mentioned"
	EventIssueTransitioned  Event = "issue_transitioned"
	EventIssueResolved      Event = "issue_resolved"
	EventIssueUpdated       Event = "issue_updated"
	EventCommentEdited      Event = "comment_edited"
	EventWorklogLogged      Event = "worklog_logged"
	EventVersionReleased    Event = "version_released"
	EventSprintStarted      Event = "sprint_started"
	EventSprintCompleted    Event = "sprint_completed"
	EventFilterSubscription Event = "filter_subscription"
)

// AllEvents lists every event in display order.
var AllEvents = []Event{
	EventIssueCreated, EventIssueAssigned, EventIssueCommented, EventIssueMentioned, EventIssueTransitioned,
	EventIssueResolved, EventIssueUpdated, EventCommentEdited, EventWorklogLogged, EventVersionReleased,
	EventSprintStarted, EventSprintCompleted, EventFilterSubscription,
}

// IsValid reports whether e is a known event.
func (e Event) IsValid() bool {
	for _, known := range AllEvents {
		if known == e {
			return true
		}
	}
	return false
}

// RecipientType says who receives a notification.
type RecipientType string

// Recipient types. reply_target is the author of the comment a reply answers.
const (
	RecipientReporter      RecipientType = "reporter"
	RecipientAssignee      RecipientType = "assignee"
	RecipientWatchers      RecipientType = "watchers"
	RecipientProjectLead   RecipientType = "project_lead"
	RecipientComponentLead RecipientType = "component_lead"
	RecipientMentioned     RecipientType = "mentioned"
	RecipientReplyTarget   RecipientType = "reply_target"
	RecipientUser          RecipientType = "user"
	RecipientTeam          RecipientType = "team"
	RecipientRole          RecipientType = "role"
)

// AllRecipientTypes lists every recipient type.
var AllRecipientTypes = []RecipientType{
	RecipientReporter, RecipientAssignee, RecipientWatchers, RecipientProjectLead, RecipientComponentLead,
	RecipientMentioned, RecipientReplyTarget, RecipientUser, RecipientTeam, RecipientRole,
}

// Channel is a delivery channel.
type Channel string

// Channels.
const (
	ChannelInApp Channel = "in_app"
	ChannelEmail Channel = "email"
)

// CodeSchemeInvalid is the stable code of a rejected scheme.
const CodeSchemeInvalid = "notification.scheme_invalid"

// ErrSchemeInvalid is the validation error for a bad scheme.
func ErrSchemeInvalid(message string) *apperror.Error {
	return apperror.WithCode(apperror.KindValidation, CodeSchemeInvalid, message)
}

// Recipient is one entry of a rule. ID is set for user and team, Role for role (a project role name).
type Recipient struct {
	Type RecipientType
	ID   uuid.UUID
	Role string
}

// Rule maps one event to recipients and channels.
type Rule struct {
	Event      Event
	Recipients []Recipient
	Channels   []Channel
}

// Scheme is a set of rules. A project scheme overrides the organization default, which overrides the built-in
// default, event by event.
type Scheme struct {
	Rules []Rule
}

const (
	maxRecipientsPerRule = 30
	maxRoleLength        = 30
)

var roleName = regexp.MustCompile("^[a-z][a-z0-9_]*$")

// DefaultScheme reproduces the behavior CoreLog had before schemes existed: assignment, mention and reply notices
// to the person concerned, in the application only. Every other event notifies nobody until a scheme enables it.
func DefaultScheme() Scheme {
	rules := make([]Rule, 0, len(AllEvents))
	for _, event := range AllEvents {
		rule := Rule{Event: event, Recipients: []Recipient{}, Channels: []Channel{ChannelInApp}}
		switch event {
		case EventIssueAssigned:
			rule.Recipients = []Recipient{{Type: RecipientAssignee}}
		case EventIssueMentioned:
			rule.Recipients = []Recipient{{Type: RecipientMentioned}}
		case EventIssueCommented:
			rule.Recipients = []Recipient{{Type: RecipientReplyTarget}}
		}
		rules = append(rules, rule)
	}
	return Scheme{Rules: rules}
}

// Validate checks and normalizes a scheme: known events (once each), known recipient types with their
// parameters, known channels, bounded sizes. Recipients and channels are de-duplicated.
func (s Scheme) Validate() (Scheme, error) {
	seen := map[Event]bool{}
	out := Scheme{Rules: make([]Rule, 0, len(s.Rules))}
	for _, rule := range s.Rules {
		if !rule.Event.IsValid() {
			return Scheme{}, ErrSchemeInvalid("Evento de notificacion desconocido: " + string(rule.Event))
		}
		if seen[rule.Event] {
			return Scheme{}, ErrSchemeInvalid("El evento " + string(rule.Event) + " esta repetido")
		}
		seen[rule.Event] = true
		if len(rule.Recipients) > maxRecipientsPerRule {
			return Scheme{}, ErrSchemeInvalid("Demasiados destinatarios en el evento " + string(rule.Event))
		}
		clean := Rule{Event: rule.Event, Recipients: []Recipient{}, Channels: []Channel{}}
		keys := map[string]bool{}
		for _, r := range rule.Recipients {
			if err := validateRecipient(r); err != nil {
				return Scheme{}, err
			}
			key := string(r.Type) + "|" + r.ID.String() + "|" + r.Role
			if !keys[key] {
				keys[key] = true
				clean.Recipients = append(clean.Recipients, r)
			}
		}
		channels := map[Channel]bool{}
		for _, c := range rule.Channels {
			if c != ChannelInApp && c != ChannelEmail {
				return Scheme{}, ErrSchemeInvalid("Canal de notificacion desconocido: " + string(c))
			}
			if !channels[c] {
				channels[c] = true
				clean.Channels = append(clean.Channels, c)
			}
		}
		out.Rules = append(out.Rules, clean)
	}
	return out, nil
}

func validateRecipient(r Recipient) error {
	known := false
	for _, t := range AllRecipientTypes {
		known = known || t == r.Type
	}
	if !known {
		return ErrSchemeInvalid("Tipo de destinatario desconocido: " + string(r.Type))
	}
	switch r.Type {
	case RecipientUser, RecipientTeam:
		if r.ID == uuid.Nil || r.Role != "" {
			return ErrSchemeInvalid("El destinatario " + string(r.Type) + " necesita un id")
		}
	case RecipientRole:
		if r.ID != uuid.Nil || len(r.Role) > maxRoleLength || !roleName.MatchString(r.Role) {
			return ErrSchemeInvalid("El destinatario role necesita un nombre de rol valido")
		}
	default:
		if r.ID != uuid.Nil || r.Role != "" {
			return ErrSchemeInvalid("El destinatario " + string(r.Type) + " no admite parametros")
		}
	}
	return nil
}

// Rule returns the rule of an event.
func (s Scheme) Rule(event Event) (Rule, bool) {
	for _, r := range s.Rules {
		if r.Event == event {
			return r, true
		}
	}
	return Rule{}, false
}

// Overlay returns base with every rule of s replacing the rule of the same event, in AllEvents order.
func (s Scheme) Overlay(base Scheme) Scheme {
	byEvent := map[Event]Rule{}
	for _, r := range base.Rules {
		byEvent[r.Event] = r
	}
	for _, r := range s.Rules {
		byEvent[r.Event] = r
	}
	events := make([]Event, 0, len(byEvent))
	for e := range byEvent {
		events = append(events, e)
	}
	order := map[Event]int{}
	for i, e := range AllEvents {
		order[e] = i
	}
	sort.Slice(events, func(a, b int) bool { return order[events[a]] < order[events[b]] })
	out := Scheme{Rules: make([]Rule, 0, len(events))}
	for _, e := range events {
		out.Rules = append(out.Rules, byEvent[e])
	}
	return out
}

// Facts are the people around an event, loaded by the use case.
type Facts struct {
	ReporterID       uuid.UUID
	AssigneeID       uuid.UUID
	WatcherIDs       []uuid.UUID
	ProjectLeadID    uuid.UUID
	ComponentLeadIDs []uuid.UUID
	MentionedIDs     []uuid.UUID
	ReplyTargetID    uuid.UUID
	// TeamMembers and RoleMembers hold the expansion of the teams and roles a rule names.
	TeamMembers map[uuid.UUID][]uuid.UUID
	RoleMembers map[string][]uuid.UUID
}

// Target is one person to notify and the channels the rule asks for.
type Target struct {
	UserID   uuid.UUID
	Channels []Channel
}

// Has reports whether the rule notifies a channel.
func (t Target) Has(c Channel) bool {
	for _, x := range t.Channels {
		if x == c {
			return true
		}
	}
	return false
}

// Resolve computes the people a rule notifies, without repeats and in a stable order. It does not apply the
// actor exclusion or user preferences.
func (r Rule) Resolve(f Facts) []Target {
	var ids []uuid.UUID
	add := func(list ...uuid.UUID) {
		for _, id := range list {
			if id != uuid.Nil {
				ids = append(ids, id)
			}
		}
	}
	for _, rec := range r.Recipients {
		switch rec.Type {
		case RecipientReporter:
			add(f.ReporterID)
		case RecipientAssignee:
			add(f.AssigneeID)
		case RecipientWatchers:
			add(f.WatcherIDs...)
		case RecipientProjectLead:
			add(f.ProjectLeadID)
		case RecipientComponentLead:
			add(f.ComponentLeadIDs...)
		case RecipientMentioned:
			add(f.MentionedIDs...)
		case RecipientReplyTarget:
			add(f.ReplyTargetID)
		case RecipientUser:
			add(rec.ID)
		case RecipientTeam:
			add(f.TeamMembers[rec.ID]...)
		case RecipientRole:
			add(f.RoleMembers[rec.Role]...)
		}
	}
	seen := map[uuid.UUID]bool{}
	targets := []Target{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		targets = append(targets, Target{UserID: id, Channels: append([]Channel(nil), r.Channels...)})
	}
	return targets
}
