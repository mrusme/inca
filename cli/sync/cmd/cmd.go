package syncCmd

import (
	"context"

	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/dav"
	"xn--gckvb8fzb.com/inca/errs"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/models/addressbook"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/models/calendar"
	"xn--gckvb8fzb.com/inca/models/calendarobject"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/runtime"
)

var Cmd = &cobra.Command{
	Use:     "sync",
	Aliases: []string{"synchronize", "refresh", "pull"},
	Short:   "inca sync",
	Long: "Synchronize the calendars, tasks and contacts of every configured " +
		"account into the local database.",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rt := runtime.New(
			runtime.GetConfigStr(cmd),
			runtime.GetLogLevel(cmd),
			runtime.GetOutputColor(cmd),
			false,
		)
		defer rt.End()

		accounts, err := rt.Config.Accounts()
		rt.NilOrDie(err)

		if len(accounts) == 0 {
			rt.NilOrDie(errs.ErrNoAccounts)
		}

		ctx := context.Background()
		for i := range accounts {
			syncAccount(rt, ctx, accounts[i])
		}
	},
}

func syncAccount(
	rt *runtime.Runtime,
	ctx context.Context,
	account config.Account,
) {
	if account.Name == "" {
		rt.Out.Put(out.Opts{Type: out.Warn},
			"Skipping an account without a name")
		return
	}
	if account.Endpoint == "" &&
		account.CalDAVEndpoint == "" &&
		account.CardDAVEndpoint == "" {
		rt.Out.Put(out.Opts{Type: out.Warn},
			"Skipping account %s without an endpoint",
			rt.Out.FG(out.ColorPrimary, "%s", account.Name))
		return
	}

	rt.Out.Put(out.Opts{Type: out.Sync},
		"Syncing account %s ...",
		rt.Out.FG(out.ColorPrimary, "%s", account.Name))

	d, err := dav.New(account)
	if err != nil {
		rt.Out.Put(out.Opts{Type: out.Error},
			"Could not set up account %s: %s", account.Name, err.Error())
		return
	}

	if d.HasCalDAV() {
		syncCalendars(rt, ctx, d, account)
	}
	if d.HasCardDAV() {
		syncAddressBooks(rt, ctx, d, account)
	}
}

func syncCalendars(
	rt *runtime.Runtime,
	ctx context.Context,
	d *dav.DAV,
	account config.Account,
) {
	calendars, err := d.FindCalendars(ctx)
	if err != nil {
		rt.Out.Put(out.Opts{Type: out.Warn},
			"No calendars for account %s: %s", account.Name, err.Error())
		return
	}

	for i := range calendars {
		c := calendars[i]

		cal := calendar.FromDAV(account.Name, c)

		prevToken := ""
		if prev, perr := calendar.Get(rt.Database, cal.GetKey()); perr == nil {
			prevToken = prev.SyncToken
		}

		updated, deleted, newToken, incremental, err := collectCalendarChanges(rt, ctx, d, c.Path, prevToken)
		if err != nil {
			rt.Out.Put(out.Opts{Type: out.Warn},
				"Could not read calendar %s: %s", displayName(c.Name, c.Path),
				err.Error())
			continue
		}

		var stored int
		for j := range updated {
			co, err := calendarobject.FromDAV(account.Name, c.Path, updated[j])
			if err != nil {
				rt.Logger.Warningf("Skipping calendar object %s: %s",
					updated[j].Path, err.Error())
				continue
			}
			if err := calendarobject.Set(rt.Database, co); err != nil {
				rt.Logger.Warningf("Could not store calendar object %s: %s",
					updated[j].Path, err.Error())
				continue
			}
			stored++
		}

		var removed int
		for _, path := range deleted {
			if err := calendarobject.DeleteByPath(
				rt.Database, account.Name, path); err != nil {
				rt.Logger.Warningf("Could not delete calendar object %s: %s",
					path, err.Error())
				continue
			}
			removed++
		}

		cal.SyncToken = newToken
		if err := calendar.Set(rt.Database, cal); err != nil {
			rt.Out.Put(out.Opts{Type: out.Error},
				"Could not store calendar %s: %s", c.Path, err.Error())
			continue
		}

		reportCollection(rt, "Calendar", displayName(c.Name, c.Path),
			stored, removed, incremental)
	}
}

