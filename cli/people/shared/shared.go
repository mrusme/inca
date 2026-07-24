package shared

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/emersion/go-vcard"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/models/addressobject"
	"xn--gckvb8fzb.com/inca/runtime"
)

const (
	FormatUnspecified = ""
	FormatCLI         = "cli"
	FormatJSON        = "json"
)

type ContactView struct {
	Key           string   `json:"key"`
	AccountName   string   `json:"account_name"`
	UID           string   `json:"uid"`
	FormattedName string   `json:"formatted_name"`
	Organization  string   `json:"organization,omitempty"`
	Title         string   `json:"title,omitempty"`
	Emails        []string `json:"emails,omitempty"`
	Phones        []string `json:"phones,omitempty"`
}

func BuildViews(
	contacts map[string]*addressobject.AddressObject,
) []ContactView {
	views := make([]ContactView, 0, len(contacts))
	for _, ao := range contacts {
		views = append(views, buildView(ao))
	}

	sort.Slice(views, func(i, j int) bool {
		return strings.ToLower(views[i].FormattedName) <
			strings.ToLower(views[j].FormattedName)
	})

	return views
}

func buildView(ao *addressobject.AddressObject) ContactView {
	view := ContactView{
		Key:           ao.GetKey(),
		AccountName:   ao.AccountName,
		UID:           ao.UID,
		FormattedName: ao.FormattedName,
	}

	if card, err := decodeCard(ao); err == nil {
		if view.FormattedName == "" {
			view.FormattedName = card.PreferredValue(vcard.FieldFormattedName)
		}
		view.Organization = card.PreferredValue(vcard.FieldOrganization)
		view.Title = card.PreferredValue(vcard.FieldTitle)
		view.Emails = nonEmptyValues(card[vcard.FieldEmail])
		view.Phones = nonEmptyValues(card[vcard.FieldTelephone])
	}

	if view.FormattedName == "" {
		view.FormattedName = "(no name)"
	}

	return view
}

func Filter(
	contacts map[string]*addressobject.AddressObject,
	terms []string,
) map[string]*addressobject.AddressObject {
	needles := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" {
			needles = append(needles, term)
		}
	}

	matched := make(map[string]*addressobject.AddressObject)
	for key, ao := range contacts {
		if matchesTerms(ao, needles) {
			matched[key] = ao
		}
	}

	return matched
}

func matchesTerms(ao *addressobject.AddressObject, needles []string) bool {
	values := contactValues(ao)

	for _, needle := range needles {
		found := false
		for _, value := range values {
			if strings.Contains(value, needle) {
				found = true
				break
			}
		}
		if found == false {
			return false
		}
	}

	return true
}

func contactValues(ao *addressobject.AddressObject) []string {
	var values []string

	if card, err := decodeCard(ao); err == nil {
		for _, fields := range card {
			for _, field := range fields {
				if field.Value != "" {
					values = append(values, strings.ToLower(field.Value))
				}
			}
		}
	}

	if ao.FormattedName != "" {
		values = append(values, strings.ToLower(ao.FormattedName))
	}

	return values
}

func decodeCard(ao *addressobject.AddressObject) (vcard.Card, error) {
	return vcard.NewDecoder(strings.NewReader(ao.Data)).Decode()
}

func nonEmptyValues(fields []*vcard.Field) []string {
	var values []string
	for _, field := range fields {
		if field.Value != "" {
			values = append(values, field.Value)
		}
	}
	return values
}

func Output(rt *runtime.Runtime, format string, views []ContactView) {
	switch strings.ToLower(format) {
	case FormatJSON:
		outputJSON(rt, views)
	default:
		outputCLI(rt, views)
	}
}

func outputCLI(rt *runtime.Runtime, views []ContactView) {
	if len(views) == 0 {
		rt.Out.Put(out.Opts{Type: out.Info}, "No contacts found")
		return
	}

	for i := range views {
		view := views[i]

		rt.Out.Put(out.Opts{Type: out.Info},
			"%s %s",
			rt.Out.Stylize(
				out.Style{FG: out.ColorPrimary}, "%s", view.FormattedName),
			rt.Out.Stylize(
				out.Style{FG: out.ColorSecondary}, "(%s)", view.AccountName),
		)

		if org := organizationLine(view); org != "" {
			rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
				rt.Out.FG(out.ColorSecondary, "%-6s", "org"), org)
		}
		if len(view.Emails) > 0 {
			rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
				rt.Out.FG(out.ColorSecondary, "%-6s", "email"),
				strings.Join(view.Emails, ", "))
		}
		if len(view.Phones) > 0 {
			rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
				rt.Out.FG(out.ColorSecondary, "%-6s", "phone"),
				strings.Join(view.Phones, ", "))
		}
	}

	noun := "contacts"
	if len(views) == 1 {
		noun = "contact"
	}
	rt.Out.Put(out.Opts{Type: out.Plain}, "%s",
		rt.Out.FG(out.ColorSecondary, "%d %s", len(views), noun))
}

func organizationLine(view ContactView) string {
	switch {
	case view.Organization != "" && view.Title != "":
		return view.Title + ", " + view.Organization
	case view.Organization != "":
		return view.Organization
	case view.Title != "":
		return view.Title
	}
	return ""
}

func outputJSON(rt *runtime.Runtime, views []ContactView) {
	prettyJSON, err := json.MarshalIndent(views, "", "  ")
	rt.NilOrDie(err)

	rt.Out.Put(out.Opts{Type: out.Plain}, "%s", string(prettyJSON))
}
