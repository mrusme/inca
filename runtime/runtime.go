package runtime

import (
	"log/slog"
	"os"
	"path"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"xn--gckvb8fzb.com/inca/database"
	"xn--gckvb8fzb.com/inca/helpers/log"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/models/config"
)

var (
	CONFIG_ENV_VAR   = "INCA_CONFIG"
	DATABASE_ENV_VAR = "INCA_DATABASE"
)

var (
	Version string
	Commit  string
	Date    string
)

type Build struct {
	Version string
	Commit  string
	Date    string
}

type Runtime struct {
	Build    Build
	Logger   *log.Logger
	Out      *out.Out
	Database *database.Database
	Config   *config.Config
}

func New(
	cfgstr string,
	lvl slog.Level,
	oc out.OutputColor,
	readOnly bool,
) *Runtime {
	var err error

	rt := new(Runtime)

	rt.Build.Version = Version
	rt.Build.Commit = Commit
	rt.Build.Date = Date

	rt.Logger = log.New(lvl)
	rt.Out = out.New(oc)

	if cfgstr == "" {
		if env, found := os.LookupEnv(CONFIG_ENV_VAR); found == true {
			cfgstr = env
		}
	}
	if cfgstr == "" {
		cfgstr, err = xdg.ConfigFile(path.Join("inca", "config.toml"))
		rt.Logger.NilOrDie(err, "Could not determine the configuration path")
	}
	rt.Logger.Debug("Loading configuration", "config", cfgstr)

	rt.Config, err = config.New(cfgstr)
	rt.Logger.NilOrDie(err, "Could not load the configuration from "+cfgstr)

	var dbpath string
	var found bool
	if dbpath, found = os.LookupEnv(DATABASE_ENV_VAR); found == false {
		dbpath = rt.Config.DatabasePath()
	}
	if dbpath == "" {
		dbpath, err = xdg.DataFile(path.Join("inca", "db"))
		rt.Logger.NilOrDie(err, "Could not determine the database directory")
	}
	rt.Logger.Debug("Opening database", "directory", dbpath, "readOnly", readOnly)

	rt.Database, err = database.New(rt.Logger, dbpath, readOnly)
	rt.Logger.NilOrDie(err, "Could not open the database at "+dbpath)

	return rt
}

func (rt *Runtime) End() {
	rt.Logger.Debug("Ending runtime ...")
	rt.Database.Close()
}

func (rt *Runtime) Exit(code int) {
	rt.End()
	os.Exit(code)
}

func (rt *Runtime) NilOrDie(err error) {
	if err != nil {
		rt.Out.Put(out.Opts{Type: out.Error}, "%s", err.Error())
		rt.Exit(1)
	}
}

func (rt *Runtime) GetStringFlag(cmd *cobra.Command, flagname string) string {
	flag, err := cmd.Flags().GetString(flagname)
	if err != nil {
		rt.Logger.Error("Could not get flag", "flag", flagname, "error", err)
		return ""
	}
	return flag
}

func (rt *Runtime) GetBoolFlag(cmd *cobra.Command, flagname string) bool {
	flag, err := cmd.Flags().GetBool(flagname)
	if err != nil {
		rt.Logger.Error("Could not get flag", "flag", flagname, "error", err)
		return false
	}
	return flag
}
