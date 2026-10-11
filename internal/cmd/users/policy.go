package users

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Univers42/grobase-cli/internal/exitcode"
)

// Password policy, checked before a create or update request leaves this machine:
// 12 to 128 characters, at least three of the four classes below, and never
// whitespace only. Breaking it is a usage error (exit 2), not a server round-trip.
const (
	minPasswordLen     = 12
	maxPasswordLen     = 128
	minPasswordClasses = 3
)

// CheckPassword reports why pw does not satisfy the password policy.
func CheckPassword(pw string) error {
	switch {
	case strings.TrimSpace(pw) == "":
		return exitcode.Usagef("password must not be only whitespace")
	case utf8.RuneCountInString(pw) < minPasswordLen:
		return exitcode.Usagef("password must be at least %d characters", minPasswordLen)
	case utf8.RuneCountInString(pw) > maxPasswordLen:
		return exitcode.Usagef("password must be at most %d characters", maxPasswordLen)
	case passwordClasses(pw) < minPasswordClasses:
		return exitcode.Usagef("password needs at least %d of the 4 character classes: lowercase, uppercase, digit, symbol", minPasswordClasses)
	}
	return nil
}

// passwordClasses counts how many of the four classes pw uses. Whitespace is not a
// symbol, so a password cannot pass on spaces alone.
func passwordClasses(pw string) int {
	var lower, upper, digit, symbol bool
	for _, r := range pw {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		case !unicode.IsSpace(r):
			symbol = true
		}
	}
	classes := 0
	for _, present := range []bool{lower, upper, digit, symbol} {
		if present {
			classes++
		}
	}
	return classes
}
