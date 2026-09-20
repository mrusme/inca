package shared

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/mattn/go-isatty"
	"xn--gckvb8fzb.com/inca/dav"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/helpers/trust"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/models/service"
	"xn--gckvb8fzb.com/inca/models/trustedhost"
	"xn--gckvb8fzb.com/inca/runtime"
	"xn--gckvb8fzb.com/maya/libs/webdav"
)

const TrustHostFlag string = "trust-host"

const TrustHostUsage string = "Approve a host outside the configured one for an account, " +
	"as account=host or account=*.domain (repeatable)"

type TrustFlags map[string][]trust.Pattern

func ParseTrustFlags(values []string, accounts []config.Account) (TrustFlags, error) {
	flags := make(TrustFlags)

	for _, value := range values {
		name, pattern, found := strings.Cut(value, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return nil, fmt.Errorf("--%s %q needs the account in front, as in account=host", TrustHostFlag, value)
		}
		if !slices.ContainsFunc(accounts, func(a config.Account) bool { return a.Name == name }) {
			return nil, fmt.Errorf("--%s %q names the account %s, which isn't configured", TrustHostFlag, value, name)
		}

		parsed, err := trust.ParsePattern(pattern)
		if err != nil {
			return nil, fmt.Errorf("--%s %q: %w", TrustHostFlag, value, err)
		}
		flags[name] = append(flags[name], parsed)
	}

	return flags, nil
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

func Terminal() *bufio.Reader {
	if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		return bufio.NewReader(os.Stdin)
	}
	return nil
}

func UserAgent(rt *runtime.Runtime) string {
	if rt.Build.Version == "" {
		return "inca"
	}
	return "inca/" + rt.Build.Version
}

type Options struct {
	Flags TrustFlags
	Input *bufio.Reader
}

type Session struct {
	DAV     *dav.DAV
	Decider *trust.Decider
	Stored  int
}

func Connect(rt *runtime.Runtime, account config.Account, opts *Options) (*Session, error) {
	approvals, err := trustedhost.ListByAccount(rt.Database, account.Name)
	if err != nil {
		return nil, err
	}

	decider := &trust.Decider{
		Flags: opts.Flags[account.Name],
		Store: func(approval trust.Approval) {
			row := trustedhost.New(account.Name, string(approval.Pattern))
			row.Source, row.Origin = approval.Source.String(), string(approval.Origin)
			if err := trustedhost.Set(rt.Database, row); err != nil {
				rt.Logger.Warningf("Could not store the approval of %s for %s: %s",
					approval.Pattern, account.Name, err.Error())
			}
		},
	}
	for _, row := range approvals {
		decider.Stored = append(decider.Stored, trust.Pattern(row.Pattern))
	}
	if opts.Input != nil {
		decider.Ask = ask(rt, opts.Input, account.Name)
	}

	session := &Session{Decider: decider}
	davOpts := &dav.Options{
		UserAgent: UserAgent(rt),
		TrustHost: decider.TrustHost,
		OnDiscover: func(found *service.Service) {
			if err := service.Set(rt.Database, found); err != nil {
				rt.Logger.Warningf("Could not store the %s service of %s: %s",
					found.Protocol, account.Name, err.Error())
			}
		},
	}

	if davOpts.CalDAV, err = storedService(rt, account.Name, service.CalDAV); err != nil {
		return nil, err
	}
	if davOpts.CardDAV, err = storedService(rt, account.Name, service.CardDAV); err != nil {
		return nil, err
	}
	for _, stored := range []*service.Service{davOpts.CalDAV, davOpts.CardDAV} {
		if stored != nil {
			session.Stored++
		}
	}

	if session.DAV, err = dav.New(account, davOpts); err != nil {
		return nil, err
	}
	return session, nil
}

func storedService(rt *runtime.Runtime, accountName string, protocol string) (*service.Service, error) {
	stored, found, err := service.Get(rt.Database, accountName, protocol)
	if err != nil || !found {
		return nil, err
	}
	return stored, nil
}

func question(req webdav.TrustRequest) string {
	switch req.Source {
	case webdav.TrustSourceRecord:
		return fmt.Sprintf("DNS names %s as the service of %s. A DNS answer can be forged.", req.Host, req.Endpoint)
	case webdav.TrustSourceRedirect:
		return fmt.Sprintf("%s redirects to %s.", req.Endpoint, req.Host)
	}
	return fmt.Sprintf("%s refers to %s for the data of this account.", req.Endpoint, req.Host)
}

func readAnswer(reader *bufio.Reader, domain string) trust.Answer {
	line, _ := reader.ReadString('\n')

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "h":
		return trust.AnswerHost
	case "d":
		if domain != "" {
			return trust.AnswerDomain
		}
	}
	return trust.AnswerNo
}

func ask(rt *runtime.Runtime, reader *bufio.Reader, accountName string) func(webdav.TrustRequest, string) trust.Answer {
	return func(req webdav.TrustRequest, domain string) trust.Answer {
		choices := "[n] no   [h] this host"
		if domain != "" {
			choices += "   [d] every host under " + domain
		}

		rt.Out.Put(out.Opts{Type: out.Warn}, "%s", question(req))
		rt.Out.Put(out.Opts{Type: out.Plain}, "  Send the credentials of %s there?",
			rt.Out.FG(out.ColorPrimary, "%s", accountName))
		rt.Out.Put(out.Opts{Type: out.Plain}, "  %s", choices)
		rt.Out.Put(out.Opts{Type: out.Plain, NoNL: true}, "  %s ", rt.Out.FG(out.ColorSecondary, ">"))

		return readAnswer(reader, domain)
	}
}

func Discover(ctx context.Context, rt *runtime.Runtime, account config.Account, opts *Options) DiscoveryView {
	view := DiscoveryView{AccountName: account.Name, Services: make([]ServiceView, 0, 2)}

	session, err := Connect(rt, account, opts)
	if err != nil {
		view.Errors = append(view.Errors, err.Error())
		return view
	}

	protocols := make([]string, 0, 2)
	if session.DAV.HasCalDAV() {
		protocols = append(protocols, service.CalDAV)
	}
	if session.DAV.HasCardDAV() {
		protocols = append(protocols, service.CardDAV)
	}
	if len(protocols) == 0 {
		view.Errors = append(view.Errors, "the account has no endpoint and no address as its username")
	}

	for _, protocol := range protocols {
		found, err := session.DAV.Discover(ctx, protocol)
		if err != nil {
			view.Errors = append(view.Errors, err.Error())
			continue
		}
		view.Services = append(view.Services, BuildServiceView(found))
	}

	if opts.Input == nil {
		for _, rejected := range session.Decider.Rejected() {
			view.Errors = append(view.Errors, Hint(account.Name, rejected))
		}
	}
	return view
}

func Hint(accountName string, req webdav.TrustRequest) string {
	return fmt.Sprintf("%s isn't %s or one of its subdomains, and nobody is here to ask. To approve it:\n  inca sync --%s %s=%s",
		req.Host, req.Endpoint, TrustHostFlag, accountName, req.Host)
}
