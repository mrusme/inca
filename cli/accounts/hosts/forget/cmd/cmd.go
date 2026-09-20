package forgetCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/runtime"
)

var Cmd = &cobra.Command{
	Use:     "forget <account> <host>",
	Aliases: []string{"remove", "rm", "f"},
	Short:   "inca accounts hosts forget",
	Long:    "Remove the approval of a host for an account, written the way 'inca accounts hosts list' shows it.",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		rt := runtime.New(
			runtime.GetConfigStr(cmd),
			runtime.GetLogLevel(cmd),
			runtime.GetOutputColor(cmd),
			false,
		)
		defer rt.End()

		pattern, err := shared.Forget(rt, args[0], args[1])
		rt.NilOrDie(err)

		rt.Out.Put(out.Opts{Type: out.Info}, "%s may no longer use %s",
			rt.Out.FG(out.ColorPrimary, "%s", args[0]), pattern)
	},
}
