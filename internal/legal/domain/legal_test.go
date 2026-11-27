package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestAnonymizeIP(t *testing.T) {
	cases := map[string]string{
		"203.0.113.77":                    "203.0.113.0",
		"203.0.113.77:5555":               "203.0.113.0",
		"::ffff:10.1.2.3":                 "10.1.2.0",
		"2001:db8:aaaa:bbbb:1:2:3:4":      "2001:db8:aaaa:bbbb::",
		"[2001:db8:aaaa:bbbb:1:2:3:4]:80": "2001:db8:aaaa:bbbb::",
		"":                                "",
		"not-an-ip":                       "",
	}
	for in, want := range cases {
		if got := AnonymizeIP(in); got != want {
			t.Errorf("AnonymizeIP(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruncateUserAgent(t *testing.T) {
	if got := TruncateUserAgent(strings.Repeat("a", 400)); len(got) != MaxUserAgent {
		t.Fatalf("length = %d, want %d", len(got), MaxUserAgent)
	}
	if got := TruncateUserAgent("  Mozilla  "); got != "Mozilla" {
		t.Fatalf("got %q", got)
	}
}

func docs() []Document {
	return []Document{
		{Slug: SlugPrivacyPolicy, Version: 2, Title: "P", Required: true},
		{Slug: SlugDataProcessing, Version: 1, Title: "D", Required: true},
		{Slug: SlugCookies, Version: 1, Title: "C", Required: false},
	}
}

func TestComputeStatus(t *testing.T) {
	st := ComputeStatus(docs(), []Accepted{{Slug: SlugPrivacyPolicy, Version: 1}, {Slug: SlugDataProcessing, Version: 1}})
	if len(st.Pending) != 1 || st.Pending[0].Slug != SlugPrivacyPolicy || st.Pending[0].Version != 2 {
		t.Fatalf("pending = %+v", st.Pending)
	}
	if len(st.Accepted) != 2 {
		t.Fatalf("accepted = %+v", st.Accepted)
	}
	if st := ComputeStatus(docs(), nil); len(st.Pending) != 2 {
		t.Fatalf("a user without consents has every required document pending: %+v", st.Pending)
	}
}

func TestMissingRequired(t *testing.T) {
	cases := []struct {
		name string
		refs []Ref
		want []Slug
	}{
		{"all present", []Ref{{SlugPrivacyPolicy, 2}, {SlugDataProcessing, 1}}, nil},
		{"optional extra is fine", []Ref{{SlugPrivacyPolicy, 2}, {SlugDataProcessing, 1}, {SlugCommunications, 1}}, nil},
		{"missing one", []Ref{{SlugPrivacyPolicy, 2}}, []Slug{SlugDataProcessing}},
		{"wrong version", []Ref{{SlugPrivacyPolicy, 1}, {SlugDataProcessing, 1}}, []Slug{SlugPrivacyPolicy}},
		{"none", nil, []Slug{SlugPrivacyPolicy, SlugDataProcessing}},
	}
	for _, c := range cases {
		got := MissingRequired(docs(), c.refs)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v want %v", c.name, got, c.want)
			}
		}
	}
}

func TestErrConsentRequiredDetails(t *testing.T) {
	err := ErrConsentRequired([]Slug{SlugPrivacyPolicy, SlugTermsOfUse})
	if err.Kind != apperror.KindValidation || err.Code != "legal.consent_required" || len(err.Details) != 2 || err.Details[1].Field != "terms_of_use" {
		t.Fatalf("unexpected error %+v", err)
	}
	if p := ErrConsentPending([]Slug{SlugCookies}); p.Kind != apperror.KindForbidden || p.Code != "legal.consent_pending" {
		t.Fatalf("unexpected error %+v", p)
	}
}

func TestNewDraft(t *testing.T) {
	yes := true
	if _, err := NewDraft("bogus", "t", "", "body", nil, uuid.Nil); apperror.CodeOf(err) != "legal.slug_invalid" {
		t.Errorf("slug: %v", err)
	}
	if _, err := NewDraft(SlugPrivacyPolicy, " ", "", "body", nil, uuid.Nil); apperror.CodeOf(err) != "validation.failed" {
		t.Errorf("title: %v", err)
	}
	if _, err := NewDraft(SlugPrivacyPolicy, "t", "", strings.Repeat("x", MaxBodyLength+1), nil, uuid.Nil); apperror.CodeOf(err) != "validation.failed" {
		t.Errorf("body length: %v", err)
	}
	if _, err := NewDraft(SlugCommunications, "t", "", "b", &yes, uuid.Nil); apperror.CodeOf(err) != "legal.communications_optional" {
		t.Errorf("communications required: %v", err)
	}
	d, err := NewDraft(SlugTermsOfUse, "t", "", "b", nil, uuid.New())
	if err != nil || !d.Required || d.CreatedBy == nil {
		t.Errorf("default required: %+v %v", d, err)
	}
	if d, _ := NewDraft(SlugCookies, "t", "", "b", nil, uuid.Nil); d.Required {
		t.Error("cookies are optional by default")
	}
}
