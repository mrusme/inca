package dav

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"xn--gckvb8fzb.com/inca/database"
	"xn--gckvb8fzb.com/inca/helpers/log"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/models/calendarobject"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/maya/libs/webdav"
	"xn--gckvb8fzb.com/maya/libs/webdav/caldav"
	"xn--gckvb8fzb.com/maya/libs/webdav/carddav"
)

type calTestBackend struct {
	principal     string
	homeSet       string
	calendars     []caldav.Calendar
	objects       map[string][]caldav.CalendarObject
	objectsByPath map[string]caldav.CalendarObject
	syncScript    map[string]*caldav.SyncResponse
}

func (b *calTestBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return b.principal, nil
}

func (b *calTestBackend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	return b.homeSet, nil
}

func (b *calTestBackend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	return b.calendars, nil
}

func (b *calTestBackend) GetCalendar(ctx context.Context, path string) (*caldav.Calendar, error) {
	for i := range b.calendars {
		if b.calendars[i].Path == path {
			return &b.calendars[i], nil
		}
	}
	return nil, fmt.Errorf("calendar not found: %s", path)
}

func (b *calTestBackend) QueryCalendarObjects(ctx context.Context, path string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	return b.objects[path], nil
}

func (b *calTestBackend) ListCalendarObjects(ctx context.Context, path string, req *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	return b.objects[path], nil
}

func (b *calTestBackend) GetCalendarObject(ctx context.Context, path string, req *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	if o, ok := b.objectsByPath[path]; ok {
		return &o, nil
	}
	return nil, fmt.Errorf("calendar object not found: %s", path)
}

func (b *calTestBackend) SyncCalendar(ctx context.Context, path string, query *caldav.SyncQuery) (*caldav.SyncResponse, error) {
	if resp, ok := b.syncScript[query.SyncToken]; ok {
		return resp, nil
	}
	return &caldav.SyncResponse{SyncToken: query.SyncToken}, nil
}

func (b *calTestBackend) UpdateCalendar(ctx context.Context, path string, update *caldav.CalendarUpdate) error {
	return nil
}

func (b *calTestBackend) CreateCalendar(ctx context.Context, calendar *caldav.Calendar) error {
	return fmt.Errorf("not implemented")
}

func (b *calTestBackend) PutCalendarObject(ctx context.Context, path string, calendar *ical.Calendar, opts *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, bool, error) {
	return nil, false, fmt.Errorf("not implemented")
}

func (b *calTestBackend) DeleteCalendarObject(ctx context.Context, path string, opts *caldav.DeleteCalendarObjectOptions) error {
	return fmt.Errorf("not implemented")
}

func (b *calTestBackend) DeleteCalendar(ctx context.Context, path string, opts *caldav.DeleteCalendarObjectOptions) error {
	return fmt.Errorf("not implemented")
}

type cardTestBackend struct {
	principal     string
	homeSet       string
	addressBooks  []carddav.AddressBook
	objects       map[string][]carddav.AddressObject
	objectsByPath map[string]carddav.AddressObject
	syncScript    map[string]*carddav.SyncResponse
}

func (b *cardTestBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return b.principal, nil
}

func (b *cardTestBackend) AddressBookHomeSetPath(ctx context.Context) (string, error) {
	return b.homeSet, nil
}

func (b *cardTestBackend) ListAddressBooks(ctx context.Context) ([]carddav.AddressBook, error) {
	return b.addressBooks, nil
}

func (b *cardTestBackend) GetAddressBook(ctx context.Context, path string) (*carddav.AddressBook, error) {
	for i := range b.addressBooks {
		if b.addressBooks[i].Path == path {
			return &b.addressBooks[i], nil
		}
	}
	return nil, fmt.Errorf("address book not found: %s", path)
}

func (b *cardTestBackend) QueryAddressObjects(ctx context.Context, path string, query *carddav.AddressBookQuery) ([]carddav.AddressObject, error) {
	return b.objects[path], nil
}

func (b *cardTestBackend) ListAddressObjects(ctx context.Context, path string, req *carddav.AddressDataRequest) ([]carddav.AddressObject, error) {
	return b.objects[path], nil
}

