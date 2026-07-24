package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	calendarsCmd "xn--gckvb8fzb.com/inca/cli/calendars/cmd"
	peopleCmd "xn--gckvb8fzb.com/inca/cli/people/cmd"
	syncCmd "xn--gckvb8fzb.com/inca/cli/sync/cmd"
	tasksCmd "xn--gckvb8fzb.com/inca/cli/tasks/cmd"
	versionCmd "xn--gckvb8fzb.com/inca/cli/version/cmd"
)

var (
	flagDebug  bool
	flagColor  string
	flagConfig string
)

var rootCmd = &cobra.Command{
	Use:   "inca",
	Short: "A command line CalDAV and CardDAV client.",
	Long: "Inca. A command line client for CalDAV and CardDAV that syncs " +
		"calendars, tasks and contacts.\n\n",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(syncCmd.Cmd)
	rootCmd.AddCommand(peopleCmd.Cmd)
	rootCmd.AddCommand(calendarsCmd.Cmd)
	rootCmd.AddCommand(tasksCmd.Cmd)
	rootCmd.AddCommand(versionCmd.Cmd)

	rootCmd.PersistentFlags().BoolVar(
		&flagDebug,
		"debug",
		false,
		"Display debugging output in the console",
	)
	rootCmd.PersistentFlags().StringVar(
		&flagColor,
		"color",
		"auto",
		"When to display colors (always, auto, never)",
	)
	rootCmd.PersistentFlags().StringVarP(
		&flagConfig,
		"config",
		"c",
		"",
		"Path or file:// URL to the configuration file",
	)
}
