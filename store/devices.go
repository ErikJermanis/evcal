package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// PairingTTL is how long an unused pairing link stays valid.
const PairingTTL = 24 * time.Hour

type Device struct {
	ID         int64
	Name       string
	CreatedAt  string
	LastSeenAt string
	RevokedAt  string
}

// Only SHA-256 hashes of pairing and session tokens are stored, so a copy of
// the database cannot be used to sign in.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() string {
	b := make([]byte, 32)
	rand.Read(b) // never returns an error since Go 1.24
	return base64.RawURLEncoding.EncodeToString(b)
}

// CreatePairingToken returns a one-use token that pairs a device called name.
func (s *Store) CreatePairingToken(name string) (string, error) {
	token := newToken()
	t := time.Now().UTC()
	_, err := s.db.Exec(`INSERT INTO pairing_tokens (token_hash, device_name, created_at, expires_at)
		VALUES (?, ?, ?, ?)`,
		hashToken(token), name, t.Format(time.RFC3339), t.Add(PairingTTL).Format(time.RFC3339))
	return token, err
}

// PairingName returns the device name for a valid, unused pairing token.
func (s *Store) PairingName(token string) (string, error) {
	var name string
	err := s.db.QueryRow(`SELECT device_name FROM pairing_tokens
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
		hashToken(token), now()).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}

// Pair consumes a pairing token and registers a device, returning the
// long-lived session token to store in the device's cookie.
func (s *Store) Pair(token string) (session string, device Device, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", Device{}, err
	}
	defer tx.Rollback()

	t := now()
	err = tx.QueryRow(`UPDATE pairing_tokens SET used_at = ?
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?
		RETURNING device_name`, t, hashToken(token), t).Scan(&device.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", Device{}, ErrNotFound
	}
	if err != nil {
		return "", Device{}, err
	}

	session = newToken()
	res, err := tx.Exec(`INSERT INTO devices (name, token_hash, created_at, last_seen_at)
		VALUES (?, ?, ?, ?)`, device.Name, hashToken(session), t, t)
	if err != nil {
		return "", Device{}, err
	}
	if device.ID, err = res.LastInsertId(); err != nil {
		return "", Device{}, err
	}
	device.CreatedAt, device.LastSeenAt = t, t
	return session, device, tx.Commit()
}

// DeviceBySession returns the active device holding session, and records
// that it was seen.
func (s *Store) DeviceBySession(session string) (Device, error) {
	var d Device
	err := s.db.QueryRow(`UPDATE devices SET last_seen_at = ?
		WHERE token_hash = ? AND revoked_at IS NULL
		RETURNING id, name, created_at, last_seen_at`,
		now(), hashToken(session)).Scan(&d.ID, &d.Name, &d.CreatedAt, &d.LastSeenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

func (s *Store) ListDevices() ([]Device, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at, last_seen_at, COALESCE(revoked_at, '')
		FROM devices ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.CreatedAt, &d.LastSeenAt, &d.RevokedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// RevokeDevice signs a device out; it takes effect on its next request.
func (s *Store) RevokeDevice(id int64) error {
	return affected(s.db.Exec(`UPDATE devices SET revoked_at = ?
		WHERE id = ? AND revoked_at IS NULL`, now(), id))
}