func (b *cardTestBackend) GetAddressObject(ctx context.Context, path string, req *carddav.AddressDataRequest) (*carddav.AddressObject, error) {
	if o, ok := b.objectsByPath[path]; ok {
		return &o, nil
	}
	return nil, fmt.Errorf("address object not found: %s", path)
}

func (b *cardTestBackend) SyncAddressBook(ctx context.Context, path string, query *carddav.SyncQuery) (*carddav.SyncResponse, error) {
	if resp, ok := b.syncScript[query.SyncToken]; ok {
		return resp, nil
	}
	return &carddav.SyncResponse{SyncToken: query.SyncToken}, nil
}

func (b *cardTestBackend) UpdateAddressBook(ctx context.Context, path string, update *carddav.AddressBookUpdate) error {
	return nil
}

func (b *cardTestBackend) CreateAddressBook(ctx context.Context, addressBook *carddav.AddressBook) error {
	return fmt.Errorf("not implemented")
}

func (b *cardTestBackend) DeleteAddressBook(ctx context.Context, path string, opts *carddav.DeleteAddressObjectOptions) error {
	return fmt.Errorf("not implemented")
}

func (b *cardTestBackend) PutAddressObject(ctx context.Context, path string, card vcard.Card, opts *carddav.PutAddressObjectOptions) (*carddav.AddressObject, bool, error) {
	return nil, false, fmt.Errorf("not implemented")
}

func (b *cardTestBackend) DeleteAddressObject(ctx context.Context, path string, opts *carddav.DeleteAddressObjectOptions) error {
	return fmt.Errorf("not implemented")
}

func makeCalendarData(uid, component, summary string) *ical.Calendar {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//inca//test//EN")

	c := ical.NewComponent(component)
	c.Props.SetText(ical.PropUID, uid)
	c.Props.SetText(ical.PropSummary, summary)
	c.Props.SetDateTime(ical.PropDateTimeStamp, time.Now())
	if component == ical.CompEvent {
		c.Props.SetDateTime(ical.PropDateTimeStart, time.Now())
	}
	cal.Children = append(cal.Children, c)
	return cal
}

func formattedName(t *testing.T, obj *carddav.AddressObject) string {
	t.Helper()

	card, err := obj.Decoded()
	if err != nil || card == nil {
		t.Fatalf("%s has no card: %v", obj.Path, err)
	}
	return card.Value(vcard.FieldFormattedName)
}

func makeCard(uid, fn string) vcard.Card {
	card := make(vcard.Card)
	card.SetValue(vcard.FieldVersion, "3.0")
	card.SetValue(vcard.FieldUID, uid)
	card.SetValue(vcard.FieldFormattedName, fn)
	return card
}

