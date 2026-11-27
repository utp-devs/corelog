package domain

import (
	"github.com/jhonsferg/corelog/internal/platform/phone"
)

// Phone is a normalized phone number with its country.
type Phone struct {
	// Country is the ISO 3166-1 alpha-2 code.
	Country string
	// National is the national significant number (digits only).
	National string
	// E164 is the international form, for example +51912345678.
	E164 string
}

// NewPhone normalizes number for country. The number may be national or carry the international prefix.
func NewPhone(country, number string) (*Phone, error) {
	n, err := phone.Parse(country, number)
	if err != nil {
		return nil, invalidPhone(err.Error())
	}
	return &Phone{Country: n.Country, National: n.National, E164: n.E164}, nil
}

// Masked returns the E.164 number with its middle digits hidden, safe for logs.
func (p *Phone) Masked() string {
	if p == nil {
		return ""
	}
	return phone.Mask(p.E164)
}
