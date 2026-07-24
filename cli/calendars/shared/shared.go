package shared

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/teambition/rrule-go"
	"xn--gckvb8fzb.com/inca/helpers/out"
	"xn--gckvb8fzb.com/inca/models/calendarobject"
	"xn--gckvb8fzb.com/inca/runtime"
	"xn--gckvb8fzb.com/zeit/helpers/timestamp"
)

const (
	FormatUnspecified = ""
	FormatCLI         = "cli"
	FormatJSON        = "json"
)

const maxOccurrences = 100000

type EventView struct {
	Key          string    `json:"key"`
	AccountName  string    `json:"account_name"`
	CalendarPath string    `json:"calendar_path"`
	UID          string    `json:"uid"`
	Summary      string    `json:"summary"`
	Location     string    `json:"location,omitempty"`
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	AllDay       bool      `json:"all_day"`
}

type eventGroup struct {
	master    *ical.Event
	overrides []*ical.Event
}

func ResolveRange(flagStart string, flagEnd string) (time.Time, time.Time, error) {
	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)

	if flagStart != "" {
		ts, err := timestamp.Parse(flagStart)
		if err != nil {
			return start, end, err
		}
		start = ts.Time
		if ts.IsRange {
			end = ts.ToTime
		}
	}

	if flagEnd != "" {
		ts, err := timestamp.Parse(flagEnd)
		if err != nil {
			return start, end, err
		}
		end = ts.Time
	}

	return start, end, nil
}

func BuildViews(
	objects map[string]*calendarobject.CalendarObject,
	start time.Time,
	end time.Time,
) []EventView {
	var views []EventView
	for _, co := range objects {
		views = append(views, buildViews(co, start, end)...)
	}

	sort.Slice(views, func(i, j int) bool {
		if views[i].Start.Equal(views[j].Start) {
			return strings.ToLower(views[i].Summary) <
				strings.ToLower(views[j].Summary)
		}
		return views[i].Start.Before(views[j].Start)
	})

	return views
}

func buildViews(
	co *calendarobject.CalendarObject,
	start time.Time,
	end time.Time,
) []EventView {
	cal, err := ical.NewDecoder(strings.NewReader(co.Data)).Decode()
	if err != nil {
		return nil
	}

	var views []EventView
	for _, group := range groupEvents(cal.Events()) {
		views = append(views, expandGroup(co, group, start, end)...)
	}
	return views
}

func groupEvents(events []ical.Event) []eventGroup {
	order := make([]string, 0, len(events))
	byUID := make(map[string]*eventGroup)
	anon := 0

	for i := range events {
		event := &events[i]

		key, _ := event.Props.Text(ical.PropUID)
		if key == "" {
			anon++
			key = fmt.Sprintf("\x00anon-%d", anon)
		}

		group, ok := byUID[key]
		if ok == false {
			group = new(eventGroup)
			byUID[key] = group
			order = append(order, key)
		}

		if event.Props.Get(ical.PropRecurrenceID) != nil || group.master != nil {
			group.overrides = append(group.overrides, event)
		} else {
			group.master = event
		}
	}

	groups := make([]eventGroup, 0, len(order))
	for _, key := range order {
		groups = append(groups, *byUID[key])
	}
	return groups
}

func expandGroup(
	co *calendarobject.CalendarObject,
	group eventGroup,
	start time.Time,
	end time.Time,
) []EventView {
	var views []EventView
	var overridden []time.Time

	for _, override := range group.overrides {
		if rid := override.Props.Get(ical.PropRecurrenceID); rid != nil {
			if t, err := rid.DateTime(time.Local); err == nil {
				overridden = append(overridden, t)
			}
		}
		if view, ok := singleView(co, override, start, end); ok {
			views = append(views, view)
		}
	}

	if group.master == nil {
		return views
	}

	masterStart, err := group.master.DateTimeStart(time.Local)
	if err != nil {
		return views
	}
	masterEnd, err := group.master.DateTimeEnd(time.Local)
	if err != nil || masterEnd.IsZero() {
		masterEnd = masterStart
	}
	duration := masterEnd.Sub(masterStart)

	roption, _ := group.master.Props.RecurrenceRule()
	rdates := propDateTimes(group.master.Props.Values(ical.PropRecurrenceDates))

	if roption == nil && len(rdates) == 0 {
		if view, ok := singleView(co, group.master, start, end); ok {
			views = append(views, view)
		}
		return views
	}

	set := new(rrule.Set)
	set.DTStart(masterStart)
	if roption != nil {
		roption.Dtstart = masterStart
		if r, err := rrule.NewRRule(*roption); err == nil {
			set.RRule(r)
		}
	} else {
		set.RDate(masterStart)
	}
	for _, d := range rdates {
		set.RDate(d)
	}
	for _, d := range propDateTimes(group.master.Props.Values(ical.PropExceptionDates)) {
		set.ExDate(d)
	}
	for _, d := range overridden {
		set.ExDate(d)
	}

	seen := make(map[int64]struct{})
	next := set.Iterator()
	for count := 0; count < maxOccurrences; count++ {
		occStart, ok := next()
		if ok == false || occStart.After(end) {
			break
		}
		if _, dup := seen[occStart.UnixNano()]; dup {
			continue
		}
		seen[occStart.UnixNano()] = struct{}{}

		occEnd := occStart.Add(duration)
		if overlaps(occStart, occEnd, start, end) == false {
			continue
		}
		views = append(views, viewFrom(co, group.master, occStart, occEnd))
	}

	return views
}

