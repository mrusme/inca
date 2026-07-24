package versionCmd

import (
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/runtime"
)

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "inca version",
	Long:  "Display inca version information",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		o := out.New(runtime.GetOutputColor(cmd))

		o.Put(out.Opts{Type: out.Info},
			"%s %s",
			o.Stylize(
				out.Style{FG: out.ColorPrimary, BG: out.ColorSecondary},
				"inca"),
			runtime.Version,
		)
		o.Put(out.Opts{Type: out.Plain},
			"  %s %s",
			o.FG(out.ColorSecondary, "Commit:"),
			runtime.Commit,
		)
		o.Put(out.Opts{Type: out.Plain},
			"  %s %s",
			o.FG(out.ColorSecondary, "Build date:"),
			runtime.Date,
		)
	},
}

func init() {
}
