package syncCmd

import (
	"context"

	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/dav"
	"xn--gckvb8fzb.com/inca/errs"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/models/addressbook"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/models/calendar"
	"xn--gckvb8fzb.com/inca/models/calendarobject"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/runtime"
	"xn--gckvb8fzb.com/maya/libs/webdav"
)

var flagTrustHost []string

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

		flags, err := shared.ParseTrustFlags(flagTrustHost, accounts)
		rt.NilOrDie(err)

		opts := &shared.Options{Flags: flags, Input: shared.Terminal()}

		ctx := context.Background()
		for i := range accounts {
			syncAccount(rt, ctx, accounts[i], opts)
		}
	},
}

func syncAccount(
	rt *runtime.Runtime,
	ctx context.Context,
	account config.Account,
	opts *shared.Options,
) {
	if account.Name == "" {
		rt.Out.Put(out.Opts{Type: out.Warn},
			"Skipping an account without a name")
		return
	}
	session, err := shared.Connect(rt, account, opts)
	if err != nil {
		rt.Out.Put(out.Opts{Type: out.Error},
			"Could not set up account %s: %s", account.Name, err.Error())
		return
	}
	d := session.DAV
	if !d.HasCalDAV() && !d.HasCardDAV() {
		rt.Out.Put(out.Opts{Type: out.Warn},
			"Skipping account %s, which has no endpoint and no address as its username",
			rt.Out.FG(out.ColorPrimary, "%s", account.Name))
		return
	}

	rt.Out.Put(out.Opts{Type: out.Sync},
		"Syncing account %s ...",
		rt.Out.FG(out.ColorPrimary, "%s", account.Name))

	if d.HasCalDAV() {
		syncCalendars(rt, ctx, d, account)
	}
	if d.HasCardDAV() {
		syncAddressBooks(rt, ctx, d, account)
	}

	if opts.Input == nil {
		for _, rejected := range session.Decider.Rejected() {
			rt.Out.Put(out.Opts{Type: out.Error}, "Account %s: %s",
				rt.Out.FG(out.ColorPrimary, "%s", account.Name),
				shared.Hint(account.Name, rejected))
		}
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

		var state webdav.SyncState
		if prev, perr := calendar.Get(rt.Database, cal.GetKey()); perr == nil {
			state = webdav.SyncState{SyncToken: prev.SyncToken, CTag: prev.CTag}
		}

		result, err := d.SynchronizeCalendar(ctx, &c, state,
			knownCalendarObjects(rt, account.Name, c.Path))
		if result == nil {
			rt.Out.Put(out.Opts{Type: out.Warn},
				"Could not read calendar %s: %s", displayName(c.Name, c.Path),
				err.Error())
			continue
		}

		var stored int
		for j := range result.Updated {
			co, err := calendarobject.FromDAV(account.Name, c.Path, result.Updated[j])
			if err != nil {
				rt.Logger.Warningf("Skipping calendar object %s: %s",
					result.Updated[j].Path, err.Error())
				continue
			}
			if err := calendarobject.Set(rt.Database, co); err != nil {
				rt.Logger.Warningf("Could not store calendar object %s: %s",
					result.Updated[j].Path, err.Error())
				continue
			}
			stored++
		}

		var removed int
		for _, path := range result.Deleted {
			if err := calendarobject.DeleteByPath(
				rt.Database, account.Name, path); err != nil {
				rt.Logger.Warningf("Could not delete calendar object %s: %s",
					path, err.Error())
				continue
			}
			removed++
		}

		cal.SyncToken, cal.CTag = result.State.SyncToken, result.State.CTag
		if err := calendar.Set(rt.Database, cal); err != nil {
			rt.Out.Put(out.Opts{Type: out.Error},
				"Could not store calendar %s: %s", c.Path, err.Error())
			continue
		}

		reportCollection(rt, "Calendar", displayName(c.Name, c.Path),
			stored, removed, result.Strategy, err)
	}
}

