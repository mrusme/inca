package trustedhost

import (
	"sort"
	"time"

	"xn--gckvb8fzb.com/inca/database"
)

type TrustedHost struct {
	key string `json:"-"`

	AccountName string    `json:"account_name"`
	Pattern     string    `json:"pattern"`
	Source      string    `json:"source"`
	Origin      string    `json:"origin"`
	CreatedAt   time.Time `json:"created_at"`
}

func New(accountName string, pattern string) *TrustedHost {
	th := new(TrustedHost)
	th.AccountName = accountName
	th.Pattern = pattern
	th.CreatedAt = time.Now()
	th.key = database.StableKey(th, accountName+"|"+pattern)
	return th
}

func (th *TrustedHost) SetKey(k string) {
	th.key = k
}

func (th *TrustedHost) GetKey() string {
	if th.key == "" {
		th.key = database.StableKey(th, th.AccountName+"|"+th.Pattern)
	}
	return th.key
}

func List(db *database.Database) (map[string]*TrustedHost, error) {
	var rows map[string]*TrustedHost = make(map[string]*TrustedHost)
	if err := database.GetPrefixedRowsAsStruct(
		db,
		database.PrefixForModel(&TrustedHost{}),
		rows,
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func ListByAccount(db *database.Database, accountName string) ([]*TrustedHost, error) {
	rows, err := List(db)
	if err != nil {
		return nil, err
	}

	var found []*TrustedHost
	for _, th := range rows {
		if th.AccountName == accountName {
			found = append(found, th)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Pattern < found[j].Pattern })
	return found, nil
}

func Set(db *database.Database, th *TrustedHost) error {
	return db.UpsertRowAsStruct(th)
}

func Delete(db *database.Database, accountName string, pattern string) (found bool, err error) {
	th := New(accountName, pattern)
	if err = db.GetRowAsStruct(th.GetKey(), new(TrustedHost)); err != nil {
		if db.IsErrKeyNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, db.DestroyRow(th.GetKey())
}
