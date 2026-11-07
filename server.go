package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ErikJermanis/evcal/event"
	"github.com/ErikJermanis/evcal/pages"
	"github.com/ErikJermanis/evcal/store"
	"github.com/a-h/templ"
)

//go:embed public
var publicFiles embed.FS

const (
	sessionCookie = "evcal_session"
	// Browsers cap cookies at ~400 days; the cookie is re-issued on every
	// visit, so a device stays paired as long as it is used once a year.
	sessionMaxAge = 400 * 24 * 60 * 60
	maxFormBytes  = 1 << 20
)

type server struct {
	cfg   config
	store *store.Store
}

func newServer(cfg config, st *store.Store) (http.Handler, error) {
	static, err := fs.Sub(publicFiles, "public")
	if err != nil {
		return nil, err
	}
	if pages.AssetVersion, err = assetVersion(static); err != nil {
		return nil, err
	}
	s := &server{cfg: cfg, store: st}

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", cacheAssets(http.FileServer(http.FS(static)))))
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "favicon.ico")
	})
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "User-agent: *\nDisallow: /\n")
	})
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /pair/{token}", s.pairPage)
	mux.HandleFunc("POST /pair", s.pair)

	mux.HandleFunc("GET /{$}", s.private(s.list))
	mux.HandleFunc("GET /new", s.private(s.newForm))
	mux.HandleFunc("POST /events", s.private(s.create))
	mux.HandleFunc("GET /events/{id}/edit", s.private(s.editForm))
	mux.HandleFunc("POST /events/{id}", s.private(s.update))
	mux.HandleFunc("POST /events/{id}/delete", s.private(s.delete))

	// Rejects cross-site form posts (Sec-Fetch-Site / Origin checks).
	return securityHeaders(http.NewCrossOriginProtection().Handler(mux)), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; "+
			"style-src 'self' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; "+
			"img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}

// assetVersion fingerprints the CSS and JS so a deploy busts browser caches.
func assetVersion(static fs.FS) (string, error) {
	h := sha256.New()
	for _, name := range []string{"styles.css", "app.js"} {
		b, err := fs.ReadFile(static, name)
		if err != nil {
			return "", err
		}
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))[:10], nil
}

func cacheAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		next.ServeHTTP(w, r)
	})
}

// private serves h only to paired devices; others get the pairing page.
func (s *server) private(h func(http.ResponseWriter, *http.Request, store.Device)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil {
			device, err := s.store.DeviceBySession(c.Value)
			if err == nil {
				s.setSession(w, c.Value)
				w.Header().Set("Cache-Control", "no-store")
				h(w, r, device)
				return
			}
			if !errors.Is(err, store.ErrNotFound) {
				s.fail(w, err)
				return
			}
		}
		render(w, r, http.StatusUnauthorized, pages.Unauthorized(""))
	}
}

func (s *server) setSession(w http.ResponseWriter, session string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    session,
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		Secure:   s.cfg.secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	fmt.Fprint(w, "ok")
}

// pairPage asks for confirmation instead of pairing on GET, so link
// previews in chat apps cannot use up the one-time link.
func (s *server) pairPage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	name, err := s.store.PairingName(token)
	if errors.Is(err, store.ErrNotFound) {
		render(w, r, http.StatusGone, pages.Unauthorized(invalidPairing))
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	render(w, r, http.StatusOK, pages.Pair(token, name))
}

const invalidPairing = "Poveznica za uparivanje nije valjana, istekla je ili je već iskorištena. Zatraži novu."

