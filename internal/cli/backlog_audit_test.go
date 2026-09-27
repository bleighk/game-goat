// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests: wiring smoke tests plus behavior cases for the
// decision-debt audit (stale / sunk-cost / drop-candidate classification).

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"game-goat-pp-cli/internal/cliutil/testenv"
	"game-goat-pp-cli/internal/store"
)

// TestNovelBacklogAuditHelpWires smoke-tests that the backlog audit command
// resolves at runtime and renders useful --help output.
func TestNovelBacklogAuditHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"backlog", "audit", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("backlog audit --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "audit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("backlog audit --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestClassifyAuditRow(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -120).Format(time.RFC3339)
	fresh := now.AddDate(0, 0, -2).Format(time.RFC3339)

	tests := []struct {
		name string
		row  store.GameBacklogRow
		want func(auditClassification) bool
	}{
		{
			name: "stale: old, backlog, never started",
			row:  store.GameBacklogRow{Title: "A", Status: store.GameBacklogStatusBacklog, AddedAt: old},
			want: func(cl auditClassification) bool { return cl.Stale && !cl.SunkCost },
		},
		{
			name: "not stale: recently added",
			row:  store.GameBacklogRow{Title: "B", Status: store.GameBacklogStatusBacklog, AddedAt: fresh},
			want: func(cl auditClassification) bool { return !cl.Stale && cl.ShelfDays <= 2 },
		},
		{
			name: "not stale: started despite age",
			row:  store.GameBacklogRow{Title: "C", Status: store.GameBacklogStatusBacklog, AddedAt: old, Hours: 2},
			want: func(cl auditClassification) bool { return !cl.Stale },
		},
		{
			name: "sunk-cost: 12h unfinished low rating",
			row:  store.GameBacklogRow{Title: "D", Status: store.GameBacklogStatusInProgress, AddedAt: fresh, Hours: 12, UserRating: 2},
			want: func(cl auditClassification) bool { return cl.SunkCost && !cl.Stale },
		},
		{
			name: "not sunk-cost: high rating anchors the hours",
			row:  store.GameBacklogRow{Title: "E", Status: store.GameBacklogStatusInProgress, AddedAt: fresh, Hours: 40, UserRating: 4.5},
			want: func(cl auditClassification) bool { return !cl.SunkCost },
		},
		{
			name: "drop candidate: stale + unrated + no notes",
			row:  store.GameBacklogRow{Title: "F", Status: store.GameBacklogStatusBacklog, AddedAt: old},
			want: func(cl auditClassification) bool { return cl.Stale && cl.DropCandidate },
		},
		{
			name: "not a drop candidate: notes anchor a stale row",
			row:  store.GameBacklogRow{Title: "G", Status: store.GameBacklogStatusBacklog, AddedAt: old, Notes: "friend insists"},
			want: func(cl auditClassification) bool { return cl.Stale && !cl.DropCandidate },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyAuditRow(tc.row, now); !tc.want(got) {
				t.Fatalf("classifyAuditRow(%q) = %+v, want predicate to hold", tc.row.Title, got)
			}
		})
	}
}

func TestMedianShelfDays(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want float64
	}{
		{"empty", nil, 0},
		{"odd", []int{10, 90, 50}, 50},
		{"even", []int{10, 90, 50, 30}, 40},
	}
	for _, tc := range tests {
		if got := medianShelfDays(tc.in); got != tc.want {
			t.Fatalf("%s: medianShelfDays = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestBuildBacklogAudit(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -200).Format(time.RFC3339)
	fresh := now.Format(time.RFC3339)
	rows := []store.GameBacklogRow{
		{RawgID: 1, Title: "Dusty", Status: store.GameBacklogStatusBacklog, AddedAt: old},                                   // stale + drop candidate
		{RawgID: 2, Title: "Anchored", Status: store.GameBacklogStatusBacklog, AddedAt: old, Notes: "keep"},                 // stale, anchored
		{RawgID: 3, Title: "Grinding", Status: store.GameBacklogStatusInProgress, AddedAt: fresh, Hours: 25, UserRating: 2}, // sunk-cost
		{RawgID: 4, Title: "Done", Status: store.GameBacklogStatusFinished, AddedAt: fresh},                                 // finished
	}

	view := buildBacklogAudit(rows, now, false)
	if view.Meta.Source != "local" || view.Meta.Total != 4 {
		t.Fatalf("meta = %+v", view.Meta)
	}
	if view.Stats.Finished != 1 || view.Stats.Unplayed != 3 || view.Stats.FinishedRatio != 0.25 {
		t.Fatalf("stats = %+v", view.Stats)
	}
	if len(view.Stale) != 2 {
		t.Fatalf("stale rows = %d, want 2 (%+v)", len(view.Stale), view.Stale)
	}
	if len(view.SunkCost) != 1 || view.SunkCost[0].Title != "Grinding" {
		t.Fatalf("sunk_cost = %+v", view.SunkCost)
	}
	if len(view.DropCandidates) != 1 || view.DropCandidates[0].Title != "Dusty" {
		t.Fatalf("drop_candidates = %+v", view.DropCandidates)
	}
	if len(view.Recommendations) == 0 {
		t.Fatal("recommendations must not be empty")
	}
	for _, rec := range view.Recommendations {
		if rec.Action == "" || rec.Command == "" || !strings.Contains(rec.Command, "game-goat-pp-cli") {
			t.Fatalf("recommendation must name an exact follow-up command: %+v", rec)
		}
	}
	if view.Stats.Total > 0 && view.Stats.MedianShelfDays <= 0 {
		t.Fatalf("median shelf time must be positive, got %v", view.Stats.MedianShelfDays)
	}

	pruned := buildBacklogAudit(rows, now, true)
	if len(pruned.Stale) != 0 || len(pruned.SunkCost) != 0 {
		t.Fatalf("prune view must narrow stale/sunk sections: %+v", pruned)
	}
	for _, rec := range pruned.Recommendations {
		if rec.Action != "drop" {
			t.Fatalf("prune recommendations must be drop actions only, got %+v", rec)
		}
	}
}
