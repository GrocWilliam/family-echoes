package config

import (
	"slices"
	"testing"
)

func TestIsEmailAllowed(t *testing.T) {
	t.Setenv("ALLOWED_EMAILS", " Mamie@Example.com ,papi@example.com,")

	cases := map[string]bool{
		"mamie@example.com":    true,
		"  PAPI@example.com  ": true,
		"other@example.com":    false,
		// A trailing comma used to yield an empty entry, letting a token
		// with no email claim (e.g. an anonymous Firebase user) through.
		"": false,
	}
	for email, want := range cases {
		if got := IsEmailAllowed(email); got != want {
			t.Errorf("IsEmailAllowed(%q) = %v, want %v", email, got, want)
		}
	}
}

func TestIsEmailAllowedWithEmptyList(t *testing.T) {
	t.Setenv("ALLOWED_EMAILS", "")
	if IsEmailAllowed("mamie@example.com") {
		t.Error("an empty allowlist must not allow anyone")
	}
}

func TestIsEmailAdminIsSeparateList(t *testing.T) {
	t.Setenv("ALLOWED_EMAILS", "mamie@example.com,papi@example.com")
	t.Setenv("ADMIN_EMAILS", "mamie@example.com")

	if !IsEmailAdmin("mamie@example.com") {
		t.Error("mamie should be an admin")
	}
	if IsEmailAdmin("papi@example.com") {
		t.Error("papi is allowed but not an admin")
	}
}

func TestSplitList(t *testing.T) {
	cases := map[string][]string{
		"":                       nil,
		" , ,":                   nil,
		"https://a.com":          {"https://a.com"},
		"https://a.com, b.com ,": {"https://a.com", "b.com"},
	}
	for input, want := range cases {
		if got := SplitList(input); !slices.Equal(got, want) {
			t.Errorf("SplitList(%q) = %q, want %q", input, got, want)
		}
	}
}
