// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests: wiring smoke test plus behavior cases for the radar
// taste profile (genre ranking, overlap scoring, reason strings).

package cli

import (
	"bytes"
	"strings"
	"testing"

	"game-goat-pp-cli/internal/cliutil/testenv"
)

// TestNovelRadarHelpWires smoke-tests that the radar command
// resolves at runtime and renders useful --help output.
func TestNovelRadarHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"radar", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("radar --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "radar"} {
		if !strings.Contains(help, want) {
			t.Fatalf("radar --help missing %q in output:\n%s", want, help)
		}
	}
	for _, want := range []string{"--limit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("radar --help missing flag %q", want)
		}
	}
}

func TestTopTasteGenres(t *testing.T) {
	counts := map[string]int{"RPG": 3, "Action": 1, "Indie": 2, "Puzzle": 1}
	top := topTasteGenres(counts, 3)
	if len(top) != 3 {
		t.Fatalf("top = %v, want 3 entries", top)
	}
	if top[0] != "RPG" || top[1] != "Indie" {
		t.Fatalf("top ordering = %v", top)
	}
	// Tie broken alphabetically for deterministic profiles.
	if top[2] != "Action" {
		t.Fatalf("tie should break alphabetically: %v", top)
	}
	if got := topTasteGenres(map[string]int{}, 3); len(got) != 0 {
		t.Fatalf("empty counts must yield no profile: %v", got)
	}
}

func TestTasteOverlapAndScore(t *testing.T) {
	profile := []string{"RPG", "Action"}
	if got := tasteOverlap([]string{"rpg", "Puzzle"}, profile); got != 1 {
		t.Fatalf("overlap = %d, want 1 (case-insensitive)", got)
	}
	if got := tasteOverlap(nil, profile); got != 0 {
		t.Fatalf("overlap = %d, want 0", got)
	}
	high := tasteScore(2, 4.5)
	low := tasteScore(1, 4.9)
	if high <= low {
		t.Fatalf("two shared genres must outrank one plus rating: %v vs %v", high, low)
	}
}

func TestRadarReason(t *testing.T) {
	profile := []string{"RPG", "Action", "Indie"}
	reason := radarReason([]string{"RPG", "Action", "Sports"}, profile)
	if !strings.Contains(reason, "matches your RPG/Action taste (2/3 backlog genres)") {
		t.Fatalf("reason = %q", reason)
	}
	if got := radarReason([]string{"Sports"}, profile); got != "" {
		t.Fatalf("no overlap must yield an empty reason, got %q", got)
	}
	if got := radarReason([]string{"RPG"}, nil); got != "" {
		t.Fatalf("no profile must yield an empty reason, got %q", got)
	}
}
