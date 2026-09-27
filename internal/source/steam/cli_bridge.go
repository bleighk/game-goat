// cli_bridge.go — thin bridge imported by internal/cli commands. Wraps the
// raw ResolveAppID + ReviewSummary (+ optional price) calls into one
// per-title enrichment lookup so commands never touch Steam's wire shapes.

package steam

import "context"

// SteamReview is the per-title Steam enrichment the CLI surfaces: the
// keyless appreviews rollup plus the current price when appdetails has one.
type SteamReview struct {
	AppID    int64          `json:"app_id"`
	Score    float64        `json:"score"`
	Desc     string         `json:"desc"`
	Positive int            `json:"positive"`
	Negative int            `json:"negative"`
	Total    int            `json:"total"`
	Price    *PriceOverview `json:"price,omitempty"`
}

// bridgeClient is the shared process-wide client the bridge uses.
var bridgeClient = New(nil)

// SteamReviewForTitle resolves a title to a Steam app and returns its
// review summary plus current price when appdetails exposes one. Price
// failures degrade to Price=nil — the review block still ships. Resolution
// or review failures return the typed error so commands can degrade the
// whole steam block (sources_missing) instead of failing the command.
func SteamReviewForTitle(ctx context.Context, title string) (*SteamReview, error) {
	return enrichTitle(ctx, bridgeClient, title)
}

func enrichTitle(ctx context.Context, c *Client, title string) (*SteamReview, error) {
	if c == nil {
		c = New(nil)
	}
	appid, err := c.ResolveAppID(ctx, title)
	if err != nil {
		return nil, err
	}
	summary, err := c.ReviewSummary(ctx, appid)
	if err != nil {
		return nil, err
	}
	review := &SteamReview{
		AppID:    appid,
		Score:    summary.Score,
		Desc:     summary.Desc,
		Positive: summary.Positive,
		Negative: summary.Negative,
		Total:    summary.Total,
	}
	// Optional price enrichment: never fail the review block for it.
	if details, derr := c.AppDetails(ctx, appid); derr == nil && details != nil {
		review.Price = details.Price
	}
	return review, nil
}
