package service

import (
	"log/slog"
	"testing"

	"xn--gckvb8fzb.com/inca/database"
	"xn--gckvb8fzb.com/inca/helpers/log"
)

func open(t *testing.T) *database.Database {
	t.Helper()

	db, err := database.New(log.New(slog.LevelError), "", false)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

func TestServices(t *testing.T) {
	db := open(t)

	first := New("icloud", CalDAV)
	first.Start, first.Endpoint, first.Principal = "https://caldav.icloud.com", "https://caldav.icloud.com/", "/1/principal/"
	cards := New("icloud", CardDAV)
	cards.Start, cards.Endpoint, cards.Principal = "https://contacts.icloud.com", "https://contacts.icloud.com/", "/1/principal/"
	for _, one := range []*Service{first, cards} {
		if err := Set(db, one); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	moved := New("icloud", CalDAV)
	moved.Start, moved.Endpoint, moved.Principal = first.Start, "https://caldav.icloud.com/new/", "/2/principal/"
	if err := Set(db, moved); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, found, err := Get(db, "icloud", CalDAV)
	if err != nil || !found || got.Endpoint != "https://caldav.icloud.com/new/" || got.DiscoveredAt.IsZero() {
		t.Fatalf("Get = %+v, %v, %v, want the second discovery in place of the first", got, found, err)
	}
	if _, found, err = Get(db, "work", CalDAV); err != nil || found {
		t.Errorf("Get of an account without a row = %v, %v", found, err)
	}

	all, err := List(db)
	if err != nil || len(all) != 2 {
		t.Fatalf("List = %d rows, %v", len(all), err)
	}

	if found, err = Delete(db, "icloud", CalDAV); err != nil || !found {
		t.Fatalf("Delete = %v, %v", found, err)
	}
	if found, err = Delete(db, "icloud", CalDAV); err != nil || found {
		t.Errorf("a second Delete = %v, %v", found, err)
	}
	if _, found, _ = Get(db, "icloud", CardDAV); !found {
		t.Error("the CardDAV row went with the CalDAV one")
	}
}
