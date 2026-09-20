package trust

import (
	"fmt"
	"net"
	"strings"

	"golang.org/x/net/publicsuffix"
)

const wildcard string = "*."

type Pattern string

func ParsePattern(value string) (Pattern, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("a host or a pattern such as *.example.com is needed")
	}
	if net.ParseIP(value) != nil {
		return Pattern(value), nil
	}

	host, isWildcard := strings.CutPrefix(value, wildcard)
	if !isHostName(host) {
		return "", fmt.Errorf("%q is no host name, and a pattern has *. in front of one and nowhere else", value)
	}
	if !isWildcard {
		return Pattern(value), nil
	}

	if _, ok := Domain(host); !ok {
		return "", fmt.Errorf("%q covers every host under %s, which is no domain anyone registered", value, host)
	}
	return Pattern(value), nil
}

func isHostName(host string) bool {
	if host == "" || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	return !strings.ContainsAny(host, "*/:@ \t\r\n")
}

func (p Pattern) IsWildcard() bool {
	return strings.HasPrefix(string(p), wildcard)
}

func (p Pattern) Matches(host string) bool {
	host = strings.ToLower(host)
	if host == "" {
		return false
	}

	if base, ok := strings.CutPrefix(string(p), wildcard); ok {
		return strings.HasSuffix(host, "."+base)
	}
	return host == string(p)
}

func Domain(host string) (string, bool) {
	host = strings.ToLower(host)
	if host == "" || net.ParseIP(host) != nil {
		return "", false
	}

	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", false
	}
	return domain, true
}

func DomainPatterns(host string) []Pattern {
	domain, ok := Domain(host)
	if !ok {
		return nil
	}

	patterns := []Pattern{Pattern(wildcard + domain)}
	if !patterns[0].Matches(host) {
		patterns = append(patterns, Pattern(strings.ToLower(host)))
	}
	return patterns
}
