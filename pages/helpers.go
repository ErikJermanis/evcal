package pages

import (
	"fmt"
	"strings"

	"github.com/ErikJermanis/evcal/event"
)

// AssetVersion fingerprints styles.css and app.js; set once at startup.
var AssetVersion string

func asset(name string) string { return "/assets/" + name + "?v=" + AssetVersion }

type Month struct {
	Label  string
	Events []event.Event
}

type ListData struct {
	Device        string
	Today         string
	Upcoming      []Month
	UpcomingCount int
	Past          []event.Event
	PastOpen      bool
	Notice        string
	Highlight     map[int64]bool
}

type FormData struct {
	Device string
	EditID int64  // 0 when adding
	Title  string // title of the event being edited
	Input  string
	Error  string
}

func (d FormData) action() string {
	if d.EditID != 0 {
		return fmt.Sprintf("/events/%d", d.EditID)
	}
	return "/events"
}

func countLabel(n int) string {
	if n == 0 {
		return "Nema nadolazećih događaja."
	}
	return fmt.Sprintf("%d %s %s", n,
		event.Plural(n, "nadolazeći", "nadolazeća", "nadolazećih"),
		event.Plural(n, "događaj", "događaja", "događaja"))
}

func anchor(e event.Event) string { return fmt.Sprintf("event-%d", e.ID) }

func editURL(e event.Event) string { return fmt.Sprintf("/events/%d/edit", e.ID) }

func deleteURL(e event.Event) string { return fmt.Sprintf("/events/%d/delete", e.ID) }

// summary is the one-line teaser under the title: venue and price.
func summary(e event.Event) string {
	parts := make([]string, 0, 2)
	if e.Venue != "" {
		parts = append(parts, e.Venue)
	}
	if e.Price != "" {
		parts = append(parts, e.Price)
	}
	return strings.Join(parts, " · ")
}
