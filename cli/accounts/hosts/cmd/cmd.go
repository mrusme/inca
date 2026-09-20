package hostsCmd

import (
	"github.com/spf13/cobra"
	forgetCmd "xn--gckvb8fzb.com/inca/cli/accounts/hosts/forget/cmd"
	listCmd "xn--gckvb8fzb.com/inca/cli/accounts/hosts/list/cmd"
	trustCmd "xn--gckvb8fzb.com/inca/cli/accounts/hosts/trust/cmd"
	"xn--gckvb8fzb.com/inca/cli/accounts/shared"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string

var Cmd = &cobra.Command{
	Use:     "hosts",
	Aliases: []string{"host", "h"},
	Short:   "inca accounts hosts",
	Long:    "View and change which hosts outside the configured one get the credentials of an account.",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rt := runtime.New(
			runtime.GetConfigStr(cmd),
			runtime.GetLogLevel(cmd),
			runtime.GetOutputColor(cmd),
			true,
		)
		defer rt.End()

		shared.ListHosts(rt, flagFormat, "")
	},
}

func init() {
	Cmd.AddCommand(listCmd.Cmd)
	Cmd.AddCommand(trustCmd.Cmd)
	Cmd.AddCommand(forgetCmd.Cmd)

	Cmd.Flags().StringVarP(
		&flagFormat,
		"format",
		"f",
		"",
		shared.FormatUsage,
	)
}
