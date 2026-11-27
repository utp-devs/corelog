package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// keyPattern is the human key of an issue: the project key, a hyphen and the issue number.
var keyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}-[0-9]+$`)

// ParseKey splits a human issue key such as CORE-12 into the project key and the number.
// ok is false when ref is not a well-formed key or the number does not fit an int32.
func ParseKey(ref string) (projectKey string, number int, ok bool) {
	if !keyPattern.MatchString(ref) {
		return "", 0, false
	}
	cut := strings.LastIndexByte(ref, '-')
	n, err := strconv.ParseInt(ref[cut+1:], 10, 32)
	if err != nil || n < 1 {
		return "", 0, false
	}
	return ref[:cut], int(n), true
}

// Key returns the human key of the issue (CORE-12), or an empty string when it is not known.
func (i *Issue) Key() string {
	if i.ProjectKey == "" || i.KeyNumber < 1 {
		return ""
	}
	return fmt.Sprintf("%s-%d", i.ProjectKey, i.KeyNumber)
}
