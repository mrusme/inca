package dav

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/models/service"
	"xn--gckvb8fzb.com/maya/libs/webdav"
	"xn--gckvb8fzb.com/maya/libs/webdav/caldav"
	"xn--gckvb8fzb.com/maya/libs/webdav/carddav"
)

type (
	CalendarObject = caldav.CalendarObject
	AddressObject  = carddav.AddressObject
)

type Options struct {
	UserAgent  string
	TrustHost  func(webdav.TrustRequest) bool
	CalDAV     *service.Service
	CardDAV    *service.Service
	OnDiscover func(found *service.Service)
}

type discoverFunc func(
	ctx context.Context,
	c webdav.HTTPClient,
	input string,
	opts *webdav.DiscoverOptions,
) (*webdav.Discovery, error)

type connection struct {
	protocol string
	start    string
	stored   *service.Service
	discover discoverFunc

	current   *service.Service
	fromStore bool
}

type DAV struct {
	account    config.Account
	opts       *Options
	httpClient webdav.HTTPClient

	cal        connection
	calClient  *caldav.Client
	card       connection
	cardClient *carddav.Client
}

func New(account config.Account, opts *Options) (*DAV, error) {
	if opts == nil {
		opts = new(Options)
	}

	d := new(DAV)
	d.account = account
	d.opts = opts
	d.httpClient = webdav.HTTPClientWithBasicAuth(
		webdav.NewHTTPClient(&webdav.HTTPClientOptions{UserAgent: opts.UserAgent}),
		account.Username,
		account.Password,
	)

	d.cal = connection{
		protocol: service.CalDAV,
		start:    firstNonEmpty(account.CalDAVEndpoint, account.Endpoint, addressOf(account.Username)),
		stored:   opts.CalDAV,
		discover: caldav.Discover,
	}
	d.card = connection{
		protocol: service.CardDAV,
		start:    firstNonEmpty(account.CardDAVEndpoint, account.Endpoint, addressOf(account.Username)),
		stored:   opts.CardDAV,
		discover: carddav.Discover,
	}

	return d, nil
}

func (d *DAV) HasCalDAV() bool {
	return d.cal.start != ""
}

func (d *DAV) HasCardDAV() bool {
	return d.card.start != ""
}

func (d *DAV) clientOptions() *webdav.ClientOptions {
	return &webdav.ClientOptions{TrustHost: d.opts.TrustHost}
}

func (d *DAV) locate(ctx context.Context, conn *connection, again bool) error {
	if !again && conn.stored != nil && conn.stored.Start == conn.start {
		conn.current, conn.fromStore = conn.stored, true
		return nil
	}

	found, err := conn.discover(ctx, d.httpClient, conn.start, &webdav.DiscoverOptions{TrustHost: d.opts.TrustHost})
	if err != nil {
		return err
	}

	current := service.New(d.account.Name, conn.protocol)
	current.Start, current.Endpoint, current.Principal = conn.start, found.Endpoint, found.Principal
	conn.current, conn.fromStore = current, false

	if d.opts.OnDiscover != nil {
		d.opts.OnDiscover(current)
	}
	return nil
}

func hasMoved(err error) bool {
	var httpErr *webdav.HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	switch httpErr.Code {
	case http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusGone:
		return true
	}
	return false
}

func (d *DAV) connectCalDAV(ctx context.Context, again bool) error {
	if d.calClient != nil && !again {
		return nil
	}

	if err := d.locate(ctx, &d.cal, again); err != nil {
		return fmt.Errorf("finding the calendar service: %w", err)
	}
	client, err := caldav.NewClient(d.httpClient, d.cal.current.Endpoint, d.clientOptions())
	if err != nil {
		return err
	}

	d.calClient = client
	return nil
}

func (d *DAV) connectCardDAV(ctx context.Context, again bool) error {
	if d.cardClient != nil && !again {
		return nil
	}

	if err := d.locate(ctx, &d.card, again); err != nil {
		return fmt.Errorf("finding the address book service: %w", err)
	}
	client, err := carddav.NewClient(d.httpClient, d.card.current.Endpoint, d.clientOptions())
	if err != nil {
		return err
	}

	d.cardClient = client
	return nil
}

func (d *DAV) Discover(ctx context.Context, protocol string) (*service.Service, error) {
	switch protocol {
	case service.CalDAV:
		if err := d.connectCalDAV(ctx, true); err != nil {
			return nil, err
		}
		return d.cal.current, nil
	case service.CardDAV:
		if err := d.connectCardDAV(ctx, true); err != nil {
			return nil, err
		}
		return d.card.current, nil
	}
	return nil, fmt.Errorf("%s isn't a protocol Inca discovers", protocol)
}

func (d *DAV) FindCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	if err := d.connectCalDAV(ctx, false); err != nil {
		return nil, err
	}

	homeSets, err := d.calClient.FindCalendarHomeSets(ctx, d.cal.current.Principal)
	if d.cal.fromStore && hasMoved(err) {
		if err = d.connectCalDAV(ctx, true); err != nil {
			return nil, err
		}
		homeSets, err = d.calClient.FindCalendarHomeSets(ctx, d.cal.current.Principal)
	}
	if err != nil {
		return nil, fmt.Errorf("finding calendar home set: %w", err)
	}

	var calendars []caldav.Calendar
	for _, homeSet := range homeSets {
		found, err := d.calClient.FindCalendars(ctx, homeSet)
		if err != nil {
			return nil, err
		}
		calendars = append(calendars, found...)
	}
	return calendars, nil
}

func (d *DAV) FindAddressBooks(ctx context.Context) ([]carddav.AddressBook, error) {
	if err := d.connectCardDAV(ctx, false); err != nil {
		return nil, err
	}

	homeSets, err := d.cardClient.FindAddressBookHomeSets(ctx, d.card.current.Principal)
	if d.card.fromStore && hasMoved(err) {
		if err = d.connectCardDAV(ctx, true); err != nil {
			return nil, err
		}
		homeSets, err = d.cardClient.FindAddressBookHomeSets(ctx, d.card.current.Principal)
	}
	if err != nil {
		return nil, fmt.Errorf("finding address book home set: %w", err)
	}

	var addressBooks []carddav.AddressBook
	for _, homeSet := range homeSets {
		found, err := d.cardClient.FindAddressBooks(ctx, homeSet)
		if err != nil {
			return nil, err
		}
		addressBooks = append(addressBooks, found...)
	}
	return addressBooks, nil
}

func (d *DAV) SynchronizeCalendar(
	ctx context.Context,
	calendar *caldav.Calendar,
	state webdav.SyncState,
	etags map[string]string,
) (*caldav.Synchronization, error) {
	if err := d.connectCalDAV(ctx, false); err != nil {
		return nil, err
	}

	return d.calClient.Synchronize(ctx, calendar, &caldav.SynchronizeOptions{
		State: state,
		ETags: etags,
	})
}

func (d *DAV) SynchronizeAddressBook(
	ctx context.Context,
	addressBook *carddav.AddressBook,
	state webdav.SyncState,
	etags map[string]string,
) (*carddav.Synchronization, error) {
	if err := d.connectCardDAV(ctx, false); err != nil {
		return nil, err
	}

	return d.cardClient.Synchronize(ctx, addressBook, &carddav.SynchronizeOptions{
		State:       state,
		ETags:       etags,
		DataRequest: carddav.AddressDataRequest{AllProp: true},
	})
}

func addressOf(username string) string {
	if i := strings.LastIndex(username, "@"); i > 0 && i < len(username)-1 {
		return username
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
