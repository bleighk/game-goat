// itad_test.go — httptest-backed coverage for the IsThereAnyDeal client.
// No live network, no real API key: every case points BaseURL at a local
// server and asserts the wire contract (path, query, header, body) plus the
// typed parsing and error mapping.

package itad

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const searchJSON = `[
  {"id":"018d937f-07fc-72ed-8517-d8e24cb1eb22","slug":"europa-universalis-iv","title":"Europa Universalis IV","type":"game","mature":false},
  {"id":"018d937e-f083-72b5-bfdf-5459a8948636","slug":"europa-universalis-iv-american-dream","title":"Europa Universalis IV: American Dream","type":"dlc","mature":false}
]`

const pricesJSON = `[
  {"id":"018d937f-012f-73b8-ab2c-898516969e6a",
   "historyLow":{"all":{"amount":0.99,"amountInt":99,"currency":"EUR"},"y1":{"amount":0.99,"amountInt":99,"currency":"EUR"},"m3":{"amount":9.99,"amountInt":999,"currency":"EUR"}},
   "deals":[{"shop":{"id":61,"name":"Steam"},"price":{"amount":9.99,"amountInt":999,"currency":"EUR"},"regular":{"amount":9.99,"amountInt":999,"currency":"EUR"},"cut":0,"storeLow":{"amount":0.99,"amountInt":99,"currency":"EUR"},"timestamp":"2024-02-11T01:47:46+01:00","expiry":null,"url":"https://itad.link/x/"}]}
]`

const historyJSON = `[
  {"timestamp":"2022-12-27T11:21:08+01:00","shop":{"id":61,"name":"Steam"},"deal":{"price":{"amount":9.99,"amountInt":999,"currency":"EUR"},"regular":{"amount":39.99,"amountInt":3999,"currency":"EUR"},"cut":75}},
  {"timestamp":"2022-12-14T00:12:29+01:00","shop":{"id":61,"name":"Steam"},"deal":{"price":{"amount":39.99,"amountInt":3999,"currency":"EUR"},"regular":{"amount":39.99,"amountInt":3999,"currency":"EUR"},"cut":0}}
]`

func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	c := New(&Config{APIKey: "test-key", Country: "us"})
	c.BaseURL = serverURL
	return c
}

func TestSearchSendsKeyHeaderAndParses(t *testing.T) {
	var gotKey, gotTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("ITAD-API-Key")
		gotTitle = r.URL.Query().Get("title")
		_, _ = io.WriteString(w, searchJSON)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	games, err := c.Search(context.Background(), "Europa Universalis IV", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if gotKey != "test-key" {
		t.Fatalf("ITAD-API-Key header = %q, want test-key", gotKey)
	}
	if gotTitle != "Europa Universalis IV" {
		t.Fatalf("title param = %q", gotTitle)
	}
	if len(games) != 2 || games[0].Type != "game" || games[0].ID == "" {
		t.Fatalf("unexpected games: %+v", games)
	}
}

func TestPricesPostsIDsAndCountry(t *testing.T) {
	var method, body, country string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		country = r.URL.Query().Get("country")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		_, _ = io.WriteString(w, pricesJSON)
	}))
	defer srv.Close()

	c := New(&Config{APIKey: "k", Country: "DE"})
	c.BaseURL = srv.URL
	results, err := c.Prices(context.Background(), []string{"018d937f-012f-73b8-ab2c-898516969e6a"}, PriceOptions{Capacity: 1})
	if err != nil {
		t.Fatalf("Prices: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %s, want POST", method)
	}
	if country != "DE" {
		t.Fatalf("country = %q, want DE", country)
	}
	var ids []string
	if err := json.Unmarshal([]byte(body), &ids); err != nil {
		t.Fatalf("body not a JSON id array: %q (%v)", body, err)
	}
	if len(ids) != 1 {
		t.Fatalf("ids = %v", ids)
	}
	if len(results) != 1 {
		t.Fatalf("results = %+v", results)
	}
	if results[0].HistoryLow.All == nil || results[0].HistoryLow.All.Currency != "EUR" {
		t.Fatalf("historyLow.all not parsed: %+v", results[0].HistoryLow)
	}
	if results[0].HistoryLow.M3 == nil || results[0].HistoryLow.M3.Amount != 9.99 {
		t.Fatalf("historyLow.m3 not parsed: %+v", results[0].HistoryLow)
	}
	if len(results[0].Deals) != 1 || results[0].Deals[0].Shop.Name != "Steam" {
		t.Fatalf("deals not parsed: %+v", results[0].Deals)
	}
}

