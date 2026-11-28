package domain

import (
	"strings"
	"unicode"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Password policy limits. bcrypt only reads the first 72 bytes, so longer passwords are rejected instead of
// being silently truncated.
const (
	PasswordMinLength      = 10
	PasswordLongLength     = 14
	PasswordMaxBytes       = 72
	passwordMinClasses     = 3
	emailLocalMinForSubstr = 3
)

// CodePasswordWeak is the stable code of every password policy failure.
const CodePasswordWeak = "user.password_weak"

// ErrPasswordWeak builds the policy error with a Spanish reason and the field in its details.
func ErrPasswordWeak(field, reason string) *apperror.Error {
	msg := "La contraseña es demasiado débil: " + reason
	return apperror.WithCode(apperror.KindValidation, CodePasswordWeak, msg).
		WithDetails(apperror.FieldError{Field: field, Message: msg})
}

// ValidatePassword applies the password policy: at least 10 characters mixing at least three of upper case,
// lower case, digits and symbols (or 14 characters or more), no more than 72 bytes, not containing the local
// part of the email and not among the most common passwords. field names the offending request field.
func ValidatePassword(password, email, field string) error {
	n := len([]rune(password))
	switch {
	case n < PasswordMinLength:
		return ErrPasswordWeak(field, "usa al menos 10 caracteres")
	case len(password) > PasswordMaxBytes:
		return ErrPasswordWeak(field, "usa como máximo 72 bytes")
	}
	if n < PasswordLongLength && characterClasses(password) < passwordMinClasses {
		return ErrPasswordWeak(field, "combina mayúsculas, minúsculas, números y símbolos, o usa 14 caracteres o más")
	}
	lower := strings.ToLower(password)
	if local, _, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@"); ok &&
		len(local) >= emailLocalMinForSubstr && strings.Contains(lower, local) {
		return ErrPasswordWeak(field, "no puede contener tu correo electrónico")
	}
	if isCommonPassword(lower) {
		return ErrPasswordWeak(field, "es una contraseña muy común")
	}
	return nil
}

func characterClasses(p string) int {
	var upper, lower, digit, symbol bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		default:
			symbol = true
		}
	}
	n := 0
	for _, b := range []bool{upper, lower, digit, symbol} {
		if b {
			n++
		}
	}
	return n
}

// isCommonPassword checks the lower-cased password and the same text without trailing digits and symbols, so
// "Password123!" is caught through "password".
func isCommonPassword(lower string) bool {
	if _, ok := commonPasswords[lower]; ok {
		return true
	}
	base := strings.TrimRightFunc(lower, func(r rune) bool { return !unicode.IsLetter(r) })
	if base != lower && len(base) > 0 {
		_, ok := commonPasswords[base]
		return ok
	}
	return false
}

var commonPasswords = func() map[string]struct{} {
	m := make(map[string]struct{}, 256)
	for _, p := range strings.Fields(commonPasswordList) {
		m[p] = struct{}{}
	}
	return m
}()

// commonPasswordList holds widely leaked passwords, lower case, space separated.
const commonPasswordList = `
password password1 password123 passw0rd p@ssw0rd p@ssword 123456 1234567 12345678 123456789 1234567890
12345678910 0123456789 111111 11111111 1111111111 000000 00000000 0000000000 123123 123123123 112233 1q2w3e4r
1q2w3e4r5t 1q2w3e4r5t6y 1qaz2wsx 1qaz2wsx3edc 1qazxsw2 qwerty qwerty123 qwertyuiop qwertyui qwerty12345
qwertyuiop123 qwe123 qweasd qweasdzxc asdfgh asdfghjkl asdfghjkl123 zxcvbn zxcvbnm zxcvbnm123 abc123 abc12345
abcd1234 abcdef abcdefg abcdefgh abcdefghij iloveyou iloveyou1 iloveyou123 letmein letmein123 welcome welcome1
welcome123 admin admin123 admin1234 administrator adminadmin root toor root123 login master dragon monkey
football baseball soccer hockey basketball superman batman spiderman starwars pokemon naruto sunshine princess
shadow michael jennifer jessica ashley daniel hunter hunter2 trustno1 changeme changeme123 default guest
secret secret123 test test123 test1234 testing testing123 demo demo123 user user123 temp temp123 qazwsx
qazwsxedc qazwsx123 zaq12wsx zaq1zaq1 passpass passwordpassword passwordpassword1 mypassword mypassword1 mypass
pass1234 pass12345 password12 password1234 password12345 contrasena contrasena1 contrasena123 contraseña
clave clave123 clave1234 micontrasena miclave bienvenido bienvenido1 bienvenido123 holamundo hola123 hola1234
holamundo123 te amo teamo123 teamo123456 amor123 amorcito carlos123 maria123 barcelona realmadrid 123qwe 123qweasd
123abc 123456a a123456 a1b2c3 a1b2c3d4 a1b2c3d4e5 qwer1234 qwerasdf asdf1234 asdfasdf asdfghjk zxcv1234
1234qwer 1234abcd 12341234 123456789a 123456789q 987654321 9876543210 987654321a 654321 666666 696969 7777777
88888888 121212 131313 159753 147258369 741852963 0987654321 1234554321 5201314 555555 pepper maverick google
google123 facebook facebook1 twitter linkedin microsoft apple123 samsung computer computer1 internet internet1
corelog corelog123 corelog1234 jira jira1234 atlassian
`
