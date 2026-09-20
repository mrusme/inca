package dav

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/models/service"
	"xn--gckvb8fzb.com/maya/libs/webdav"
)

type scripted struct {
	*httptest.Server

	mu       sync.Mutex
	requests []string
}

func (s *scripted) seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func newScripted(t *testing.T, respond func(w http.ResponseWriter, r *http.Request)) *scripted {
	t.Helper()

	s := new(scripted)
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		s.mu.Lock()
		s.requests = append(s.requests, r.Method+" "+r.URL.Path)
		s.mu.Unlock()
		respond(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func multistatus(w http.ResponseWriter, responses ...string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	fmt.Fprintf(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">%s</D:multistatus>`,
		strings.Join(responses, ""))
}

func propResponse(href string, props string) string {
	return fmt.Sprintf(`<D:response><D:href>%s</D:href><D:propstat><D:prop>%s</D:prop>`+
		`<D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`, href, props)
}

func calendarProps(name string) string {
	return `<D:resourcetype><D:collection/><C:calendar/></D:resourcetype><D:displayname>` + name + `</D:displayname>`
}

func davServer(t *testing.T, root string, homeSet func() string) *scripted {
	t.Helper()

	return newScripted(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/.well-known/"):
			http.Redirect(w, r, root, http.StatusMovedPermanently)
		case r.URL.Path == root:
			multistatus(w, propResponse(root, `<D:current-user-principal><D:href>`+root+`alice/</D:href></D:current-user-principal>`))
		case r.URL.Path == root+"alice/":
			multistatus(w, propResponse(r.URL.Path, `<C:calendar-home-set><D:href>`+homeSet()+`</D:href></C:calendar-home-set>`))
		case r.URL.Path == root+"alice/calendars/":
			multistatus(w,
				propResponse(r.URL.Path, `<D:resourcetype><D:collection/></D:resourcetype>`),
				propResponse(r.URL.Path+"work/", calendarProps("Work")),
			)
		default:
			http.NotFound(w, r)
		}
	})
}

