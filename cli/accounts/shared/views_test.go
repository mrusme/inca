package shared

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"xn--gckvb8fzb.com/inca/helpers/trust"
	"xn--gckvb8fzb.com/inca/models/config"
	"xn--gckvb8fzb.com/inca/models/service"
	"xn--gckvb8fzb.com/inca/models/trustedhost"
)

func TestAccountViewsHaveNoPassword(t *testing.T) {
	configured := []config.Account{
		{Name: "work", Username: "alice", Password: "hunter2-secret", Endpoint: "https://dav.example.com"},
		{Name: "icloud", Username: "alice@icloud.com", Password: "app-specific-secret"},
	}
	cal := service.New("icloud", service.CalDAV)
	cal.Start, cal.Endpoint, cal.Principal = "alice@icloud.com", "https://caldav.icloud.com/", "/1/principal/"
	orphan := service.New("gone", service.CardDAV)

	views := BuildAccountViews(configured, map[string]*service.Service{cal.GetKey(): cal, orphan.GetKey(): orphan})
	if len(views) != 2 || views[0].Name != "icloud" || views[1].Name != "work" {
		t.Fatalf("views = %+v, want the two configured accounts by name", views)
	}
	if len(views[0].Services) != 1 || views[0].Services[0].Endpoint != "https://caldav.icloud.com/" || len(views[1].Services) != 0 {
		t.Errorf("services = %+v and %+v", views[0].Services, views[1].Services)
	}

	encoded, err := json.Marshal(views)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret") {
		t.Errorf("the JSON has a password in it: %s", encoded)
	}
}

func TestHostViews(t *testing.T) {
	configured := []config.Account{{Name: "icloud"}, {Name: "work"}}
	rows := make(map[string]*trustedhost.TrustedHost)
	for _, one := range []struct{ account, pattern string }{
		{"work", "dav.provider.net"},
		{"icloud", "p42-caldav.icloud.com"},
		{"icloud", "*.icloud.com"},
		{"gone", "old.example.com"},
	} {
		row := trustedhost.New(one.account, one.pattern)
		rows[row.GetKey()] = row
	}

	all := BuildHostViews(rows, configured, "")
	order := make([]string, 0, len(all))
	for _, view := range all {
		order = append(order, view.AccountName+" "+view.Pattern)
	}
	if !slices.Equal(order, []string{"gone old.example.com", "icloud *.icloud.com", "icloud p42-caldav.icloud.com", "work dav.provider.net"}) {
		t.Errorf("order = %v", order)
	}
	if all[0].Configured || !all[1].Configured {
		t.Errorf("configured = %v and %v, want false for the account that is gone", all[0].Configured, all[1].Configured)
	}

	if only := BuildHostViews(rows, configured, "icloud"); len(only) != 2 {
		t.Errorf("the hosts of icloud = %+v", only)
	}
}

func TestTrustAndForget(t *testing.T) {
	rt := newRuntime(t)
	configured := []config.Account{{Name: "icloud"}}

	pattern, err := Trust(rt, configured, "icloud", " *.iCloud.com ")
	if err != nil || pattern != "*.icloud.com" {
		t.Fatalf("Trust = %q, %v", pattern, err)
	}
	rows, _ := trustedhost.ListByAccount(rt.Database, "icloud")
	if len(rows) != 1 || rows[0].Pattern != "*.icloud.com" || rows[0].Origin != string(trust.OriginCommand) || rows[0].Source != "" {
		t.Errorf("rows = %+v", rows)
	}

	if _, err = Trust(rt, configured, "unknown", "*.icloud.com"); err == nil {
		t.Error("an approval for an account that isn't configured was stored")
	}
	if _, err = Trust(rt, configured, "icloud", "*.com"); err == nil {
		t.Error("a public suffix was stored")
	}

	if _, err = Forget(rt, "icloud", "p42-caldav.icloud.com"); err == nil {
		t.Error("an approval that doesn't exist was forgotten")
	}
	if pattern, err = Forget(rt, "icloud", "*.ICLOUD.com"); err != nil || pattern != "*.icloud.com" {
		t.Fatalf("Forget = %q, %v", pattern, err)
	}
	if rows, _ = trustedhost.ListByAccount(rt.Database, "icloud"); len(rows) != 0 {
		t.Errorf("rows = %+v after Forget", rows)
	}

	orphan := trustedhost.New("gone", "old.example.com")
	if err = trustedhost.Set(rt.Database, orphan); err != nil {
		t.Fatal(err)
	}
	if _, err = Forget(rt, "gone", "old.example.com"); err != nil {
		t.Errorf("Forget for an account that is no longer configured: %v", err)
	}
}
