// price_query_test.go — pure-helper and view-shaping tests for the
// IsThereAnyDeal price commands. No network: the source client is covered by
// internal/source/itad, so these tests exercise only the local logic.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/itad"
)

func TestNormalizeITADCountry(t *testing.T) {
	if got, err := normalizeITADCountry(" de "); err != nil || got != "DE" {
		t.Fatalf("normalizeITADCountry = %q, %v", got, err)
	}
	if got, err := normalizeITADCountry(""); err != nil || got != "" {
		t.Fatalf("empty = %q, %v; want empty,nil", got, err)
	}
	for _, bad := range []string{"USA", "D", "D1", "12"} {
		if _, err := normalizeITADCountry(bad); err == nil {
			t.Fatalf("normalizeITADCountry(%q) should error", bad)
		}
	}
}

func TestResolveITADCountryPrecedence(t *testing.T) {
	t.Setenv("ITAD_COUNTRY", "de")
	if got, err := resolveITADCountry("gb"); err != nil || got != "GB" {
		t.Fatalf("flag should win: got %q, %v", got, err)
	}
	if got, err := resolveITADCountry(""); err != nil || got != "DE" {
		t.Fatalf("env should apply: got %q, %v", got, err)
	}
	t.Setenv("ITAD_COUNTRY", "")
	if got, err := resolveITADCountry(""); err != nil || got != itad.DefaultCountry {
		t.Fatalf("default should apply: got %q, %v", got, err)
	}
	t.Setenv("ITAD_COUNTRY", "not-a-code")
	if got, err := resolveITADCountry(""); err != nil || got != itad.DefaultCountry {
		t.Fatalf("bad env should fall back to default: got %q, %v", got, err)
	}
}

func TestParseITADID(t *testing.T) {
	if id, ok := parseITADID("018d937f-07fc-72ed-8517-d8e24cb1eb22"); !ok || id != "018d937f-07fc-72ed-8517-d8e24cb1eb22" {
		t.Fatalf("valid uuid not parsed: %q %v", id, ok)
	}
	for _, bad := range []string{"", "3498", "018d937f07fc72ed8517d8e24cb1eb22", "zzzd937f-07fc-72ed-8517-d8e24cb1eb22", "018d937f-07fc-72ed-8517"} {
		if _, ok := parseITADID(bad); ok {
			t.Fatalf("parseITADID(%q) should fail", bad)
		}
	}
}

func TestNormalizeSince(t *testing.T) {
	if got, err := normalizeSince(""); err != nil || got != "" {
		t.Fatalf("empty = %q, %v", got, err)
	}
	got, err := normalizeSince("2024-01-01")
	if err != nil || got != "2024-01-01T00:00:00Z" {
		t.Fatalf("date = %q, %v", got, err)
	}
	if _, err := normalizeSince("nope"); err == nil {
		t.Fatal("invalid since should error")
	}
}

