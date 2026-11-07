// Package event defines the event record, parses pasted ChatGPT JSON into it,
// and formats it for Croatian display.
package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata" // the server may lack a system tz database
)

// Zagreb is the time zone used for "today" and every displayed date.
var Zagreb = mustLocation("Europe/Zagreb")

// Event is one stored event. The JSON names are the contract with the
// ChatGPT prompt (see PROMPT.md); change them in both places together.
type Event struct {
	ID        int64  `json:"-"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	StartDate string `json:"start_date"` // YYYY-MM-DD, required
	StartTime string `json:"start_time"` // HH:MM or ""
	EndDate   string `json:"end_date"`   // YYYY-MM-DD or "" for single-day events
	EndTime   string `json:"end_time"`   // HH:MM or ""
	City      string `json:"city"`
	Venue     string `json:"venue"`
	Organizer string `json:"organizer"`
	Price     string `json:"price"`
	Audience  string `json:"audience"`
	Why       string `json:"why"`
	URL       string `json:"url"`
}

// Example is shown on the add page as a reference for the expected format.
const Example = `{
  "title": "RIpost konferencija 2.0",
  "type": "konferencija",
  "start_date": "2026-11-19",
  "start_time": "15:00",
  "end_date": "2026-11-20",
  "end_time": null,
  "city": "Rijeka",
  "venue": "Circolo / Palača Modello, Uljarska 1",
  "organizer": "RIpost",
  "price": "50 €, prijava preko Entria",
  "audience": "Marketingaši, vlasnici digitalnih biznisa, agencije, poduzetnici",
  "why": "Publika je ciljna skupina; AI blok i večernje druženje otvaraju razgovor o automatizaciji.",
  "url": "https://www.entrio.hr/event/ripost-konferencija-20-34319"
}`

// fields maps accepted JSON keys (including a few aliases a model might use)
// to the event field they fill. The first key per field is canonical.
var fields = []struct {
	keys []string
	ptr  func(*Event) *string
}{
	{[]string{"title", "name"}, func(e *Event) *string { return &e.Title }},
	{[]string{"type"}, func(e *Event) *string { return &e.Type }},
	{[]string{"start_date", "date"}, func(e *Event) *string { return &e.StartDate }},
	{[]string{"start_time", "time"}, func(e *Event) *string { return &e.StartTime }},
	{[]string{"end_date"}, func(e *Event) *string { return &e.EndDate }},
	{[]string{"end_time"}, func(e *Event) *string { return &e.EndTime }},
	{[]string{"city"}, func(e *Event) *string { return &e.City }},
	{[]string{"venue", "location"}, func(e *Event) *string { return &e.Venue }},
	{[]string{"organizer"}, func(e *Event) *string { return &e.Organizer }},
	{[]string{"price"}, func(e *Event) *string { return &e.Price }},
	{[]string{"audience"}, func(e *Event) *string { return &e.Audience }},
	{[]string{"why"}, func(e *Event) *string { return &e.Why }},
	{[]string{"url", "link"}, func(e *Event) *string { return &e.URL }},
}

// Parse extracts every event from pasted text. It accepts a single object, an
// array of objects, or {"events": [...]}, and tolerates surrounding prose or
// Markdown code fences, so pasting a whole ChatGPT answer also works.
func Parse(input string) ([]Event, error) {
	var events []Event
	var firstErr error
	for offset := 0; offset < len(input); {
		i := strings.IndexAny(input[offset:], "{[")
		if i < 0 {
			break
		}
		start := offset + i
		dec := json.NewDecoder(strings.NewReader(input[start:]))
		dec.UseNumber()
		var value any
		if err := dec.Decode(&value); err != nil {
			if firstErr == nil && input[start] == '{' {
				firstErr = fmt.Errorf("JSON nije ispravan: %v", err)
			}
			offset = start + 1
			continue
		}
		found, err := collect(value, len(events))
		if err != nil {
			return nil, err
		}
		if len(found) == 0 { // e.g. a "[1]" footnote in surrounding prose
			offset = start + 1
			continue
		}
		events = append(events, found...)
		offset = start + int(dec.InputOffset())
	}
	if len(events) > 0 {
		return events, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, errors.New("Nisam pronašao nijedan događaj. Zalijepi JSON blok koji počinje s { ili [.")
}

// collect returns the events in a decoded JSON value, or none when the value
// does not look like event data. before is the number of events parsed so far,
// used to number error messages.
func collect(value any, before int) ([]Event, error) {
	var items []any
	switch v := value.(type) {
	case []any:
		items = v
	case map[string]any:
		if list, ok := v["events"].([]any); ok {
			items = list
		} else {
			items = []any{v}
		}
	}
	var events []Event
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok || !looksLikeEvent(obj) {
			continue
		}
		e, err := fromObject(obj)
		if err != nil && (before > 0 || len(items) > 1) {
			return nil, fmt.Errorf("Događaj %d: %w", before+len(events)+1, err)
		}
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func looksLikeEvent(obj map[string]any) bool {
	for _, key := range []string{"title", "name", "start_date", "date"} {
		if _, ok := obj[key]; ok {
			return true
		}
	}
	return false
}

func fromObject(obj map[string]any) (Event, error) {
	var e Event
	for _, f := range fields {
		for _, key := range f.keys {
			raw, ok := obj[key]
			if !ok {
				continue
			}
			switch v := raw.(type) {
			case nil:
			case string:
				*f.ptr(&e) = v
			case json.Number:
				*f.ptr(&e) = v.String()
			default:
				return Event{}, fmt.Errorf("polje %q mora biti tekst", key)
			}
			break
		}
	}
	return e, e.Normalize()
}

// Normalize trims every field, canonicalises dates, times and the URL, and
// reports the first problem in Croatian.
func (e *Event) Normalize() error {
	for _, f := range fields {
		p := f.ptr(e)
		*p = strings.Join(strings.Fields(*p), " ")
	}
	if e.Title == "" {
		return errors.New("nedostaje naziv (title)")
	}
	if e.StartDate == "" {
		return errors.New("nedostaje datum početka (start_date)")
	}

	var err error
	if e.StartDate, e.StartTime, err = splitDateTime("start_date", e.StartDate, e.StartTime); err != nil {
		return err
	}
	if e.EndDate, e.EndTime, err = splitDateTime("end_date", e.EndDate, e.EndTime); err != nil {
		return err
	}
	if e.StartTime, err = normalizeTime("start_time", e.StartTime); err != nil {
		return err
	}
	if e.EndTime, err = normalizeTime("end_time", e.EndTime); err != nil {
		return err
	}
	if e.EndDate == e.StartDate {
		e.EndDate = ""
	}
	if e.EndDate != "" && e.EndDate < e.StartDate {
		return errors.New("end_date je prije start_date")
	}
	if e.URL, err = normalizeURL(e.URL); err != nil {
		return err
	}
	return nil
}

// splitDateTime accepts "2026-11-19" and also tolerates a combined
// "2026-11-19T15:00[:00][+01:00]", moving the time into its own field.
func splitDateTime(name, date, clock string) (string, string, error) {
	if date == "" {
		return "", clock, nil
	}
	if len(date) > 10 && (date[10] == 'T' || date[10] == ' ') {
		if clock == "" && len(date) >= 16 {
			clock = date[11:16]
		}
		date = date[:10]
	}
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return "", "", fmt.Errorf("%s mora biti u obliku GGGG-MM-DD (npr. 2026-11-19), a ne %q", name, date)
	}
	return date, clock, nil
}

func normalizeTime(name, clock string) (string, error) {
	if clock == "" {
		return "", nil
	}
	for _, layout := range []string{"15:04", "15:04:05", "15.04"} {
		if t, err := time.Parse(layout, clock); err == nil {
			return t.Format("15:04"), nil
		}
	}
	return "", fmt.Errorf("%s mora biti u obliku HH:MM (npr. 09:30), a ne %q", name, clock)
}

func normalizeURL(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("url mora biti http(s) poveznica, a ne %q", raw)
	}
	// ChatGPT appends utm_source=chatgpt.com to links; drop tracking params.
	if u.RawQuery != "" {
		q := u.Query()
		for key := range q {
			if strings.HasPrefix(strings.ToLower(key), "utm_") {
				q.Del(key)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// JSON returns the event in the paste format, for editing.
func (e Event) JSON() string {
	out, _ := json.MarshalIndent(e, "", "  ")
	return string(out)
}

func mustLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}
