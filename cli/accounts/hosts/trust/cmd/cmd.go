package trustCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/runtime"
)

var Cmd = &cobra.Command{
	Use:     "trust <account> <host>",
	Aliases: []string{"add", "t"},
	Short:   "inca accounts hosts trust",
	Long: "Approve a host for an account, as a host name, an address or *.domain for every host under a domain. " +
		"The credentials of the account are sent there when the server refers to it.",
	Args: cobra.ExactArgs(2),
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

		pattern, err := shared.Trust(rt, accounts, args[0], args[1])
		rt.NilOrDie(err)

		rt.Out.Put(out.Opts{Type: out.Info}, "%s may use %s",
			rt.Out.FG(out.ColorPrimary, "%s", args[0]), pattern)
	},
}
