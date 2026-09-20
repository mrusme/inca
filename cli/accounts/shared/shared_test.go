package shared

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"xn--gckvb8fzb.com/inca/database"
	"xn--gckvb8fzb.com/inca/helpers/log"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/helpers/trust"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/models/service"
	"xn--gckvb8fzb.com/inca/models/trustedhost"
	"xn--gckvb8fzb.com/inca/runtime"
	"xn--gckvb8fzb.com/maya/libs/webdav"
)

func newRuntime(t *testing.T) *runtime.Runtime {
	t.Helper()

	rt := new(runtime.Runtime)
	rt.Logger = log.New(slog.LevelError)
	rt.Out = out.New(out.ColorNever)

	db, err := database.New(rt.Logger, "", false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	rt.Database = db

	return rt
}

var accounts = []config.Account{{Name: "icloud"}, {Name: "work"}}

func TestParseTrustFlags(t *testing.T) {
	flags, err := ParseTrustFlags([]string{
		"icloud=*.icloud.com", "icloud=P42-CalDAV.iCloud.com", "work = dav.provider.net ",
	}, accounts)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !slices.Equal(flags["icloud"], []trust.Pattern{"*.icloud.com", "p42-caldav.icloud.com"}) ||
		!slices.Equal(flags["work"], []trust.Pattern{"dav.provider.net"}) {
		t.Errorf("flags = %v", flags)
	}

	if none, err := ParseTrustFlags(nil, accounts); err != nil || len(none) != 0 {
		t.Errorf("no flag: %v, %v", none, err)
	}

	for _, value := range []string{
		"*.icloud.com",
		"=*.icloud.com",
		"icloud=",
		"icloud",
		"unknown=*.icloud.com",
		"icloud=*.com",
		"icloud=https://p42-caldav.icloud.com",
	} {
		if got, err := ParseTrustFlags([]string{value}, accounts); err == nil {
			t.Errorf("--trust-host %q = %v, want an error", value, got)
		}
	}
}

func TestReadAnswer(t *testing.T) {
	tests := []struct {
		input  string
		domain string
		want   trust.Answer
	}{
		{"h\n", "icloud.com", trust.AnswerHost},
		{" H \n", "icloud.com", trust.AnswerHost},
		{"d\n", "icloud.com", trust.AnswerDomain},
		{"D\r\n", "icloud.com", trust.AnswerDomain},
		{"d\n", "", trust.AnswerNo},
		{"n\n", "icloud.com", trust.AnswerNo},
		{"\n", "icloud.com", trust.AnswerNo},
		{"yes\n", "icloud.com", trust.AnswerNo},
		{"", "icloud.com", trust.AnswerNo},
		{"h", "icloud.com", trust.AnswerHost},
	}
	for _, tt := range tests {
		got := readAnswer(bufio.NewReader(strings.NewReader(tt.input)), tt.domain)
		if got != tt.want {
			t.Errorf("readAnswer(%q) with the domain %q = %v, want %v", tt.input, tt.domain, got, tt.want)
		}
	}
}

func TestQuestion(t *testing.T) {
	for source, want := range map[webdav.TrustSource]string{
		webdav.TrustSourceReference: "refers to",
		webdav.TrustSourceRedirect:  "redirects to",
		webdav.TrustSourceRecord:    "DNS",
	} {
		text := question(webdav.TrustRequest{Endpoint: "caldav.icloud.com", Host: "p42-caldav.icloud.com", Source: source})
		if !strings.Contains(text, want) || !strings.Contains(text, "caldav.icloud.com") || !strings.Contains(text, "p42-caldav.icloud.com") {
			t.Errorf("question for a %v = %q, want %q and both hosts", source, text, want)
		}
	}

	forged := question(webdav.TrustRequest{Endpoint: "example.com", Host: "dav.provider.net", Source: webdav.TrustSourceRecord})
	if !strings.Contains(forged, "forged") {
		t.Errorf("the question about a record = %q, want the warning that DNS can be forged", forged)
	}
}

func TestHint(t *testing.T) {
	hint := Hint("icloud", webdav.TrustRequest{Endpoint: "caldav.icloud.com", Host: "p42-caldav.icloud.com"})
	if !strings.Contains(hint, "inca sync --trust-host icloud=p42-caldav.icloud.com") {
		t.Errorf("hint = %q, want the command that approves the host", hint)
	}
}

func splitServers(t *testing.T) (first *httptest.Server, foreign string) {
	t.Helper()

	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		fmt.Fprint(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
<D:response><D:href>/1/calendars/home/</D:href><D:propstat><D:prop>
<D:resourcetype><D:collection/><C:calendar/></D:resourcetype><D:displayname>Home</D:displayname>
</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response></D:multistatus>`)
	}))
	t.Cleanup(second.Close)
	foreign = strings.Replace(second.URL, "127.0.0.1", "localhost", 1)

	first = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)

		prop := `<D:current-user-principal><D:href>/dav/alice/</D:href></D:current-user-principal>`
		if r.URL.Path == "/dav/alice/" {
			prop = `<C:calendar-home-set><D:href>` + foreign + `/1/calendars/</D:href></C:calendar-home-set>`
		}
		fmt.Fprintf(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
<D:response><D:href>%s</D:href><D:propstat><D:prop>%s</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>
</D:multistatus>`, r.URL.Path, prop)
	}))
	t.Cleanup(first.Close)

	return first, foreign
}

