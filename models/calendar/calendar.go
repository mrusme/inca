package calendar

import (
	"github.com/emersion/go-webdav/caldav"
	"xn--gckvb8fzb.com/inca/database"
)

type Calendar struct {
	key string `json:"-"`

	AccountName         string   `json:"account_name"`
	Path                string   `json:"path"`
	Name                string   `json:"name"`
	Description         string   `json:"description"`
	Color               string   `json:"color"`
	SupportedComponents []string `json:"supported_components"`
	MaxResourceSize     int64    `json:"max_resource_size"`
	SyncToken           string   `json:"sync_token"`
}

func New(accountName string, path string) *Calendar {
	c := new(Calendar)
	c.AccountName = accountName
	c.Path = path
	c.key = database.StableKey(c, accountName+"|"+path)
	return c
}

func FromDAV(accountName string, c caldav.Calendar) *Calendar {
	cal := New(accountName, c.Path)
	cal.Name = c.Name
	cal.Description = c.Description
	cal.Color = c.Color
	cal.SupportedComponents = c.SupportedComponentSet
	cal.MaxResourceSize = c.MaxResourceSize
	cal.SyncToken = c.SyncToken
	return cal
}

func (c *Calendar) SetKey(k string) {
	c.key = k
}

func (c *Calendar) GetKey() string {
	if c.key == "" {
		c.key = database.StableKey(c, c.AccountName+"|"+c.Path)
	}
	return c.key
}

func List(db *database.Database) (map[string]*Calendar, error) {
	var rows map[string]*Calendar = make(map[string]*Calendar)
	if err := database.GetPrefixedRowsAsStruct(
		db,
		database.PrefixForModel(&Calendar{}),
		rows,
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func Get(db *database.Database, key string) (*Calendar, error) {
	c := new(Calendar)
	if err := db.GetRowAsStruct(key, c); err != nil {
		return nil, err
	}
	return c, nil
}

func Set(db *database.Database, c *Calendar) error {
	return db.UpsertRowAsStruct(c)
}
