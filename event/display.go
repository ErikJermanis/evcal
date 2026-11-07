package event

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	weekdaysShort = [...]string{"ned", "pon", "uto", "sri", "čet", "pet", "sub"}
	weekdaysLong  = [...]string{"nedjelja", "ponedjeljak", "utorak", "srijeda", "četvrtak", "petak", "subota"}
	// Nominative for month headings, genitive for dates ("19. studenoga").
	monthsNom = [...]string{"", "Siječanj", "Veljača", "Ožujak", "Travanj", "Svibanj", "Lipanj", "Srpanj", "Kolovoz", "Rujan", "Listopad", "Studeni", "Prosinac"}
	monthsGen = [...]string{"", "siječnja", "veljače", "ožujka", "travnja", "svibnja", "lipnja", "srpnja", "kolovoza", "rujna", "listopada", "studenoga", "prosinca"}
)

// Today returns the current date in Zagreb as YYYY-MM-DD.
func Today() string {
	return time.Now().In(Zagreb).Format(time.DateOnly)
}

func day(date string) time.Time {
	t, _ := time.Parse(time.DateOnly, date)
	return t
}

// LastDate is the final day of the event.
func (e Event) LastDate() string {
	if e.EndDate != "" {
		return e.EndDate
	}
	return e.StartDate
}

// IsPast reports whether the event ended before today.
func (e Event) IsPast(today string) bool { return e.LastDate() < today }

// Weekday is the short weekday of the first day, e.g. "čet".
func (e Event) Weekday() string { return weekdaysShort[day(e.StartDate).Weekday()] }

// DayMonth is the first day, e.g. "19. 11.".
func (e Event) DayMonth() string { return shortDate(e.StartDate) }

// Until describes the last day of a multi-day event, e.g. "do 20. 11.".
func (e Event) Until() string {
	if e.EndDate == "" {
		return ""
	}
	return "do " + shortDate(e.EndDate)
}

// shortDate is e.g. "19. 11." with a no-break space, so it never wraps.
func shortDate(date string) string {
	t := day(date)
	return fmt.Sprintf("%d.\u00a0%d.", t.Day(), int(t.Month()))
}

// Clock is the start time, with the end time on single-day events.
func (e Event) Clock() string {
	if e.StartTime != "" && e.EndTime != "" && e.EndDate == "" {
		return e.StartTime + "–" + e.EndTime
	}
	return e.StartTime
}

// Relative says how far away the event is: "Danas", "Sutra", "Za 5 dana".
func (e Event) Relative(today string) string {
	now := day(today)
	start, last := day(e.StartDate), day(e.LastDate())
	switch {
	case last.Before(now):
		return ""
	case start.Before(now):
		return "U tijeku"
	case start.Equal(now):
		return "Danas"
	}
	n := int(start.Sub(now).Hours() / 24)
	if n == 1 {
		return "Sutra"
	}
	return fmt.Sprintf("Za %d %s", n, Plural(n, "dan", "dana", "dana"))
}

// When is the full date and time, e.g.
// "četvrtak, 19. studenoga 2026., 15:00 – petak, 20. studenoga 2026.".
func (e Event) When() string {
	s := longDate(e.StartDate)
	if e.StartTime != "" {
		s += ", " + e.StartTime
	}
	if e.EndDate != "" {
		s += " – " + longDate(e.EndDate)
		if e.EndTime != "" {
			s += ", " + e.EndTime
		}
	} else if e.EndTime != "" {
		s += "–" + e.EndTime
	}
	return s
}

func longDate(date string) string {
	t := day(date)
	return fmt.Sprintf("%s, %d. %s %d.", weekdaysLong[t.Weekday()], t.Day(), monthsGen[t.Month()], t.Year())
}

// Month is the heading the event is grouped under, e.g. "Studeni 2026".
func (e Event) Month() string {
	t := day(e.StartDate)
	return fmt.Sprintf("%s %d", monthsNom[t.Month()], t.Year())
}

// Where joins the venue and city, skipping whichever is missing.
func (e Event) Where() string {
	parts := make([]string, 0, 2)
	if e.Venue != "" {
		parts = append(parts, e.Venue)
	}
	if e.City != "" && !strings.Contains(e.Venue, e.City) {
		parts = append(parts, e.City)
	}
	return strings.Join(parts, ", ")
}

// Host is the link's domain without "www.", used as the link label.
func (e Event) Host() string {
	u, err := url.Parse(e.URL)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// Plural picks the Croatian form for n: one (1, 21…), few (2–4, 22–24…), many.
func Plural(n int, one, few, many string) string {
	if n%10 == 1 && n%100 != 11 {
		return one
	}
	if n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14) {
		return few
	}
	return many
}
