// Package phone validates and normalizes telephone numbers to E.164 and exposes the table of countries
// (ISO 3166-1 alpha-2 code, calling code, trunk prefixes) the platform supports. It has no dependencies.
package phone

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Errors returned by Parse. Messages are Spanish because they reach the user.
var (
	ErrUnknownCountry = errors.New("el pais del telefono no es valido")
	ErrEmpty          = errors.New("el numero de telefono es obligatorio")
	ErrInvalidChars   = errors.New("el numero de telefono solo admite digitos, espacios, guiones, puntos y parentesis")
	ErrPrefixMismatch = errors.New("el prefijo internacional del numero no corresponde al pais elegido")
	ErrLength         = errors.New("la longitud del numero de telefono no es valida para el pais elegido")
	ErrAreaCode       = errors.New("el codigo de area del numero no es valido para el pais elegido")
)

const (
	minE164Digits = 7
	maxE164Digits = 15
)

// Country describes one entry of the country table.
type Country struct {
	// Code is the ISO 3166-1 alpha-2 code.
	Code string
	// Name is the Spanish name of the country.
	Name string
	// DialCode is the calling code without "+" (for example "51").
	DialCode string
	// Areas lists the NANP area codes of countries that share +1; empty otherwise.
	Areas []string
	// Trunk lists the domestic dialing prefixes removed from national numbers.
	Trunk []string
	// MinNational and MaxNational bound the national significant number; zero means unknown.
	MinNational int
	MaxNational int
	// Example is a national number that passes validation; it may be empty.
	Example string
}

// Number is a normalized phone number.
type Number struct {
	// Country is the ISO 3166-1 alpha-2 code.
	Country string
	// National is the national significant number: digits only, without trunk prefix or calling code.
	National string
	// E164 is "+" followed by the calling code and National.
	E164 string
}

var (
	loadOnce  sync.Once
	byCode    map[string]Country
	byName    []Country
	e164Check = func(s string) bool { return len(s) >= minE164Digits+1 && len(s) <= maxE164Digits+1 }
)

func load() {
	loadOnce.Do(func() {
		byCode = make(map[string]Country, 256)
		for _, line := range strings.Split(countryTable, "\n") {
			f := strings.Split(strings.TrimSpace(line), "|")
			if len(f) != 7 {
				continue
			}
			c := Country{Code: f[0], Name: f[1], DialCode: f[2], Example: f[6]}
			if f[3] != "" {
				c.Areas = strings.Split(f[3], ",")
			}
			if f[4] != "" {
				c.Trunk = strings.Split(f[4], ",")
			}
			if f[5] != "" {
				lo, hi, _ := strings.Cut(f[5], "-")
				c.MinNational, _ = strconv.Atoi(lo)
				c.MaxNational = c.MinNational
				if hi != "" {
					c.MaxNational, _ = strconv.Atoi(hi)
				}
			}
			byCode[c.Code] = c
			byName = append(byName, c)
		}
		sort.Slice(byName, func(i, j int) bool { return byName[i].Name < byName[j].Name })
	})
}

// Countries returns every country sorted by Spanish name. The slice is a copy.
func Countries() []Country {
	load()
	out := make([]Country, len(byName))
	copy(out, byName)
	return out
}

// Lookup finds a country by ISO code (case-insensitive).
func Lookup(code string) (Country, bool) {
	load()
	c, ok := byCode[strings.ToUpper(strings.TrimSpace(code))]
	return c, ok
}

// Parse normalizes input for the given country. The input may be national ("912 345 678", "0912345678")
// or international ("+51 912 345 678", "0051912345678"); separators (spaces, dashes, dots, parentheses)
// are ignored. An international prefix that does not match the country is rejected.
func Parse(country, input string) (Number, error) {
	c, ok := Lookup(country)
	if !ok {
		return Number{}, ErrUnknownCountry
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return Number{}, ErrEmpty
	}

	international := false
	var digits strings.Builder
	for i, r := range input {
		switch {
		case r >= '0' && r <= '9':
			digits.WriteRune(r)
		case r == '+' && i == 0:
			international = true
		case r == ' ' || r == '-' || r == '.' || r == '(' || r == ')' || r == ' ':
		default:
			return Number{}, ErrInvalidChars
		}
	}
	d := digits.String()
	if d == "" {
		return Number{}, ErrEmpty
	}
	if !international && strings.HasPrefix(d, "00") {
		international = true
		d = d[2:]
	}

	var national string
	if international {
		if !strings.HasPrefix(d, c.DialCode) {
			return Number{}, ErrPrefixMismatch
		}
		national = stripTrunk(c, d[len(c.DialCode):])
	} else {
		national = stripTrunk(c, d)
		// A national number typed with its calling code but no "+" is accepted only when the length
		// makes it unambiguous.
		if c.MaxNational > 0 && c.MinNational == c.MaxNational && strings.HasPrefix(national, c.DialCode) &&
			len(national) == len(c.DialCode)+c.MaxNational {
			national = national[len(c.DialCode):]
		}
	}

	if c.MaxNational > 0 && (len(national) < c.MinNational || len(national) > c.MaxNational) {
		return Number{}, ErrLength
	}
	if len(c.Areas) > 0 {
		if len(national) < 3 || !contains(c.Areas, national[:3]) {
			return Number{}, ErrAreaCode
		}
	}
	e164 := "+" + c.DialCode + national
	if !e164Check(e164) {
		return Number{}, ErrLength
	}
	return Number{Country: c.Code, National: national, E164: e164}, nil
}

// stripTrunk removes one domestic prefix. Prefixes starting with zero never begin a national significant
// number, so they are removed whenever the rest still satisfies the minimum length. Other prefixes (the
// "8" of Russia or Kazakhstan, which can begin an area code) are removed only when the number is too long
// without removing them. Without length bounds the prefix is always removed.
func stripTrunk(c Country, national string) string {
	for _, t := range c.Trunk {
		if !strings.HasPrefix(national, t) || len(national) <= len(t) {
			continue
		}
		rest := national[len(t):]
		switch {
		case c.MaxNational == 0:
			return rest
		case t[0] == '0' && len(rest) >= c.MinNational:
			return rest
		case len(national) > c.MaxNational && len(rest) >= c.MinNational:
			return rest
		}
	}
	return national
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Mask hides the middle of an E.164 number for logs: "+51912345678" becomes "+51******678".
func Mask(e164 string) string {
	if len(e164) <= 6 {
		return strings.Repeat("*", len(e164))
	}
	return e164[:3] + strings.Repeat("*", len(e164)-6) + e164[len(e164)-3:]
}
