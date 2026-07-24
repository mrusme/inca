package config

import (
	"net/url"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"xn--gckvb8fzb.com/inca/errs"
)

type Account struct {
	Name            string `koanf:"name"`
	Endpoint        string `koanf:"endpoint"`
	Username        string `koanf:"username"`
	Password        string `koanf:"password"`
	CalDAVEndpoint  string `koanf:"caldav_endpoint"`
	CardDAVEndpoint string `koanf:"carddav_endpoint"`
}

type Config struct {
	cfgstr   string
	k        *koanf.Koanf
	provider koanf.Provider
}

func New(cfgstr string) (cfg *Config, err error) {
	cfg = new(Config)
	cfg.cfgstr = cfgstr

	var path string
	if path, err = cfg.parsePath(); err != nil {
		return nil, err
	}

	cfg.k = koanf.New(".")
	cfg.provider = file.Provider(path)
	if err = cfg.k.Load(cfg.provider, toml.Parser()); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *Config) parsePath() (string, error) {
	u, err := url.Parse(cfg.cfgstr)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "":
		return cfg.cfgstr, nil
	case "file":
		if u.Host != "" {
			return u.Host + u.Path, nil
		}
		return u.Path, nil
	default:
		return "", errs.ErrConfigTypeUnsupported
	}
}

func (cfg *Config) DatabasePath() string {
	return cfg.k.String("database.path")
}

func (cfg *Config) Accounts() ([]Account, error) {
	var accounts []Account
	if err := cfg.k.Unmarshal("account", &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}
