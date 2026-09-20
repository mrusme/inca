package trustedhost

import (
	"log/slog"
	"slices"
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

func TestTrustedHosts(t *testing.T) {
	db := open(t)

	for _, one := range []struct{ account, pattern string }{
		{"icloud", "*.icloud.com"},
		{"icloud", "p42-caldav.icloud.com"},
		{"icloud", "*.icloud.com"},
		{"work", "dav.provider.net"},
	} {
		th := New(one.account, one.pattern)
		th.Source, th.Origin = "reference", "prompt"
		if err := Set(db, th); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	all, err := List(db)
	if err != nil || len(all) != 3 {
		t.Fatalf("List = %d rows, %v, want three, because one approval was stored twice", len(all), err)
	}

	icloud, err := ListByAccount(db, "icloud")
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	patterns := make([]string, 0, len(icloud))
	for _, one := range icloud {
		patterns = append(patterns, one.Pattern)
		if one.Source != "reference" || one.Origin != "prompt" || one.CreatedAt.IsZero() {
			t.Errorf("row = %+v", one)
		}
	}
	if !slices.Equal(patterns, []string{"*.icloud.com", "p42-caldav.icloud.com"}) {
		t.Errorf("patterns of icloud = %v, want its two in order", patterns)
	}

	found, err := Delete(db, "icloud", "*.icloud.com")
	if err != nil || !found {
		t.Fatalf("Delete = %v, %v", found, err)
	}
	if found, err = Delete(db, "icloud", "*.icloud.com"); err != nil || found {
		t.Errorf("a second Delete = %v, %v, want false", found, err)
	}
	if found, err = Delete(db, "work", "p42-caldav.icloud.com"); err != nil || found {
		t.Errorf("Delete of another account's pattern = %v, %v", found, err)
	}

	if left, _ := ListByAccount(db, "icloud"); len(left) != 1 || left[0].Pattern != "p42-caldav.icloud.com" {
		t.Errorf("left = %+v", left)
	}
	if work, _ := ListByAccount(db, "work"); len(work) != 1 {
		t.Errorf("work = %+v", work)
	}
}
