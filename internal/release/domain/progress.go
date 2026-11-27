package domain

// Status categories of the workflow statuses, as returned by the database category function.
const (
	CategoryTodo       = "todo"
	CategoryInProgress = "in_progress"
	CategoryDone       = "done"
)

// Progress counts the fix issues of a version by status category, with their story points.
type Progress struct {
	IssuesTotal int
	IssuesDone  int
	ByCategory  map[string]int
	PointsTotal float64
	PointsDone  float64
}

// CategoryCount is one row of the grouped progress query: issues and points of one category.
type CategoryCount struct {
	Category string
	Issues   int
	Points   float64
}

// NewProgress builds the progress of a version from the grouped counts. Every category is present, even at zero.
func NewProgress(rows []CategoryCount) Progress {
	p := Progress{ByCategory: map[string]int{CategoryTodo: 0, CategoryInProgress: 0, CategoryDone: 0}}
	for _, row := range rows {
		p.IssuesTotal += row.Issues
		p.PointsTotal += row.Points
		if _, known := p.ByCategory[row.Category]; known {
			p.ByCategory[row.Category] += row.Issues
		}
		if row.Category == CategoryDone {
			p.IssuesDone += row.Issues
			p.PointsDone += row.Points
		}
	}
	return p
}
