// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests: wiring smoke test plus behavior cases for the mood
// vocabulary (coverage, lookup, recipes, surprise-me determinism).

package cli

import (
	"bytes"
	"strings"
	"testing"

	"game-goat-pp-cli/internal/cliutil/testenv"
)

// TestNovelMoodsListHelpWires smoke-tests that the moods list command
// resolves at runtime and renders useful --help output.
func TestNovelMoodsListHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"moods", "list", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("moods list --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "list"} {
		if !strings.Contains(help, want) {
			t.Fatalf("moods list --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMoodVocabularyCoverage(t *testing.T) {
	moods := allMoods()
	if len(moods) < 10 {
		t.Fatalf("vocabulary has %d moods, want >= 10", len(moods))
	}
	required := []string{"cozy", "story-heavy", "challenging", "quick-hit", "co-op", "retro", "open-world", "puzzle", "horror", "power-fantasy"}
	for _, name := range required {
		m, ok := moodByName(name)
		if !ok {
			t.Fatalf("required mood %q missing from vocabulary", name)
		}
		if m.Description == "" {
			t.Fatalf("mood %q must carry a description", name)
		}
		if m.Recipe.Ordering == "" {
			t.Fatalf("mood %q must carry a recipe ordering", name)
		}
	}
}

func TestMoodByName(t *testing.T) {
	if _, ok := moodByName("CoZy"); !ok {
		t.Fatal("moodByName must be case-insensitive")
	}
	if _, ok := moodByName("nope"); ok {
		t.Fatal("unknown mood must not resolve")
	}
}

func TestPickSurpriseMood(t *testing.T) {
	n := int64(len(gameMoods))
	if n == 0 {
		t.Fatal("vocabulary is empty")
	}
	a := pickSurpriseMood(3)
	b := pickSurpriseMood(3)
	if a.Name != b.Name {
		t.Fatalf("surprise mood must be deterministic per seed: %q vs %q", a.Name, b.Name)
	}
	if pickSurpriseMood(3+n).Name != a.Name {
		t.Fatal("surprise mood must wrap by vocabulary size")
	}
	if pickSurpriseMood(-1).Name == "" {
		t.Fatal("negative seeds must wrap, not miss")
	}
}

func TestMoodRecipeParams(t *testing.T) {
	m, _ := moodByName("cozy")
	params := moodRecipeParams(m, 20)
	if params["ordering"] != m.Recipe.Ordering {
		t.Fatalf("params ordering = %q", params["ordering"])
	}
	if len(m.Recipe.Genres) > 0 && params["genres"] != strings.Join(m.Recipe.Genres, ",") {
		t.Fatalf("params genres = %q", params["genres"])
	}
	if params["page_size"] != "20" {
		t.Fatalf("page_size = %q", params["page_size"])
	}
}