func TestPriceVerdict(t *testing.T) {
	all := &priceLowRow{Window: "all", Amount: 5, Currency: "USD"}
	cases := []struct {
		name    string
		current *priceDealRow
		want    string
	}{
		{"no current", nil, "no current price"},
		{"free now", &priceDealRow{Amount: 0}, "free right now"},
		{"at low", &priceDealRow{Amount: 5}, "at historical low — cheapest it has ever been"},
		{"near", &priceDealRow{Amount: 5.4}, "near historical low (within 10%)"},
	}
	for _, tc := range cases {
		if got := priceVerdict(tc.current, all); got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := priceVerdict(&priceDealRow{Amount: 5}, nil); got != "no historical low recorded" {
		t.Fatalf("no low: got %q", got)
	}
	if got := priceVerdict(&priceDealRow{Amount: 5}, &priceLowRow{Amount: 0}); got != "above historical low (previously free)" {
		t.Fatalf("previously free: got %q", got)
	}
	if got := priceVerdict(&priceDealRow{Amount: 10}, all); got[:6] != "above " {
		t.Fatalf("above low: got %q", got)
	}
}

func TestBuildLowRowsSkipsMissingWindows(t *testing.T) {
	rows := buildLowRows(itad.HistoryLow{
		All: &itad.Money{Amount: 1, Currency: "USD"},
		M3:  &itad.Money{Amount: 3, Currency: "USD"},
	})
	if len(rows) != 2 || rows[0].Window != "all" || rows[1].Window != "3m" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestBuildPriceHistoryView(t *testing.T) {
	prices := []itad.PriceResult{{
		HistoryLow: itad.HistoryLow{
			All: &itad.Money{Amount: 4, Currency: "GBP"},
			Y1:  &itad.Money{Amount: 6, Currency: "GBP"},
			M3:  &itad.Money{Amount: 8, Currency: "GBP"},
		},
		Deals: []itad.Deal{
			{Shop: itad.ShopRef{Name: "GOG"}, Price: &itad.Money{Amount: 22, Currency: "GBP"}, Cut: 0},
			{Shop: itad.ShopRef{Name: "Steam"}, Price: &itad.Money{Amount: 11, Currency: "GBP"}, Cut: 50, Regular: &itad.Money{Amount: 22, Currency: "GBP"}},
		},
	}}
	history := []itad.HistoryEntry{
		{Timestamp: "2024-02-01T00:00:00Z", Shop: itad.ShopRef{Name: "Steam"}, Deal: itad.DealDelta{Price: &itad.Money{Amount: 11, Currency: "GBP"}, Cut: 50}},
		{Timestamp: "2024-05-01T00:00:00Z", Shop: itad.ShopRef{Name: "Steam"}, Deal: itad.DealDelta{Price: &itad.Money{Amount: 8, Currency: "GBP"}, Cut: 60}},
	}
	view := buildPriceHistoryView(itad.Game{ID: "id-1", Title: "Hades"}, "title", "GB", "", nil, prices, history)

	if view.Current == nil || view.Current.Shop != "Steam" || view.Current.Amount != 11 {
		t.Fatalf("current = %+v, want cheapest Steam deal", view.Current)
	}
	if view.Meta.Currency != "GBP" || view.Meta.Country != "GB" || view.Meta.ItadID != "id-1" {
		t.Fatalf("meta = %+v", view.Meta)
	}
	if len(view.Lows) != 3 {
		t.Fatalf("lows = %+v", view.Lows)
	}
	if len(view.Changes) != 2 || view.Changes[0].At != "2024-05-01T00:00:00Z" {
		t.Fatalf("changes not newest-first: %+v", view.Changes)
	}
	if view.Verdict[:6] != "above " {
		t.Fatalf("verdict = %q", view.Verdict)
	}
}

func TestBuildChangesLimit(t *testing.T) {
	entries := []itad.HistoryEntry{
		{Timestamp: "2024-01-01T00:00:00Z"},
		{Timestamp: "2024-03-01T00:00:00Z"},
		{Timestamp: "2024-02-01T00:00:00Z"},
	}
	rows := buildChanges(entries, 2)
	if len(rows) != 2 || rows[0].At != "2024-03-01T00:00:00Z" || rows[1].At != "2024-02-01T00:00:00Z" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestBuildPricesViewFiltersAndLimits(t *testing.T) {
	prices := []itad.PriceResult{{
		HistoryLow: itad.HistoryLow{All: &itad.Money{Amount: 4, Currency: "EUR"}},
		Deals: []itad.Deal{
			{Shop: itad.ShopRef{Name: "Full"}, Price: &itad.Money{Amount: 30, Currency: "EUR"}, Cut: 0},
			{Shop: itad.ShopRef{Name: "Sale"}, Price: &itad.Money{Amount: 10, Currency: "EUR"}, Cut: 40},
			{Shop: itad.ShopRef{Name: "Cheap"}, Price: &itad.Money{Amount: 7, Currency: "EUR"}, Cut: 70},
		},
	}}
	view := buildPricesView(itad.Game{ID: "g", Title: "RimWorld"}, "title", "DE", nil, prices, 10, true)
	if len(view.Results) != 2 {
		t.Fatalf("deals-only results = %+v", view.Results)
	}
	if view.Results[0].Shop != "Cheap" || view.Results[1].Shop != "Sale" {
		t.Fatalf("not cheapest-first: %+v", view.Results)
	}
	if view.Meta.Currency != "EUR" {
		t.Fatalf("currency = %q", view.Meta.Currency)
	}
	if view.HistoryLow == nil || view.HistoryLow.Amount != 4 {
		t.Fatalf("history low = %+v", view.HistoryLow)
	}

	limited := buildPricesView(itad.Game{ID: "g", Title: "RimWorld"}, "title", "DE", nil, prices, 1, false)
	if len(limited.Results) != 1 || limited.Results[0].Shop != "Cheap" {
		t.Fatalf("limit not applied: %+v", limited.Results)
	}
}

func TestFormatMoney(t *testing.T) {
	if got := formatMoney(9.5, "USD"); got != "9.50 USD" {
		t.Fatalf("formatMoney = %q", got)
	}
	if got := formatMoney(9.5, ""); got != "9.50" {
		t.Fatalf("formatMoney no currency = %q", got)
	}
}

func TestBuildLowRowsKeepsFreeZero(t *testing.T) {
	rows := buildLowRows(itad.HistoryLow{All: &itad.Money{Amount: 0, Currency: "USD"}})
	if len(rows) != 1 || rows[0].Amount != 0 || rows[0].Currency != "USD" {
		t.Fatalf("a zero low is a free price and must be kept: %+v", rows)
	}
}

// TestITADTitleAcceptable documents the guard that keeps a bogus title from
// resolving to an unrelated game while still accepting abbreviated queries. A
// non-exact ITAD hit is accepted when it continues the query at a word boundary
// or contains the query as a whole-word run.
func TestITADTitleAcceptable(t *testing.T) {
	cases := []struct {
		query, candidate string
		want             bool
	}{
		{"the witcher 3", "The Witcher 3: Wild Hunt", true}, // continuation
		{"witcher 3", "The Witcher 3: Wild Hunt", true},     // abbreviation (prefix of core title)
		{"witcher", "The Witcher 3: Wild Hunt", true},       // single-word abbreviation
		{"hunt", "The Witcher 3: Wild Hunt", false},         // suffix word is a different game
		{"__printing_press_invalid__", "19th-century Printing Press Experience VR", false},
		{"elden ringg", "Elden Ring", false},
	}
	for _, tc := range cases {
		if got := itadTitleAcceptable(tc.query, tc.candidate); got != tc.want {
			t.Fatalf("itadTitleAcceptable(%q, %q) = %v, want %v", tc.query, tc.candidate, got, tc.want)
		}
	}
}

func TestBuildChangesSortsByInstantNotText(t *testing.T) {
	entries := []itad.HistoryEntry{
		{Timestamp: "2024-01-01T01:00:00+01:00"}, // 00:00 UTC — older, text-sorts later
		{Timestamp: "2024-01-01T00:30:00+00:00"}, // 00:30 UTC — newer
	}
	rows := buildChanges(entries, 10)
	if len(rows) != 2 || rows[0].At != "2024-01-01T00:30:00+00:00" {
		t.Fatalf("changes must sort by instant, not text: %+v", rows)
	}
}
