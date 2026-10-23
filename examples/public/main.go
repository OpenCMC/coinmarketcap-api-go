// Prices from the keyless public API: no API key or account needed.
//
// Run:
//
//	go run ./examples/public
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/OpenCMC/coinmarketcap-api-go/v5"
)

func main() {
	cmc, err := coinmarketcap.New(coinmarketcap.WithEnvironment(coinmarketcap.EnvironmentPublic))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	prices, err := cmc.Cryptocurrency.SimplePrice(ctx, &coinmarketcap.CryptocurrencySimplePriceParams{
		ID: coinmarketcap.String("1,1027"), // Bitcoin, Ethereum
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, coin := range prices.Data {
		for _, q := range coin.Quotes {
			fmt.Printf("%s: %.2f %s\n", coin.Name, q.Price, q.Symbol)
		}
	}

	top, err := cmc.Cryptocurrency.ListingsLatest(ctx, &coinmarketcap.CryptocurrencyListingsLatestParams{
		Limit: coinmarketcap.String("5"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Top 5 by market cap:")
	for _, coin := range top.Data {
		if coin.CmcRank != nil && coin.Name != nil {
			fmt.Printf("%d. %s\n", *coin.CmcRank, *coin.Name)
		}
	}
}
