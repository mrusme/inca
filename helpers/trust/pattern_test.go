package trust

import (
	"slices"
	"testing"
)

func TestParsePattern(t *testing.T) {
	valid := map[string]Pattern{
		"p42-caldav.icloud.com":   "p42-caldav.icloud.com",
		" P42-CalDAV.iCloud.com ": "p42-caldav.icloud.com",
		"*.icloud.com":            "*.icloud.com",
		"*.dav.example.co.uk":     "*.dav.example.co.uk",
		"*.example.co.uk":         "*.example.co.uk",
		"*.corp.lan":              "*.corp.lan",
		"localhost":               "localhost",
		"192.0.2.7":               "192.0.2.7",
		"2001:db8::7":             "2001:db8::7",
	}
	for value, want := range valid {
		got, err := ParsePattern(value)
		if err != nil || got != want {
			t.Errorf("ParsePattern(%q) = %q, %v, want %q", value, got, err, want)
		}
	}

	invalid := []string{
		"",
		"   ",
		"*",
		"*.",
		"*.com",
		"*.co.uk",
		"*.github.io",
		"*.lan",
		"*.192.0.2.7",
		"p*.icloud.com",
		"*.*.icloud.com",
		"caldav.*.com",
		"https://p42-caldav.icloud.com",
		"p42-caldav.icloud.com:443",
		"p42-caldav.icloud.com/path",
		"user@icloud.com",
		"icloud .com",
		".icloud.com",
		"icloud.com.",
	}
	for _, value := range invalid {
		if got, err := ParsePattern(value); err == nil {
			t.Errorf("ParsePattern(%q) = %q, want an error", value, got)
		}
	}
}

func TestPatternMatches(t *testing.T) {
	tests := []struct {
		pattern Pattern
		host    string
		want    bool
	}{
		{"p42-caldav.icloud.com", "p42-caldav.icloud.com", true},
		{"p42-caldav.icloud.com", "P42-CalDAV.iCloud.COM", true},
		{"p42-caldav.icloud.com", "p43-caldav.icloud.com", false},
		{"p42-caldav.icloud.com", "evil.p42-caldav.icloud.com", false},
		{"*.icloud.com", "p42-caldav.icloud.com", true},
		{"*.icloud.com", "a.b.icloud.com", true},
		{"*.icloud.com", "icloud.com", false},
		{"*.icloud.com", "evilicloud.com", false},
		{"*.icloud.com", "icloud.com.evil.net", false},
		{"*.icloud.com", "", false},
		{"192.0.2.7", "192.0.2.7", true},
		{"192.0.2.7", "192.0.2.70", false},
	}
	for _, tt := range tests {
		if got := tt.pattern.Matches(tt.host); got != tt.want {
			t.Errorf("%q matches %q = %v, want %v", tt.pattern, tt.host, got, tt.want)
		}
	}
}

func TestDomain(t *testing.T) {
	tests := []struct {
		host   string
		domain string
		ok     bool
	}{
		{"p42-caldav.icloud.com", "icloud.com", true},
		{"icloud.com", "icloud.com", true},
		{"dav.example.co.uk", "example.co.uk", true},
		{"alice.github.io", "alice.github.io", true},
		{"deep.alice.github.io", "alice.github.io", true},
		{"dav.corp.lan", "corp.lan", true},
		{"github.io", "", false},
		{"co.uk", "", false},
		{"localhost", "", false},
		{"192.0.2.7", "", false},
		{"2001:db8::7", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		domain, ok := Domain(tt.host)
		if domain != tt.domain || ok != tt.ok {
			t.Errorf("Domain(%q) = %q, %v, want %q, %v", tt.host, domain, ok, tt.domain, tt.ok)
		}
	}
}

func TestDomainPatterns(t *testing.T) {
	for host, want := range map[string][]Pattern{
		"p42-caldav.icloud.com": {"*.icloud.com"},
		"provider.net":          {"*.provider.net", "provider.net"},
		"localhost":             nil,
	} {
		got := DomainPatterns(host)
		if !slices.Equal(got, want) {
			t.Errorf("DomainPatterns(%q) = %v, want %v", host, got, want)
		}
		if len(got) > 0 && !slices.ContainsFunc(got, func(one Pattern) bool { return one.Matches(host) }) {
			t.Errorf("none of %v matches %q, the host the question was about", got, host)
		}
	}
}
