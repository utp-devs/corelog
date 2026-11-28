package domain

import "testing"

func TestValidatePasswordMatrix(t *testing.T) {
	t.Parallel()
	const email = "carolina.perez@example.com"
	tests := []struct {
		name     string
		password string
		weak     bool
	}{
		{"strong mix", "Tr0ub4dor&3x!", false},
		{"three classes at ten chars", "Rmnbvcx987", false},
		{"long passphrase only lower", "correcthorsebattery", false},
		{"too short", "Ab1!xyz", true},
		{"nine chars", "Abcdef12!", true},
		{"ten chars one class", "abcdefghij", true},
		{"ten chars two classes", "abcdefgh12", true},
		{"contains email local part", "xxcarolina.perezXX1!", true},
		{"common with decoration", "Password123!", true},
		{"common exact long", "passwordpassword", true},
		{"common digits", "1234567890", true},
		{"over 72 bytes", "Aa1!" + string(make([]byte, 0)) + "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", true},
		{"unicode counts as letters", "Contraseñá-Fuerte7", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePassword(tt.password, email, "password")
			if tt.weak {
				if err == nil {
					t.Fatalf("%q accepted", tt.password)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q rejected: %v", tt.password, err)
			}
		})
	}
}

func TestValidatePasswordCode(t *testing.T) {
	t.Parallel()
	err := ValidatePassword("short", "a@b.co", "new_password")
	if err == nil || ErrPasswordWeak("f", "r").Code != CodePasswordWeak {
		t.Fatalf("unexpected %v", err)
	}
}
