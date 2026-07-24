package findCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/people/shared"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/runtime"
)

var flagFormat string

var Cmd = &cobra.Command{
	Use:     "find [terms...]",
	Aliases: []string{"fd", "f"},
	Short:   "inca people find",
	Long:    "Find synced contacts matching every search term.",
	Args:    cobra.MinimumNArgs(1),
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

		matched := shared.Filter(contacts, args)
		views := shared.BuildViews(matched)
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
