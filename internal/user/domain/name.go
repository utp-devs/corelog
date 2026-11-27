package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Limits of the name parts, in characters.
const (
	MaxFirstNameLength = 80
	MaxLastNameLength  = 120
	maxFullNameLength  = 120
)

// NormalizeNamePart trims the value and collapses runs of whitespace into one space.
func NormalizeNamePart(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// validNameRune reports whether r may appear in a person name: letters (any script, with accents),
// combining marks, spaces, hyphens and apostrophes.
func validNameRune(r rune) bool {
	switch {
	case unicode.IsLetter(r), unicode.IsMark(r):
		return true
	case r == ' ', r == '-', r == '\'', r == '’':
		return true
	default:
		return false
	}
}

func validNamePart(s string, max int) bool {
	if s == "" || utf8.RuneCountInString(s) > max {
		return false
	}
	hasLetter := false
	for _, r := range s {
		if !validNameRune(r) {
			return false
		}
		if unicode.IsLetter(r) {
			hasLetter = true
		}
	}
	return hasLetter
}

// ParseNames normalizes and validates a first and last name. Both are required: the first name up to 80
// characters and the last name up to 120, made of letters, spaces, hyphens and apostrophes only.
func ParseNames(first, last string) (string, string, error) {
	first, last = NormalizeNamePart(first), NormalizeNamePart(last)
	if !validNamePart(first, MaxFirstNameLength) {
		return "", "", errInvalidNamePart("nombre", MaxFirstNameLength)
	}
	if !validNamePart(last, MaxLastNameLength) {
		return "", "", errInvalidNamePart("apellido", MaxLastNameLength)
	}
	return first, last, nil
}

// ParseLegacyName accepts the single "name" field of older clients: it is validated like a name and split
// with SplitName. A single word becomes a first name with an empty last name.
func ParseLegacyName(full string) (string, string, error) {
	full = NormalizeNamePart(full)
	first, last := SplitName(full)
	if !validNamePart(first, MaxFirstNameLength) || (last != "" && !validNamePart(last, MaxLastNameLength)) ||
		utf8.RuneCountInString(full) > maxFullNameLength {
		return "", "", errInvalidNamePart("nombre", maxFullNameLength)
	}
	return first, last, nil
}

// SplitName splits a full name into first and last name. One word is a first name; two words are first
// and last; three words keep the first word as first name (the others are surnames); four or more keep
// two words as first name. The SQL function users_split_name implements the same rule.
func SplitName(full string) (first, last string) {
	parts := strings.Fields(full)
	switch n := len(parts); {
	case n == 0:
		return "", ""
	case n <= 3:
		return truncate(parts[0], MaxFirstNameLength), truncate(strings.Join(parts[1:], " "), MaxLastNameLength)
	default:
		return truncate(strings.Join(parts[:2], " "), MaxFirstNameLength), truncate(strings.Join(parts[2:], " "), MaxLastNameLength)
	}
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// JoinNames builds the full name from its parts.
func JoinNames(first, last string) string {
	return truncate(strings.TrimSpace(first+" "+last), maxFullNameLength)
}
