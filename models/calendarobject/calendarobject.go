package calendarobject

import (
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
	"xn--gckvb8fzb.com/inca/database"
)

type CalendarObject struct {
	key string `json:"-"`

	AccountName  string    `json:"account_name"`
	CalendarPath string    `json:"calendar_path"`
	Path         string    `json:"path"`
	UID          string    `json:"uid"`
	Component    string    `json:"component"`
	ETag         string    `json:"etag"`
	ModTime      time.Time `json:"mod_time"`
	Data         string    `json:"data"`
}

func New(accountName string, calendarPath string, path string) *CalendarObject {
	co := new(CalendarObject)
	co.AccountName = accountName
	co.CalendarPath = calendarPath
	co.Path = path
	co.key = database.StableKey(co, accountName+"|"+path)
	return co
}

func FromDAV(
	accountName string,
	calendarPath string,
	obj caldav.CalendarObject,
) (*CalendarObject, error) {
	var buf strings.Builder
	if err := ical.NewEncoder(&buf).Encode(obj.Data); err != nil {
		return nil, err
	}

	co := New(accountName, calendarPath, obj.Path)
	co.ETag = obj.ETag
	co.ModTime = obj.ModTime
	co.Data = buf.String()
	co.Component, co.UID = extractMeta(obj.Data)
	return co, nil
}

func extractMeta(cal *ical.Calendar) (component string, uid string) {
	if cal == nil {
		return "", ""
	}
	for _, child := range cal.Children {
		if child.Name == ical.CompTimezone {
			continue
		}
		if component == "" {
			component = child.Name
		}
		if uid == "" {
			if u, err := child.Props.Text(ical.PropUID); err == nil {
				uid = u
			}
		}
	}
	return component, uid
}

func (co *CalendarObject) SetKey(k string) {
	co.key = k
}

func (co *CalendarObject) GetKey() string {
	if co.key == "" {
		co.key = database.StableKey(co, co.AccountName+"|"+co.Path)
	}
	return co.key
}

func List(db *database.Database) (map[string]*CalendarObject, error) {
	var rows map[string]*CalendarObject = make(map[string]*CalendarObject)
	if err := database.GetPrefixedRowsAsStruct(
		db,
		database.PrefixForModel(&CalendarObject{}),
		rows,
	); err != nil {
		return nil, err
	}
	return rows, nil
}

func Set(db *database.Database, co *CalendarObject) error {
	return db.UpsertRowAsStruct(co)
}

func DeleteByPath(db *database.Database, accountName string, path string) error {
	co := New(accountName, "", path)
	return db.DestroyRow(co.GetKey())
}
