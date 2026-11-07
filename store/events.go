package store

import (
	"database/sql"
	"errors"

	"github.com/ErikJermanis/evcal/event"
)

const eventColumns = `id, title, type, start_date, start_time, end_date, end_time,
	city, venue, organizer, price, audience, why, url`

func scanEvent(row interface{ Scan(...any) error }) (event.Event, error) {
	var e event.Event
	err := row.Scan(&e.ID, &e.Title, &e.Type, &e.StartDate, &e.StartTime, &e.EndDate, &e.EndTime,
		&e.City, &e.Venue, &e.Organizer, &e.Price, &e.Audience, &e.Why, &e.URL)
	return e, err
}

// ListEvents returns every event in chronological order.
func (s *Store) ListEvents() ([]event.Event, error) {
	rows, err := s.db.Query(`SELECT ` + eventColumns + ` FROM events ORDER BY start_date, start_time, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []event.Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (s *Store) GetEvent(id int64) (event.Event, error) {
	e, err := scanEvent(s.db.QueryRow(`SELECT `+eventColumns+` FROM events WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrNotFound
	}
	return e, err
}

// SaveEvent inserts e, or updates the existing event on the same start date
// with the same title or link, so pasting an event twice does not duplicate
// it. It returns the event's id and whether an existing event was updated.
func (s *Store) SaveEvent(e event.Event) (id int64, updated bool, err error) {
	err = s.db.QueryRow(`SELECT id FROM events
		WHERE start_date = ? AND (lower(title) = lower(?) OR (url != '' AND url = ?))
		ORDER BY id LIMIT 1`, e.StartDate, e.Title, e.URL).Scan(&id)
	switch {
	case err == nil:
		return id, true, s.UpdateEvent(id, e)
	case !errors.Is(err, sql.ErrNoRows):
		return 0, false, err
	}
	t := now()
	res, err := s.db.Exec(`INSERT INTO events
		(title, type, start_date, start_time, end_date, end_time, city, venue,
		 organizer, price, audience, why, url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Title, e.Type, e.StartDate, e.StartTime, e.EndDate, e.EndTime, e.City, e.Venue,
		e.Organizer, e.Price, e.Audience, e.Why, e.URL, t, t)
	if err != nil {
		return 0, false, err
	}
	id, err = res.LastInsertId()
	return id, false, err
}

func (s *Store) UpdateEvent(id int64, e event.Event) error {
	res, err := s.db.Exec(`UPDATE events SET
		title = ?, type = ?, start_date = ?, start_time = ?, end_date = ?, end_time = ?,
		city = ?, venue = ?, organizer = ?, price = ?, audience = ?, why = ?, url = ?,
		updated_at = ?
		WHERE id = ?`,
		e.Title, e.Type, e.StartDate, e.StartTime, e.EndDate, e.EndTime,
		e.City, e.Venue, e.Organizer, e.Price, e.Audience, e.Why, e.URL, now(), id)
	return affected(res, err)
}

func (s *Store) DeleteEvent(id int64) error {
	return affected(s.db.Exec(`DELETE FROM events WHERE id = ?`, id))
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
