package dav

import (
	"context"
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

func makeCard(uid, fn string) vcard.Card {
	card := make(vcard.Card)
	card.SetValue(vcard.FieldVersion, "3.0")
	card.SetValue(vcard.FieldUID, uid)
	card.SetValue(vcard.FieldFormattedName, fn)
	return card
}

func TestSyncRoundTrip(t *testing.T) {
	calBackend := &calTestBackend{
		principal: "/dav-principal",
		homeSet:   "/dav-principal/cal",
		calendars: []caldav.Calendar{{
			Path:                  "/dav-principal/cal/personal",
			Name:                  "Personal",
			SupportedComponentSet: []string{"VEVENT", "VTODO"},
		}},
		objects: map[string][]caldav.CalendarObject{
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
		},
	}
	cardBackend := &cardTestBackend{
		principal: "/dav-principal",
		homeSet:   "/dav-principal/card",
		addressBooks: []carddav.AddressBook{{
			Path: "/dav-principal/card/personal",
			Name: "Contacts",
		}},
		objects: map[string][]carddav.AddressObject{
			"/dav-principal/card/personal": {
				{
					Path: "/dav-principal/card/personal/alice.vcf",
					ETag: "etag-alice",
					Card: makeCard("contact-1", "Alice Example"),
				},
			},
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

	d, err := New(account)
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

	calObjects, err := d.QueryCalendarObjects(ctx, calendars[0].Path)
	if err != nil {
		t.Fatalf("QueryCalendarObjects: %s", err)
	}
	if len(calObjects) != 2 {
		t.Fatalf("got %d calendar objects, want 2", len(calObjects))
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

	cardObjects, err := d.QueryAddressObjects(ctx, books[0].Path)
	if err != nil {
		t.Fatalf("QueryAddressObjects: %s", err)
	}
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
	})
	if err != nil {
		t.Fatalf("dav.New: %s", err)
	}
	ctx := context.Background()

	first, err := d.SyncCalendar(ctx, calPath, "")
	if err != nil {
		t.Fatalf("SyncCalendar initial: %s", err)
	}
	if first.SyncToken != "cal-1" {
		t.Fatalf("initial token = %q, want cal-1", first.SyncToken)
	}
	if len(first.Updated) != 2 || len(first.Deleted) != 0 {
		t.Fatalf("initial: %d updated, %d deleted; want 2, 0",
			len(first.Updated), len(first.Deleted))
	}

	second, err := d.SyncCalendar(ctx, calPath, first.SyncToken)
	if err != nil {
		t.Fatalf("SyncCalendar incremental: %s", err)
	}
	if second.SyncToken != "cal-2" {
		t.Fatalf("incremental token = %q, want cal-2", second.SyncToken)
	}
	if len(second.Updated) != 1 || second.Updated[0].Path != event2.Path {
		t.Fatalf("incremental updated = %+v, want only event2", second.Updated)
	}
	if len(second.Deleted) != 1 || second.Deleted[0] != task.Path {
		t.Fatalf("incremental deleted = %v, want [%s]", second.Deleted, task.Path)
	}
	if second.Updated[0].Data == nil {
		t.Fatal("incremental updated object has no data")
	}

	firstCard, err := d.SyncAddressBook(ctx, cardPath, "")
	if err != nil {
		t.Fatalf("SyncAddressBook initial: %s", err)
	}
	if len(firstCard.Updated) != 1 || firstCard.Updated[0].Card == nil {
		t.Fatalf("initial card sync did not return card data: %+v", firstCard.Updated)
	}
	if got := firstCard.Updated[0].Card.Value(vcard.FieldFormattedName); got != "Alice Example" {
		t.Fatalf("initial card FN = %q, want Alice Example", got)
	}

	secondCard, err := d.SyncAddressBook(ctx, cardPath, firstCard.SyncToken)
	if err != nil {
		t.Fatalf("SyncAddressBook incremental: %s", err)
	}
	if secondCard.SyncToken != "card-2" {
		t.Fatalf("incremental card token = %q, want card-2", secondCard.SyncToken)
	}
	if len(secondCard.Updated) != 1 || secondCard.Updated[0].Card == nil {
		t.Fatalf("incremental card sync missing data: %+v", secondCard.Updated)
	}
	if got := secondCard.Updated[0].Card.Value(vcard.FieldFormattedName); got != "Alice Updated" {
		t.Fatalf("incremental card FN = %q, want Alice Updated", got)
	}
}

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
	})
	if err != nil {
		t.Fatalf("dav.New: %s", err)
	}

	result, err := d.SyncAddressBook(context.Background(), "/card/contacts/", "")
	if err != nil {
		t.Fatalf("SyncAddressBook: %s", err)
	}

	if got := strings.Join(reports, ","); got != "sync-collection,addressbook-multiget" {
		t.Fatalf("reports = %q, want sync-collection,addressbook-multiget", got)
	}
	if result.SyncToken != "card-1" {
		t.Fatalf("token = %q, want card-1", result.SyncToken)
	}
	if len(result.Updated) != 1 || result.Updated[0].Card == nil {
		t.Fatalf("sync did not return card data: %+v", result.Updated)
	}
	if got := result.Updated[0].Card.Value(vcard.FieldFormattedName); got != "Alice Example" {
		t.Fatalf("card FN = %q, want Alice Example", got)
	}
}