func TestConnectStoresWhatItLearns(t *testing.T) {
	rt := newRuntime(t)
	first, _ := splitServers(t)
	account := config.Account{Name: "split", Username: "alice", Password: "secret", Endpoint: first.URL + "/dav/"}
	ctx := context.Background()

	rejecting, err := Connect(rt, account, &Options{})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err = rejecting.DAV.FindCalendars(ctx); err == nil {
		t.Fatal("the foreign host was used with nobody to ask and no flag")
	}
	if rejected := rejecting.Decider.Rejected(); len(rejected) != 1 || rejected[0].Host != "localhost" {
		t.Errorf("rejected = %+v", rejected)
	}
	if rows, _ := trustedhost.ListByAccount(rt.Database, "split"); len(rows) != 0 {
		t.Errorf("stored %+v after a rejection", rows)
	}

	flags, err := ParseTrustFlags([]string{"split=localhost"}, []config.Account{account})
	if err != nil {
		t.Fatalf("ParseTrustFlags: %v", err)
	}
	approving, err := Connect(rt, account, &Options{Flags: flags})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	calendars, err := approving.DAV.FindCalendars(ctx)
	if err != nil || len(calendars) != 1 {
		t.Fatalf("calendars = %+v, %v", calendars, err)
	}

	rows, err := trustedhost.ListByAccount(rt.Database, "split")
	if err != nil || len(rows) != 1 || rows[0].Pattern != "localhost" || rows[0].Origin != string(trust.OriginFlag) || rows[0].Source != "reference" {
		t.Errorf("approvals = %+v, %v", rows, err)
	}
	stored, found, err := service.Get(rt.Database, "split", service.CalDAV)
	if err != nil || !found || stored.Endpoint != first.URL+"/dav/" || stored.Principal != "/dav/alice/" {
		t.Errorf("service = %+v, %v, %v", stored, found, err)
	}

	later, err := Connect(rt, account, &Options{})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if calendars, err = later.DAV.FindCalendars(ctx); err != nil || len(calendars) != 1 {
		t.Errorf("a later run without a flag and without a terminal: %+v, %v", calendars, err)
	}
	if later.DAV == nil || len(later.Decider.Rejected()) != 0 {
		t.Errorf("rejected = %+v although the approval is stored", later.Decider.Rejected())
	}

}

func TestDiscoverReplacesWhatIsStored(t *testing.T) {
	rt := newRuntime(t)
	first, _ := splitServers(t)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	account := config.Account{
		Name: "split", Username: "alice", Password: "secret",
		CalDAVEndpoint: first.URL + "/dav/", CardDAVEndpoint: closed.URL + "/dav/",
	}
	for _, protocol := range []string{service.CalDAV, service.CardDAV} {
		stale := service.New("split", protocol)
		stale.Start, stale.Endpoint, stale.Principal = first.URL+"/dav/", first.URL+"/stale/", "/stale/alice/"
		if err := service.Set(rt.Database, stale); err != nil {
			t.Fatal(err)
		}
	}

	view := Discover(context.Background(), rt, account, &Options{})
	if view.AccountName != "split" || len(view.Services) != 1 || view.Services[0].Endpoint != first.URL+"/dav/" {
		t.Errorf("services = %+v", view.Services)
	}
	if len(view.Errors) != 1 || !strings.Contains(view.Errors[0], "address book") {
		t.Errorf("errors = %v, want the one of the address book service", view.Errors)
	}

	cal, _, _ := service.Get(rt.Database, "split", service.CalDAV)
	card, _, _ := service.Get(rt.Database, "split", service.CardDAV)
	if cal.Endpoint != first.URL+"/dav/" || card.Endpoint != first.URL+"/stale/" {
		t.Errorf("stored = %q and %q, want the found one and the one a failure left alone", cal.Endpoint, card.Endpoint)
	}

	nowhere := Discover(context.Background(), rt, config.Account{Name: "empty", Username: "alice"}, &Options{})
	if len(nowhere.Services) != 0 || len(nowhere.Errors) != 1 {
		t.Errorf("an account without a start = %+v", nowhere)
	}
}

func TestDiscoverNamesTheHostNobodyApproved(t *testing.T) {
	rt := newRuntime(t)
	target := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(target.Close)
	foreign := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)

	start := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", foreign+"/dav/")
		w.WriteHeader(http.StatusPermanentRedirect)
	}))
	t.Cleanup(start.Close)

	account := config.Account{Name: "moved", Username: "alice", Password: "secret", CalDAVEndpoint: start.URL + "/dav/"}
	view := Discover(context.Background(), rt, account, &Options{})

	if len(view.Services) != 0 || len(view.Errors) != 2 {
		t.Fatalf("view = %+v, want the failure and the hint", view)
	}
	if !strings.Contains(view.Errors[1], "--trust-host moved=localhost") {
		t.Errorf("hint = %q", view.Errors[1])
	}
}