func TestSyncRoundTrip(t *testing.T) {
	calObjectsOnServer := map[string][]caldav.CalendarObject{
		"/dav-principal/cal/personal": {
			{
				Path: "/dav-principal/cal/personal/event.ics",
				ETag: "etag-event",
				Data: makeCalendarData("event-1", ical.CompEvent, "Lunch"),
			},
			{
				Path: "/dav-principal/cal/personal/task.ics",
				ETag: "etag-task",
				Data: makeCalendarData("task-1", ical.CompToDo, "Buy milk"),
			},
		},
	}
	calBackend := &calTestBackend{
		principal: "/dav-principal",
		homeSet:   "/dav-principal/cal",
		calendars: []caldav.Calendar{{
			Path:                  "/dav-principal/cal/personal",
			Name:                  "Personal",
			SupportedComponentSet: []string{"VEVENT", "VTODO"},
		}},
		objects: calObjectsOnServer,
		syncScript: map[string]*caldav.SyncResponse{
			"": {SyncToken: "cal-1", Updated: calObjectsOnServer["/dav-principal/cal/personal"]},
		},
	}

	cardObjectsOnServer := map[string][]carddav.AddressObject{
		"/dav-principal/card/personal": {
			{
				Path: "/dav-principal/card/personal/alice.vcf",
				ETag: "etag-alice",
				Card: makeCard("contact-1", "Alice Example"),
			},
		},
	}
	cardBackend := &cardTestBackend{
		principal: "/dav-principal",
		homeSet:   "/dav-principal/card",
		addressBooks: []carddav.AddressBook{{
			Path: "/dav-principal/card/personal",
			Name: "Contacts",
		}},
		objects: cardObjectsOnServer,
		syncScript: map[string]*carddav.SyncResponse{
			"": {SyncToken: "card-1", Updated: cardObjectsOnServer["/dav-principal/card/personal"]},
		},
	}

	calServer := httptest.NewServer(&caldav.Handler{Backend: calBackend})
	defer calServer.Close()
	cardServer := httptest.NewServer(&carddav.Handler{Backend: cardBackend})
	defer cardServer.Close()

	account := config.Account{
		Name:            "test",
		Username:        "alice",
		Password:        "secret",
		CalDAVEndpoint:  calServer.URL,
		CardDAVEndpoint: cardServer.URL,
	}

	d, err := New(account, &Options{UserAgent: "inca/test"})
	if err != nil {
		t.Fatalf("dav.New: %s", err)
	}

	ctx := context.Background()

	db, err := database.New(log.New(slog.LevelError), "", false)
	if err != nil {
		t.Fatalf("database.New: %s", err)
	}
	defer db.Close()

	calendars, err := d.FindCalendars(ctx)
	if err != nil {
		t.Fatalf("FindCalendars: %s", err)
	}
	if len(calendars) != 1 {
		t.Fatalf("got %d calendars, want 1", len(calendars))
	}

	calResult, err := d.SynchronizeCalendar(ctx, &calendars[0], webdav.SyncState{}, nil)
	if err != nil {
		t.Fatalf("SynchronizeCalendar: %s", err)
	}
	calObjects := calResult.Updated
	if len(calObjects) != 2 || calResult.Strategy != webdav.SyncFull || calResult.State.SyncToken != "cal-1" {
		t.Fatalf("got %d calendar objects by a %v run with the state %+v", len(calObjects), calResult.Strategy, calResult.State)
	}

	components := map[string]int{}
	for i := range calObjects {
		co, err := calendarobject.FromDAV(account.Name, calendars[0].Path, calObjects[i])
		if err != nil {
			t.Fatalf("calendarobject.FromDAV: %s", err)
		}
		if err := calendarobject.Set(db, co); err != nil {
			t.Fatalf("calendarobject.Set: %s", err)
		}
		components[co.Component]++
	}
	if components["VEVENT"] != 1 || components["VTODO"] != 1 {
		t.Fatalf("expected one VEVENT and one VTODO, got %v", components)
	}

	books, err := d.FindAddressBooks(ctx)
	if err != nil {
		t.Fatalf("FindAddressBooks: %s", err)
	}
	if len(books) != 1 {
		t.Fatalf("got %d address books, want 1", len(books))
	}

	cardResult, err := d.SynchronizeAddressBook(ctx, &books[0], webdav.SyncState{}, nil)
	if err != nil {
		t.Fatalf("SynchronizeAddressBook: %s", err)
	}
	cardObjects := cardResult.Updated
	if len(cardObjects) != 1 {
		t.Fatalf("got %d address objects, want 1", len(cardObjects))
	}

	ao, err := addressobject.FromDAV(account.Name, books[0].Path, cardObjects[0])
	if err != nil {
		t.Fatalf("addressobject.FromDAV: %s", err)
	}
	if err := addressobject.Set(db, ao); err != nil {
		t.Fatalf("addressobject.Set: %s", err)
	}
	if ao.FormattedName != "Alice Example" {
		t.Fatalf("got formatted name %q, want %q", ao.FormattedName, "Alice Example")
	}

	storedCal, err := calendarobject.List(db)
	if err != nil {
		t.Fatalf("calendarobject.List: %s", err)
	}
	if len(storedCal) != 2 {
		t.Fatalf("stored %d calendar objects, want 2", len(storedCal))
	}

	storedCards, err := addressobject.List(db)
	if err != nil {
		t.Fatalf("addressobject.List: %s", err)
	}
	if len(storedCards) != 1 {
		t.Fatalf("stored %d address objects, want 1", len(storedCards))
	}
}

