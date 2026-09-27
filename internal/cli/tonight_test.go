// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// Test cases for the hand-written 'tonight' novel command: session-budget
// fit, playtime-band filter, and pick ranking (lead > backlog > rest).

package cli

import (
	"bytes"
	"strings"
	"testing"

	"game-goat-pp-cli/internal/cliutil/testenv"
)

func TestTonightHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"tonight", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tonight --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "tonight"} {
		if !strings.Contains(help, want) {
			t.Fatalf("tonight --help missing %q in output:\n%s", want, help)
		}
	}
	for _, want := range []string{"--mood", "--time", "--limit", "--seed"} {
		if !strings.Contains(help, want) {
			t.Fatalf("tonight --help missing flag %q", want)
		}
	}
}

func TestFitsTimeBudget(t *testing.T) {
	tests := []struct {
		name        string
		playtimeHrs int
		minutes     int
		want        bool
	}{
		{"60-minute budget, 1h game fits (60 <= 90)", 1, 60, true},
		{"60-minute budget, 2h game does not fit (120 > 90)", 2, 60, false},
		// RAWG playtime 0 means unknown, not "very short": an unmeasured
		// duration cannot be verified to fit the session (UAT F-U10).
		{"unknown playtime (0h) never fits a 30-minute budget", 0, 30, false},
		{"unknown playtime (0h) never fits any budget", 0, 10, false},
	}
	for _, tc := range tests {
		if got := fitsTimeBudget(tc.playtimeHrs, tc.minutes); got != tc.want {
			t.Fatalf("%s: fitsTimeBudget = %v", tc.name, got)
		}
	}
	// The documented rule: playtime*60 <= time*1.5
	if !fitsTimeBudget(1, 40) { // 60 <= 60
		t.Fatal("1h game must exactly fit a 40-minute budget (60 <= 60)")
	}
}

func TestFilterByPlaytimeBand(t *testing.T) {
	games := []rawgGame{
		{Name: "Short", Playtime: 1},  // 60 min
		{Name: "Medium", Playtime: 4}, // 240 min
		{Name: "Epic", Playtime: 60},  // 3600 min
	}
	band := filterByPlaytimeBand(games, 120, 300)
	if len(band) != 1 || band[0].Name != "Medium" {
		t.Fatalf("band = %+v, want only Medium", band)
	}
	open := filterByPlaytimeBand(games, 0, 0)
	if len(open) != 3 {
		t.Fatalf("unbounded band = %+v", open)
	}
	upperOnly := filterByPlaytimeBand(games, 0, 120)
	if len(upperOnly) != 1 || upperOnly[0].Name != "Short" {
		t.Fatalf("upper-only band = %+v", upperOnly)
	}
}

func TestRankTonightPicks(t *testing.T) {
	lead := &tonightPick{ID: 0, Name: "Resume Me", Reason: "pick up where you left off"}
	picks := []tonightPick{
		{ID: 1, Name: "Fresh", Rating: 4.9},
		{ID: 2, Name: "Owned", Rating: 4.0, OnBacklog: true},
		{ID: 3, Name: "Owned Too", Rating: 3.0, OnBacklog: true},
	}
	got := rankTonightPicks(lead, picks, 2)
	if len(got) != 2 {
		t.Fatalf("ranked = %+v, want limit 2", got)
	}
	if got[0].Name != "Resume Me" || got[1].Name != "Owned" {
		t.Fatalf("lead must come first, then backlog picks: %+v", got)
	}
	noLead := rankTonightPicks(nil, picks, 10)
	if len(noLead) != 3 || noLead[0].Name != "Owned" {
		t.Fatalf("without a lead, backlog picks lead: %+v", noLead)
	}
	if noLead[1].Name != "Owned Too" || noLead[2].Name != "Fresh" {
		t.Fatalf("rating order within tiers must be preserved: %+v", noLead)
	}
}

func TestTonightDryRunEnvelope(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"tonight", "--mood=quick-hit", "--time=30", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("tonight --dry-run --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run":true`) {
		t.Fatalf("dry-run envelope missing: %s", out.String())
	}
}

func TestTonightUnknownMoodIsTypedError(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"tonight", "--mood=bogus"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("unknown mood must fail")
	}
	var cliErr *cliError
	if !As(err, &cliErr) || cliErr.code != 2 {
		t.Fatalf("unknown mood must be a usage error (exit 2), got %v", err)
	}
	if !strings.Contains(out.String()+err.Error(), "valid moods") {
		t.Fatalf("error must list valid moods, got: %v / %s", err, out.String())
	}
}
