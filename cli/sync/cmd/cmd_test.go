package syncCmd

import (
	"log/slog"
	"slices"
	"testing"

	"xn--gckvb8fzb.com/inca/database"
	"xn--gckvb8fzb.com/inca/dav"
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

func TestStaleCalendarObjects(t *testing.T) {
	rt := newRuntime(t)

	for _, one := range []struct{ account, calendar, path string }{
		{"home", "/dav/alice/calendars/personal/", "/dav/alice/calendars/personal/kept.ics"},
		{"home", "/dav/alice/calendars/personal/", "/dav/alice/calendars/personal/gone.ics"},
		{"home", "/dav/alice/calendars/work/", "/dav/alice/calendars/work/other-calendar.ics"},
		{"office", "/dav/alice/calendars/personal/", "/dav/alice/calendars/personal/other-account.ics"},
	} {
		if err := calendarobject.Set(rt.Database, calendarobject.New(one.account, one.calendar, one.path)); err != nil {
			t.Fatal(err)
		}
	}

	stale := staleCalendarObjects(rt, "home", "/dav/alice/calendars/personal/",
		[]dav.CalendarObject{{Path: "/dav/alice/calendars/personal/kept.ics"}, {Path: "/dav/alice/calendars/personal/new.ics"}})
	if !slices.Equal(stale, []string{"/dav/alice/calendars/personal/gone.ics"}) {
		t.Errorf("stale = %v, want the one object of this calendar and account that the server no longer lists", stale)
	}

	if all := staleCalendarObjects(rt, "home", "/dav/alice/calendars/work/", nil); len(all) != 1 {
		t.Errorf("a calendar the server lists as empty: %v, want its one local object", all)
	}
}

func TestStaleAddressObjects(t *testing.T) {
	rt := newRuntime(t)

	for _, one := range []struct{ account, book, path string }{
		{"home", "/dav/alice/contacts/personal/", "/dav/alice/contacts/personal/kept.vcf"},
		{"home", "/dav/alice/contacts/personal/", "/dav/alice/contacts/personal/gone.vcf"},
		{"home", "/dav/alice/contacts/clients/", "/dav/alice/contacts/clients/other-book.vcf"},
	} {
		if err := addressobject.Set(rt.Database, addressobject.New(one.account, one.book, one.path)); err != nil {
			t.Fatal(err)
		}
	}

	stale := staleAddressObjects(rt, "home", "/dav/alice/contacts/personal/",
		[]dav.AddressObject{{Path: "/dav/alice/contacts/personal/kept.vcf"}})
	if !slices.Equal(stale, []string{"/dav/alice/contacts/personal/gone.vcf"}) {
		t.Errorf("stale = %v", stale)
	}
}