func TestIncrementalSync(t *testing.T) {
	const calPath = "/dav-principal/cal/personal"
	const cardPath = "/dav-principal/card/personal"

	event := caldav.CalendarObject{
		Path: calPath + "/event.ics",
		ETag: "e1",
		Data: makeCalendarData("event-1", ical.CompEvent, "Lunch"),
	}
	task := caldav.CalendarObject{
		Path: calPath + "/task.ics",
		ETag: "t1",
		Data: makeCalendarData("task-1", ical.CompToDo, "Buy milk"),
	}
	event2 := caldav.CalendarObject{
		Path: calPath + "/event2.ics",
		ETag: "e2",
		Data: makeCalendarData("event-2", ical.CompEvent, "Dentist"),
	}

	calBackend := &calTestBackend{
		principal: "/dav-principal",
		homeSet:   "/dav-principal/cal",
		calendars: []caldav.Calendar{{Path: calPath, Name: "Personal"}},
		objectsByPath: map[string]caldav.CalendarObject{
			event.Path: event, task.Path: task, event2.Path: event2,
		},
		syncScript: map[string]*caldav.SyncResponse{
			"": {
				SyncToken: "cal-1",
				Updated:   []caldav.CalendarObject{event, task},
			},
			"cal-1": {
				SyncToken: "cal-2",
				Updated:   []caldav.CalendarObject{event2},
				Deleted:   []string{task.Path},
			},
		},
	}

	alice := carddav.AddressObject{
		Path: cardPath + "/alice.vcf",
		ETag: "a1",
		Card: makeCard("contact-1", "Alice Example"),
	}
	aliceUpdated := carddav.AddressObject{
		Path: cardPath + "/alice.vcf",
		ETag: "a2",
		Card: makeCard("contact-1", "Alice Updated"),
	}

	cardBackend := &cardTestBackend{
		principal:    "/dav-principal",
		homeSet:      "/dav-principal/card",
		addressBooks: []carddav.AddressBook{{Path: cardPath, Name: "Contacts"}},
		syncScript: map[string]*carddav.SyncResponse{
			"":       {SyncToken: "card-1", Updated: []carddav.AddressObject{alice}},
			"card-1": {SyncToken: "card-2", Updated: []carddav.AddressObject{aliceUpdated}},
		},
	}

	calServer := httptest.NewServer(&caldav.Handler{Backend: calBackend})
	defer calServer.Close()
	cardServer := httptest.NewServer(&carddav.Handler{Backend: cardBackend})
	defer cardServer.Close()

	d, err := New(config.Account{
		Name:            "test",
		Username:        "alice",
		Password:        "secret",
		CalDAVEndpoint:  calServer.URL,
		CardDAVEndpoint: cardServer.URL,
	}, &Options{UserAgent: "inca/test"})
	if err != nil {
		t.Fatalf("dav.New: %s", err)
	}
	ctx := context.Background()

	calendars, err := d.FindCalendars(ctx)
	if err != nil || len(calendars) != 1 {
		t.Fatalf("FindCalendars: %d, %v", len(calendars), err)
	}

	first, err := d.SynchronizeCalendar(ctx, &calendars[0], webdav.SyncState{}, nil)
	if err != nil {
		t.Fatalf("SynchronizeCalendar initial: %s", err)
	}
	if first.State.SyncToken != "cal-1" || first.Strategy != webdav.SyncFull {
		t.Fatalf("initial: token %q by a %v run, want cal-1 by a full one", first.State.SyncToken, first.Strategy)
	}
	if len(first.Updated) != 2 || len(first.Deleted) != 0 {
		t.Fatalf("initial: %d updated, %d deleted; want 2, 0",
			len(first.Updated), len(first.Deleted))
	}

	etags := map[string]string{}
	for i := range first.Updated {
		etags[first.Updated[i].Path] = first.Updated[i].ETag
	}

	second, err := d.SynchronizeCalendar(ctx, &calendars[0], first.State, etags)
	if err != nil {
		t.Fatalf("SynchronizeCalendar incremental: %s", err)
	}
	if second.State.SyncToken != "cal-2" || second.Strategy != webdav.SyncIncremental {
		t.Fatalf("incremental: token %q by a %v run, want cal-2 by an incremental one", second.State.SyncToken, second.Strategy)
	}
	if len(second.Updated) != 1 || second.Updated[0].Path != event2.Path {
		t.Fatalf("incremental updated = %+v, want only event2", second.Updated)
	}
	if len(second.Deleted) != 1 || second.Deleted[0] != task.Path {
		t.Fatalf("incremental deleted = %v, want [%s]", second.Deleted, task.Path)
	}
	if data, err := second.Updated[0].Decoded(); err != nil || data == nil {
		t.Fatalf("incremental updated object has no data: %v", err)
	}

	books, err := d.FindAddressBooks(ctx)
	if err != nil || len(books) != 1 {
		t.Fatalf("FindAddressBooks: %d, %v", len(books), err)
	}

	firstCard, err := d.SynchronizeAddressBook(ctx, &books[0], webdav.SyncState{}, nil)
	if err != nil {
		t.Fatalf("SynchronizeAddressBook initial: %s", err)
	}
	if len(firstCard.Updated) != 1 {
		t.Fatalf("initial card sync returned %d cards, want 1", len(firstCard.Updated))
	}
	if got := formattedName(t, &firstCard.Updated[0]); got != "Alice Example" {
		t.Fatalf("initial card FN = %q, want Alice Example", got)
	}

	secondCard, err := d.SynchronizeAddressBook(ctx, &books[0], firstCard.State, map[string]string{alice.Path: alice.ETag})
	if err != nil {
		t.Fatalf("SynchronizeAddressBook incremental: %s", err)
	}
	if secondCard.State.SyncToken != "card-2" {
		t.Fatalf("incremental card token = %q, want card-2", secondCard.State.SyncToken)
	}
	if len(secondCard.Updated) != 1 {
		t.Fatalf("incremental card sync returned %d cards, want 1", len(secondCard.Updated))
	}
	if got := formattedName(t, &secondCard.Updated[0]); got != "Alice Updated" {
		t.Fatalf("incremental card FN = %q, want Alice Updated", got)
	}
}

