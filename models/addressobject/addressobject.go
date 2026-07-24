package addressobject

import (
	"strings"
	"time"

	"github.com/emersion/go-vcard"
	"github.com/emersion/go-webdav/carddav"
	"xn--gckvb8fzb.com/inca/database"
)

type AddressObject struct {
	key string `json:"-"`

	AccountName     string    `json:"account_name"`
	AddressBookPath string    `json:"address_book_path"`
	Path            string    `json:"path"`
	UID             string    `json:"uid"`
	FormattedName   string    `json:"formatted_name"`
	ETag            string    `json:"etag"`
	ModTime         time.Time `json:"mod_time"`
	Data            string    `json:"data"`
}

func New(
	accountName string,
	addressBookPath string,
	path string,
) *AddressObject {
	ao := new(AddressObject)
	ao.AccountName = accountName
	ao.AddressBookPath = addressBookPath
	ao.Path = path
	ao.key = database.StableKey(ao, accountName+"|"+path)
	return ao
}

func FromDAV(
	accountName string,
	addressBookPath string,
	obj carddav.AddressObject,
) (*AddressObject, error) {
	card := obj.Card
	if card.Get(vcard.FieldVersion) == nil {
		card.SetValue(vcard.FieldVersion, "3.0")
	}

	var buf strings.Builder
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		return nil, err
	}

	ao := New(accountName, addressBookPath, obj.Path)
	ao.ETag = obj.ETag
	ao.ModTime = obj.ModTime
	ao.Data = buf.String()
	ao.UID = card.Value(vcard.FieldUID)
	ao.FormattedName = card.PreferredValue(vcard.FieldFormattedName)
	return ao, nil
}

func (ao *AddressObject) SetKey(k string) {
	ao.key = k
}

func (ao *AddressObject) GetKey() string {
	if ao.key == "" {
		ao.key = database.StableKey(ao, ao.AccountName+"|"+ao.Path)
	}
	return ao.key
}

func List(db *database.Database) (map[string]*AddressObject, error) {
	var rows map[string]*AddressObject = make(map[string]*AddressObject)
	if err := database.GetPrefixedRowsAsStruct(
		db,
		database.PrefixForModel(&AddressObject{}),
		rows,
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func Set(db *database.Database, ao *AddressObject) error {
	return db.UpsertRowAsStruct(ao)
}

func DeleteByPath(db *database.Database, accountName string, path string) error {
	ao := New(accountName, "", path)
	return db.DestroyRow(ao.GetKey())
}
