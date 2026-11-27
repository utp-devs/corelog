package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestComputeChildProgress(t *testing.T) {
	child := func(status Status) *Issue { return &Issue{ID: uuid.New(), Status: status} }
	cases := []struct {
		name     string
		children []*Issue
		want     ChildProgress
	}{
		{name: "no children", want: ChildProgress{}},
		{name: "none done", children: []*Issue{child(StatusOpen), child(StatusInProgress)}, want: ChildProgress{Total: 2, Done: 0, Percent: 0}},
		{name: "resolved and closed count as done", children: []*Issue{child(StatusResolved), child(StatusClosed), child(StatusOpen)}, want: ChildProgress{Total: 3, Done: 2, Percent: 67}},
		{name: "all done", children: []*Issue{child(StatusClosed)}, want: ChildProgress{Total: 1, Done: 1, Percent: 100}},
		{name: "half rounds up", children: []*Issue{child(StatusClosed), child(StatusOpen), child(StatusOpen), child(StatusOpen), child(StatusOpen), child(StatusOpen), child(StatusOpen), child(StatusOpen)}, want: ChildProgress{Total: 8, Done: 1, Percent: 13}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeChildProgress(tc.children); got != tc.want {
				t.Fatalf("ComputeChildProgress = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNewIssueLinkRules(t *testing.T) {
	a, b, actor := uuid.New(), uuid.New(), uuid.New()
	at := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	if _, err := NewIssueLink(a, a, actor, IssueLinkRelates, at); err != ErrSelfLink {
		t.Fatalf("self link = %v", err)
	}
	if _, err := NewIssueLink(a, b, actor, IssueLinkKind("nope"), at); err != ErrUnknownLinkKind {
		t.Fatalf("unknown kind = %v", err)
	}
	if _, err := NewIssueLink(a, b, uuid.Nil, IssueLinkRelates, at); err != ErrInvalidIssueLink {
		t.Fatalf("missing actor = %v", err)
	}
	link, err := NewIssueLink(a, b, actor, IssueLinkCauses, at)
	if err != nil || link.Kind != IssueLinkCauses || !link.CreatedAt.Equal(at) {
		t.Fatalf("valid link = %+v, %v", link, err)
	}
	if !IssueLinkBlocks.ForbidsCycles() || IssueLinkRelates.ForbidsCycles() {
		t.Fatal("only blocks forbids cycles")
	}
}
