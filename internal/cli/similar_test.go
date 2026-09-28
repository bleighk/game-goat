// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// Test cases for the hand-written 'similar' novel command: shared-genre
// scoring, tiebreaks, and the /games?genres= id join helper.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil/testenv"
)

func TestSimilarHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"similar", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("similar --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "similar"} {
		if !strings.Contains(help, want) {
			t.Fatalf("similar --help missing %q in output:\n%s", want, help)
		}
	}
	for _, want := range []string{"--year", "--limit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("similar --help missing flag %q", want)
		}
	}
}

func TestScoreSimilarity(t *testing.T) {
	source := map[string]bool{"adventure": true, "platformer": true}
	meta := func(v int) *int { return &v }

	candidate := rawgGame{
		Name: "Shares Two", Genres: []rawgNamedRef{{Name: "Adventure"}, {Name: "Platformer"}, {Name: "Puzzle"}},
		Rating: 3.0, Metacritic: meta(80),
	}
	if got := scoreSimilarity(candidate, source); got.Shared != 2 {
		t.Fatalf("shared = %d, want 2 (case-insensitive)", got.Shared)
	}

	zero := rawgGame{Name: "Shares None", Genres: []rawgNamedRef{{Name: "Racing"}}, Rating: 4.8}
	if got := scoreSimilarity(zero, source); got.Shared != 0 || got.Metacritic != 0 {
		t.Fatalf("no-overlap score = %+v", got)
	}
}

func TestLessSimilarity(t *testing.T) {
	twoShared := similarityScore{Shared: 2, Metacritic: 70, Rating: 3}
	oneShared := similarityScore{Shared: 1, Metacritic: 95, Rating: 4.9}
	if !lessSimilarity(twoShared, oneShared) {
		t.Fatal("shared-genre count must dominate metacritic")
	}
	if lessSimilarity(oneShared, twoShared) {
		t.Fatal("less must be asymmetric")
	}

	a := similarityScore{Shared: 1, Metacritic: 90, Rating: 3}
	b := similarityScore{Shared: 1, Metacritic: 85, Rating: 4.9}
	if !lessSimilarity(a, b) {
		t.Fatal("metacritic must break shared-count ties")
	}
	c := similarityScore{Shared: 1, Metacritic: 90, Rating: 4.2}
	if !lessSimilarity(c, a) {
		t.Fatal("rating must break metacritic ties")
	}
	d := similarityScore{Shared: 1, Metacritic: 90, Rating: 3}
	if lessSimilarity(d, a) || lessSimilarity(a, d) {
		t.Fatal("equal scores must not compare less")
	}
}

func TestGenreIDList(t *testing.T) {
	g := rawgGame{Genres: []rawgNamedRef{{ID: 3, Name: "Adventure"}, {ID: 83, Name: "Platformer"}}}
	got := genreIDList(g)
	if len(got) != 2 || got[0] != 3 || got[1] != 83 {
		t.Fatalf("genreIDList = %v", got)
	}
	if got := genreIDList(rawgGame{}); len(got) != 0 {
		t.Fatalf("empty genres = %v", got)
	}
	_ = strings.TrimSpace
}

func TestSimilarDryRunEnvelope(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"similar", "Hollow Knight", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("similar --dry-run --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run":true`) {
		t.Fatalf("dry-run envelope missing: %s", out.String())
	}
}

func TestIDCSV(t *testing.T) {
	if got := idCSV([]int{4, 51, 83}); got != "4,51,83" {
		t.Fatalf("idCSV = %q", got)
	}
	if got := idCSV(nil); got != "" {
		t.Fatalf("idCSV(nil) = %q", got)
	}
}

func TestIsBundleOrSoundtrack(t *testing.T) {
	for _, name := range []string{"Hollow Knight: Silksong & Soundtrack Bundle", "Halo Bundle", "Celeste Original Soundtrack"} {
		if !isBundleOrSoundtrack(name) {
			t.Fatalf("%q should be treated as packaging", name)
		}
	}
	for _, name := range []string{"Hollow Knight: Silksong", "Halo 4", "The Witcher 2: Assassins of Kings"} {
		if isBundleOrSoundtrack(name) {
			t.Fatalf("%q is a game, not packaging", name)
		}
	}
}

func TestConfidentOnly(t *testing.T) {
	gs := []rawgGame{{ID: 1, RatingsCount: 6}, {ID: 2, RatingsCount: 130}, {ID: 3, RatingsCount: 20}}
	got := confidentOnly(gs)
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 3 {
		t.Fatalf("confidentOnly = %+v, want ids 2,3", got)
	}
	thin := []rawgGame{{ID: 9, RatingsCount: 1}}
	if got := confidentOnly(thin); len(got) != 1 {
		t.Fatalf("confidentOnly must return input when nothing clears the floor, got %+v", got)
	}
}

func TestStudioCap(t *testing.T) {
	for limit, want := range map[int]int{1: 1, 2: 1, 5: 3, 10: 5, 20: 10} {
		if got := studioCap(limit); got != want {
			t.Fatalf("studioCap(%d) = %d, want %d", limit, got, want)
		}
	}
}

func TestStudioLabel(t *testing.T) {
	one := []rawgNamedRef{{Name: "Team Cherry"}}
	if got := studioLabel(one); got != "Team Cherry" {
		t.Fatalf("studioLabel(one) = %q", got)
	}
	many := []rawgNamedRef{{Name: "Bethesda Softworks"}, {Name: "id Software"}, {Name: "Panic Button"}, {Name: "Virtuos"}}
	if got := studioLabel(many); got != "Bethesda Softworks, id Software +2 more" {
		t.Fatalf("studioLabel(many) = %q", got)
	}
}
