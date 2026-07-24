package peopleCmd

import (
	"github.com/spf13/cobra"
	findCmd "xn--gckvb8fzb.com/inca/cli/people/find/cmd"
	listCmd "xn--gckvb8fzb.com/inca/cli/people/list/cmd"
	"xn--gckvb8fzb.com/inca/cli/people/shared"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string

var Cmd = &cobra.Command{
	Use:     "people",
	Aliases: []string{"person", "p", "contacts", "contact"},
	Short:   "inca people",
	Long:    "View and search synced contacts.",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rt := runtime.New(
			runtime.GetConfigStr(cmd),
			runtime.GetLogLevel(cmd),
			runtime.GetOutputColor(cmd),
			true,
		)
		defer rt.End()

		contacts, err := addressobject.List(rt.Database)
		rt.NilOrDie(err)

		views := shared.BuildViews(contacts)
		shared.Output(rt, flagFormat, views)
	},
}

func init() {
	Cmd.AddCommand(listCmd.Cmd)
	Cmd.AddCommand(findCmd.Cmd)

	Cmd.Flags().StringVarP(
		&flagFormat,
		"format",
		"f",
		"",
		"Output format (cli, json) (default \"cli\")",
	)
}
