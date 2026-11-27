package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/favorite/application/port"
	"github.com/jhonsferg/corelog/internal/favorite/domain"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
)

// fakeRepo keeps favorites and recents in memory with the same idempotency and retention rules as PostgreSQL.
type fakeRepo struct {
	favorites map[domain.Favorite]struct{}
	recent    map[uuid.UUID]domain.RecentItem
	entries   []port.Entry
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{favorites: map[domain.Favorite]struct{}{}, recent: map[uuid.UUID]domain.RecentItem{}}
}

func (f *fakeRepo) AddFavorite(_ context.Context, fav domain.Favorite) error {
	key := domain.Favorite{UserID: fav.UserID, Type: fav.Type, EntityID: fav.EntityID}
	f.favorites[key] = struct{}{}
	return nil
}

func (f *fakeRepo) RemoveFavorite(_ context.Context, userID uuid.UUID, entityType domain.EntityType, entityID uuid.UUID) error {
	delete(f.favorites, domain.Favorite{UserID: userID, Type: entityType, EntityID: entityID})
	return nil
}

func (f *fakeRepo) ListFavorites(context.Context, uuid.UUID, domain.EntityType) ([]port.Entry, error) {
	return f.entries, nil
}

func (f *fakeRepo) TouchRecent(_ context.Context, item domain.RecentItem) error {
	f.recent[item.EntityID] = item
	return nil
}

func (f *fakeRepo) ListRecent(_ context.Context, _ uuid.UUID, _ domain.EntityType, _ int) ([]port.Entry, error) {
	return f.entries, nil
}

// fakeVisibility answers from a fixed set of visible entity ids.
type fakeVisibility struct {
	visible map[uuid.UUID]bool
}

func (v fakeVisibility) CanView(_ context.Context, _, entityID uuid.UUID) (bool, error) {
	return v.visible[entityID], nil
}

func newTestService(repo *fakeRepo, visible ...uuid.UUID) *Service {
	set := map[uuid.UUID]bool{}
	for _, id := range visible {
		set[id] = true
	}
	clk := clock.FixedClock{T: time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)}
	return NewService(repo, map[domain.EntityType]port.EntityVisibility{
		domain.EntityProject: fakeVisibility{visible: set},
	}, clk)
}

func TestStarIsIdempotent(t *testing.T) {
	repo := newFakeRepo()
	user, project := uuid.New(), uuid.New()
	svc := newTestService(repo, project)
	for i := 0; i < 2; i++ {
		if err := svc.Star(context.Background(), user, "project", project); err != nil {
			t.Fatalf("star #%d: %v", i+1, err)
		}
	}
	if len(repo.favorites) != 1 {
		t.Fatalf("favorites stored = %d, want 1", len(repo.favorites))
	}
}

func TestStarInvisibleEntityIsNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	err := svc.Star(context.Background(), uuid.New(), "project", uuid.New())
	if apperror.KindOf(err) != apperror.KindNotFound || apperror.CodeOf(err) != "favorite.entity_not_found" {
		t.Fatalf("invisible star: kind %v code %q", apperror.KindOf(err), apperror.CodeOf(err))
	}
	if len(repo.favorites) != 0 {
		t.Fatal("invisible entity must not be stored")
	}
}

func TestUnsupportedTypeIsRejected(t *testing.T) {
	svc := newTestService(newFakeRepo(), uuid.New())
	err := svc.Star(context.Background(), uuid.New(), "issue", uuid.New())
	if apperror.KindOf(err) != apperror.KindValidation || apperror.CodeOf(err) != "favorite.type_not_supported" {
		t.Fatalf("unsupported type: kind %v code %q", apperror.KindOf(err), apperror.CodeOf(err))
	}
	err = svc.Star(context.Background(), uuid.New(), "epic", uuid.New())
	if apperror.CodeOf(err) != "favorite.invalid_type" {
		t.Fatalf("unknown type: code %q", apperror.CodeOf(err))
	}
}

func TestUnstarDoesNotRequireVisibility(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	if err := svc.Unstar(context.Background(), uuid.New(), "project", uuid.New()); err != nil {
		t.Fatalf("unstar of an invisible entity must succeed: %v", err)
	}
}

func TestListFavoritesFiltersInvisibleEntries(t *testing.T) {
	visible, hidden := uuid.New(), uuid.New()
	repo := newFakeRepo()
	repo.entries = []port.Entry{
		{Type: domain.EntityProject, ID: visible, Name: "Visible"},
		{Type: domain.EntityProject, ID: hidden, Name: "Hidden"},
	}
	svc := newTestService(repo, visible)
	entries, err := svc.ListFavorites(context.Background(), uuid.New(), "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != visible {
		t.Fatalf("entries = %+v, want only the visible project", entries)
	}
}

func TestListRecentLimits(t *testing.T) {
	repo := newFakeRepo()
	ids := make([]uuid.UUID, 0, 60)
	for i := 0; i < 60; i++ {
		id := uuid.New()
		ids = append(ids, id)
		repo.entries = append(repo.entries, port.Entry{Type: domain.EntityProject, ID: id})
	}
	svc := newTestService(repo, ids...)
	ctx := context.Background()

	cases := map[int]int{0: domain.DefaultRecentLimit, 5: 5, 500: domain.MaxRecentLimit}
	for limit, want := range cases {
		entries, err := svc.ListRecent(ctx, uuid.New(), "project", limit)
		if err != nil || len(entries) != want {
			t.Fatalf("limit %d: got %d entries, err %v; want %d", limit, len(entries), err, want)
		}
	}
	if _, err := svc.ListRecent(ctx, uuid.New(), "project", -1); apperror.CodeOf(err) != "favorite.invalid_limit" {
		t.Fatalf("negative limit: code %q", apperror.CodeOf(err))
	}
}

func TestRecordViewChecksVisibility(t *testing.T) {
	repo := newFakeRepo()
	project := uuid.New()
	svc := newTestService(repo, project)
	if err := svc.RecordView(context.Background(), uuid.New(), "project", uuid.New()); apperror.KindOf(err) != apperror.KindNotFound {
		t.Fatalf("hidden view: kind %v", apperror.KindOf(err))
	}
	if err := svc.RecordView(context.Background(), uuid.New(), "project", project); err != nil {
		t.Fatalf("visible view: %v", err)
	}
	if len(repo.recent) != 1 {
		t.Fatalf("recent items = %d, want 1", len(repo.recent))
	}
}
