package trust

import (
	"slices"

	"xn--gckvb8fzb.com/maya/libs/webdav"
)

type Answer int

const (
	AnswerNo Answer = iota
	AnswerHost
	AnswerDomain
)

type Origin string

const (
	OriginPrompt  Origin = "prompt"
	OriginFlag    Origin = "flag"
	OriginCommand Origin = "command"
)

type Approval struct {
	Pattern Pattern
	Source  webdav.TrustSource
	Origin  Origin
}

type Decider struct {
	Stored []Pattern
	Flags  []Pattern
	Ask    func(req webdav.TrustRequest, domain string) Answer
	Store  func(Approval)

	answers  map[string]bool
	rejected []webdav.TrustRequest
}

func (d *Decider) TrustHost(req webdav.TrustRequest) bool {
	if answer, ok := d.answers[req.Host]; ok {
		return answer
	}

	answer := d.decide(req)
	if d.answers == nil {
		d.answers = make(map[string]bool)
	}
	d.answers[req.Host] = answer
	if !answer {
		d.rejected = append(d.rejected, req)
	}
	return answer
}

func (d *Decider) Rejected() []webdav.TrustRequest {
	return d.rejected
}

func matches(patterns []Pattern, host string) (Pattern, bool) {
	i := slices.IndexFunc(patterns, func(one Pattern) bool { return one.Matches(host) })
	if i < 0 {
		return "", false
	}
	return patterns[i], true
}

func (d *Decider) decide(req webdav.TrustRequest) bool {
	if _, ok := matches(d.Stored, req.Host); ok {
		return true
	}
	if pattern, ok := matches(d.Flags, req.Host); ok {
		d.approve(req, OriginFlag, pattern)
		return true
	}
	if d.Ask == nil {
		return false
	}

	domain, _ := Domain(req.Host)
	switch d.Ask(req, domain) {
	case AnswerHost:
		d.approve(req, OriginPrompt, Pattern(req.Host))
		return true
	case AnswerDomain:
		patterns := DomainPatterns(req.Host)
		d.approve(req, OriginPrompt, patterns...)
		return len(patterns) > 0
	}
	return false
}

func (d *Decider) approve(req webdav.TrustRequest, origin Origin, patterns ...Pattern) {
	for _, pattern := range patterns {
		d.Stored = append(d.Stored, pattern)
		if d.Store != nil {
			d.Store(Approval{Pattern: pattern, Source: req.Source, Origin: origin})
		}
	}
}
