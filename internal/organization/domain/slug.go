package domain

import (
	"regexp"
	"strings"
)

// MaxSlugLength is the longest slug accepted (the pattern allows 3 to 40 characters).
const MaxSlugLength = 40

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)

// reservedSlugs are paths the platform uses itself and can never be organization slugs.
var reservedSlugs = map[string]struct{}{
	"admin": {}, "api": {}, "app": {}, "www": {}, "new": {}, "settings": {}, "login": {},
	"logout": {}, "register": {}, "onboarding": {}, "invite": {}, "invitations": {}, "help": {},
	"support": {}, "static": {}, "assets": {}, "health": {}, "ready": {}, "corelog-admin": {},
}

// accentReplacer maps accented letters to their ASCII base letters before the slug is built.
var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ä", "a", "ã", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o", "ø", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ñ", "n", "ç", "c", "ý", "y", "ÿ", "y", "ß", "ss",
)

// ValidateSlug checks the slug pattern first, then the reserved list.
func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return ErrSlugInvalid
	}
	if _, reserved := reservedSlugs[slug]; reserved {
		return ErrSlugReserved
	}
	return nil
}

// NormalizeSlug derives a candidate slug from a display name: lowercase, accents removed,
// every run of characters outside [a-z0-9] becomes a single hyphen, no leading or trailing
// hyphen, at most MaxSlugLength characters. The result is not validated; call ValidateSlug.
func NormalizeSlug(name string) string {
	s := accentReplacer.Replace(strings.ToLower(strings.TrimSpace(name)))
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > MaxSlugLength {
		out = strings.TrimRight(out[:MaxSlugLength], "-")
	}
	return out
}
