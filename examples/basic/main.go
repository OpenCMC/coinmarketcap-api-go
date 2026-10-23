// Latest Bitcoin and Ethereum prices with a Pro API key.
//
// Run:
//
//	CMC_PRO_API_KEY=your-key go run ./examples/basic
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/OpenCMC/coinmarketcap-api-go/v5"
)

func main() {
	cmc, err := coinmarketcap.New(coinmarketcap.WithAPIKey(os.Getenv("CMC_PRO_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}

	// 1 = Bitcoin, 1027 = Ethereum.
	res, err := cmc.Cryptocurrency.QuotesLatest(context.Background(), &coinmarketcap.CryptocurrencyQuotesLatestParams{
		ID: coinmarketcap.String("1,1027"),
	})
	var rateLimited *coinmarketcap.RateLimitError
	switch {
	case errors.As(err, &rateLimited):
		log.Fatalf("rate limited, retry after %ss", rateLimited.Header.Get("Retry-After"))
	case err != nil:
		log.Fatal(err)
	}

	for _, coin := range res.Data {
		if coin.Name == nil || coin.Quote == nil {
			continue
		}
		for _, q := range *coin.Quote {
			if q.Symbol != nil && *q.Symbol == "USD" && q.Price != nil {
				fmt.Printf("%s: $%.2f\n", *coin.Name, *q.Price)
			}
		}
	}
}
