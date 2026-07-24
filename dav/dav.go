package dav

import (
	"context"
	"fmt"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
	"xn--gckvb8fzb.com/inca/models/config"
)

const maxSyncPages = 1000

type (
	CalendarObject = caldav.CalendarObject
	AddressObject  = carddav.AddressObject
)

type DAV struct {
	account    config.Account
	httpClient webdav.HTTPClient
	calClient  *caldav.Client
	cardClient *carddav.Client
}

func New(account config.Account) (*DAV, error) {
	d := new(DAV)
	d.account = account
	d.httpClient = webdav.HTTPClientWithBasicAuth(
		newRedirectFollower(),
		account.Username,
		account.Password,
	)

	calEndpoint := firstNonEmpty(account.CalDAVEndpoint, account.Endpoint)
	cardEndpoint := firstNonEmpty(account.CardDAVEndpoint, account.Endpoint)

	var err error
	if calEndpoint != "" {
		if d.calClient, err = caldav.NewClient(
			d.httpClient,
			calEndpoint,
		); err != nil {
			return nil, err
		}
	}
	if cardEndpoint != "" {
		if d.cardClient, err = carddav.NewClient(
			d.httpClient,
			cardEndpoint,
		); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *DAV) HasCalDAV() bool {
	return d.calClient != nil
}

func (d *DAV) HasCardDAV() bool {
	return d.cardClient != nil
}

func (d *DAV) FindCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	principal := d.calPrincipal(ctx)

	homeSet, err := d.calClient.FindCalendarHomeSet(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("finding calendar home set: %w", err)
	}

	return d.calClient.FindCalendars(ctx, homeSet)
}

func (d *DAV) calPrincipal(ctx context.Context) string {
	if client, err := d.discoverCalClient(ctx); err == nil {
		if principal, perr := client.FindCurrentUserPrincipal(ctx); perr == nil && principal != "" {
			d.calClient = client
			return principal
		}
	}
	if principal, err := d.calClient.FindCurrentUserPrincipal(ctx); err == nil && principal != "" {
		return principal
	}
	return d.principalFallback()
}

func (d *DAV) QueryCalendarObjects(
	ctx context.Context,
	path string,
) ([]caldav.CalendarObject, error) {
	query := &caldav.CalendarQuery{
		CompFilter: caldav.CompFilter{Name: ical.CompCalendar},
	}
	return d.calClient.QueryCalendar(ctx, path, query)
}

func (d *DAV) FindAddressBooks(ctx context.Context) ([]carddav.AddressBook, error) {
	principal := d.cardPrincipal(ctx)

	homeSet, err := d.cardClient.FindAddressBookHomeSet(ctx, principal)
	if err != nil {
		return nil, fmt.Errorf("finding address book home set: %w", err)
	}

	return d.cardClient.FindAddressBooks(ctx, homeSet)
}

func (d *DAV) cardPrincipal(ctx context.Context) string {
	if client, err := d.discoverCardClient(ctx); err == nil {
		if principal, perr := client.FindCurrentUserPrincipal(ctx); perr == nil && principal != "" {
			d.cardClient = client
			return principal
		}
	}
	if principal, err := d.cardClient.FindCurrentUserPrincipal(ctx); err == nil && principal != "" {
		return principal
	}
	return d.principalFallback()
}

func (d *DAV) QueryAddressObjects(
	ctx context.Context,
	path string,
) ([]carddav.AddressObject, error) {
	query := &carddav.AddressBookQuery{
		DataRequest: carddav.AddressDataRequest{AllProp: true},
	}
	return d.cardClient.QueryAddressBook(ctx, path, query)
}

type CalendarSyncResult struct {
	SyncToken string
	Updated   []caldav.CalendarObject
	Deleted   []string
}