func knownCalendarObjects(
	rt *runtime.Runtime,
	accountName string,
	calendarPath string,
) map[string]string {
	local, err := calendarobject.List(rt.Database)
	if err != nil {
		rt.Logger.Warningf("Could not list the calendar objects of %s: %s", calendarPath, err.Error())
		return nil
	}

	known := make(map[string]string)
	for _, co := range local {
		if co.AccountName == accountName && co.CalendarPath == calendarPath {
			known[co.Path] = co.ETag
		}
	}

	return known
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

		var state webdav.SyncState
		if prev, perr := addressbook.Get(rt.Database, book.GetKey()); perr == nil {
			state = webdav.SyncState{SyncToken: prev.SyncToken, CTag: prev.CTag}
		}

		result, err := d.SynchronizeAddressBook(ctx, &ab, state,
			knownAddressObjects(rt, account.Name, ab.Path))
		if result == nil {
			rt.Out.Put(out.Opts{Type: out.Warn},
				"Could not read address book %s: %s",
				displayName(ab.Name, ab.Path), err.Error())
			continue
		}

		var stored int
		for j := range result.Updated {
			ao, err := addressobject.FromDAV(account.Name, ab.Path, result.Updated[j])
			if err != nil {
				rt.Logger.Warningf("Skipping address object %s: %s",
					result.Updated[j].Path, err.Error())
				continue
			}
			if err := addressobject.Set(rt.Database, ao); err != nil {
				rt.Logger.Warningf("Could not store address object %s: %s",
					result.Updated[j].Path, err.Error())
				continue
			}
			stored++
		}

		var removed int
		for _, path := range result.Deleted {
			if err := addressobject.DeleteByPath(
				rt.Database, account.Name, path); err != nil {
				rt.Logger.Warningf("Could not delete address object %s: %s",
					path, err.Error())
				continue
			}
			removed++
		}

		book.SyncToken, book.CTag = result.State.SyncToken, result.State.CTag
		if err := addressbook.Set(rt.Database, book); err != nil {
			rt.Out.Put(out.Opts{Type: out.Error},
				"Could not store address book %s: %s", ab.Path, err.Error())
			continue
		}

		reportCollection(rt, "Address book", displayName(ab.Name, ab.Path),
			stored, removed, result.Strategy, err)
	}
}

func knownAddressObjects(
	rt *runtime.Runtime,
	accountName string,
	addressBookPath string,
) map[string]string {
	local, err := addressobject.List(rt.Database)
	if err != nil {
		rt.Logger.Warningf("Could not list the address objects of %s: %s", addressBookPath, err.Error())
		return nil
	}

	known := make(map[string]string)
	for _, ao := range local {
		if ao.AccountName == accountName && ao.AddressBookPath == addressBookPath {
			known[ao.Path] = ao.ETag
		}
	}

	return known
}

func reportCollection(
	rt *runtime.Runtime,
	kind string,
	name string,
	stored int,
	removed int,
	strategy webdav.SyncStrategy,
	incomplete error,
) {
	if incomplete != nil {
		rt.Out.Put(out.Opts{Type: out.Warn},
			"%s %s: %s updated, %s removed %s, then: %s",
			kind,
			rt.Out.FG(out.ColorPrimary, "%s", name),
			rt.Out.FG(out.ColorCyan, "%d", stored),
			rt.Out.FG(out.ColorCyan, "%d", removed),
			rt.Out.FG(out.ColorSecondary, "(%s)", strategy),
			incomplete.Error())
		return
	}

	rt.Out.Put(out.Opts{Type: out.Ok},
		"%s %s: %s updated, %s removed %s",
		kind,
		rt.Out.FG(out.ColorPrimary, "%s", name),
		rt.Out.FG(out.ColorCyan, "%d", stored),
		rt.Out.FG(out.ColorCyan, "%d", removed),
		rt.Out.FG(out.ColorSecondary, "(%s)", strategy))
}

func displayName(name string, path string) string {
	if name != "" {
		return name
	}
	return path
}

func init() {
	Cmd.Flags().StringArrayVar(
		&flagTrustHost,
		shared.TrustHostFlag,
		nil,
		shared.TrustHostUsage,
	)
}
