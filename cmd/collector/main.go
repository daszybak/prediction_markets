package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/daszybak/prediction_markets/internal/polymarket/clob"
	"github.com/daszybak/prediction_markets/internal/polymarket/websocket"
	"github.com/daszybak/prediction_markets/internal/store"
)

func main() {
	configPath := flag.String("config", "configs/collector/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := readConfig(configPath)
	if err != nil {
		log.Fatalf("Couldn't read config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := store.NewPool(ctx, store.PoolConfig{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Database,
		PoolSize: cfg.Database.PoolSize,
		SSLMode:  cfg.Database.SSLMode,
	})
	if err != nil {
		log.Fatalf("Couldn't connect to database: %v", err)
	}
	_ = store.NewStore(pool)
	defer pool.Close()

	log.Println("Connected to database")

	polymarketClobClient := clob.New(cfg.Platforms.PolyMarket.ClobURL)

	// NOTE We should retrieve the markets from cache and run a separate go routine which will
	// scan for new markets across different prediction market platforms and match them together
	// or find correlation.
	markets, err := polymarketClobClient.GetAllMarkets()
	if err != nil {
		log.Printf("Couldn't get all markets: %v", err)
	}

	polymarketWebsocket, err := websocket.New(ctx, cfg.Platforms.PolyMarket.WebsocketURL+"/market")
	if err != nil {
		log.Fatalf("Couldn't open websocket connection: %v", err)
	}
	defer polymarketWebsocket.Close(ctx)

	tokenIDs := make([]string, 0)
	for _, m := range markets {
		for _, t := range m.Tokens {
			tokenIDs = append(tokenIDs, t.TokenID)
		}
	}

	fmt.Printf("token ids: %s", tokenIDs)
	if err := polymarketWebsocket.SubscribeMarket(ctx, tokenIDs, true, nil); err != nil {
		log.Fatalf("Couldn't send subscription: %v", err)
	}

	for {
		msg, err := polymarketWebsocket.ReadMessage(ctx)
		if err != nil {
			log.Printf("Couldn't read message: %v", err)
		}
		log.Printf("message: %s", msg)
	}
}
