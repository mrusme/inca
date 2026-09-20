package accountsCmd

import (
	"github.com/spf13/cobra"
	discoverCmd "xn--gckvb8fzb.com/inca/cli/accounts/discover/cmd"
	hostsCmd "xn--gckvb8fzb.com/inca/cli/accounts/hosts/cmd"
	listCmd "xn--gckvb8fzb.com/inca/cli/accounts/list/cmd"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string

var Cmd = &cobra.Command{
	Use:     "accounts",
	Aliases: []string{"account", "acc", "a"},
	Short:   "inca accounts",
	Long:    "View the configured accounts, what was discovered for them and which hosts they may use.",
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
	Cmd.AddCommand(listCmd.Cmd)
	Cmd.AddCommand(discoverCmd.Cmd)
	Cmd.AddCommand(hostsCmd.Cmd)

	Cmd.Flags().StringVarP(
		&flagFormat,
		"format",
		"f",
		"",
		shared.FormatUsage,
	)
}