func TestAHomeSetOnAnotherHostNeedsApproval(t *testing.T) {
	second := newScripted(t, func(w http.ResponseWriter, r *http.Request) {
		multistatus(w,
			propResponse("/1/calendars/", `<D:resourcetype><D:collection/></D:resourcetype>`),
			propResponse("/1/calendars/home/", calendarProps("Home")),
		)
	})
	secondURL := strings.Replace(second.URL, "127.0.0.1", "localhost", 1)
	first := davServer(t, "/dav/", func() string { return secondURL + "/1/calendars/" })

	account := config.Account{Name: "split", Username: "alice", Password: "secret", Endpoint: first.URL}

	strict, err := New(account, &Options{UserAgent: "inca/test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = strict.FindCalendars(context.Background())
	var untrusted *webdav.UntrustedHostError
	if !errors.As(err, &untrusted) || untrusted.Host != "localhost" {
		t.Fatalf("err = %v, want an UntrustedHostError for localhost", err)
	}
	if len(second.seen()) != 0 {
		t.Fatalf("the second host got %v without approval", second.seen())
	}

	var asked []webdav.TrustRequest
	trusting, err := New(account, &Options{
		UserAgent: "inca/test",
		TrustHost: func(req webdav.TrustRequest) bool {
			asked = append(asked, req)
			return true
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	calendars, err := trusting.FindCalendars(context.Background())
	if err != nil || len(calendars) != 1 || calendars[0].Path != secondURL+"/1/calendars/home/" {
		t.Fatalf("calendars = %+v, %v", calendars, err)
	}
	want := webdav.TrustRequest{Endpoint: "127.0.0.1", Host: "localhost", Source: webdav.TrustSourceReference}
	if !slices.Contains(asked, want) {
		t.Errorf("asked = %+v, want a question about a reference", asked)
	}
}

func TestAStoredServiceCostsNoDiscovery(t *testing.T) {
	server := davServer(t, "/dav/", func() string { return "/dav/alice/calendars/" })
	account := config.Account{Name: "home", Username: "alice", Password: "secret", Endpoint: server.URL}

	var discovered []*service.Service
	first, err := New(account, &Options{OnDiscover: func(found *service.Service) { discovered = append(discovered, found) }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err = first.FindCalendars(context.Background()); err != nil {
		t.Fatalf("FindCalendars: %v", err)
	}
	if len(discovered) != 1 {
		t.Fatalf("discovered = %+v, want one service", discovered)
	}
	found := discovered[0]
	if found.AccountName != "home" || found.Protocol != service.CalDAV || found.Start != server.URL ||
		found.Endpoint != server.URL+"/dav/" || found.Principal != "/dav/alice/" {
		t.Errorf("found = %+v", found)
	}
	if !slices.Contains(server.seen(), "PROPFIND /.well-known/caldav") {
		t.Errorf("requests = %v, want a discovery", server.seen())
	}

	before := len(server.seen())
	discovered = nil
	second, err := New(account, &Options{CalDAV: found, OnDiscover: func(found *service.Service) { discovered = append(discovered, found) }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	calendars, err := second.FindCalendars(context.Background())
	if err != nil || len(calendars) != 1 {
		t.Fatalf("calendars = %+v, %v", calendars, err)
	}
	if got := server.seen()[before:]; !slices.Equal(got, []string{"PROPFIND /dav/alice/", "PROPFIND /dav/alice/calendars/"}) {
		t.Errorf("requests = %v, want the home set and the calendars alone", got)
	}
	if len(discovered) != 0 {
		t.Errorf("discovered = %+v for a service that was stored", discovered)
	}
}

func TestAStoredServiceOfAnotherStartIsIgnored(t *testing.T) {
	server := davServer(t, "/dav/", func() string { return "/dav/alice/calendars/" })
	account := config.Account{Name: "home", Username: "alice", Password: "secret", Endpoint: server.URL}

	stale := service.New("home", service.CalDAV)
	stale.Start, stale.Endpoint, stale.Principal = "https://old.example.com", "https://old.example.com/dav/", "/dav/alice/"

	var discovered []*service.Service
	d, err := New(account, &Options{CalDAV: stale, OnDiscover: func(found *service.Service) { discovered = append(discovered, found) }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err = d.FindCalendars(context.Background()); err != nil {
		t.Fatalf("FindCalendars: %v", err)
	}
	if len(discovered) != 1 || discovered[0].Endpoint != server.URL+"/dav/" {
		t.Errorf("discovered = %+v, want the endpoint of the account as it is configured now", discovered)
	}
}

func TestAServiceThatMovedIsDiscoveredAgain(t *testing.T) {
	server := davServer(t, "/new/", func() string { return "/new/alice/calendars/" })
	account := config.Account{Name: "home", Username: "alice", Password: "secret", Endpoint: server.URL}

	old := service.New("home", service.CalDAV)
	old.Start, old.Endpoint, old.Principal = server.URL, server.URL+"/old/", "/old/alice/"

	var discovered []*service.Service
	d, err := New(account, &Options{CalDAV: old, OnDiscover: func(found *service.Service) { discovered = append(discovered, found) }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	calendars, err := d.FindCalendars(context.Background())
	if err != nil || len(calendars) != 1 || calendars[0].Path != "/new/alice/calendars/work/" {
		t.Fatalf("calendars = %+v, %v", calendars, err)
	}
	if len(discovered) != 1 || discovered[0].Endpoint != server.URL+"/new/" || discovered[0].Principal != "/new/alice/" {
		t.Errorf("discovered = %+v", discovered)
	}
	if got := server.seen(); got[0] != "PROPFIND /old/alice/" {
		t.Errorf("requests = %v, want the stored principal first", got)
	}
}

func TestAnErrorThatIsNoMoveStartsNoDiscovery(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := newScripted(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			})
			account := config.Account{Name: "home", Username: "alice", Password: "secret", Endpoint: server.URL}

			stored := service.New("home", service.CalDAV)
			stored.Start, stored.Endpoint, stored.Principal = server.URL, server.URL+"/dav/", "/dav/alice/"

			discoveries := 0
			d, err := New(account, &Options{CalDAV: stored, OnDiscover: func(*service.Service) { discoveries++ }})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			_, err = d.FindCalendars(context.Background())
			var httpErr *webdav.HTTPError
			if !errors.As(err, &httpErr) || httpErr.Code != status {
				t.Fatalf("err = %v, want the %d", err, status)
			}
			if got := server.seen(); !slices.Equal(got, []string{"PROPFIND /dav/alice/"}) || discoveries != 0 {
				t.Errorf("requests = %v with %d discoveries, want the one request and no search", got, discoveries)
			}
		})
	}
}

func TestAnAddressBookServiceThatMovedIsDiscoveredAgain(t *testing.T) {
	server := newScripted(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/carddav":
			http.Redirect(w, r, "/new/", http.StatusMovedPermanently)
		case "/new/":
			multistatus(w, propResponse("/new/", `<D:current-user-principal><D:href>/new/alice/</D:href></D:current-user-principal>`))
		case "/new/alice/":
			multistatus(w, propResponse("/new/alice/",
				`<R:addressbook-home-set xmlns:R="urn:ietf:params:xml:ns:carddav"><D:href>/new/alice/contacts/</D:href></R:addressbook-home-set>`))
		case "/new/alice/contacts/":
			multistatus(w,
				propResponse("/new/alice/contacts/", `<D:resourcetype><D:collection/></D:resourcetype>`),
				propResponse("/new/alice/contacts/friends/",
					`<D:resourcetype><D:collection/><R:addressbook xmlns:R="urn:ietf:params:xml:ns:carddav"/></D:resourcetype><D:displayname>Friends</D:displayname>`),
			)
		default:
			w.WriteHeader(http.StatusGone)
		}
	})
	account := config.Account{Name: "home", Username: "alice", Password: "secret", Endpoint: server.URL}

	old := service.New("home", service.CardDAV)
	old.Start, old.Endpoint, old.Principal = server.URL, server.URL+"/old/", "/old/alice/"

	var discovered []*service.Service
	d, err := New(account, &Options{CardDAV: old, OnDiscover: func(found *service.Service) { discovered = append(discovered, found) }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	books, err := d.FindAddressBooks(context.Background())
	if err != nil || len(books) != 1 || books[0].Path != "/new/alice/contacts/friends/" {
		t.Fatalf("books = %+v, %v", books, err)
	}
	if len(discovered) != 1 || discovered[0].Protocol != service.CardDAV || discovered[0].Principal != "/new/alice/" {
		t.Errorf("discovered = %+v", discovered)
	}
}

func TestDiscoverIgnoresWhatIsStored(t *testing.T) {
	server := davServer(t, "/dav/", func() string { return "/dav/alice/calendars/" })
	account := config.Account{Name: "home", Username: "alice", Password: "secret", Endpoint: server.URL}

	stored := service.New("home", service.CalDAV)
	stored.Start, stored.Endpoint, stored.Principal = server.URL, server.URL+"/stale/", "/stale/alice/"

	var discovered []*service.Service
	d, err := New(account, &Options{CalDAV: stored, OnDiscover: func(found *service.Service) { discovered = append(discovered, found) }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	found, err := d.Discover(context.Background(), service.CalDAV)
	if err != nil || found.Endpoint != server.URL+"/dav/" || found.Principal != "/dav/alice/" {
		t.Fatalf("found = %+v, %v", found, err)
	}
	if len(discovered) != 1 || discovered[0] != found {
		t.Errorf("discovered = %+v", discovered)
	}
	if !slices.Contains(server.seen(), "PROPFIND /.well-known/caldav") {
		t.Errorf("requests = %v, want a discovery although a service was stored", server.seen())
	}

	if _, err = d.Discover(context.Background(), "gopher"); err == nil {
		t.Error("a protocol that doesn't exist was discovered")
	}
}
