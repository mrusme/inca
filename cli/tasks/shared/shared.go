package shared

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
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

type TaskView struct {
	Key             string     `json:"key"`
	AccountName     string     `json:"account_name"`
	CalendarPath    string     `json:"calendar_path"`
	UID             string     `json:"uid"`
	Summary         string     `json:"summary"`
	Status          string     `json:"status,omitempty"`
	Priority        int        `json:"priority,omitempty"`
	PercentComplete int        `json:"percent_complete,omitempty"`
	Due             *time.Time `json:"due,omitempty"`
	DueAllDay       bool       `json:"due_all_day,omitempty"`
	Start           *time.Time `json:"start,omitempty"`
}

func ResolveRange(flagStart string, flagEnd string) (time.Time, time.Time, error) {
	var start, end time.Time

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
) []TaskView {
	var views []TaskView
	for _, co := range objects {
		views = append(views, buildViews(co, start, end)...)
	}

	sort.Slice(views, func(i, j int) bool {
		a, b := views[i], views[j]
		if (a.Due == nil) != (b.Due == nil) {
			return a.Due != nil
		}
		if a.Due != nil && b.Due != nil && a.Due.Equal(*b.Due) == false {
			return a.Due.Before(*b.Due)
		}
		return strings.ToLower(a.Summary) < strings.ToLower(b.Summary)
	})

	return views
}

func buildViews(
	co *calendarobject.CalendarObject,
	start time.Time,
	end time.Time,
) []TaskView {
	cal, err := ical.NewDecoder(strings.NewReader(co.Data)).Decode()
	if err != nil {
		return nil
	}

	var views []TaskView
	for _, child := range cal.Children {
		if child.Name != ical.CompToDo {
			continue
		}
		view := buildView(co, child)
		if withinDue(view.Due, start, end) == false {
			continue
		}
		views = append(views, view)
	}
	return views
}

func buildView(co *calendarobject.CalendarObject, todo *ical.Component) TaskView {
	view := TaskView{
		Key:          co.GetKey(),
		AccountName:  co.AccountName,
		CalendarPath: co.CalendarPath,
		UID:          co.UID,
	}

	if summary, err := todo.Props.Text(ical.PropSummary); err == nil {
		view.Summary = summary
	}
	if view.Summary == "" {
		view.Summary = "(no summary)"
	}
	if status, err := todo.Props.Text(ical.PropStatus); err == nil {
		view.Status = status
	}
	if due, ok := propTime(todo, ical.PropDue); ok {
		view.Due = &due
		view.DueAllDay = isDate(todo, ical.PropDue)
	}
	if begin, ok := propTime(todo, ical.PropDateTimeStart); ok {
		view.Start = &begin
	}
	if prop := todo.Props.Get(ical.PropPriority); prop != nil {
		if n, err := prop.Int(); err == nil {
			view.Priority = n
		}
	}
	if prop := todo.Props.Get(ical.PropPercentComplete); prop != nil {
		if n, err := prop.Int(); err == nil {
			view.PercentComplete = n
		}
	}

	return view
}

func withinDue(due *time.Time, start time.Time, end time.Time) bool {
	if start.IsZero() && end.IsZero() {
		return true
	}
	if due == nil {
		return false
	}
	if start.IsZero() == false && due.Before(start) {
		return false
	}
	if end.IsZero() == false && due.After(end) {
		return false
	}
	return true
}

func propTime(comp *ical.Component, name string) (time.Time, bool) {
	prop := comp.Props.Get(name)
	if prop == nil {
		return time.Time{}, false
	}
	t, err := prop.DateTime(time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func isDate(comp *ical.Component, name string) bool {
	prop := comp.Props.Get(name)
	return prop != nil && prop.ValueType() == ical.ValueDate
}

func Output(rt *runtime.Runtime, format string, views []TaskView) {
	switch strings.ToLower(format) {
	case FormatJSON:
		outputJSON(rt, views)
	default:
		outputCLI(rt, views)
	}
}

func outputCLI(rt *runtime.Runtime, views []TaskView) {
	if len(views) == 0 {
		rt.Out.Put(out.Opts{Type: out.Info}, "No tasks found")
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

		if due := formatDue(view); due != "" {
			rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
				rt.Out.FG(out.ColorSecondary, "%-6s", "due"), due)
		}
		if view.Status != "" {
			rt.Out.Put(out.Opts{Type: out.Plain}, "  %s %s",
				rt.Out.FG(out.ColorSecondary, "%-6s", "status"), view.Status)
		}
	}

	noun := "tasks"
	if len(views) == 1 {
		noun = "task"
	}
	rt.Out.Put(out.Opts{Type: out.Plain}, "%s",
		rt.Out.FG(out.ColorSecondary, "%d %s", len(views), noun))
}

func formatDue(view TaskView) string {
	if view.Due == nil {
		return ""
	}
	if view.DueAllDay {
		return view.Due.Format("Mon 2006-01-02")
	}
	return view.Due.Format("Mon 2006-01-02 15:04")
}

func outputJSON(rt *runtime.Runtime, views []TaskView) {
	prettyJSON, err := json.MarshalIndent(views, "", "  ")
	rt.NilOrDie(err)

	rt.Out.Put(out.Opts{Type: out.Plain}, "%s", string(prettyJSON))
}
