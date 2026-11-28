package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestDefaultSchemeCoversEveryEventAndKeepsTodaysRules(t *testing.T) {
	scheme := DefaultScheme()
	if len(scheme.Rules) != len(AllEvents) {
		t.Fatalf("%d rules for %d events", len(scheme.Rules), len(AllEvents))
	}
	active := map[Event]RecipientType{EventIssueAssigned: RecipientAssignee, EventIssueMentioned: RecipientMentioned, EventIssueCommented: RecipientReplyTarget}
	for _, rule := range scheme.Rules {
		want, on := active[rule.Event]
		if on && (len(rule.Recipients) != 1 || rule.Recipients[0].Type != want) {
			t.Errorf("%s: %+v", rule.Event, rule.Recipients)
		}
		if !on && len(rule.Recipients) != 0 {
			t.Errorf("%s must notify nobody by default", rule.Event)
		}
		if len(rule.Channels) != 1 || rule.Channels[0] != ChannelInApp {
			t.Errorf("%s channels %v: the default is in-app only", rule.Event, rule.Channels)
		}
	}
	if _, err := scheme.Validate(); err != nil {
		t.Fatalf("the default scheme is invalid: %v", err)
	}
}

func TestSchemeValidate(t *testing.T) {
	id := uuid.New()
	long := make([]Recipient, 31)
	for i := range long {
		long[i] = Recipient{Type: RecipientReporter}
	}
	cases := map[string]struct {
		rules []Rule
		ok    bool
	}{
		"empty":              {nil, true},
		"user":               {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientUser, ID: id}}, Channels: []Channel{ChannelEmail}}}, true},
		"team":               {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientTeam, ID: id}}}}, true},
		"role":               {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientRole, Role: "developer"}}}}, true},
		"unknown event":      {[]Rule{{Event: "x"}}, false},
		"duplicate event":    {[]Rule{{Event: EventIssueCreated}, {Event: EventIssueCreated}}, false},
		"unknown type":       {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: "boss"}}}}, false},
		"user without id":    {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientUser}}}}, false},
		"team with role":     {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientTeam, ID: id, Role: "x"}}}}, false},
		"role bad name":      {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientRole, Role: "Dev Ops"}}}}, false},
		"role with id":       {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientRole, Role: "dev", ID: id}}}}, false},
		"reporter with id":   {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientReporter, ID: id}}}}, false},
		"unknown channel":    {[]Rule{{Event: EventIssueCreated, Channels: []Channel{"sms"}}}, false},
		"too many entries":   {[]Rule{{Event: EventIssueCreated, Recipients: long}}, false},
		"role name too long": {[]Rule{{Event: EventIssueCreated, Recipients: []Recipient{{Type: RecipientRole, Role: strings.Repeat("a", 31)}}}}, false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Scheme{Rules: c.rules}.Validate()
			if c.ok != (err == nil) {
				t.Fatalf("err = %v, want ok=%v", err, c.ok)
			}
			if err != nil && apperror.CodeOf(err) != CodeSchemeInvalid {
				t.Fatalf("code = %q", apperror.CodeOf(err))
			}
		})
	}
}

func TestOverlayKeepsDisplayOrderAndPrefersTheOverlay(t *testing.T) {
	over := Scheme{Rules: []Rule{{Event: EventSprintStarted, Recipients: []Recipient{{Type: RecipientProjectLead}}}}}
	merged := over.Overlay(DefaultScheme())
	if len(merged.Rules) != len(AllEvents) {
		t.Fatalf("%d rules", len(merged.Rules))
	}
	for i, rule := range merged.Rules {
		if rule.Event != AllEvents[i] {
			t.Fatalf("rule %d is %s, want %s", i, rule.Event, AllEvents[i])
		}
	}
	if r, _ := merged.Rule(EventSprintStarted); len(r.Recipients) != 1 {
		t.Fatalf("overlay lost: %+v", r)
	}
	if _, ok := merged.Rule("nope"); ok {
		t.Fatal("unknown rule found")
	}
}

func TestDescribeAndTruncate(t *testing.T) {
	cases := []struct {
		event                       Event
		key, summary, actor, detail string
		want                        string
	}{
		{EventIssueCreated, "WEB-1", "Rota", "Ana", "", "Ana creo WEB-1: Rota"},
		{EventIssueAssigned, "WEB-1", "Rota", "", "", "Te asignaron la incidencia: WEB-1: Rota"},
		{EventIssueTransitioned, "WEB-1", "", "Ana", "A -> B", "Ana cambio el estado de WEB-1 (A -> B)"},
		{EventSprintStarted, "", "", "", "Sprint 3", "Alguien inicio el sprint (Sprint 3)"},
		{EventVersionReleased, "", "", "Ana", "1.0", "Ana publico la version (1.0)"},
		{EventIssueCommented, "", "", "Ana", "", "Ana comento en una incidencia"},
	}
	for _, c := range cases {
		if got := Describe(c.event, c.key, c.summary, c.actor, c.detail); got != c.want {
			t.Errorf("%s: %q, want %q", c.event, got, c.want)
		}
	}
	for _, event := range AllEvents {
		if Describe(event, "K-1", "s", "a", "d") == "" {
			t.Errorf("%s has no text", event)
		}
	}
	long := Describe(EventIssueUpdated, "K-1", strings.Repeat("x", 400), "Ana", "")
	if n := len([]rune(long)); n != 300 || !strings.HasSuffix(long, "…") {
		t.Fatalf("truncated to %d runes: %q", n, long[len(long)-5:])
	}
}

func TestEmailModeValidity(t *testing.T) {
	for _, m := range []EmailMode{EmailImmediate, EmailDaily, EmailOff} {
		if !m.IsValid() {
			t.Errorf("%s must be valid", m)
		}
	}
	if EmailMode("weekly").IsValid() || EmailMode("").IsValid() {
		t.Fatal("invalid mode accepted")
	}
	if p := DefaultPreferences(); !p.InApp || p.Email != EmailImmediate || p.NotifyOwnActions {
		t.Fatalf("defaults %+v", p)
	}
}

func TestTargetHas(t *testing.T) {
	target := Target{Channels: []Channel{ChannelEmail}}
	if !target.Has(ChannelEmail) || target.Has(ChannelInApp) {
		t.Fatal("Has is wrong")
	}
}
