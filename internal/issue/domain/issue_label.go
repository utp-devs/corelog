package domain

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// MaxIssueLabels bounds the labels of one issue.
const MaxIssueLabels = 20

// MaxLabelNameLength bounds the name of a label (characters, not bytes).
const MaxLabelNameLength = 40

// ErrInvalidIssueLabel reports a label name that is empty, too long or there are too many labels.
var ErrInvalidIssueLabel = apperror.WithCode(apperror.KindValidation, "issue.invalid_label", "las etiquetas no son validas: cada una necesita un nombre de 1 a 40 caracteres y hay un maximo de 20")

// NormalizeLabelNames trims the names, drops repeated names (case-insensitive, first spelling wins) and
// validates the count and length. The result keeps the order of the input.
func NormalizeLabelNames(names []string) ([]string, error) {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" || utf8.RuneCountInString(trimmed) > MaxLabelNameLength {
			return nil, ErrInvalidIssueLabel
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	if len(out) > MaxIssueLabels {
		return nil, ErrInvalidIssueLabel
	}
	return out, nil
}

// LabelIDsText renders label ids as the stored change value: sorted, comma separated, empty when there are none.
func LabelIDsText(ids []string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",")
}
