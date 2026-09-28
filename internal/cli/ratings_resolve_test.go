package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/config"
	"github.com/spf13/cobra"
)

// PR #2057 P1 review: when --year removes every exact-title match, the
// fallback must not resolve a year-matching search hit whose title differs
// from the requested one — it must report that the title was not found in
// the pinned year instead.
func TestResolveTitleForMultiSourceYearPinRequiresExactTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":2,"results":[`+
			`{"id":11226,"name":"BioShock","released":"2007-08-21","added":11226},`+
			`{"id":28614,"name":"Halo 3: ODST","released":"2009-09-22","added":3063}`+
			`]}`)
	}))
	defer srv.Close()

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	c.NoCache = true
	var errBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&errBuf)
	flags := &rootFlags{}

	// "halo 3" --year 2007: no game titled "Halo 3" was released in 2007,
	// but a different 2007 game (BioShock) sits in the top search results.
	// The old year-only filter resolved to it; the fix must not-found.
	_, _, err := resolveTitleForMultiSource(context.Background(), cmd, c, flags, "halo 3", "2007")
	if err == nil {
		t.Fatal("year-pinned resolution with no exact-title match must error, not fall back to a different title")
	}
	if !strings.Contains(err.Error(), `no game titled "halo 3" released in 2007`) {
		t.Fatalf("unexpected error: %v", err)
	}

	// Without the pin, the partial-title fallback still resolves the
	// best-known hit with a notice (regression guard for the shared path).
	game, _, err := resolveTitleForMultiSource(context.Background(), cmd, c, flags, "halo 3", "")
	if err != nil {
		t.Fatalf("no-year fallback must still resolve: %v", err)
	}
	if game.ID != 11226 {
		t.Fatalf("no-year fallback resolved id %d (%q), want BioShock (11226)", game.ID, game.Name)
	}
	if !strings.Contains(errBuf.String(), "no exact title match") {
		t.Fatalf("expected fallback notice on stderr, got: %q", errBuf.String())
	}
}

// PR #2057 follow-up P1: a year-pinned partial title ("the witcher 3"
// --year 2015) must resolve to the same-year release whose full name
// continues the query at a word boundary, instead of failing hard.
func TestResolveTitleForMultiSourceYearPinnedPartialTitleResolves(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":2,"results":[`+
			`{"id":12742,"name":"The Witcher Adventure Game","released":"2014-11-27","added":51},`+
			`{"id":26153,"name":"The Witcher 3: Wild Hunt","released":"2015-05-19","added":10364}`+
			`]}`)
	}))
	defer srv.Close()

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	c.NoCache = true
	var errBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&errBuf)
	flags := &rootFlags{}

	// "the witcher 3" --year 2015: the exact title is "The Witcher 3: Wild
	// Hunt", which normalizes differently, but it continues the query at a
	// word boundary and was released in the pinned year.
	game, _, err := resolveTitleForMultiSource(context.Background(), cmd, c, flags, "the witcher 3", "2015")
	if err != nil {
		t.Fatalf("year-pinned partial title must resolve: %v", err)
	}
	if game.ID != 26153 {
		t.Fatalf("resolved id %d (%q), want The Witcher 3: Wild Hunt (26153)", game.ID, game.Name)
	}
	if !strings.Contains(errBuf.String(), "no exact title match") {
		t.Fatalf("expected partial-title notice on stderr, got: %q", errBuf.String())
	}
}

// PR #2057 follow-up P2: the year-pin not-found error promises a RAWG id
// recovery path; a bare-numeric argument must resolve as that id without
// a search round-trip.
func TestResolveTitleForMultiSourceAcceptsRawgID(t *testing.T) {
	// The server fails every request: id resolution must short-circuit
	// before any HTTP call.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	c.NoCache = true
	cmd := &cobra.Command{}
	flags := &rootFlags{}

	game, _, err := resolveTitleForMultiSource(context.Background(), cmd, c, flags, "26153", "")
	if err != nil {
		t.Fatalf("numeric RAWG id must resolve without error: %v", err)
	}
	if game.ID != 26153 {
		t.Fatalf("resolved id %d, want 26153", game.ID)
	}
}
