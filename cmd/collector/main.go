package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	configtypes "github.com/daszybak/prediction_markets/internal/config"
	"github.com/daszybak/prediction_markets/internal/polymarket/clob"
	"github.com/daszybak/prediction_markets/internal/polymarket/websocket"
	"github.com/daszybak/prediction_markets/internal/store"
	"go.yaml.in/yaml/v4"
)

type config struct {
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
		PoolSize int    `yaml:"pool_size"`
		SSLMode  string `yaml:"ssl_mode"`
	} `yaml:"database"`
	Platforms struct {
		PolyMarket struct {
			WebsocketURL string `yaml:"ws_url"`
			GammaURL     string `yaml:"gamma_url"`
			ClobURL      string `yaml:"clob_url"`
		} `yaml:"polymarket"`
		Kalshi struct {
			APIURL        string                    `yaml:"api_url"`
			WSURL         string                    `yaml:"ws_url"`
			APIKeyID      string                    `yaml:"api_key_id"`
			APIPrivateKey configtypes.RSAPrivateKey `yaml:"api_private_key"`
		} `yaml:"kalshi"`
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
	db := store.New(pool)
	defer db.Close()

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
			log.Fatalf("Couldn't read message: %v", err)
		}
		log.Printf("message: %s", msg)
	}
}
