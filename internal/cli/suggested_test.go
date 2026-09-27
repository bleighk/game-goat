// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
// Test cases for the hand-written 'suggested' novel command: tier-gate
// detection and the /games?genres= id join helper.

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"game-goat-pp-cli/internal/client"
	"game-goat-pp-cli/internal/cliutil/testenv"
)

func TestSuggestedHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"suggested", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("suggested --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "suggested"} {
		if !strings.Contains(help, want) {
			t.Fatalf("suggested --help missing %q in output:\n%s", want, help)
		}
	}
	for _, want := range []string{"--year", "--limit"} {
		if !strings.Contains(help, want) {
			t.Fatalf("suggested --help missing flag %q", want)
		}
	}
}

func TestIsSuggestedTierGate(t *testing.T) {
	for _, code := range []int{401, 403} {
		err := &client.APIError{Method: "GET", Path: "/games/1/suggested", StatusCode: code}
		if !isSuggestedTierGate(err) {
			t.Fatalf("HTTP %d must be detected as the tier gate", code)
		}
		// wrapped errors must still match (the client may decorate them)
		if !isSuggestedTierGate(fmt.Errorf("call failed: %w", err)) {
			t.Fatalf("wrapped HTTP %d must still be detected", code)
		}
	}
	for _, code := range []int{404, 429, 500} {
		err := &client.APIError{Method: "GET", Path: "/games/1/suggested", StatusCode: code}
		if isSuggestedTierGate(err) {
			t.Fatalf("HTTP %d must NOT be treated as the tier gate", code)
		}
	}
	if isSuggestedTierGate(errors.New("connection refused")) {
		t.Fatal("plain errors must not be treated as the tier gate")
	}
	if isSuggestedTierGate(nil) {
		t.Fatal("nil must not be treated as the tier gate")
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

func TestSuggestedDryRunEnvelope(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"suggested", "Dark Souls III", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("suggested --dry-run --json error = %v", err)
	}
	if !strings.Contains(out.String(), `"dry_run":true`) {
		t.Fatalf("dry-run envelope missing: %s", out.String())
	}
}
