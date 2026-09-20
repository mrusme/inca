package listCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string

var Cmd = &cobra.Command{
	Use:     "list [account]",
	Aliases: []string{"ls", "l"},
	Short:   "inca accounts hosts list",
	Long:    "List the approved hosts of every account, or of the one that is named.",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		rt := runtime.New(
			runtime.GetConfigStr(cmd),
			runtime.GetLogLevel(cmd),
			runtime.GetOutputColor(cmd),
			true,
		)
		defer rt.End()

		var accountName string
		if len(args) == 1 {
			accountName = args[0]
		}
		shared.ListHosts(rt, flagFormat, accountName)
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
