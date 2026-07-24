package listCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/cli/tasks/shared"
	"xn--gckvb8fzb.com/inca/models/calendarobject"
	"xn--gckvb8fzb.com/inca/runtime"
)

var (
	flagFormat string
	flagStart  string
	flagEnd    string
)

var Cmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "inca tasks list",
	Long:    "List tasks, filtered by due date when a range is given.",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		rt := runtime.New(
			runtime.GetConfigStr(cmd),
			runtime.GetLogLevel(cmd),
			runtime.GetOutputColor(cmd),
			true,
		)
		defer rt.End()

		start, end, err := shared.ResolveRange(flagStart, flagEnd)
		rt.NilOrDie(err)

		objects, err := calendarobject.List(rt.Database)
		rt.NilOrDie(err)

		views := shared.BuildViews(objects, start, end)
		shared.Output(rt, flagFormat, views)
	},
}

func init() {
	Cmd.Flags().StringVarP(
		&flagStart,
		"start",
		"s",
		"",
		"Only list tasks due on or after this",
	)
	Cmd.Flags().StringVarP(
		&flagEnd,
		"end",
		"e",
		"",
		"Only list tasks due on or before this",
	)
	Cmd.Flags().StringVarP(
		&flagFormat,
		"format",
		"f",
		"",
		"Output format (cli, json) (default \"cli\")",
	)
}
