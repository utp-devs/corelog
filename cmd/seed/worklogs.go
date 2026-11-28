package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// updateTimeSpentSQL recomputes the time spent of every issue from its worklogs. It is idempotent.
const updateTimeSpentSQL = `UPDATE issues SET time_spent_seconds = (
		SELECT COALESCE(SUM(w.time_spent_seconds), 0) FROM worklogs w
		WHERE w.issue_id = issues.id AND w.deleted_at IS NULL)`

// worklogTable returns the worklogs table of the dataset. The first call registers it together with the statement
// that recomputes the time spent of the issues once every table is written.
func worklogTable(ds *dataset) *table {
	_, existed := ds.byName["worklogs"]
	t := ds.ignoreDuplicates("worklogs", "id", "issue_id", "author_id", "started_at", "time_spent_seconds",
		"description", "remaining_mode", "remaining_adjust_seconds")
	if !existed {
		ds.runFinally(updateTimeSpentSQL)
	}
	return t
}

// addDemoWorklogs logs time on a deterministic part of the demo issues, authored by their assignees. Worklog
// ids derive from the issue position and the entry number, so a repeated seed inserts nothing new. The issue
// aggregate is recomputed from the worklogs after every table is written.
func addDemoWorklogs(ds *dataset, recs []issueRec) {
	t := worklogTable(ds)
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
}

// addIssueWorklogs logs time on one issue: one entry per duration (seconds), one per day going back from the
// start day, authored by author, and sets the original estimate so the remaining estimate and the time
// tracking report have data. Ids derive from the issue and the entry number.
func addIssueWorklogs(ds *dataset, issue, author uuid.UUID, startDay int, estimate int64, durations ...int64) {
	t := worklogTable(ds)
	for k, seconds := range durations {
		started := daysFromBase(startDay - k).Add(9 * time.Hour)
		t.add(idFor("worklog", "estimated", issue.String(), fmt.Sprint(k)), issue, author, started, seconds,
			"Avance registrado en la incidencia", "auto", int64(0))
	}
	ds.runFinally(fmt.Sprintf(`UPDATE issues SET original_estimate_seconds = %d,
		remaining_estimate_seconds = GREATEST(%d - time_spent_seconds, 0)
		WHERE id = '%s' AND original_estimate_seconds IS NULL`, estimate, estimate, issue))
}