func (s *server) pair(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	session, _, err := s.store.Pair(pairingToken(r.FormValue("token")))
	if errors.Is(err, store.ErrNotFound) {
		render(w, r, http.StatusGone, pages.Unauthorized(invalidPairing))
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	s.setSession(w, session)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// pairingToken accepts either the bare token or the whole pairing link.
func pairingToken(input string) string {
	input = strings.TrimSpace(input)
	if i := strings.LastIndex(input, "/pair/"); i >= 0 {
		input = input[i+len("/pair/"):]
	}
	if i := strings.IndexAny(input, "/?#"); i >= 0 {
		input = input[:i]
	}
	return input
}

func (s *server) list(w http.ResponseWriter, r *http.Request, device store.Device) {
	events, err := s.store.ListEvents()
	if err != nil {
		s.fail(w, err)
		return
	}
	q := r.URL.Query()
	saved, updated := ids(q.Get("saved")), ids(q.Get("updated"))
	data := pages.ListData{
		Device:    device.Name,
		Today:     event.Today(),
		Highlight: map[int64]bool{},
	}
	for _, id := range append(saved, updated...) {
		data.Highlight[id] = true
	}
	for _, e := range events {
		if e.IsPast(data.Today) {
			data.Past = append([]event.Event{e}, data.Past...) // most recent first
			data.PastOpen = data.PastOpen || data.Highlight[e.ID]
			continue
		}
		data.UpcomingCount++
		if n := len(data.Upcoming); n > 0 && data.Upcoming[n-1].Label == e.Month() {
			data.Upcoming[n-1].Events = append(data.Upcoming[n-1].Events, e)
		} else {
			data.Upcoming = append(data.Upcoming, pages.Month{Label: e.Month(), Events: []event.Event{e}})
		}
	}
	data.Notice = notice(events, saved, updated, q.Has("deleted"))
	render(w, r, http.StatusOK, pages.List(data))
}

func notice(events []event.Event, saved, updated []int64, deleted bool) string {
	if deleted {
		return "Događaj je obrisan."
	}
	if len(saved)+len(updated) == 1 {
		for _, e := range events {
			switch {
			case len(saved) == 1 && e.ID == saved[0]:
				return "Spremljeno: " + e.Title
			case len(updated) == 1 && e.ID == updated[0]:
				return "Ažurirano (već je bilo spremljeno): " + e.Title
			}
		}
	}
	var parts []string
	if len(saved) > 0 {
		parts = append(parts, fmt.Sprintf("Novih događaja: %d", len(saved)))
	}
	if len(updated) > 0 {
		parts = append(parts, fmt.Sprintf("ažuriranih: %d", len(updated)))
	}
	return strings.Join(parts, ", ")
}

func (s *server) newForm(w http.ResponseWriter, r *http.Request, device store.Device) {
	render(w, r, http.StatusOK, pages.Form(pages.FormData{Device: device.Name}))
}

func (s *server) create(w http.ResponseWriter, r *http.Request, device store.Device) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	input := r.FormValue("json")
	events, err := event.Parse(input)
	if err != nil {
		render(w, r, http.StatusUnprocessableEntity, pages.Form(pages.FormData{Device: device.Name, Input: input, Error: err.Error()}))
		return
	}
	var saved, updated []string
	for _, e := range events {
		id, wasUpdated, err := s.store.SaveEvent(e)
		if err != nil {
			s.fail(w, err)
			return
		}
		if wasUpdated {
			updated = append(updated, strconv.FormatInt(id, 10))
		} else {
			saved = append(saved, strconv.FormatInt(id, 10))
		}
	}
	q := url.Values{}
	if len(saved) > 0 {
		q.Set("saved", strings.Join(saved, ","))
	}
	if len(updated) > 0 {
		q.Set("updated", strings.Join(updated, ","))
	}
	first := append(saved, updated...)[0]
	http.Redirect(w, r, "/?"+q.Encode()+"#event-"+first, http.StatusSeeOther)
}

func (s *server) editForm(w http.ResponseWriter, r *http.Request, device store.Device) {
	e, ok := s.event(w, r)
	if !ok {
		return
	}
	render(w, r, http.StatusOK, pages.Form(pages.FormData{Device: device.Name, EditID: e.ID, Title: e.Title, Input: e.JSON()}))
}

func (s *server) update(w http.ResponseWriter, r *http.Request, device store.Device) {
	e, ok := s.event(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	input := r.FormValue("json")
	events, err := event.Parse(input)
	if err == nil && len(events) != 1 {
		err = errors.New("Kod uređivanja zalijepi točno jedan događaj.")
	}
	if err != nil {
		render(w, r, http.StatusUnprocessableEntity, pages.Form(pages.FormData{Device: device.Name, EditID: e.ID, Title: e.Title, Input: input, Error: err.Error()}))
		return
	}
	if err := s.store.UpdateEvent(e.ID, events[0]); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/?updated=%d#event-%d", e.ID, e.ID), http.StatusSeeOther)
}

func (s *server) delete(w http.ResponseWriter, r *http.Request, device store.Device) {
	e, ok := s.event(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteEvent(e.ID); err != nil && !errors.Is(err, store.ErrNotFound) {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/?deleted=1", http.StatusSeeOther)
}

// event loads the event named by the {id} path value, writing a 404 if
// there is none.
func (s *server) event(w http.ResponseWriter, r *http.Request) (event.Event, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return event.Event{}, false
	}
	e, err := s.store.GetEvent(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return e, false
	}
	if err != nil {
		s.fail(w, err)
		return e, false
	}
	return e, true
}

func ids(list string) []int64 {
	var out []int64
	for _, part := range strings.Split(list, ",") {
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func (s *server) fail(w http.ResponseWriter, err error) {
	log.Print(err)
	http.Error(w, "Greška na poslužitelju.", http.StatusInternalServerError)
}

func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	var buf bytes.Buffer
	if err := c.Render(r.Context(), &buf); err != nil {
		log.Print(err)
		http.Error(w, "Greška na poslužitelju.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}
