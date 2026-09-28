// series_anchor_test.go — regression test for PR #2057 review finding
// "Numeric series anchors lack details": a bare RAWG id must load the game's
// detail so the play-order anchor has a name and release date.

package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/config"
)

func TestSeriesAnchorLoadsDetailForBareID(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/games/3498" {
			hits++
			fmt.Fprint(w, `{"id":3498,"name":"Grand Theft Auto V","released":"2013-09-17"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	c.NoCache = true
	got, err := seriesAnchor(context.Background(), c, rawgGame{ID: 3498})
	if err != nil {
		t.Fatalf("seriesAnchor: %v", err)
	}
	if got.Name != "Grand Theft Auto V" || got.Released != "2013-09-17" {
		t.Fatalf("anchor detail not loaded: %+v", got)
	}
	if hits != 1 {
		t.Fatalf("expected one detail fetch, got %d", hits)
	}
}

func TestSeriesAnchorPassesThroughTitleResolved(t *testing.T) {
	// Point at a dead address: a title-resolved anchor must not make a request.
	c := client.New(&config.Config{BaseURL: "http://127.0.0.1:1"}, 0, 0)
	got, err := seriesAnchor(context.Background(), c, rawgGame{ID: 1, Name: "Halo", Released: "2001-11-15"})
	if err != nil {
		t.Fatalf("seriesAnchor passthrough: %v", err)
	}
	if got.Name != "Halo" || got.Released != "2001-11-15" {
		t.Fatalf("passthrough changed the anchor: %+v", got)
	}
}
