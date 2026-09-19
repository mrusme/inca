package shared

import (
	"strings"
	"testing"

	"github.com/emersion/go-vcard"
	"xn--gckvb8fzb.com/inca/models/addressobject"
)

func contact(account, uid, fn string, lines ...string) *addressobject.AddressObject {
	card := make(vcard.Card)
	card.SetValue(vcard.FieldVersion, "3.0")
	card.SetValue(vcard.FieldUID, uid)
	card.SetValue(vcard.FieldFormattedName, fn)
	for _, line := range lines {
		field, value, _ := strings.Cut(line, "=")
		card.AddValue(field, value)
	}

	var buf strings.Builder
	if err := vcard.NewEncoder(&buf).Encode(card); err != nil {
		panic(err)
	}

	ao := addressobject.New(account, "/contacts", "/contacts/"+uid+".vcf")
	ao.UID = uid
	ao.FormattedName = fn
	ao.Data = buf.String()
	return ao
}

func store(contacts ...*addressobject.AddressObject) map[string]*addressobject.AddressObject {
	m := make(map[string]*addressobject.AddressObject)
	for _, c := range contacts {
		m[c.GetKey()] = c
	}
	return m
}

func TestFilterMatchesAcrossAttributes(t *testing.T) {
	tomAto := contact("acc", "tom-ato", "Tom Ato",
		vcard.FieldName+"=Ato;Tom;;;")
	tomBaker := contact("acc", "tom-baker", "Tom Baker",
		vcard.FieldName+"=Baker;Tom;;;",
		vcard.FieldAddress+"=;;ATO Building 5;Metropolis;;12345;Country")
	bob := contact("acc", "bob", "Bob Jones",
		vcard.FieldName+"=Jones;Bob;;;")

	contacts := store(tomAto, tomBaker, bob)

	matched := Filter(contacts, []string{"tom", "ato"})
	if len(matched) != 2 {
		t.Fatalf("got %d matches, want 2: %v", len(matched), names(matched))
	}
	if _, ok := matched[tomAto.GetKey()]; !ok {
		t.Error("Tom Ato did not match")
	}
	if _, ok := matched[tomBaker.GetKey()]; !ok {
		t.Error("Tom Baker (ATO Building address) did not match")
	}
	if _, ok := matched[bob.GetKey()]; ok {
		t.Error("Bob Jones matched but should not have")
	}
}

func TestFilterIsCaseInsensitive(t *testing.T) {
	alice := contact("acc", "alice", "Alice Example",
		vcard.FieldEmail+"=Alice@Example.com")
	contacts := store(alice)

	if len(Filter(contacts, []string{"ALICE"})) != 1 {
		t.Error("uppercase term did not match lowercased name")
	}
	if len(Filter(contacts, []string{"example.com"})) != 1 {
		t.Error("term did not match mixed-case email")
	}
}

func TestFilterDoesNotMatchPropertyNames(t *testing.T) {
	alice := contact("acc", "alice", "Alice Example",
		vcard.FieldEmail+"=alice@example.com")
	contacts := store(alice)

	if got := len(Filter(contacts, []string{"email"})); got != 0 {
		t.Errorf("term matched a property name, got %d matches", got)
	}
}

func TestFilterRequiresEveryTerm(t *testing.T) {
	tomAto := contact("acc", "tom-ato", "Tom Ato",
		vcard.FieldName+"=Ato;Tom;;;")
	contacts := store(tomAto)

	if got := len(Filter(contacts, []string{"tom", "zzz"})); got != 0 {
		t.Errorf("a term that matches nothing still returned %d matches", got)
	}
}

func TestBuildViewsSortedByName(t *testing.T) {
	contacts := store(
		contact("acc", "c", "Charlie"),
		contact("acc", "a", "alice"),
		contact("acc", "b", "Bob"),
	)

	views := BuildViews(contacts)
	if len(views) != 3 {
		t.Fatalf("got %d views, want 3", len(views))
	}
	got := []string{views[0].FormattedName, views[1].FormattedName, views[2].FormattedName}
	want := []string{"alice", "Bob", "Charlie"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("views sorted as %v, want %v", got, want)
		}
	}
}

func TestBuildViewExtractsFields(t *testing.T) {
	c := contact("acc", "tom-ato", "Tom Ato",
		vcard.FieldOrganization+"=Vegetable Corp",
		vcard.FieldTitle+"=Chief Ripeness Officer",
		vcard.FieldEmail+"=tom.ato@example.com",
		vcard.FieldTelephone+"=+1-555-0201")

	views := BuildViews(store(c))
	if len(views) != 1 {
		t.Fatalf("got %d views, want 1", len(views))
	}
	v := views[0]
	if v.Organization != "Vegetable Corp" {
		t.Errorf("organization = %q", v.Organization)
	}
	if v.Title != "Chief Ripeness Officer" {
		t.Errorf("title = %q", v.Title)
	}
	if len(v.Emails) != 1 || v.Emails[0] != "tom.ato@example.com" {
		t.Errorf("emails = %v", v.Emails)
	}
	if len(v.Phones) != 1 || v.Phones[0] != "+1-555-0201" {
		t.Errorf("phones = %v", v.Phones)
	}
}

func names(contacts map[string]*addressobject.AddressObject) []string {
	var out []string
	for _, c := range contacts {
		out = append(out, c.FormattedName)
	}
	return out
}
