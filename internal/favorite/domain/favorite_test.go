package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestParseEntityType(t *testing.T) {
	valid := []string{"project", "filter", "board", "dashboard", "issue", "team"}
	for _, raw := range valid {
		got, err := ParseEntityType(raw)
		if err != nil || string(got) != raw {
			t.Errorf("ParseEntityType(%q) = %q, %v; want the same type", raw, got, err)
		}
	}
	for _, raw := range []string{"", "Project", "epic", "projects"} {
		if _, err := ParseEntityType(raw); apperror.CodeOf(err) != "favorite.invalid_type" {
			t.Errorf("ParseEntityType(%q) code = %q, want favorite.invalid_type", raw, apperror.CodeOf(err))
		}
	}
}

func TestNewFavoriteRejectsNilIdentifiers(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	id := uuid.New()
	if _, err := NewFavorite(uuid.Nil, EntityProject, id, now); apperror.CodeOf(err) != "favorite.invalid_id" {
		t.Fatalf("nil user: code = %q", apperror.CodeOf(err))
	}
	if _, err := NewFavorite(id, EntityProject, uuid.Nil, now); apperror.CodeOf(err) != "favorite.invalid_id" {
		t.Fatalf("nil entity: code = %q", apperror.CodeOf(err))
	}
	fav, err := NewFavorite(id, EntityProject, id, now)
	if err != nil || !fav.CreatedAt.Equal(now) {
		t.Fatalf("valid favorite: %v, created_at %v", err, fav.CreatedAt)
	}
}

func TestNewRecentItemRejectsNilIdentifiers(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	if _, err := NewRecentItem(uuid.New(), EntityProject, uuid.Nil, now); apperror.CodeOf(err) != "favorite.invalid_id" {
		t.Fatalf("nil entity: code = %q", apperror.CodeOf(err))
	}
	item, err := NewRecentItem(uuid.New(), EntityProject, uuid.New(), now)
	if err != nil || !item.ViewedAt.Equal(now) {
		t.Fatalf("valid recent item: %v, viewed_at %v", err, item.ViewedAt)
	}
}
