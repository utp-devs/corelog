package port

import "strings"

// CustomRef normalizes the spelling of a custom field reference: lower case, with the key of cf[<key>] trimmed.
// Bare names are only lower-cased.
func CustomRef(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(lower, "cf[") && strings.HasSuffix(lower, "]") {
		return "cf[" + strings.TrimSpace(lower[3:len(lower)-1]) + "]"
	}
	return lower
}
