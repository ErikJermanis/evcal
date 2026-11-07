package store

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/ErikJermanis/evcal/event"
)

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data", "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestEvents(t *testing.T) {
	s := open(t)
	a := event.Event{Title: "RIpost", StartDate: "2026-11-19", URL: "https://entrio.hr/ripost"}
	id, updated, err := s.SaveEvent(a)
	if err != nil || updated {
		t.Fatalf("first save: id=%d updated=%v err=%v", id, updated, err)
	}

	// The same event pasted again (title differs, link matches) updates it.
	a.Title, a.Price = "RIpost 2.0", "50 €"
	again, updated, err := s.SaveEvent(a)
	if err != nil || !updated || again != id {
		t.Fatalf("second save: id=%d updated=%v err=%v", again, updated, err)
	}
	// A different event on the same day is a new row.
	if _, updated, err := s.SaveEvent(event.Event{Title: "Info dan", StartDate: "2026-11-19"}); err != nil || updated {
		t.Fatalf("other event: updated=%v err=%v", updated, err)
	}
	if _, _, err := s.SaveEvent(event.Event{Title: "Ranije", StartDate: "2026-11-02", StartTime: "09:00"}); err != nil {
		t.Fatal(err)
	}

	events, err := s.ListEvents()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[0].Title != "Ranije" || events[1].Title != "RIpost 2.0" || events[1].Price != "50 €" {
		t.Fatalf("unexpected list: %+v", events)
	}

	if err := s.DeleteEvent(id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetEvent(id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted event: err=%v", err)
	}
	if err := s.UpdateEvent(id, a); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update of deleted event: err=%v", err)
	}
}

func TestPairing(t *testing.T) {
	s := open(t)
	token, err := s.CreatePairingToken("Erik - iPhone")
	if err != nil {
		t.Fatal(err)
	}
	if name, err := s.PairingName(token); err != nil || name != "Erik - iPhone" {
		t.Fatalf("PairingName = %q, %v", name, err)
	}

	session, device, err := s.Pair(token)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Pair(token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token reused: err=%v", err)
	}
	if got, err := s.DeviceBySession(session); err != nil || got.ID != device.ID {
		t.Fatalf("DeviceBySession = %+v, %v", got, err)
	}
	if _, err := s.DeviceBySession("wrong"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong session: err=%v", err)
	}

	if err := s.RevokeDevice(device.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeviceBySession(session); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked session still works: err=%v", err)
	}
	devices, err := s.ListDevices()
	if err != nil || len(devices) != 1 || devices[0].RevokedAt == "" {
		t.Fatalf("devices = %+v, %v", devices, err)
	}
}

func TestExpiredPairingToken(t *testing.T) {
	s := open(t)
	token, err := s.CreatePairingToken("Stari")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE pairing_tokens SET expires_at = '2000-01-01T00:00:00Z'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Pair(token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired token paired: err=%v", err)
	}
}

func TestBackupAndReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "live.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SaveEvent(event.Event{Title: "A", StartDate: "2026-11-19"}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "backups", "copy.sqlite")
	if err := s.Backup(dest); err != nil {
		t.Fatal(err)
	}
	s.Close()

	copy, err := Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	if events, err := copy.ListEvents(); err != nil || len(events) != 1 {
		t.Fatalf("backup events = %v, %v", events, err)
	}
}
