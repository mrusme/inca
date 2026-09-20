package service

import (
	"time"

	"xn--gckvb8fzb.com/inca/database"
)

const (
	CalDAV  string = "caldav"
	CardDAV string = "carddav"
)

type Service struct {
	key string `json:"-"`

	AccountName  string    `json:"account_name"`
	Protocol     string    `json:"protocol"`
	Start        string    `json:"start"`
	Endpoint     string    `json:"endpoint"`
	Principal    string    `json:"principal"`
	DiscoveredAt time.Time `json:"discovered_at"`
}

func New(accountName string, protocol string) *Service {
	s := new(Service)
	s.AccountName = accountName
	s.Protocol = protocol
	s.DiscoveredAt = time.Now()
	s.key = database.StableKey(s, accountName+"|"+protocol)
	return s
}

func (s *Service) SetKey(k string) {
	s.key = k
}

func (s *Service) GetKey() string {
	if s.key == "" {
		s.key = database.StableKey(s, s.AccountName+"|"+s.Protocol)
	}
	return s.key
}

func List(db *database.Database) (map[string]*Service, error) {
	var rows map[string]*Service = make(map[string]*Service)
	if err := database.GetPrefixedRowsAsStruct(
		db,
		database.PrefixForModel(&Service{}),
		rows,
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func Get(db *database.Database, accountName string, protocol string) (s *Service, found bool, err error) {
	s = New(accountName, protocol)
	if err = db.GetRowAsStruct(s.GetKey(), s); err != nil {
		if db.IsErrKeyNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return s, true, nil
}

func Set(db *database.Database, s *Service) error {
	return db.UpsertRowAsStruct(s)
}

func Delete(db *database.Database, accountName string, protocol string) (found bool, err error) {
	if _, found, err = Get(db, accountName, protocol); err != nil || !found {
		return false, err
	}
	return true, db.DestroyRow(New(accountName, protocol).GetKey())
}
