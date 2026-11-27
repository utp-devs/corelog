package domain

// IsDone reports whether the status belongs to the done category. Legacy statuses resolved and closed are done.
func (s Status) IsDone() bool {
	return s == StatusResolved || s == StatusClosed
}

// ChildProgress summarises how many child issues are done.
type ChildProgress struct {
	Total   int
	Done    int
	Percent int
}

// ComputeChildProgress counts the children in the done category. Percent is rounded to the nearest integer and is
// zero when there are no children.
func ComputeChildProgress(children []*Issue) ChildProgress {
	progress := ChildProgress{Total: len(children)}
	for _, child := range children {
		if child.Status.IsDone() {
			progress.Done++
		}
	}
	if progress.Total > 0 {
		progress.Percent = (progress.Done*100 + progress.Total/2) / progress.Total
	}
	return progress
}
