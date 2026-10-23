//go:build live

// Live tests call the real API: one endpoint per category with an API key,
// plus keyless calls in the public environment. Run them with
//
//	CMC_PRO_API_KEY=your-key go test -tags live -run Live -v ./...
//
// The keyless API allows only a few anonymous calls a minute, so those calls
// are spaced out.
package coinmarketcap

import (
	"context"
	"os"
	"testing"
	"time"
)

const (
	liveUSDC = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	livePool = "0x88e6a0c2ddd26feeb64f039a2c41296fcb3f5640" // Uniswap v3 USDC/WETH
)

type liveCall struct {
	name string
	call func(ctx context.Context, c *Client) error
}

func ignore[T any](_ T, err error) error { return err }

func TestLivePro(t *testing.T) {
	key := os.Getenv("CMC_PRO_API_KEY")
	if key == "" {
		t.Skip("CMC_PRO_API_KEY is not set")
	}
	cmc := mustNew(t, WithAPIKey(key))
	calls := []liveCall{
		{"CmcAi.CoinsLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.CmcAi.CoinsLatest(ctx, &CmcAiCoinsLatestParams{CryptoID: String("1")}))
		}},
		{"CmcIndex.IndexCMC20Historical", func(ctx context.Context, c *Client) error {
			return ignore(c.CmcIndex.IndexCMC20Historical(ctx, &CmcIndexIndexCMC20HistoricalParams{Count: String("3")}))
		}},
		{"Community.TrendingToken", func(ctx context.Context, c *Client) error {
			return ignore(c.Community.TrendingToken(ctx, &CommunityTrendingTokenParams{Limit: Int(3)}))
		}},
		{"Content.Latest", func(ctx context.Context, c *Client) error {
			return ignore(c.Content.Latest(ctx, &ContentLatestParams{Limit: Int(3)}))
		}},
		{"CryptoOthers.BlockchainStatisticsLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.CryptoOthers.BlockchainStatisticsLatest(ctx, &CryptoOthersBlockchainStatisticsLatestParams{Symbol: String("BTC")}))
		}},
		{"Cryptocurrency.QuotesLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.Cryptocurrency.QuotesLatest(ctx, &CryptocurrencyQuotesLatestParams{ID: String("1,1027")}))
		}},
		{"Cryptocurrency.ListingsLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.Cryptocurrency.ListingsLatest(ctx, &CryptocurrencyListingsLatestParams{Limit: String("3")}))
		}},
		{"Deprecated.CryptocurrencyInfo", func(ctx context.Context, c *Client) error {
			return ignore(c.Deprecated.CryptocurrencyInfo(ctx, &DeprecatedCryptocurrencyInfoParams{ID: String("1")}))
		}},
		{"Derivatives.LiquidationsQuotesLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.Derivatives.LiquidationsQuotesLatest(ctx, nil))
		}},
		{"Exchange.Info", func(ctx context.Context, c *Client) error {
			return ignore(c.Exchange.Info(ctx, &ExchangeInfoParams{ID: String("270")}))
		}},
		{"GlobalMetrics.QuotesLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.GlobalMetrics.QuotesLatest(ctx, nil))
		}},
		{"Holder.Count", func(ctx context.Context, c *Client) error {
			return ignore(c.Holder.Count(ctx, &HolderCountParams{Platform: "ethereum", TokenAddress: liveUSDC}))
		}},
		{"Ohlcv.KlineCandles", func(ctx context.Context, c *Client) error {
			return ignore(c.Ohlcv.KlineCandles(ctx, &OhlcvKlineCandlesParams{
				Platform: String("ethereum"), Address: String(livePool), Interval: String("1h"), Limit: Ptr(int32(3)),
			}))
		}},
		{"Platform.List", func(ctx context.Context, c *Client) error {
			return ignore(c.Platform.List(ctx))
		}},
		{"RealWorldAssets.Map", func(ctx context.Context, c *Client) error {
			return ignore(c.RealWorldAssets.Map(ctx, &RealWorldAssetsMapParams{Limit: Int(3)}))
		}},
		{"Token.BatchGetTokenPrice (POST)", func(ctx context.Context, c *Client) error {
			tokens := []PlatformAddress{{Platform: String("ethereum"), Address: String(liveUSDC)}}
			return ignore(c.Token.BatchGetTokenPrice(ctx, TokenBatchGetTokenPriceBody{Tokens: &tokens}))
		}},
		{"Token.Price", func(ctx context.Context, c *Client) error {
			return ignore(c.Token.Price(ctx, &TokenPriceParams{Platform: String("ethereum"), Address: String(liveUSDC)}))
		}},
		{"Tools.KeyInfo", func(ctx context.Context, c *Client) error {
			return ignore(c.Tools.KeyInfo(ctx))
		}},
	}
	runLive(t, cmc, calls, 300*time.Millisecond)
}

func TestLiveKeyless(t *testing.T) {
	cmc := mustNew(t, WithEnvironment(EnvironmentPublic), WithMaxRetries(4))
	calls := []liveCall{
		{"Cryptocurrency.SimplePrice", func(ctx context.Context, c *Client) error {
			return ignore(c.Cryptocurrency.SimplePrice(ctx, &CryptocurrencySimplePriceParams{ID: String("1,1027")}))
		}},
		{"Cryptocurrency.ListingsLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.Cryptocurrency.ListingsLatest(ctx, &CryptocurrencyListingsLatestParams{Limit: String("3")}))
		}},
		{"GlobalMetrics.FearandgreedLatest", func(ctx context.Context, c *Client) error {
			return ignore(c.GlobalMetrics.FearandgreedLatest(ctx))
		}},
		{"Token.Search", func(ctx context.Context, c *Client) error {
			return ignore(c.Token.Search(ctx, &TokenSearchParams{Q: String("pepe"), Limit: Ptr(int32(3))}))
		}},
		{"Holder.Holders (POST)", func(ctx context.Context, c *Client) error {
			return ignore(c.Holder.Holders(ctx, HolderHoldersBody{
				Platform: String("ethereum"), TokenAddress: String(liveUSDC), Tag: String("tag_all"),
			}))
		}},
	}
	runLive(t, cmc, calls, 3*time.Second)
}

func runLive(t *testing.T, cmc *Client, calls []liveCall, pause time.Duration) {
	for i, c := range calls {
		if i > 0 {
			time.Sleep(pause)
		}
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := c.call(ctx, cmc); err != nil {
				t.Error(err)
			}
		})
	}
}
