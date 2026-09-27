package cli

import "testing"

func TestIsFranchiseContinuation(t *testing.T) {
	cases := []struct {
		query, name string
		want        bool
	}{
		{"halo", "Halo Infinite", true},
		{"halo", "Halo: Reach", true},
		{"halo", "Halo 3: ODST", true},
		{"halo", "Halo's Adventure", false}, // apostrophe is not a boundary
		{"halo", "HALO", false},             // not longer than the query
		{"halo", "HALO (2016)", false},      // year-strip makes it equal, not longer
		{"zelda", "Zelda II: The Adventure of Link", true},
		{"zelda", "The Legend of Zelda", false}, // not a continuation
		{"megabonk", "Duke Nukem 3D: Megaton Edition", false},
	}
	for _, tc := range cases {
		if got := isFranchiseContinuation(tc.query, tc.name); got != tc.want {
			t.Fatalf("isFranchiseContinuation(%q, %q) = %v, want %v", tc.query, tc.name, got, tc.want)
		}
	}
}

func TestFranchiseOverride(t *testing.T) {
	obscureHalo := rawgGame{ID: 609130, Name: "Halo", Added: 3}
	ranked := []rawgGame{
		obscureHalo,
		{ID: 2, Name: "Halo (itch)", Added: 21},
		{ID: 58751, Name: "Halo Infinite", Added: 7888},
		{ID: 28613, Name: "Halo: Reach", Added: 3063},
		{ID: 450393, Name: "Halo (itch)", Added: 21},
	}
	best, ok := franchiseOverride(obscureHalo, ranked, "halo")
	if !ok || best.ID != 58751 {
		t.Fatalf("franchiseOverride halo = (%d, %v), want Halo Infinite", best.ID, ok)
	}

	// Confident exact hit: no override even when a continuation is bigger.
	doom2016 := rawgGame{ID: 1, Name: "DOOM (2016)", Added: 14084}
	if _, ok := franchiseOverride(doom2016, ranked, "doom"); ok {
		t.Fatal("franchiseOverride must not fire for a confident exact hit")
	}

	// Continuation exists but does not dominate: no override.
	junkZelda := rawgGame{ID: 9, Name: "Zelda", Added: 60}
	weak := []rawgGame{
		junkZelda,
		{ID: 10, Name: "Zelda II: The Adventure of Link", Added: 300}, // < 10x60
	}
	if _, ok := franchiseOverride(junkZelda, weak, "zelda"); ok {
		t.Fatal("franchiseOverride must not fire without dominance")
	}

	// No continuation at all: no override (megabonk case).
	mega := rawgGame{ID: 1010539, Name: "Megabonk", Added: 132}
	if _, ok := franchiseOverride(mega, []rawgGame{mega, {ID: 5, Name: "Duke Nukem 3D: Megaton Edition", Added: 929}}, "megabonk"); ok {
		t.Fatal("franchiseOverride must not fire without a continuation")
	}
}

func TestSharesOnlyBroadGenre(t *testing.T) {
	seedGenres := map[string]bool{"indie": true, "action": true}
	if !sharesOnlyBroadGenre(rawgGame{Name: "X", Genres: []rawgNamedRef{{Name: "Indie"}}}, seedGenres) {
		t.Fatal("only-Indie shared must be weak")
	}
	if !sharesOnlyBroadGenre(rawgGame{Name: "X", Genres: []rawgNamedRef{{Name: "Indie"}, {Name: "Platformer"}}}, seedGenres) {
		t.Fatal("one shared broad genre among unshared others is still weak")
	}
	if sharesOnlyBroadGenre(rawgGame{Name: "X", Genres: []rawgNamedRef{{Name: "Indie"}, {Name: "Action"}}}, seedGenres) {
		t.Fatal("two shared genres is not weak")
	}
	if sharesOnlyBroadGenre(rawgGame{Name: "X", Genres: []rawgNamedRef{{Name: "Strategy"}}}, seedGenres) {
		t.Fatal("no shared genre is not a weak match")
	}
	strategySeed := map[string]bool{"strategy": true}
	if sharesOnlyBroadGenre(rawgGame{Name: "X", Genres: []rawgNamedRef{{Name: "Strategy"}}}, strategySeed) {
		t.Fatal("a specific shared genre is not weak")
	}
}

func TestBestKnownGame(t *testing.T) {
	games := []rawgGame{
		{ID: 1, Name: "Halo", Added: 3},
		{ID: 2, Name: "Halo: Reach", Added: 3063},
		{ID: 3, Name: "Halo Infinite", Added: 7888},
	}
	if got := bestKnownGame(games); got.ID != 3 {
		t.Fatalf("bestKnownGame = %d, want 3 (Halo Infinite)", got.ID)
	}
	if got := bestKnownGame(nil); got.ID != 0 {
		t.Fatalf("bestKnownGame(nil) = %+v, want zero value", got)
	}
}
