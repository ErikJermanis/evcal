package event

import (
	"strings"
	"testing"
)

func TestParseExample(t *testing.T) {
	events, err := Parse(Example)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.Title != "RIpost konferencija 2.0" || e.StartDate != "2026-11-19" || e.StartTime != "15:00" ||
		e.EndDate != "2026-11-20" || e.EndTime != "" || e.City != "Rijeka" {
		t.Errorf("unexpected event: %+v", e)
	}
}

func TestParseWholeAnswerWithSeveralBlocks(t *testing.T) {
	input := "Nove relevantne najave\n1. RIpost — [ulaznice](https://entrio.hr)\n\n```json\n" +
		`{"title": "A", "start_date": "2026-11-19"}` + "\n```\n\nVidi [1].\n```json\n" +
		`[{"title": "B", "start_date": "2026-11-11"}, {"name": "C", "date": "2026-11-12", "time": "9:00"}]` + "\n```"
	events, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, e := range events {
		titles = append(titles, e.Title)
	}
	if got := strings.Join(titles, ","); got != "A,B,C" {
		t.Fatalf("titles = %s, want A,B,C", got)
	}
	if events[2].StartTime != "09:00" {
		t.Errorf("alias time = %q, want 09:00", events[2].StartTime)
	}
}

func TestParseEventsWrapper(t *testing.T) {
	events, err := Parse(`{"events": [{"title": "A", "start_date": "2026-11-19"}]}`)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %v, err = %v", events, err)
	}
}

func TestNormalize(t *testing.T) {
	events, err := Parse(`{
		"title": "  Info   dan HBOR-a ",
		"start_date": "2026-11-11T10:30:00+01:00",
		"end_date": "2026-11-11",
		"end_time": "12:00:00",
		"price": 0,
		"organizer": null,
		"is_new": true,
		"url": "www.uo-rijeka.hr/info-dan?utm_source=chatgpt.com&id=7"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	e := events[0]
	want := Event{
		Title: "Info dan HBOR-a", StartDate: "2026-11-11", StartTime: "10:30", EndTime: "12:00",
		Price: "0", URL: "https://www.uo-rijeka.hr/info-dan?id=7",
	}
	if e != want {
		t.Errorf("got  %+v\nwant %+v", e, want)
	}
}

func TestParseErrors(t *testing.T) {
	for input, want := range map[string]string{
		"nema ničega":                            "Nisam pronašao",
		`{"title": "A", "start_date": `:          "JSON nije ispravan",
		`{"title": "A"}`:                         "start_date",
		`{"start_date": "2026-11-19"}`:           "naziv",
		`{"title": "A", "start_date": "19.11."}`: "GGGG-MM-DD",
		`{"title": "A", "start_date": "2026-11-19", "start_time": "popodne"}`:      "HH:MM",
		`{"title": "A", "start_date": "2026-11-19", "end_date": "2026-11-18"}`:     "prije",
		`{"title": "A", "start_date": "2026-11-19", "url": "javascript:alert(1)"}`: "http",
		`{"title": ["A"], "start_date": "2026-11-19"}`:                             "tekst",
		`[{"title": "A", "start_date": "2026-11-19"}, {"title": "B"}]`:             "Događaj 2",
	} {
		_, err := Parse(input)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%s) error = %v, want it to mention %q", input, err, want)
		}
	}
}

func TestJSONRoundTrip(t *testing.T) {
	events, err := Parse(Example)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Parse(events[0].JSON())
	if err != nil {
		t.Fatal(err)
	}
	if again[0] != events[0] {
		t.Errorf("round trip changed the event:\n%+v\n%+v", events[0], again[0])
	}
}

func TestDisplay(t *testing.T) {
	e := Event{StartDate: "2026-11-19", StartTime: "15:00", EndDate: "2026-11-20"}
	if got := e.When(); got != "četvrtak, 19. studenoga 2026., 15:00 – petak, 20. studenoga 2026." {
		t.Errorf("When = %q", got)
	}
	if got := e.Weekday() + " " + e.DayMonth() + " " + e.Until(); got != "čet 19.\u00a011. do 20.\u00a011." {
		t.Errorf("short date = %q", got)
	}
	single := Event{StartDate: "2026-11-11", StartTime: "09:00", EndTime: "12:00"}
	if got := single.When(); got != "srijeda, 11. studenoga 2026., 09:00–12:00" {
		t.Errorf("When = %q", got)
	}
	if got := single.Clock(); got != "09:00–12:00" {
		t.Errorf("Clock = %q", got)
	}
	for today, want := range map[string]string{
		"2026-11-07": "Za 12 dana",
		"2026-11-18": "Sutra",
		"2026-11-19": "Danas",
		"2026-11-20": "U tijeku",
		"2026-11-21": "",
		"2026-10-29": "Za 21 dan",
	} {
		if got := e.Relative(today); got != want {
			t.Errorf("Relative(%s) = %q, want %q", today, got, want)
		}
	}
	if !e.IsPast("2026-11-21") || e.IsPast("2026-11-20") {
		t.Error("IsPast should use the last day")
	}
	if got := e.Month(); got != "Studeni 2026" {
		t.Errorf("Month = %q", got)
	}
}

func TestPlural(t *testing.T) {
	for n, want := range map[int]string{1: "a", 2: "b", 4: "b", 5: "c", 11: "c", 12: "c", 21: "a", 22: "b", 111: "c"} {
		if got := Plural(n, "a", "b", "c"); got != want {
			t.Errorf("Plural(%d) = %s, want %s", n, got, want)
		}
	}
}
