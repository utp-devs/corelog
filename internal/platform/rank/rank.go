// Package rank provides fractional string ranks used to order items such as
// backlog issues, board cards, sprints and dashboard gadgets.
//
// Ranks are strings over the base-62 alphabet "0-9A-Za-z". That alphabet is in
// ASCII order, so plain string comparison gives the rank order. Valid ranks never
// end in the lowest digit '0', which guarantees that something can always be
// inserted before any rank. A rank can be read as a base-62 fraction in (0, 1),
// where the empty string stands for the open bounds 0 and 1.
package rank

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const (
	alphabet  = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	zeroDigit = '0'
	// rebalanceLen is the length above which ranks should be regenerated with Rebalance.
	rebalanceLen = 32
)

var (
	// ErrInvalid reports a rank with characters outside the alphabet or ending in '0'.
	ErrInvalid = errors.New("rank: invalid rank")
	// ErrOrder reports a lower bound that is not strictly below the upper bound.
	ErrOrder = errors.New("rank: lower bound must be less than upper bound")
)

// Initial returns the middle rank, used as the first position in an empty list.
func Initial() string {
	return alphabet[len(alphabet)/2 : len(alphabet)/2+1]
}

// Between returns a rank r with a < r < b in string order. An empty a means no
// lower bound (insert at the start) and an empty b means no upper bound (insert
// at the end). Both empty returns Initial. It returns ErrInvalid for characters
// outside the alphabet or a bound ending in '0', and ErrOrder when both bounds
// are set and a >= b.
func Between(a, b string) (string, error) {
	if err := validate(a); err != nil {
		return "", err
	}
	if err := validate(b); err != nil {
		return "", err
	}
	if a != "" && b != "" && a >= b {
		return "", ErrOrder
	}
	if b == "" {
		return midpoint(a, "", true), nil
	}
	return midpoint(a, b, false), nil
}

// NeedsRebalance reports whether a rank has grown long enough that the whole
// list should be re-spread with Rebalance.
func NeedsRebalance(s string) bool {
	return len(s) > rebalanceLen
}

// Rebalance returns n strictly increasing ranks spread evenly over the key space.
// Every rank is short and valid for Between, so callers can use the result to
// reset the positions of an existing list. For n <= 0 it returns an empty slice.
func Rebalance(n int) []string {
	if n <= 0 {
		return []string{}
	}
	// Pick the smallest k with 62^k >= n+1 so the spacing between ranks is at least 1.
	base := big.NewInt(int64(len(alphabet)))
	space := big.NewInt(int64(len(alphabet)))
	k := 1
	need := big.NewInt(int64(n) + 1)
	for space.Cmp(need) < 0 {
		space.Mul(space, base)
		k++
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		// v = floor((i+1) * space / (n+1)) is strictly increasing in i.
		v := new(big.Int).Mul(space, big.NewInt(int64(i+1)))
		v.Quo(v, need)
		// Appending a non-zero digit keeps the rank valid for Between.
		out[i] = encodeFixed(v, k) + Initial()
	}
	return out
}

// midpoint returns a string strictly between a and b, where b is ignored when
// unbounded is true. Both inputs are assumed valid and a < b.
func midpoint(a, b string, unbounded bool) string {
	if !unbounded {
		// Shared prefix is copied unchanged; the rest is solved recursively.
		n := 0
		for n < len(b) && charAt(a, n) == b[n] {
			n++
		}
		if n > 0 {
			return b[:n] + midpoint(suffix(a, n), suffix(b, n), false)
		}
	}

	digitA := 0
	if a != "" {
		digitA = strings.IndexByte(alphabet, a[0])
	}
	digitB := len(alphabet)
	if !unbounded {
		digitB = strings.IndexByte(alphabet, b[0])
	}

	if digitB-digitA > 1 {
		return string(alphabet[(digitA+digitB)/2])
	}
	// The first digits are adjacent. A longer b already sits strictly above a
	// with a smaller first digit, so its first digit is a valid answer.
	if !unbounded && len(b) > 1 {
		return b[:1]
	}
	// Keep a's first digit and find a value above the rest of a.
	return alphabet[digitA:digitA+1] + midpoint(suffix(a, 1), "", true)
}

// validate checks the alphabet and rejects ranks ending in the lowest digit.
// The empty string is valid and stands for an open bound.
func validate(s string) error {
	for i := 0; i < len(s); i++ {
		if strings.IndexByte(alphabet, s[i]) < 0 {
			return fmt.Errorf("%w: character %q is outside the alphabet", ErrInvalid, s[i])
		}
	}
	if strings.HasSuffix(s, string(zeroDigit)) {
		return fmt.Errorf("%w: %q ends in '0'", ErrInvalid, s)
	}
	return nil
}

// charAt returns the digit at position n, treating positions past the end as '0'.
func charAt(s string, n int) byte {
	if n < len(s) {
		return s[n]
	}
	return zeroDigit
}

// suffix returns s without its first n bytes, or "" when n is past the end.
func suffix(s string, n int) string {
	if n >= len(s) {
		return ""
	}
	return s[n:]
}

// encodeFixed writes v as exactly k base-62 digits, padded with leading zeros.
func encodeFixed(v *big.Int, k int) string {
	buf := make([]byte, k)
	x := new(big.Int).Set(v)
	base := big.NewInt(int64(len(alphabet)))
	r := new(big.Int)
	for j := k - 1; j >= 0; j-- {
		x.QuoRem(x, base, r)
		buf[j] = alphabet[r.Int64()]
	}
	return string(buf)
}
