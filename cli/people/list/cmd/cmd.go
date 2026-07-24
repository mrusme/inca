package listCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/people/shared"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string

var Cmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "inca people list",
	Long:    "List every synced contact.",
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
	Cmd.Flags().StringVarP(
		&flagFormat,
		"format",
		"f",
		"",
		"Output format (cli, json) (default \"cli\")",
	)
}
