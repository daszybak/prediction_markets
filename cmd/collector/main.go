package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/daszybak/prediction_markets/internal/platform"
	"github.com/daszybak/prediction_markets/internal/polymarket"
	"github.com/daszybak/prediction_markets/internal/store"
)

type collector struct {
	platforms map[string]platform.Platform
	store     *store.Store
	logger    *slog.Logger
}

func main() {
	configPath := flag.String("config", "configs/collector/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := readConfig(configPath)
	if err != nil {
		slog.Error("couldn't read config", "error", err)
		os.Exit(1)
	}

	var logLevel slog.Level
	if cfg.LogLevel != "" {
		if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
			slog.Error("invalid log_level", "value", cfg.LogLevel, "error", err)
			os.Exit(1)
		}
	}

	collector := &collector{
		platforms: make(map[string]platform.Platform),
	}
	collector.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	dbLogger := collector.logger.With("component", "database")
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
		dbLogger.Error("couldn't connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	dbLogger.Info("connected to database")

	collector.store = store.NewStore(pool)

	polymarketLogger := collector.logger.With("component", "polymarket")
	collector.platforms["polymarket"] = polymarket.New(polymarket.Config{
		ClobURL:      cfg.Platforms.PolyMarket.ClobURL,
		GammaURL:     cfg.Platforms.PolyMarket.GammaURL,
		WebsocketURL: cfg.Platforms.PolyMarket.WebsocketURL,
		MarketSyncInterval: cfg.Platforms.PolyMarket.MarketSyncInterval.Duration(),
	}, collector.store, polymarketLogger)

	for platformName, platform := range collector.platforms {
		err = platform.Start(ctx); if err != nil {
			collector.logger.Error("starting platform", "plaftorm", platformName, "error", err)
		}
	}
}