const principalOnly = `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:">
  <response>
    <href>/</href>
    <propstat>
      <prop><current-user-principal><href>/card/</href></current-user-principal></prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>
`

const syncWithoutAddressData = `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <response>
    <href>/card/contacts/alice.vcf</href>
    <propstat>
      <prop><getetag>"a1"</getetag></prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
    <propstat>
      <prop><C:address-data/></prop>
      <status>HTTP/1.1 404 Not Found</status>
    </propstat>
  </response>
  <sync-token>card-1</sync-token>
</multistatus>
`

const multigetWithAddressData = `<?xml version="1.0" encoding="UTF-8"?>
<multistatus xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <response>
    <href>/card/contacts/alice.vcf</href>
    <propstat>
      <prop>
        <getetag>"a1"</getetag>
        <C:address-data>BEGIN:VCARD
VERSION:3.0
UID:contact-1
FN:Alice Example
END:VCARD
</C:address-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>
`

func TestSyncAddressBookFetchesMissingCards(t *testing.T) {
	var reports []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusMultiStatus)
		switch {
		case r.Method == "PROPFIND":
			io.WriteString(w, principalOnly)
		case strings.Contains(string(body), "sync-collection"):
			reports = append(reports, "sync-collection")
			io.WriteString(w, syncWithoutAddressData)
		case strings.Contains(string(body), "addressbook-multiget"):
			reports = append(reports, "addressbook-multiget")
			io.WriteString(w, multigetWithAddressData)
		}
	}))
	defer server.Close()

	d, err := New(config.Account{
		Name:            "test",
		Username:        "alice",
		Password:        "secret",
		CardDAVEndpoint: server.URL,
	}, &Options{UserAgent: "inca/test"})
	if err != nil {
		t.Fatalf("dav.New: %s", err)
	}

	result, err := d.SynchronizeAddressBook(context.Background(), &carddav.AddressBook{
		Path:             "/card/contacts/",
		SupportedReports: []xml.Name{webdav.SyncCollectionName},
	}, webdav.SyncState{}, nil)
	if err != nil {
		t.Fatalf("SynchronizeAddressBook: %s", err)
	}

	if got := strings.Join(reports, ","); got != "sync-collection,addressbook-multiget" {
		t.Fatalf("reports = %q, want sync-collection,addressbook-multiget", got)
	}
	if result.State.SyncToken != "card-1" {
		t.Fatalf("token = %q, want card-1", result.State.SyncToken)
	}
	if len(result.Updated) != 1 {
		t.Fatalf("sync returned %d cards, want 1", len(result.Updated))
	}
	if got := formattedName(t, &result.Updated[0]); got != "Alice Example" {
		t.Fatalf("card FN = %q, want Alice Example", got)
	}
}
