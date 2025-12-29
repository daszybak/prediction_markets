package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/daszybak/prediction_markets/internal/polymarket/clob"
	"github.com/daszybak/prediction_markets/internal/polymarket/websocket"
	"go.yaml.in/yaml/v4"
)

type config struct {
	Platforms struct {
		PolyMarket struct {
			WebsocketURL string `yaml:"ws_url"`
			GammaURL     string `yaml:"gamma_url"`
			ClobURL      string `yaml:"clob_url"`
		} `yaml:"polymarket"`
	} `yaml:"platforms"`
}

func main() {
	configPath := flag.String("config", "configs/collector/config.yaml", "path to config file")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rawConfig, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatalf("Couldn't read config %s: %v", *configPath, err)
	}

	cfg := &config{}
	if err = yaml.Unmarshal(rawConfig, cfg); err != nil {
		log.Fatalf("Couldn't parse config: %v", err)
	}

	clobClient := clob.New(cfg.Platforms.PolyMarket.ClobURL)

	// NOTE We should retrieve the markets from cache and run a separate go routine which will
	// scan for new markets across different prediction market platforms and match them together
	// or find correlation.
	markets, err := clobClient.GetAllMarkets()
	if err != nil {
		log.Printf("Couldn't get all markets: %v", err)
	}

	ws, err := websocket.New(ctx, cfg.Platforms.PolyMarket.WebsocketURL+"/market")
	if err != nil {
		log.Fatalf("Couldn't open websocket connection: %v", err)
	}
	defer ws.Close(ctx)

	tokenIDs := make([]string, 0)
	for _, m := range markets {
		for _, t := range m.Tokens {
			tokenIDs = append(tokenIDs, t.TokenID)
		}
	}

	if err := ws.SubscribeMarket(ctx, tokenIDs, true, nil); err != nil {
		log.Fatalf("Couldn't send subscription: %v", err)
	}

	for {
		msg, err := ws.ReadMessage(ctx)
		if err != nil {
			log.Fatalf("Couldn't read message: %v", err)
		}
		log.Printf("message: %s", msg)
	}
}
