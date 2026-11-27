package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestParseNames(t *testing.T) {
	tests := []struct {
		name, first, last string
		wantFirst         string
		wantLast          string
		wantErr           bool
	}{
		{"plain", "Ana", "Perez", "Ana", "Perez", false},
		{"accents", "José Ángel", "Núñez", "José Ángel", "Núñez", false},
		{"hyphen", "Anne-Marie", "Smith-Jones", "Anne-Marie", "Smith-Jones", false},
		{"apostrophe", "Conan", "O'Brien", "Conan", "O'Brien", false},
		{"typographic apostrophe", "Conan", "O’Brien", "Conan", "O’Brien", false},
		{"collapses spaces", "  Luis   Miguel ", "  Gomez  Ruiz ", "Luis Miguel", "Gomez Ruiz", false},
		{"non latin", "Дмитрий", "Иванов", "Дмитрий", "Иванов", false},
		{"digits", "Ana2", "Perez", "", "", true},
		{"symbols", "Ana", "Perez!", "", "", true},
		{"at sign", "Ana@", "Perez", "", "", true},
		{"dot", "Ana", "Jr.", "", "", true},
		{"empty first", "", "Perez", "", "", true},
		{"empty last", "Ana", "", "", "", true},
		{"only punctuation", "-", "'", "", "", true},
		{"first too long", strings.Repeat("a", 81), "Perez", "", "", true},
		{"first at limit", strings.Repeat("a", 80), "Perez", strings.Repeat("a", 80), "Perez", false},
		{"last too long", "Ana", strings.Repeat("a", 121), "", "", true},
		{"emoji", "Ana😀", "Perez", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, l, err := ParseNames(tc.first, tc.last)
			if tc.wantErr {
				if err == nil || apperror.CodeOf(err) != "user.invalid_name" || apperror.KindOf(err) != apperror.KindValidation {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil || f != tc.wantFirst || l != tc.wantLast {
				t.Errorf("got %q %q %v", f, l, err)
			}
		})
	}
}

func TestSplitNameAndLegacy(t *testing.T) {
	tests := []struct{ in, first, last string }{
		{"Madonna", "Madonna", ""},
		{"Ana Perez", "Ana", "Perez"},
		{"Ana Perez Lopez", "Ana", "Perez Lopez"},
		{"Juan Carlos Perez Lopez", "Juan Carlos", "Perez Lopez"},
		{"  Luis   Gomez  ", "Luis", "Gomez"},
		{"", "", ""},
	}
	for _, tc := range tests {
		f, l := SplitName(tc.in)
		if f != tc.first || l != tc.last {
			t.Errorf("SplitName(%q) = %q, %q", tc.in, f, l)
		}
	}
	if f, l, err := ParseLegacyName("Madonna"); err != nil || f != "Madonna" || l != "" {
		t.Errorf("single word: %q %q %v", f, l, err)
	}
	if _, _, err := ParseLegacyName("User 1"); apperror.CodeOf(err) != "user.invalid_name" {
		t.Errorf("digits accepted: %v", err)
	}
}

func TestUserIdentity(t *testing.T) {
	u, err := NewUser("Ana Perez", "a@example.com", "h", RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if u.FirstName != "Ana" || u.LastName != "Perez" || u.FullName() != "Ana Perez" || u.Locale != "es" {
		t.Errorf("%+v", u)
	}
	u.SetNames("Ana María", "Pérez")
	if u.Name != "Ana María Pérez" || u.DisplayName != "Ana María Pérez" {
		t.Errorf("display name must follow: %+v", u)
	}
	u.DisplayName = "Anita"
	u.SetNames("Ana", "Gomez")
	if u.DisplayName != "Anita" {
		t.Errorf("custom display name overwritten: %q", u.DisplayName)
	}
}

func TestUserPhone(t *testing.T) {
	u, _ := NewUser("Ana Perez", "a@example.com", "h", RoleUser)
	p, err := NewPhone("PE", "+51 912 345 678")
	if err != nil {
		t.Fatal(err)
	}
	u.SetPhone(p)
	u.MarkPhoneVerified(u.UpdatedAt)
	if !u.PhoneVerified() {
		t.Fatal("not verified")
	}
	same, _ := NewPhone("PE", "912345678")
	u.SetPhone(same)
	if !u.PhoneVerified() {
		t.Error("the same number must keep its verification")
	}
	other, _ := NewPhone("PE", "987654321")
	u.SetPhone(other)
	if u.PhoneVerified() || u.PhoneVerifiedAt != nil {
		t.Error("a new number must clear the verification")
	}
	u.SetPhone(nil)
	if u.Phone != nil {
		t.Error("phone not cleared")
	}
	if _, err := NewPhone("PE", "+52 5512345678"); !errors.Is(err, ErrInvalidPhone) && apperror.CodeOf(err) != "user.invalid_phone" {
		t.Errorf("mismatch error: %v", err)
	}
	if got := p.Masked(); got != "+51******678" {
		t.Errorf("masked %q", got)
	}
}

func TestPasswordConfirmation(t *testing.T) {
	if err := CheckPasswordConfirmation("abc12345", "abc12345", "password_confirmation"); err != nil {
		t.Errorf("equal: %v", err)
	}
	for _, c := range []string{"", "abc12345 ", " abc12345", "ABC12345"} {
		err := CheckPasswordConfirmation("abc12345", c, "password_confirmation")
		if apperror.CodeOf(err) != "user.password_mismatch" || len(apperror.DetailsOf(err)) != 1 {
			t.Errorf("confirmation %q: %v", c, err)
		}
	}
}