func (d *DAV) SyncCalendar(
	ctx context.Context,
	path string,
	syncToken string,
) (*CalendarSyncResult, error) {
	result := &CalendarSyncResult{SyncToken: syncToken}

	for i := 0; i < maxSyncPages; i++ {
		resp, err := d.calClient.SyncCollection(ctx, path, &caldav.SyncQuery{
			SyncToken: result.SyncToken,
		})
		if err != nil {
			return nil, err
		}

		result.Updated = append(result.Updated, resp.Updated...)
		result.Deleted = append(result.Deleted, resp.Deleted...)

		previous := result.SyncToken
		result.SyncToken = resp.SyncToken
		if !resp.Truncated || resp.SyncToken == previous {
			break
		}
	}

	var missing []string
	for i := range result.Updated {
		if result.Updated[i].Data == nil {
			missing = append(missing, result.Updated[i].Path)
		}
	}
	if len(missing) > 0 {
		objects, err := d.calClient.MultiGetCalendar(ctx, path, &caldav.CalendarMultiGet{
			Paths: missing,
		})
		if err != nil {
			return nil, err
		}
		byPath := make(map[string]*ical.Calendar, len(objects))
		for i := range objects {
			byPath[objects[i].Path] = objects[i].Data
		}
		for i := range result.Updated {
			if result.Updated[i].Data == nil {
				result.Updated[i].Data = byPath[result.Updated[i].Path]
			}
		}
	}

	return result, nil
}

type AddressBookSyncResult struct {
	SyncToken string
	Updated   []carddav.AddressObject
	Deleted   []string
}

func (d *DAV) SyncAddressBook(
	ctx context.Context,
	path string,
	syncToken string,
) (*AddressBookSyncResult, error) {
	result := &AddressBookSyncResult{SyncToken: syncToken}

	for i := 0; i < maxSyncPages; i++ {
		resp, err := d.cardClient.SyncCollection(ctx, path, &carddav.SyncQuery{
			DataRequest: carddav.AddressDataRequest{AllProp: true},
			SyncToken:   result.SyncToken,
		})
		if err != nil {
			return nil, err
		}

		result.Updated = append(result.Updated, resp.Updated...)
		result.Deleted = append(result.Deleted, resp.Deleted...)

		previous := result.SyncToken
		result.SyncToken = resp.SyncToken
		if !resp.Truncated || resp.SyncToken == previous {
			break
		}
	}

	if len(result.Updated) > 0 {
		paths := make([]string, len(result.Updated))
		for i := range result.Updated {
			paths[i] = result.Updated[i].Path
		}

		objects, err := d.cardClient.MultiGetAddressBook(ctx, path, &carddav.AddressBookMultiGet{
			Paths:       paths,
			DataRequest: carddav.AddressDataRequest{AllProp: true},
		})
		if err != nil {
			return nil, err
		}
		byPath := make(map[string]vcard.Card, len(objects))
		for i := range objects {
			byPath[objects[i].Path] = objects[i].Card
		}
		for i := range result.Updated {
			result.Updated[i].Card = byPath[result.Updated[i].Path]
		}
	}

	return result, nil
}

func (d *DAV) discoverCalClient(ctx context.Context) (*caldav.Client, error) {
	domain := domainFromUsername(d.account.Username)
	if domain == "" {
		return nil, fmt.Errorf("no domain to discover the caldav service from")
	}
	endpoint, err := caldav.DiscoverContextURL(ctx, domain)
	if err != nil {
		return nil, err
	}
	return caldav.NewClient(d.httpClient, endpoint)
}

func (d *DAV) discoverCardClient(ctx context.Context) (*carddav.Client, error) {
	domain := domainFromUsername(d.account.Username)
	if domain == "" {
		return nil, fmt.Errorf("no domain to discover the carddav service from")
	}
	endpoint, err := carddav.DiscoverContextURL(ctx, domain)
	if err != nil {
		return nil, err
	}
	return carddav.NewClient(d.httpClient, endpoint)
}

func domainFromUsername(username string) string {
	if i := strings.LastIndex(username, "@"); i >= 0 {
		return username[i+1:]
	}
	return ""
}

func (d *DAV) principalFallback() string {
	if d.account.Username == "" {
		return "/"
	}
	return fmt.Sprintf("/principals/%s/", d.account.Username)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
