package trust

import (
	"slices"
	"testing"

	"xn--gckvb8fzb.com/maya/libs/webdav"
)

type recorder struct {
	asked   []webdav.TrustRequest
	domains []string
	stored  []Approval
	answer  Answer
}

func (r *recorder) ask(req webdav.TrustRequest, domain string) Answer {
	r.asked = append(r.asked, req)
	r.domains = append(r.domains, domain)
	return r.answer
}

func (r *recorder) store(approval Approval) {
	r.stored = append(r.stored, approval)
}

func request(host string, source webdav.TrustSource) webdav.TrustRequest {
	return webdav.TrustRequest{Endpoint: "caldav.icloud.com", Host: host, Source: source}
}

func TestDeciderUsesWhatIsStored(t *testing.T) {
	var rec recorder
	decider := &Decider{
		Stored: []Pattern{"dav.provider.net", "*.icloud.com"},
		Ask:    rec.ask,
		Store:  rec.store,
	}

	for _, host := range []string{"dav.provider.net", "p42-caldav.icloud.com", "p42-contacts.icloud.com"} {
		if !decider.TrustHost(request(host, webdav.TrustSourceReference)) {
			t.Errorf("%s is rejected although it is stored", host)
		}
	}
	if len(rec.asked) != 0 || len(rec.stored) != 0 {
		t.Errorf("asked %v and stored %v for hosts that were approved before", rec.asked, rec.stored)
	}
}

func TestDeciderTakesTheFlagAndStoresItOnUse(t *testing.T) {
	var rec recorder
	decider := &Decider{
		Flags: []Pattern{"*.icloud.com", "never-met.example.com"},
		Ask:   rec.ask,
		Store: rec.store,
	}

	for range 3 {
		if !decider.TrustHost(request("p42-caldav.icloud.com", webdav.TrustSourceReference)) {
			t.Fatal("the flag doesn't approve the host it matches")
		}
	}
	if !decider.TrustHost(request("p42-contacts.icloud.com", webdav.TrustSourceRedirect)) {
		t.Error("the second host under the pattern is rejected")
	}

	want := []Approval{{Pattern: "*.icloud.com", Source: webdav.TrustSourceReference, Origin: OriginFlag}}
	if !slices.Equal(rec.stored, want) {
		t.Errorf("stored = %+v, want the pattern of the flag once, and not the value no host ever met", rec.stored)
	}
	if len(rec.asked) != 0 {
		t.Errorf("asked = %v although the flag answers", rec.asked)
	}
}

func TestDeciderAsksOncePerHost(t *testing.T) {
	rec := recorder{answer: AnswerNo}
	decider := &Decider{Flags: []Pattern{"other.example.com"}, Ask: rec.ask, Store: rec.store}

	for range 4 {
		if decider.TrustHost(request("p42-caldav.icloud.com", webdav.TrustSourceReference)) {
			t.Fatal("a no is a yes")
		}
	}
	if len(rec.asked) != 1 || len(rec.stored) != 0 {
		t.Errorf("asked %d times and stored %v, want one question and no stored no", len(rec.asked), rec.stored)
	}
	if rec.domains[0] != "icloud.com" {
		t.Errorf("the question offers %q, want icloud.com", rec.domains[0])
	}

	rec.answer = AnswerHost
	if decider.TrustHost(request("p42-caldav.icloud.com", webdav.TrustSourceReference)) {
		t.Error("a no of this run was asked again")
	}
}

func TestDeciderStoresTheHost(t *testing.T) {
	rec := recorder{answer: AnswerHost}
	decider := &Decider{Ask: rec.ask, Store: rec.store}

	if !decider.TrustHost(request("dav.provider.net", webdav.TrustSourceRecord)) {
		t.Fatal("rejected after a yes")
	}
	if !decider.TrustHost(request("dav.provider.net", webdav.TrustSourceReference)) {
		t.Fatal("rejected on the second request")
	}
	decider.TrustHost(request("other.provider.net", webdav.TrustSourceReference))
	if len(rec.asked) != 2 {
		t.Errorf("asked %d times, want a second question for another host of the domain", len(rec.asked))
	}

	want := Approval{Pattern: "dav.provider.net", Source: webdav.TrustSourceRecord, Origin: OriginPrompt}
	if len(rec.stored) < 1 || rec.stored[0] != want {
		t.Errorf("stored = %+v, want %+v first", rec.stored, want)
	}
}

func TestDeciderStoresTheDomain(t *testing.T) {
	rec := recorder{answer: AnswerDomain}
	decider := &Decider{Ask: rec.ask, Store: rec.store}

	for _, host := range []string{"p42-caldav.icloud.com", "p42-contacts.icloud.com", "p07-caldav.icloud.com"} {
		if !decider.TrustHost(request(host, webdav.TrustSourceReference)) {
			t.Errorf("%s is rejected", host)
		}
	}
	want := []Approval{{Pattern: "*.icloud.com", Source: webdav.TrustSourceReference, Origin: OriginPrompt}}
	if len(rec.asked) != 1 || !slices.Equal(rec.stored, want) {
		t.Errorf("asked %d times and stored %+v, want one question and the pattern", len(rec.asked), rec.stored)
	}

	apex := &Decider{Ask: rec.ask, Store: rec.store}
	rec.stored = nil
	if !apex.TrustHost(request("provider.net", webdav.TrustSourceRedirect)) {
		t.Fatal("the domain itself is rejected after a yes for the domain")
	}
	if len(rec.stored) != 2 || rec.stored[0].Pattern != "*.provider.net" || rec.stored[1].Pattern != "provider.net" {
		t.Errorf("stored = %+v, want the pattern and the host it doesn't match", rec.stored)
	}
}

func TestDeciderWithoutADomainToOffer(t *testing.T) {
	rec := recorder{answer: AnswerDomain}
	decider := &Decider{Ask: rec.ask, Store: rec.store}

	if decider.TrustHost(request("localhost", webdav.TrustSourceReference)) {
		t.Error("a domain was approved for a host that has none")
	}
	if len(rec.domains) != 1 || rec.domains[0] != "" || len(rec.stored) != 0 {
		t.Errorf("offered %q and stored %v", rec.domains, rec.stored)
	}
}

func TestDeciderWithoutATerminal(t *testing.T) {
	var rec recorder
	decider := &Decider{Flags: []Pattern{"other.example.com"}, Store: rec.store}

	for range 2 {
		if decider.TrustHost(request("p42-caldav.icloud.com", webdav.TrustSourceReference)) {
			t.Fatal("approved with nobody to ask")
		}
	}
	if len(rec.stored) != 0 {
		t.Errorf("stored = %v", rec.stored)
	}
	if got := decider.Rejected(); len(got) != 1 || got[0].Host != "p42-caldav.icloud.com" {
		t.Errorf("Rejected = %+v, want the host once", got)
	}
}
