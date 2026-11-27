package main

import (
	"fmt"
	"time"
)

// addDemoWorklogs logs time on a deterministic part of the demo issues, authored by their assignees. Worklog
// ids derive from the issue position and the entry number, so a repeated seed inserts nothing new. The issue
// aggregate is recomputed from the worklogs after every table is written.
func addDemoWorklogs(ds *dataset, recs []issueRec) {
	t := ds.ignoreDuplicates("worklogs", "id", "issue_id", "author_id", "started_at", "time_spent_seconds",
		"description", "remaining_mode", "remaining_adjust_seconds")
	durations := []int64{1800, 3600, 7200}
	for idx, r := range recs {
		n := idx + 1
		if r.assignee == nil || n%4 != 0 {
			continue
		}
		entries := 1 + n%3
		for k := 0; k < entries; k++ {
			started := daysFromBase(-(n % 20) - k).Add(10 * time.Hour)
			id := idFor("worklog", fmt.Sprint(n), fmt.Sprint(k))
			t.add(id, r.id, *r.assignee, started, durations[(n+k)%len(durations)], "Trabajo en la incidencia", "leave", int64(0))
		}
	}
	ds.runFinally(`UPDATE issues SET time_spent_seconds = (
		SELECT COALESCE(SUM(w.time_spent_seconds), 0) FROM worklogs w
		WHERE w.issue_id = issues.id AND w.deleted_at IS NULL)`)
}
