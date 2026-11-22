package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/team/domain"
)

type fakeTeamRepo struct {
	teams map[uuid.UUID]*domain.Team
}

func newFakeTeamRepo() *fakeTeamRepo {
	return &fakeTeamRepo{teams: make(map[uuid.UUID]*domain.Team)}
}

func (r *fakeTeamRepo) Save(_ context.Context, t *domain.Team) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	r.teams[t.ID] = &copied
	return nil
}

func (r *fakeTeamRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Team, error) {
	t, ok := r.teams[id]
	if !ok {
		return nil, domain.ErrTeamNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	return &copied, nil
}

func (r *fakeTeamRepo) FindByName(_ context.Context, name string) (*domain.Team, error) {
	for _, t := range r.teams {
		if t.Name == name {
			copied := *t
			return &copied, nil
		}
	}
	return nil, domain.ErrTeamNotFound
}

func (r *fakeTeamRepo) FindAll(_ context.Context) ([]*domain.Team, error) {
	var list []*domain.Team
	for _, t := range r.teams {
		copied := *t
		list = append(list, &copied)
	}
	return list, nil
}

func (r *fakeTeamRepo) Update(_ context.Context, t *domain.Team) error {
	if _, ok := r.teams[t.ID]; !ok {
		return domain.ErrTeamNotFound
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	copied := *t
	r.teams[t.ID] = &copied
	return nil
}

func (r *fakeTeamRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.teams[id]; !ok {
		return domain.ErrTeamNotFound
	}
	delete(r.teams, id)
	return nil
}

func TestTeamService_Create(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTeamRepo()
	svc := NewTeamService(repo)

	t.Run("successfully creates team", func(t *testing.T) {
		team, err := svc.Create(ctx, "Frontend Team")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if team.Name != "Frontend Team" {
			t.Errorf("got name %s, want 'Frontend Team'", team.Name)
		}

		stored, err := repo.FindByID(ctx, team.ID)
		if err != nil {
			t.Fatalf("team should be stored in repository: %v", err)
		}
		if stored.Name != "Frontend Team" {
			t.Errorf("got stored name %s, want 'Frontend Team'", stored.Name)
		}
	})

	t.Run("rejects duplicate team name", func(t *testing.T) {
		_, err := svc.Create(ctx, "Frontend Team")
		if !errors.Is(err, domain.ErrTeamNameTaken) {
			t.Errorf("expected ErrTeamNameTaken, got %v", err)
		}
	})

	t.Run("rejects invalid team name", func(t *testing.T) {
		_, err := svc.Create(ctx, "   ")
		if !errors.Is(err, domain.ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})
}

func TestTeamService_GetAndList(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTeamRepo()
	svc := NewTeamService(repo)

	team1, _ := svc.Create(ctx, "Core")
	team2, _ := svc.Create(ctx, "DevOps")

	t.Run("Get returns team by ID", func(t *testing.T) {
		got, err := svc.Get(ctx, team1.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.ID != team1.ID {
			t.Errorf("got ID %v, want %v", got.ID, team1.ID)
		}
	})

	t.Run("Get returns not found for unknown ID", func(t *testing.T) {
		_, err := svc.Get(ctx, uuid.New())
		if !errors.Is(err, domain.ErrTeamNotFound) {
			t.Errorf("expected ErrTeamNotFound, got %v", err)
		}
	})

	t.Run("List returns all teams", func(t *testing.T) {
		list, err := svc.List(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("got %d teams, want 2", len(list))
		}
		_ = team2
	})
}

func TestTeamService_Rename(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTeamRepo()
	svc := NewTeamService(repo)

	team1, _ := svc.Create(ctx, "Backend")
	team2, _ := svc.Create(ctx, "QA")

	t.Run("renames team successfully", func(t *testing.T) {
		updated, err := svc.Rename(ctx, team1.ID, "Platform")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if updated.Name != "Platform" {
			t.Errorf("got name %s, want 'Platform'", updated.Name)
		}
	})

	t.Run("keeping same name is allowed", func(t *testing.T) {
		_, err := svc.Rename(ctx, team1.ID, "Platform")
		if err != nil {
			t.Errorf("unexpected error renaming to same name: %v", err)
		}
	})

	t.Run("rejects renaming to existing team name", func(t *testing.T) {
		_, err := svc.Rename(ctx, team1.ID, "QA")
		if !errors.Is(err, domain.ErrTeamNameTaken) {
			t.Errorf("expected ErrTeamNameTaken, got %v", err)
		}
	})

	t.Run("rejects invalid name", func(t *testing.T) {
		_, err := svc.Rename(ctx, team1.ID, "")
		if !errors.Is(err, domain.ErrInvalidName) {
			t.Errorf("expected ErrInvalidName, got %v", err)
		}
	})

	t.Run("returns not found for unknown ID", func(t *testing.T) {
		_, err := svc.Rename(ctx, uuid.New(), "New")
		if !errors.Is(err, domain.ErrTeamNotFound) {
			t.Errorf("expected ErrTeamNotFound, got %v", err)
		}
	})
	_ = team2
}

func TestTeamService_Delete(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTeamRepo()
	svc := NewTeamService(repo)

	team, _ := svc.Create(ctx, "To Delete")

	t.Run("deletes team successfully", func(t *testing.T) {
		err := svc.Delete(ctx, team.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = svc.Get(ctx, team.ID)
		if !errors.Is(err, domain.ErrTeamNotFound) {
			t.Errorf("expected ErrTeamNotFound after delete, got %v", err)
		}
	})

	t.Run("deleting non-existent team returns not found", func(t *testing.T) {
		err := svc.Delete(ctx, uuid.New())
		if !errors.Is(err, domain.ErrTeamNotFound) {
			t.Errorf("expected ErrTeamNotFound, got %v", err)
		}
	})
}
