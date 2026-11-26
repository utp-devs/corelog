package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

func init() { Register(scaleScenario{}) }

// scaleScenario seeds the demo organization plus extra accounts (one per 1000
// issues) and --issues issues with their comments, labels and watchers. It is
// meant for performance checks, so it skips links, sprints and notifications.
type scaleScenario struct{}

func (scaleScenario) Name() string { return "scale" }

func (scaleScenario) Run(ctx context.Context, db *sqlx.DB, opts Options) error {
	if opts.Issues <= 0 {
		return errors.New("--issues debe ser mayor que 0")
	}
	return persist(ctx, db, buildScale(opts.PasswordHash, opts.Issues))
}

func buildScale(passwordHash string, issues int) *dataset {
	ds := newDataset()
	rng := newRNG(streamScale)

	spec := demoOrgSpec()
	for k := 1; k <= issues/1000; k++ {
		role := "requester"
		if k%5 == 0 {
			role = "agent"
		}
		spec.people = append(spec.people, person{
			name:  fmt.Sprintf("Usuario de escala %03d", k),
			email: fmt.Sprintf("escala%03d@corelog.test", k),
			role:  role,
			team:  1 + k%len(spec.teams),
		})
	}

	org := buildOrg(ds, spec, passwordHash)
	emitIssues(ds, generateIssues(rng, org, issues))
	return ds
}
