package listCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string

var Cmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "inca accounts list",
	Long:    "List every configured account with the services that were discovered for it.",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rt := runtime.New(
			runtime.GetConfigStr(cmd),
			runtime.GetLogLevel(cmd),
			runtime.GetOutputColor(cmd),
			true,
		)
		defer rt.End()

		shared.ListAccounts(rt, flagFormat)
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
}
