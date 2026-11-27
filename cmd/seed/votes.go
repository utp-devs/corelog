package main

import (
	"math/rand/v2"

	"github.com/google/uuid"
)

// addDemoVotes gives about a quarter of the demo issues a few votes from the organization users. The vote_count
// column of every issue is set from the same votes, so the counter matches the vote rows.
func addDemoVotes(ds *dataset, rng *rand.Rand, recs []issueRec, users []uuid.UUID) {
	if len(users) == 0 {
		return
	}
	votes := ds.ignoreDuplicates("issue_votes", "issue_id", "user_id", "created_at")
	counts := map[uuid.UUID]int{}
	for _, r := range recs {
		if rng.Float64() >= 0.25 {
			continue
		}
		voters := 1 + rng.IntN(len(users))
		for _, i := range rng.Perm(len(users))[:voters] {
			votes.add(r.id, users[i], baseTime)
			counts[r.id]++
		}
	}
	issues := ds.byName["issues"]
	idIdx, counterIdx := columnIndex(issues, "id"), columnIndex(issues, "vote_count")
	for _, row := range issues.rows {
		row[counterIdx] = counts[row[idIdx].(uuid.UUID)]
	}
}
