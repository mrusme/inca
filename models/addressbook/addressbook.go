package addressbook

import (
	"github.com/emersion/go-webdav/carddav"
	"xn--gckvb8fzb.com/inca/database"
)

type AddressBook struct {
	key string `json:"-"`

	AccountName     string `json:"account_name"`
	Path            string `json:"path"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	MaxResourceSize int64  `json:"max_resource_size"`
	SyncToken       string `json:"sync_token"`
}

func New(accountName string, path string) *AddressBook {
	ab := new(AddressBook)
	ab.AccountName = accountName
	ab.Path = path
	ab.key = database.StableKey(ab, accountName+"|"+path)
	return ab
}

func FromDAV(accountName string, ab carddav.AddressBook) *AddressBook {
	book := New(accountName, ab.Path)
	book.Name = ab.Name
	book.Description = ab.Description
	book.MaxResourceSize = ab.MaxResourceSize
	book.SyncToken = ab.SyncToken
	return book
}

func (ab *AddressBook) SetKey(k string) {
	ab.key = k
}

func (ab *AddressBook) GetKey() string {
	if ab.key == "" {
		ab.key = database.StableKey(ab, ab.AccountName+"|"+ab.Path)
	}
	return ab.key
}

func List(db *database.Database) (map[string]*AddressBook, error) {
	var rows map[string]*AddressBook = make(map[string]*AddressBook)
	if err := database.GetPrefixedRowsAsStruct(
		db,
		database.PrefixForModel(&AddressBook{}),
		rows,
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func Get(db *database.Database, key string) (*AddressBook, error) {
	ab := new(AddressBook)
	if err := db.GetRowAsStruct(key, ab); err != nil {
		return nil, err
	}
	return ab, nil
}

func Set(db *database.Database, ab *AddressBook) error {
	return db.UpsertRowAsStruct(ab)
}