func collectCalendarChanges(
	rt *runtime.Runtime,
	ctx context.Context,
	d *dav.DAV,
	path string,
	prevToken string,
) (updated []dav.CalendarObject, deleted []string, newToken string, incremental bool, err error) {
	res, err := d.SyncCalendar(ctx, path, prevToken)
	if err != nil && prevToken != "" {
		rt.Logger.Debugf("Calendar %s: sync token rejected (%s), syncing fresh",
			path, err.Error())
		res, err = d.SyncCalendar(ctx, path, "")
	}
	if err != nil {
		rt.Logger.Debugf("Calendar %s: sync-collection unavailable (%s), "+
			"falling back to a full query", path, err.Error())
		objects, qerr := d.QueryCalendarObjects(ctx, path)
		if qerr != nil {
			return nil, nil, "", false, qerr
		}
		return objects, nil, "", false, nil
	}
	return res.Updated, res.Deleted, res.SyncToken, true, nil
}

func syncAddressBooks(
	rt *runtime.Runtime,
	ctx context.Context,
	d *dav.DAV,
	account config.Account,
) {
	books, err := d.FindAddressBooks(ctx)
	if err != nil {
		rt.Out.Put(out.Opts{Type: out.Warn},
			"No address books for account %s: %s", account.Name, err.Error())
		return
	}

	for i := range books {
		ab := books[i]

		book := addressbook.FromDAV(account.Name, ab)

		prevToken := ""
		if prev, perr := addressbook.Get(rt.Database, book.GetKey()); perr == nil {
			prevToken = prev.SyncToken
		}

		updated, deleted, newToken, incremental, err := collectAddressChanges(rt, ctx, d, ab.Path, prevToken)
		if err != nil {
			rt.Out.Put(out.Opts{Type: out.Warn},
				"Could not read address book %s: %s",
				displayName(ab.Name, ab.Path), err.Error())
			continue
		}

		var stored int
		for j := range updated {
			ao, err := addressobject.FromDAV(account.Name, ab.Path, updated[j])
			if err != nil {
				rt.Logger.Warningf("Skipping address object %s: %s",
					updated[j].Path, err.Error())
				continue
			}
			if err := addressobject.Set(rt.Database, ao); err != nil {
				rt.Logger.Warningf("Could not store address object %s: %s",
					updated[j].Path, err.Error())
				continue
			}
			stored++
		}

		var removed int
		for _, path := range deleted {
			if err := addressobject.DeleteByPath(
				rt.Database, account.Name, path); err != nil {
				rt.Logger.Warningf("Could not delete address object %s: %s",
					path, err.Error())
				continue
			}
			removed++
		}

		book.SyncToken = newToken
		if err := addressbook.Set(rt.Database, book); err != nil {
			rt.Out.Put(out.Opts{Type: out.Error},
				"Could not store address book %s: %s", ab.Path, err.Error())
			continue
		}

		reportCollection(rt, "Address book", displayName(ab.Name, ab.Path),
			stored, removed, incremental)
	}
}

func collectAddressChanges(
	rt *runtime.Runtime,
	ctx context.Context,
	d *dav.DAV,
	path string,
	prevToken string,
) (updated []dav.AddressObject, deleted []string, newToken string, incremental bool, err error) {
	res, err := d.SyncAddressBook(ctx, path, prevToken)
	if err != nil && prevToken != "" {
		rt.Logger.Debugf("Address book %s: sync token rejected (%s), syncing fresh",
			path, err.Error())
		res, err = d.SyncAddressBook(ctx, path, "")
	}
	if err != nil {
		rt.Logger.Debugf("Address book %s: sync-collection unavailable (%s), "+
			"falling back to a full query", path, err.Error())
		objects, qerr := d.QueryAddressObjects(ctx, path)
		if qerr != nil {
			return nil, nil, "", false, qerr
		}
		return objects, nil, "", false, nil
	}
	return res.Updated, res.Deleted, res.SyncToken, true, nil
}

func reportCollection(
	rt *runtime.Runtime,
	kind string,
	name string,
	stored int,
	removed int,
	incremental bool,
) {
	mode := "full"
	if incremental {
		mode = "sync"
	}

	rt.Out.Put(out.Opts{Type: out.Ok},
		"%s %s: %s updated, %s removed %s",
		kind,
		rt.Out.FG(out.ColorPrimary, "%s", name),
		rt.Out.FG(out.ColorCyan, "%d", stored),
		rt.Out.FG(out.ColorCyan, "%d", removed),
		rt.Out.FG(out.ColorSecondary, "(%s)", mode))
}

func displayName(name string, path string) string {
	if name != "" {
		return name
	}
	return path
}

func init() {
}
