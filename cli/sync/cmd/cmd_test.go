package syncCmd

import (
	"log/slog"
	"maps"
	"testing"

	"xn--gckvb8fzb.com/inca/database"
	"xn--gckvb8fzb.com/inca/helpers/log"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/models/calendarobject"
	"xn--gckvb8fzb.com/inca/runtime"
)

func newRuntime(t *testing.T) *runtime.Runtime {
	t.Helper()

	rt := new(runtime.Runtime)
	rt.Logger = log.New(slog.LevelError)

	db, err := database.New(rt.Logger, "", false)
	if err != nil {
		t.Fatal(err)
	}
	rt.Database = db

	return rt
}

func TestKnownCalendarObjects(t *testing.T) {
	rt := newRuntime(t)

	for _, one := range []struct{ account, calendar, path, etag string }{
		{"home", "/dav/alice/calendars/personal/", "/dav/alice/calendars/personal/a.ics", "1"},
		{"home", "/dav/alice/calendars/personal/", "/dav/alice/calendars/personal/b.ics", ""},
		{"home", "/dav/alice/calendars/work/", "/dav/alice/calendars/work/other-calendar.ics", "3"},
		{"office", "/dav/alice/calendars/personal/", "/dav/alice/calendars/personal/other-account.ics", "4"},
	} {
		co := calendarobject.New(one.account, one.calendar, one.path)
		co.ETag = one.etag
		if err := calendarobject.Set(rt.Database, co); err != nil {
			t.Fatal(err)
		}
	}

	known := knownCalendarObjects(rt, "home", "/dav/alice/calendars/personal/")
	want := map[string]string{
		"/dav/alice/calendars/personal/a.ics": "1",
		"/dav/alice/calendars/personal/b.ics": "",
	}
	if !maps.Equal(known, want) {
		t.Errorf("known = %v, want the objects of this calendar and account alone", known)
	}

	if none := knownCalendarObjects(rt, "home", "/dav/alice/calendars/empty/"); len(none) != 0 {
		t.Errorf("a calendar without local objects: %v", none)
	}
}

func TestKnownAddressObjects(t *testing.T) {
	rt := newRuntime(t)

	for _, one := range []struct{ account, book, path, etag string }{
		{"home", "/dav/alice/contacts/personal/", "/dav/alice/contacts/personal/a.vcf", "1"},
		{"home", "/dav/alice/contacts/clients/", "/dav/alice/contacts/clients/other-book.vcf", "2"},
		{"office", "/dav/alice/contacts/personal/", "/dav/alice/contacts/personal/other-account.vcf", "3"},
	} {
		ao := addressobject.New(one.account, one.book, one.path)
		ao.ETag = one.etag
		if err := addressobject.Set(rt.Database, ao); err != nil {
			t.Fatal(err)
		}
	}

	known := knownAddressObjects(rt, "home", "/dav/alice/contacts/personal/")
	if !maps.Equal(known, map[string]string{"/dav/alice/contacts/personal/a.vcf": "1"}) {
		t.Errorf("known = %v", known)
	}
}
