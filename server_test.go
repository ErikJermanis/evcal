package main

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ErikJermanis/evcal/store"
)

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	handler, err := newServer(config{PublicBaseURL: "http://localhost"}, st)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, st
}

func newClient(t *testing.T) *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

// read returns a response's status and body: read(t)(client.Get(url)).
func read(t *testing.T) func(*http.Response, error) (int, string) {
	return func(res *http.Response, err error) (int, string) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, string(b)
	}
}

func TestUnpairedDeviceIsLockedOut(t *testing.T) {
	srv, _ := newTestServer(t)
	client := newClient(t)
	for _, path := range []string{"/", "/new", "/events/1/edit"} {
		if status, page := read(t)(client.Get(srv.URL + path)); status != http.StatusUnauthorized || !strings.Contains(page, "nije uparen") {
			t.Errorf("GET %s = %d", path, status)
		}
	}
	status, _ := read(t)(client.PostForm(srv.URL+"/events", url.Values{"json": {`{"title":"A","start_date":"2026-11-19"}`}}))
	if status != http.StatusUnauthorized {
		t.Errorf("POST /events = %d", status)
	}
	if status, body := read(t)(client.Get(srv.URL + "/healthz")); status != http.StatusOK || body != "ok" {
		t.Errorf("healthz = %d %q", status, body)
	}
}

func TestPairAddEditDelete(t *testing.T) {
	srv, st := newTestServer(t)
	client := newClient(t)
	token, err := st.CreatePairingToken("Test Mac")
	if err != nil {
		t.Fatal(err)
	}

	// Opening the link only asks for confirmation, so previews cannot consume it.
	status, page := read(t)(client.Get(srv.URL + "/pair/" + token))
	if status != http.StatusOK || !strings.Contains(page, "Test Mac") {
		t.Fatalf("pair page = %d", status)
	}
	if _, err := st.PairingName(token); err != nil {
		t.Fatalf("GET consumed the token: %v", err)
	}
	// Pasting the whole link into the form works too.
	status, page = read(t)(client.PostForm(srv.URL+"/pair", url.Values{"token": {" " + srv.URL + "/pair/" + token + " "}}))
	if status != http.StatusOK || !strings.Contains(page, "Uređaj: Test Mac") {
		t.Fatalf("after pairing = %d", status)
	}
	if status, _ := read(t)(newClient(t).PostForm(srv.URL+"/pair", url.Values{"token": {token}})); status != http.StatusGone {
		t.Fatalf("reused token = %d", status)
	}

	status, page = read(t)(client.PostForm(srv.URL+"/events", url.Values{"json": {"```json\n" + `{"title":"RIpost <b>2.0</b>","start_date":"2099-11-19","start_time":"15:00","url":"https://entrio.hr/x?utm_source=chatgpt.com"}` + "\n```"}}))
	if status != http.StatusOK || !strings.Contains(page, "Spremljeno: RIpost &lt;b&gt;2.0&lt;/b&gt;") || !strings.Contains(page, `href="https://entrio.hr/x"`) {
		t.Fatalf("after add = %d\n%s", status, page)
	}

	status, page = read(t)(client.PostForm(srv.URL+"/events", url.Values{"json": {`{"title":"Bez datuma"}`}}))
	if status != http.StatusUnprocessableEntity || !strings.Contains(page, "start_date") || !strings.Contains(page, "Bez datuma") {
		t.Fatalf("invalid add = %d", status)
	}

	events, _ := st.ListEvents()
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	id := events[0].ID
	editPath := srv.URL + "/events/" + itoa(id)

	status, page = read(t)(client.Get(editPath + "/edit"))
	if status != http.StatusOK || !strings.Contains(page, "&#34;start_date&#34;: &#34;2099-11-19&#34;") {
		t.Fatalf("edit page = %d\n%s", status, page)
	}
	status, page = read(t)(client.PostForm(editPath, url.Values{"json": {`{"title":"RIpost","start_date":"2099-11-20"}`}}))
	if status != http.StatusOK || !strings.Contains(page, "Ažurirano") {
		t.Fatalf("after edit = %d", status)
	}
	if e, _ := st.GetEvent(id); e.StartDate != "2099-11-20" || e.URL != "" {
		t.Fatalf("edited event = %+v", e)
	}

	status, page = read(t)(client.PostForm(editPath+"/delete", nil))
	if status != http.StatusOK || !strings.Contains(page, "obrisan") {
		t.Fatalf("after delete = %d", status)
	}
	if events, _ := st.ListEvents(); len(events) != 0 {
		t.Fatalf("events after delete = %+v", events)
	}
}

func TestCrossSitePostIsRejected(t *testing.T) {
	srv, st := newTestServer(t)
	token, _ := st.CreatePairingToken("Mac")
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/pair", strings.NewReader("token="+token))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if status, _ := read(t)(http.DefaultClient.Do(req)); status != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d", status)
	}
}

func TestPairingToken(t *testing.T) {
	for input, want := range map[string]string{
		"abc":                               "abc",
		" https://x.hr/pair/abc ":           "abc",
		"https://x.hr/pair/abc/?utm=1#frag": "abc",
		"http://localhost:8081/pair/a-b_c":  "a-b_c",
	} {
		if got := pairingToken(input); got != want {
			t.Errorf("pairingToken(%q) = %q, want %q", input, got, want)
		}
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
