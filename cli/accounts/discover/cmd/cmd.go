package discoverCmd

import (
	"context"

	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/errs"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string
var flagTrustHost []string

var Cmd = &cobra.Command{
	Use:     "discover [account]",
	Aliases: []string{"disco", "d"},
	Short:   "inca accounts discover",
	Long: "Discover the services of every account, or of the one that is named, " +
		"and store what was found in place of what was stored before.",
	Args: cobra.MaximumNArgs(1),
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

		if len(args) == 1 {
			account, err := shared.FindAccount(accounts, args[0])
			rt.NilOrDie(err)
			accounts = []config.Account{account}
		}

		opts := &shared.Options{Flags: flags, Input: shared.Terminal()}

		ctx := context.Background()
		views := make([]shared.DiscoveryView, 0, len(accounts))
		for i := range accounts {
			views = append(views, shared.Discover(ctx, rt, accounts[i], opts))
		}
		shared.OutputDiscoveries(rt, flagFormat, views)
	},
}

func init() {
	Cmd.Flags().StringVarP(
		&flagFormat,
		"format",
		"f",
		"",
		shared.FormatUsage,
	)
	Cmd.Flags().StringArrayVar(
		&flagTrustHost,
		shared.TrustHostFlag,
		nil,
		shared.TrustHostUsage,
	)
}
