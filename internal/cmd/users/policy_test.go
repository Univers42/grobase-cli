package users_test

import (
	"strings"
	"testing"

	"github.com/Univers42/grobase-cli/internal/cmd/users"
	"github.com/Univers42/grobase-cli/internal/exitcode"
)

func TestCheckPassword(t *testing.T) {
	for _, tc := range []struct {
		name string
		pw   string
		want string // substring of the reason, empty when the password is fine
	}{
		{"empty", "", "whitespace"},
		{"spaces only", strings.Repeat(" ", 12), "whitespace"},
		{"tabs only", strings.Repeat("\t", 16), "whitespace"},
		{"too short", "Ab3$x", "12"},
		{"eleven characters", "Abcdefgh1!x", "12"},
		{"twelve characters, three classes", "Abcdefghijk1", ""},
		{"lowercase only", "abcdefghijkl", "classes"},
		{"uppercase only", "ABCDEFGHIJKL", "classes"},
		{"digits only", "123456789012", "classes"},
		{"symbols only", "............", "classes"},
		{"lower and digit is two classes", "abcdefghijk1", "classes"},
		{"lower and upper is two classes", "Abcdefghijkl", "classes"},
		{"spaces are not a symbol", "abcdefghij k", "classes"},
		{"lower and digit plus spaces", "abcd efgh 12", "classes"},
		{"lower, upper, digit", "Abcdefghijk1", ""},
		{"lower, upper, symbol", "Abcdefghijk!", ""},
		{"lower, digit, symbol", "abcdefghijk1!", ""},
		{"upper, digit, symbol", "ABCDEFGHIJK1!", ""},
		{"all four classes", "Abcdefghijk1!", ""},
		{"all four with surrounding spaces", " Abcdefghijk1! ", ""},
		{"accented letters count as lowercase", strings.Repeat("é", 10) + "A1!", ""},
		{"exactly the maximum length", strings.Repeat("Ab1!", 32), ""},
		{"one character too long", strings.Repeat("Ab1!", 33), "128"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := users.CheckPassword(tc.pw)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("CheckPassword(%q...) = %v, want nil", tc.pw, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("CheckPassword(%q...) = nil, want a refusal", tc.pw)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("reason = %q, want it to mention %q", err, tc.want)
			}
			if code := exitcode.Of(err); code != exitcode.Usage {
				t.Errorf("exit %d, want 2", code)
			}
		})
	}
}