func singleView(
	co *calendarobject.CalendarObject,
	event *ical.Event,
	start time.Time,
	end time.Time,
) (EventView, bool) {
	evStart, err := event.DateTimeStart(time.Local)
	if err != nil {
		return EventView{}, false
	}
	evEnd, err := event.DateTimeEnd(time.Local)
	if err != nil || evEnd.IsZero() {
		evEnd = evStart
	}
	if overlaps(evStart, evEnd, start, end) == false {
		return EventView{}, false
	}
	return viewFrom(co, event, evStart, evEnd), true
}

func viewFrom(
	co *calendarobject.CalendarObject,
	event *ical.Event,
	evStart time.Time,
	evEnd time.Time,
) EventView {
	view := EventView{
		Key:          co.GetKey(),
		AccountName:  co.AccountName,
		CalendarPath: co.CalendarPath,
		UID:          co.UID,
		Start:        evStart,
		End:          evEnd,
		AllDay:       isAllDay(event),
	}
	if summary, err := event.Props.Text(ical.PropSummary); err == nil {
		view.Summary = summary
	}
	if location, err := event.Props.Text(ical.PropLocation); err == nil {
		view.Location = location
	}
	if view.Summary == "" {
		view.Summary = "(no summary)"
	}
	return view
}

func propDateTimes(props []ical.Prop) []time.Time {
	var out []time.Time
	for i := range props {
		for _, value := range strings.Split(props[i].Value, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			sub := ical.Prop{
				Name:   props[i].Name,
				Params: props[i].Params,
				Value:  value,
			}
			if t, err := sub.DateTime(time.Local); err == nil {
				out = append(out, t)
			}
		}
	}
	return out
}

func overlaps(evStart, evEnd, start, end time.Time) bool {
	if evEnd.After(evStart) {
		return evStart.Before(end) && evEnd.After(start)
	}
	return evStart.Before(end) == true && evStart.Before(start) == false
}

func isAllDay(event *ical.Event) bool {
	prop := event.Props.Get(ical.PropDateTimeStart)
	return prop != nil && prop.ValueType() == ical.ValueDate
}

func Output(rt *runtime.Runtime, format string, views []EventView) {
	switch strings.ToLower(format) {
	case FormatJSON:
		outputJSON(rt, views)
	default:
		outputCLI(rt, views)
	}
}

func outputCLI(rt *runtime.Runtime, views []EventView) {
	if len(views) == 0 {
		rt.Out.Put(out.Opts{Type: out.Info}, "No calendar entries found")
		return
	}

	for i := range views {
		view := views[i]

		rt.Out.Put(out.Opts{Type: out.Info},
			"%s %s",
			rt.Out.Stylize(
				out.Style{FG: out.ColorPrimary}, "%s", view.Summary),
			rt.Out.Stylize(
				out.Style{FG: out.ColorSecondary}, "(%s)", view.AccountName),
		)

		rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
			rt.Out.FG(out.ColorSecondary, "%-6s", "when"), formatWhen(view))

		if view.Location != "" {
			rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
				rt.Out.FG(out.ColorSecondary, "%-6s", "where"), view.Location)
		}
	}

	noun := "entries"
	if len(views) == 1 {
		noun = "entry"
	}
	rt.Out.Put(out.Opts{Type: out.Plain}, "%s",
		rt.Out.FG(out.ColorSecondary, "%d %s", len(views), noun))
}

func formatWhen(view EventView) string {
	if view.AllDay {
		last := view.End.Add(-24 * time.Hour)
		if last.After(view.Start) {
			return view.Start.Format("Mon 2006-01-02") + " → " +
				last.Format("Mon 2006-01-02") + " (all day)"
		}
		return view.Start.Format("Mon 2006-01-02") + " (all day)"
	}

	if view.Start.Year() == view.End.Year() &&
		view.Start.YearDay() == view.End.YearDay() {
		return view.Start.Format("Mon 2006-01-02 15:04") + " → " +
			view.End.Format("15:04")
	}

	return view.Start.Format("Mon 2006-01-02 15:04") + " → " +
		view.End.Format("Mon 2006-01-02 15:04")
}

func outputJSON(rt *runtime.Runtime, views []EventView) {
	prettyJSON, err := json.MarshalIndent(views, "", "  ")
	rt.NilOrDie(err)

	rt.Out.Put(out.Opts{Type: out.Plain}, "%s", string(prettyJSON))
}
