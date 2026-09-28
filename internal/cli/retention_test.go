// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command tests: wiring smoke tests plus behavior cases for the crowd
// retention verdict (added_by_status percentages, aspirational-trap flag).

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/cliutil/testenv"
)

// TestNovelRetentionHelpWires smoke-tests that the retention command
// resolves at runtime and renders useful --help output.
func TestNovelRetentionHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"retention", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("retention --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "retention", "--year"} {
		if !strings.Contains(help, want) {
			t.Fatalf("retention --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestComputeRetentionStats(t *testing.T) {
	t.Run("retained classic", func(t *testing.T) {
		st := computeRetentionStats(map[string]int{
			"beaten": 60, "dropped": 10, "playing": 5, "yet": 20, "owned": 5,
		})
		if st.Total != 100 {
			t.Fatalf("total = %d, want 100", st.Total)
		}
		if st.BeatenPct != 60 || st.DroppedPct != 10 || st.PlayingPct != 5 || st.YetPct != 20 {
			t.Fatalf("percentages = %+v", st)
		}
		if st.AspirationalTrap {
			t.Fatal("beaten-heavy game must not be flagged aspirational-trap")
		}
		if st.Verdict != "community-retained" {
			t.Fatalf("verdict = %q", st.Verdict)
		}
	})

	t.Run("aspirational trap", func(t *testing.T) {
		st := computeRetentionStats(map[string]int{
			"beaten": 10, "dropped": 30, "playing": 5, "yet": 40, "toplay": 15,
		})
		if !st.AspirationalTrap {
			t.Fatal("intent (40+15) outweighing finishes (10) must be flagged")
		}
		if st.Verdict != "aspirational-trap" {
			t.Fatalf("verdict = %q", st.Verdict)
		}
	})

	t.Run("mixed verdict", func(t *testing.T) {
		st := computeRetentionStats(map[string]int{
			"beaten": 35, "dropped": 25, "playing": 10, "yet": 30,
		})
		if st.AspirationalTrap {
			t.Fatal("intent (30) not outweighing finishes (35) must not be flagged")
		}
		if st.Verdict != "mixed" {
			t.Fatalf("verdict = %q, want mixed (beaten < 50%% but no trap)", st.Verdict)
		}
	})

	t.Run("zero counts", func(t *testing.T) {
		st := computeRetentionStats(nil)
		if st.Total != 0 || st.AspirationalTrap {
			t.Fatalf("empty counts = %+v", st)
		}
		if st.Verdict != "no community signal yet" {
			t.Fatalf("verdict = %q", st.Verdict)
		}
	})
}

func TestRetentionStatusRows(t *testing.T) {
	st := computeRetentionStats(map[string]int{"beaten": 50, "yet": 50})
	rows := retentionStatusRows(st)
	if len(rows) != 6 {
		t.Fatalf("rows = %d, want all six statuses", len(rows))
	}
	if rows[0]["status"] != "beaten" || rows[0]["share"] != "50.0%" {
		t.Fatalf("first row = %+v", rows[0])
	}
	if rows[1]["status"] != "yet" || rows[1]["share"] != "50.0%" {
		t.Fatalf("second row = %+v", rows[1])
	}
}

func TestParseRetentionDetail(t *testing.T) {
	fixtures := []string{
		`{"id":3498,"name":"Grand Theft Auto V","added":12345,"added_by_status":{"yet":1000,"owned":2000,"beaten":3000,"dropped":500,"playing":400,"toplay":600}}`,
		`{"id":1,"name":"Zero","added":0}`,
	}
	for _, fx := range fixtures {
		d, err := parseRetentionDetail([]byte(fx))
		if err != nil {
			t.Fatalf("parse error on %s: %v", fx, err)
		}
		if d.Name == "" || d.ID == 0 {
			t.Fatalf("parsed detail incomplete: %+v", d)
		}
	}
	d, _ := parseRetentionDetail([]byte(fixtures[0]))
	if d.AddedByStatus["beaten"] != 3000 {
		t.Fatalf("added_by_status = %+v", d.AddedByStatus)
	}
	if _, err := parseRetentionDetail([]byte(`{nope`)); err == nil {
		t.Fatal("malformed JSON must fail")
	}
}

func TestParseGameID(t *testing.T) {
	if id, ok := parseGameID("3498"); !ok || id != 3498 {
		t.Fatalf("parseGameID(3498) = %d, %v", id, ok)
	}
	for _, bad := range []string{"", " ", "-5", "x3498", "34.98", "3498 ", "0"} {
		if _, ok := parseGameID(bad); ok {
			t.Fatalf("parseGameID(%q) must not resolve", bad)
		}
	}
}

func TestRetentionDryRunEnvelope(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"retention", "Elden Ring", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("retention --dry-run --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run":true`) {
		t.Fatalf("dry-run envelope missing: %s", out.String())
	}
}

func TestRetentionNeedsExactlyOneGame(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"retention", "--json"}) // no positional, but a flag: must be a usage error, not help
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("retention without a game must fail")
	}
	if !strings.Contains(out.String()+err.Error(), "exactly one") {
		t.Fatalf("error must explain the positional contract: %v / %s", err, out.String())
	}
}
