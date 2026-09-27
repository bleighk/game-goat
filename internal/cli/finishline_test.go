// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests: wiring smoke test plus behavior cases for the
// finishline projection (rate, divide-by-zero guard, milestones).

package cli

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"game-goat-pp-cli/internal/cliutil/testenv"
	"game-goat-pp-cli/internal/store"
)

// TestNovelFinishlineHelpWires smoke-tests that the finishline command
// resolves at runtime and renders useful --help output.
func TestNovelFinishlineHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"finishline", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("finishline --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "finishline"} {
		if !strings.Contains(help, want) {
			t.Fatalf("finishline --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestComputeFinishlineStats(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	monthsAgo := func(m int) string { return now.AddDate(0, -m, 0).Format(time.RFC3339) }

	t.Run("empty backlog", func(t *testing.T) {
		st := computeFinishlineStats(nil, now)
		if st.Total != 0 || st.FinishedPerMonth != 0 || st.MonthsToZero != 0 {
			t.Fatalf("empty stats = %+v", st)
		}
	})

	t.Run("rate and projection", func(t *testing.T) {
		rows := []store.GameBacklogRow{
			{Title: "A", Status: store.GameBacklogStatusFinished, AddedAt: monthsAgo(4)},
			{Title: "B", Status: store.GameBacklogStatusFinished, AddedAt: monthsAgo(3)},
			{Title: "C", Status: store.GameBacklogStatusBacklog, AddedAt: monthsAgo(2)},
			{Title: "D", Status: store.GameBacklogStatusBacklog, AddedAt: monthsAgo(1)},
		}
		st := computeFinishlineStats(rows, now)
		if st.Finished != 2 || st.Total != 4 || st.FinishedPct != 50 {
			t.Fatalf("counts = %+v", st)
		}
		if st.FinishedPerMonth <= 0 {
			t.Fatalf("rate must be positive: %+v", st)
		}
		if got, want := st.MonthsToZero, math.Ceil(2/st.FinishedPerMonth); got != want {
			t.Fatalf("months_to_zero = %v, want ceil(remaining/rate) = %v", got, want)
		}
	})

	t.Run("no finished games means no projection", func(t *testing.T) {
		rows := []store.GameBacklogRow{
			{Title: "X", Status: store.GameBacklogStatusBacklog, AddedAt: monthsAgo(1)},
		}
		st := computeFinishlineStats(rows, now)
		if st.FinishedPerMonth != 0 || st.MonthsToZero != 0 {
			t.Fatalf("zero-finished projection must be zeroed: %+v", st)
		}
	})

	t.Run("guard divide-by-zero on a day-old backlog with a finish", func(t *testing.T) {
		rows := []store.GameBacklogRow{
			{Title: "Fast", Status: store.GameBacklogStatusFinished, AddedAt: now.Add(-2 * time.Hour).Format(time.RFC3339)},
			{Title: "Queued", Status: store.GameBacklogStatusBacklog, AddedAt: now.Add(-time.Hour).Format(time.RFC3339)},
		}
		st := computeFinishlineStats(rows, now)
		if st.FinishedPerMonth != 1 { // minimum one-month window
			t.Fatalf("instant-finish rate must clamp to 1/month, got %v", st.FinishedPerMonth)
		}
		if st.MonthsToZero != 1 {
			t.Fatalf("months_to_zero = %v, want 1", st.MonthsToZero)
		}
	})
}

func TestBuildFinishlineMilestones(t *testing.T) {
	active := []store.GameBacklogRow{
		{Title: "Oldest", AddedAt: "2026-01-01T00:00:00Z"},
		{Title: "Middle", AddedAt: "2026-03-01T00:00:00Z"},
		{Title: "Newest", AddedAt: "2026-05-01T00:00:00Z"},
	}

	t.Run("ten checkpoints at every 10 percent", func(t *testing.T) {
		ms := buildFinishlineMilestones(10, 0, active)
		if len(ms) != 10 || ms[0].Percent != 10 || ms[9].Percent != 100 {
			t.Fatalf("milestones = %+v", ms)
		}
	})

	t.Run("next game counts oldest-active-first", func(t *testing.T) {
		ms := buildFinishlineMilestones(3, 0, active)
		if len(ms) != 10 {
			t.Fatalf("milestones = %d, want 10", len(ms))
		}
		if ms[0].FinishedNeeded != 1 || ms[0].Game != "Oldest" || ms[0].Status != "next" {
			t.Fatalf("first milestone = %+v", ms[0])
		}
		if ms[2].FinishedNeeded != 1 || ms[2].Game != "Oldest" { // ceil(30% of 3) = 1
			t.Fatalf("third milestone = %+v", ms[2])
		}
		if ms[9].FinishedNeeded != 3 || ms[9].Game != "Newest" || ms[9].Status != "next" {
			t.Fatalf("final milestone = %+v", ms[9])
		}
	})

	t.Run("reached checkpoints carry no game", func(t *testing.T) {
		ms := buildFinishlineMilestones(2, 1, active[:1])
		if ms[0].FinishedNeeded != 1 || ms[0].Status != "reached" || ms[0].Game != "" {
			t.Fatalf("reached milestone = %+v", ms[0])
		}
		if ms[9].FinishedNeeded != 2 || ms[9].Game != "Oldest" || ms[9].Status != "next" {
			t.Fatalf("final milestone = %+v", ms[9])
		}
	})

	t.Run("empty backlog has no milestones", func(t *testing.T) {
		if ms := buildFinishlineMilestones(0, 0, nil); len(ms) != 0 {
			t.Fatalf("milestones = %+v, want none", ms)
		}
	})
}

func TestBuildFinishlineView(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	view := buildFinishlineView(nil, now)
	if view.Meta.Source != "local" || len(view.Milestones) != 0 {
		t.Fatalf("empty view = %+v", view)
	}
	rows := []store.GameBacklogRow{
		{Title: "A", Status: store.GameBacklogStatusFinished, AddedAt: now.Format(time.RFC3339)},
	}
	view = buildFinishlineView(rows, now)
	if view.Stats.FinishedPct != 100 || len(view.Milestones) != 10 {
		t.Fatalf("one-finished view = %+v", view)
	}
}
