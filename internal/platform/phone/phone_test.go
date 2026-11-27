package phone

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name, country, in string
		wantE164          string
		wantNational      string
		wantErr           error
	}{
		{"PE national", "PE", "912 345 678", "+51912345678", "912345678", nil},
		{"PE trunk zero", "PE", "0912345678", "+51912345678", "912345678", nil},
		{"PE international plus", "PE", "+51 912-345-678", "+51912345678", "912345678", nil},
		{"PE international 00", "PE", "0051 (912) 345 678", "+51912345678", "912345678", nil},
		{"PE landline Lima", "PE", "01 234 5678", "+5112345678", "12345678", nil},
		{"MX national", "MX", "55 1234 5678", "+525512345678", "5512345678", nil},
		{"MX international", "MX", "+52 55 1234 5678", "+525512345678", "5512345678", nil},
		{"CO mobile", "CO", "310 123 4567", "+573101234567", "3101234567", nil},
		{"CO international", "CO", "+573101234567", "+573101234567", "3101234567", nil},
		{"AR trunk zero", "AR", "011 2345 6789", "+541123456789", "1123456789", nil},
		{"AR mobile with 9", "AR", "+54 9 11 2345 6789", "+5491123456789", "91123456789", nil},
		{"CL mobile", "CL", "9 1234 5678", "+56912345678", "912345678", nil},
		{"ES national", "ES", "612 34 56 78", "+34612345678", "612345678", nil},
		{"ES dial code without plus", "ES", "34612345678", "+34612345678", "612345678", nil},
		{"US national", "US", "(202) 555-0123", "+12025550123", "2025550123", nil},
		{"US with leading 1", "US", "1 202 555 0123", "+12025550123", "2025550123", nil},
		{"CA", "CA", "+1 416 555 0123", "+14165550123", "4165550123", nil},
		{"BR mobile", "BR", "(11) 91234-5678", "+5511912345678", "11912345678", nil},
		{"BR trunk zero", "BR", "011 91234 5678", "+5511912345678", "11912345678", nil},
		{"GB trunk zero", "GB", "07911 123456", "+447911123456", "7911123456", nil},
		{"GB international with (0)", "GB", "+44 (0) 7911 123456", "+447911123456", "7911123456", nil},
		{"FR trunk zero", "FR", "06 12 34 56 78", "+33612345678", "612345678", nil},
		{"IT keeps its leading zero", "IT", "+39 06 1234 5678", "+390612345678", "0612345678", nil},
		{"DE", "DE", "0151 12345678", "+4915112345678", "15112345678", nil},
		{"RU trunk eight", "RU", "8 912 345 67 89", "+79123456789", "9123456789", nil},
		{"RU area code starting with 8 is kept", "RU", "812 345 67 89", "+78123456789", "8123456789", nil},
		{"IN", "IN", "098765 43210", "+919876543210", "9876543210", nil},
		{"DO area code", "DO", "809 234 5678", "+18092345678", "8092345678", nil},
		{"lowercase country", "pe", "912345678", "+51912345678", "912345678", nil},
		{"mismatched prefix", "PE", "+52 5512345678", "", "", ErrPrefixMismatch},
		{"mismatched 00 prefix", "ES", "0051912345678", "", "", ErrPrefixMismatch},
		{"unknown country", "ZZ", "912345678", "", "", ErrUnknownCountry},
		{"empty", "PE", "  ", "", "", ErrEmpty},
		{"only symbols", "PE", "+", "", "", ErrEmpty},
		{"letters", "PE", "91234abcd", "", "", ErrInvalidChars},
		{"plus in the middle", "PE", "912+345678", "", "", ErrInvalidChars},
		{"too short for the country", "PE", "9123", "", "", ErrLength},
		{"too long for the country", "MX", "55123456789", "", "", ErrLength},
		{"US bad area", "DO", "212 555 0123", "", "", ErrAreaCode},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.country, tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.E164 != tc.wantE164 || got.National != tc.wantNational {
				t.Errorf("got %+v, want e164=%s national=%s", got, tc.wantE164, tc.wantNational)
			}
		})
	}
}

func TestCountryTable(t *testing.T) {
	list := Countries()
	if len(list) < 240 {
		t.Fatalf("only %d countries", len(list))
	}
	seen := map[string]bool{}
	for _, c := range list {
		if len(c.Code) != 2 || seen[c.Code] {
			t.Errorf("bad or duplicate code %q", c.Code)
		}
		seen[c.Code] = true
		if c.Name == "" || c.DialCode == "" {
			t.Errorf("%s: incomplete entry", c.Code)
		}
		// Every documented example must parse back to itself.
		if c.Example != "" {
			n, err := Parse(c.Code, c.Example)
			if err != nil || n.National != c.Example {
				t.Errorf("%s: example %q: %+v %v", c.Code, c.Example, n, err)
			}
		}
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].Name > list[i].Name {
			t.Fatalf("not sorted: %s > %s", list[i-1].Name, list[i].Name)
		}
	}
}

func TestMask(t *testing.T) {
	if got := Mask("+51912345678"); got != "+51******678" {
		t.Errorf("got %q", got)
	}
	if got := Mask("+123"); got != "****" {
		t.Errorf("short: %q", got)
	}
}