func TestHistoryParsesAndSendsSince(t *testing.T) {
	var gotID, gotSince, gotCountry string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.URL.Query().Get("id")
		gotSince = r.URL.Query().Get("since")
		gotCountry = r.URL.Query().Get("country")
		_, _ = io.WriteString(w, historyJSON)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	entries, err := c.History(context.Background(), "game-uuid", "2024-01-01")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if gotID != "game-uuid" || gotSince != "2024-01-01" || gotCountry != "US" {
		t.Fatalf("query id=%q since=%q country=%q", gotID, gotSince, gotCountry)
	}
	if len(entries) != 2 || entries[0].Deal.Cut != 75 || entries[0].Deal.Price == nil {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestMissingAPIKeyShortCircuits(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	c := New(&Config{})
	c.BaseURL = srv.URL
	_, err := c.Search(context.Background(), "x", 5)
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("err = %v, want ErrMissingAPIKey", err)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("made %d HTTP calls with no key", calls)
	}
}

func TestResolveGamePrefersExactGameTypedMatch(t *testing.T) {
	const body = `[
	  {"id":"dlc","title":"Hades II Soundtrack","type":"dlc"},
	  {"id":"game","title":"Hades","type":"game"},
	  {"id":"fuzzy","title":"Hades II","type":"game"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	game, candidates, _, err := ResolveGame(context.Background(), c, "Hades", 20)
	if err != nil {
		t.Fatalf("ResolveGame: %v", err)
	}
	if game.ID != "game" {
		t.Fatalf("resolved %q, want exact game match", game.ID)
	}
	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want 1 exact match", candidates)
	}
}

func TestResolveGameAmbiguousReturnsCandidates(t *testing.T) {
	const body = `[
	  {"id":"a","title":"DOOM","type":"game"},
	  {"id":"b","title":"Doom","type":"game"}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, candidates, _, err := ResolveGame(context.Background(), c, "doom", 20)
	if err != nil {
		t.Fatalf("ResolveGame: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %+v, want 2", candidates)
	}
}

func TestResolveGameEmptyIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, _, _, err := ResolveGame(context.Background(), c, "nothing", 20)
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("err = %v, want ErrGameNotFound", err)
	}
}

func TestRateLimitRetriesOnceThenSucceeds(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, searchJSON)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.Search(context.Background(), "x", 5); err != nil {
		t.Fatalf("Search after 429: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestAuthErrorClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"reason_phrase":"Missing api key"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.Search(context.Background(), "x", 5)
	if err == nil || !IsAuthError(err) {
		t.Fatalf("err = %v, want an auth error", err)
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("err should mention 403: %v", err)
	}
}

func TestSortDealsByPriceCheapestFirstNilLast(t *testing.T) {
	deals := []Deal{
		{Shop: ShopRef{Name: "B"}, Price: &Money{Amount: 20}},
		{Shop: ShopRef{Name: "C"}, Price: nil},
		{Shop: ShopRef{Name: "A"}, Price: &Money{Amount: 5}},
	}
	out := SortDealsByPrice(deals)
	if out[0].Shop.Name != "A" || out[1].Shop.Name != "B" || out[2].Shop.Name != "C" {
		t.Fatalf("order = %s,%s,%s", out[0].Shop.Name, out[1].Shop.Name, out[2].Shop.Name)
	}
}

func TestNormalizeHelpers(t *testing.T) {
	if got := normalizeCountry(" de "); got != "DE" {
		t.Fatalf("normalizeCountry = %q", got)
	}
	if got := normalizeCountry("USA"); got != "" {
		t.Fatalf("normalizeCountry(USA) = %q, want empty", got)
	}
	if got := normalizeCountry("D1"); got != "" {
		t.Fatalf("normalizeCountry(D1) = %q, want empty", got)
	}
	if got := normalizeTitle("  DOOM: Eternal™ "); got != "doom eternal" {
		t.Fatalf("normalizeTitle = %q", got)
	}
}

func TestResolveGameFuzzyFallbackIsInexact(t *testing.T) {
	const body = `[{"id":"a","title":"Hades II","type":"game"}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	game, _, exact, err := ResolveGame(context.Background(), c, "Hades", 20)
	if err != nil {
		t.Fatalf("ResolveGame: %v", err)
	}
	if exact {
		t.Fatal("a fuzzy fallback must report exact=false so the caller can warn")
	}
	if game.ID != "a" {
		t.Fatalf("fallback game = %q", game.ID)
	}
}

func TestInfoReturnsGameTitle(t *testing.T) {
	var gotID string
	const body = `{"id":"g1","slug":"hades","title":"Hades","type":"game"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = r.URL.Query().Get("id")
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	game, err := c.Info(context.Background(), "g1")
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if gotID != "g1" || game.ID != "g1" || game.Title != "Hades" {
		t.Fatalf("info id=%q game=%+v", gotID, game)
	}
}

func TestInfoEmptyIsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.Info(context.Background(), "missing"); !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("err = %v, want ErrGameNotFound", err)
	}
}
