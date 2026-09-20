package shared

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/helpers/trust"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/models/service"
	"xn--gckvb8fzb.com/inca/models/trustedhost"
	"xn--gckvb8fzb.com/inca/runtime"
)

const (
	FormatUnspecified = ""
	FormatCLI         = "cli"
	FormatJSON        = "json"
)

const FormatUsage string = "Output format (cli, json) (default \"cli\")"

const dateLayout string = "2006-01-02 15:04"

type ServiceView struct {
	Protocol     string    `json:"protocol"`
	Start        string    `json:"start"`
	Endpoint     string    `json:"endpoint"`
	Principal    string    `json:"principal"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

type AccountView struct {
	Name            string        `json:"name"`
	Username        string        `json:"username"`
	Endpoint        string        `json:"endpoint,omitempty"`
	CalDAVEndpoint  string        `json:"caldav_endpoint,omitempty"`
	CardDAVEndpoint string        `json:"carddav_endpoint,omitempty"`
	Services        []ServiceView `json:"services"`
}

type DiscoveryView struct {
	AccountName string        `json:"account_name"`
	Services    []ServiceView `json:"services"`
	Errors      []string      `json:"errors,omitempty"`
}

type HostView struct {
	AccountName string    `json:"account_name"`
	Configured  bool      `json:"configured"`
	Pattern     string    `json:"pattern"`
	Source      string    `json:"source,omitempty"`
	Origin      string    `json:"origin"`
	CreatedAt   time.Time `json:"created_at"`
}

func BuildServiceView(s *service.Service) ServiceView {
	return ServiceView{
		Protocol:     s.Protocol,
		Start:        s.Start,
		Endpoint:     s.Endpoint,
		Principal:    s.Principal,
		DiscoveredAt: s.DiscoveredAt,
	}
}

func BuildAccountViews(accounts []config.Account, services map[string]*service.Service) []AccountView {
	views := make([]AccountView, 0, len(accounts))

	for _, account := range accounts {
		view := AccountView{
			Name:            account.Name,
			Username:        account.Username,
			Endpoint:        account.Endpoint,
			CalDAVEndpoint:  account.CalDAVEndpoint,
			CardDAVEndpoint: account.CardDAVEndpoint,
			Services:        make([]ServiceView, 0, 2),
		}
		for _, s := range services {
			if s.AccountName == account.Name {
				view.Services = append(view.Services, BuildServiceView(s))
			}
		}
		slices.SortFunc(view.Services, func(a, b ServiceView) int { return cmp.Compare(a.Protocol, b.Protocol) })

		views = append(views, view)
	}

	slices.SortFunc(views, func(a, b AccountView) int { return cmp.Compare(a.Name, b.Name) })
	return views
}

func BuildHostViews(rows map[string]*trustedhost.TrustedHost, accounts []config.Account, accountName string) []HostView {
	views := make([]HostView, 0, len(rows))

	for _, row := range rows {
		if accountName != "" && row.AccountName != accountName {
			continue
		}
		views = append(views, HostView{
			AccountName: row.AccountName,
			Configured:  isConfigured(accounts, row.AccountName),
			Pattern:     row.Pattern,
			Source:      row.Source,
			Origin:      row.Origin,
			CreatedAt:   row.CreatedAt,
		})
	}

	slices.SortFunc(views, func(a, b HostView) int {
		return cmp.Or(cmp.Compare(a.AccountName, b.AccountName), cmp.Compare(a.Pattern, b.Pattern))
	})
	return views
}

func isConfigured(accounts []config.Account, name string) bool {
	return slices.ContainsFunc(accounts, func(a config.Account) bool { return a.Name == name })
}

func FindAccount(accounts []config.Account, name string) (config.Account, error) {
	for _, account := range accounts {
		if account.Name == name {
			return account, nil
		}
	}
	return config.Account{}, fmt.Errorf("the account %s isn't configured", name)
}

func Trust(rt *runtime.Runtime, accounts []config.Account, accountName string, raw string) (trust.Pattern, error) {
	if _, err := FindAccount(accounts, accountName); err != nil {
		return "", err
	}
	pattern, err := trust.ParsePattern(raw)
	if err != nil {
		return "", err
	}

	row := trustedhost.New(accountName, string(pattern))
	row.Origin = string(trust.OriginCommand)
	if err = trustedhost.Set(rt.Database, row); err != nil {
		return "", err
	}
	return pattern, nil
}

func Forget(rt *runtime.Runtime, accountName string, raw string) (trust.Pattern, error) {
	pattern, err := trust.ParsePattern(raw)
	if err != nil {
		return "", err
	}

	found, err := trustedhost.Delete(rt.Database, accountName, string(pattern))
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%s has no approval for %s", accountName, pattern)
	}
	return pattern, nil
}

func ListAccounts(rt *runtime.Runtime, format string) {
	accounts, err := rt.Config.Accounts()
	rt.NilOrDie(err)

	services, err := service.List(rt.Database)
	rt.NilOrDie(err)

	OutputAccounts(rt, format, BuildAccountViews(accounts, services))
}

func ListHosts(rt *runtime.Runtime, format string, accountName string) {
	accounts, err := rt.Config.Accounts()
	rt.NilOrDie(err)

	rows, err := trustedhost.List(rt.Database)
	rt.NilOrDie(err)

	OutputHosts(rt, format, BuildHostViews(rows, accounts, accountName))
}

func OutputAccounts(rt *runtime.Runtime, format string, views []AccountView) {
	if format == FormatJSON {
		outputJSON(rt, views)
		return
	}

	if len(views) == 0 {
		rt.Out.Put(out.Opts{Type: out.Info}, "No accounts configured")
		return
	}
	for _, view := range views {
		rt.Out.Put(out.Opts{Type: out.Info}, "%s %s",
			rt.Out.Stylize(out.Style{FG: out.ColorPrimary}, "%s", view.Name),
			rt.Out.Stylize(out.Style{FG: out.ColorSecondary}, "(%s)", view.Username),
		)
		if len(view.Services) == 0 {
			rt.Out.Put(out.Opts{Type: out.Plain}, "  %s",
				rt.Out.FG(out.ColorSecondary, "nothing discovered yet"))
		}
		for _, s := range view.Services {
			outputService(rt, s)
		}
	}
}

func outputService(rt *runtime.Runtime, view ServiceView) {
	rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
		rt.Out.FG(out.ColorSecondary, "%-9s", view.Protocol), view.Endpoint)
	rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
		rt.Out.FG(out.ColorSecondary, "%-9s", "principal"), view.Principal)
	rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
		rt.Out.FG(out.ColorSecondary, "%-9s", "found"), view.DiscoveredAt.Local().Format(dateLayout))
}

func OutputDiscoveries(rt *runtime.Runtime, format string, views []DiscoveryView) {
	if format == FormatJSON {
		outputJSON(rt, views)
		return
	}

	for _, view := range views {
		rt.Out.Put(out.Opts{Type: out.Info}, "%s",
			rt.Out.Stylize(out.Style{FG: out.ColorPrimary}, "%s", view.AccountName))
		for _, s := range view.Services {
			outputService(rt, s)
		}
		for _, message := range view.Errors {
			rt.Out.Put(out.Opts{Type: out.Error}, "%s", message)
		}
	}
}

func OutputHosts(rt *runtime.Runtime, format string, views []HostView) {
	if format == FormatJSON {
		outputJSON(rt, views)
		return
	}

	if len(views) == 0 {
		rt.Out.Put(out.Opts{Type: out.Info}, "No approved hosts")
		return
	}
	for _, view := range views {
		account := rt.Out.Stylize(out.Style{FG: out.ColorSecondary}, "(%s)", view.AccountName)
		if !view.Configured {
			account = rt.Out.Stylize(out.Style{FG: out.ColorSecondary}, "(%s, no longer configured)", view.AccountName)
		}
		rt.Out.Put(out.Opts{Type: out.Info}, "%s %s",
			rt.Out.Stylize(out.Style{FG: out.ColorPrimary}, "%s", view.Pattern), account)

		approved := view.Origin
		if view.Source != "" {
			approved += ", after a " + view.Source
		}
		rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
			rt.Out.FG(out.ColorSecondary, "%-8s", "approved"),
			fmt.Sprintf("%s (%s)", view.CreatedAt.Local().Format(dateLayout), approved))
	}
}

func outputJSON[T any](rt *runtime.Runtime, views []T) {
	if views == nil {
		views = make([]T, 0)
	}

	encoded, err := json.MarshalIndent(views, "", "  ")
	rt.NilOrDie(err)
	rt.Out.Put(out.Opts{Type: out.Plain}, "%s", string(encoded))
}
