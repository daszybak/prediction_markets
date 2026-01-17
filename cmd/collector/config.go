package main

import (
	"fmt"
	"os"

	configtypes "github.com/daszybak/prediction_markets/internal/config"
	"go.yaml.in/yaml/v4"
)

type config struct {
	LogLevel string `yaml:"log_level"` // debug, info, warn, error
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
			WS struct {
				WebsocketURL       string               `yaml:"url"`
				MarketEndpoint     string               `yaml:"market_endpoint"`
			}
			GammaURL           string               `yaml:"gamma_url"`
			ClobURL            string               `yaml:"clob_url"`
			MarketSyncInterval configtypes.Duration `yaml:"market_sync_interval"`
		} `yaml:"polymarket"`
		Kalshi struct {
			APIURL        string                    `yaml:"api_url"`
			WSURL         string                    `yaml:"ws_url"`
			APIKeyID      string                    `yaml:"api_key_id"`
			APIPrivateKey configtypes.RSAPrivateKey `yaml:"api_private_key"`
		} `yaml:"kalshi"`
	} `yaml:"platforms"`
}

func readConfig(configPath *string) (*config, error) {
	rawConfig, err := os.ReadFile(*configPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't read file %s: %w", *configPath, err)
	}

	cfg := &config{}
	if err = yaml.Unmarshal(rawConfig, cfg); err != nil {
		return nil, fmt.Errorf("couldn't parse config: %w", err)
	}

	err = validateConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("couldn't validate config: %w", err)
	}

	return cfg, nil
}

func validateConfig(cfg *config) error {
	// Database
	if cfg.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if cfg.Database.Port <= 0 || cfg.Database.Port > 65535 {
		return fmt.Errorf("database.port must be between 1 and 65535")
	}
	if cfg.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if cfg.Database.Password == "" {
		return fmt.Errorf("database.password is required")
	}
	if cfg.Database.Database == "" {
		return fmt.Errorf("database.database is required")
	}
	if cfg.Database.PoolSize <= 0 {
		return fmt.Errorf("database.pool_size must be greater than 0")
	}
	if cfg.Database.SSLMode == "" {
		return fmt.Errorf("database.ssl_mode is required")
	}

	// Polymarket
	if cfg.Platforms.PolyMarket.WS.WebsocketURL == "" {
		return fmt.Errorf("platforms.polymarket.ws.url is required")
	}
	if cfg.Platforms.PolyMarket.WS.MarketEndpoint == "" {
		return fmt.Errorf("platforms.polymarket.ws.market_endpoint is required")
	}
	if cfg.Platforms.PolyMarket.GammaURL == "" {
		return fmt.Errorf("platforms.polymarket.gamma_url is required")
	}
	if cfg.Platforms.PolyMarket.ClobURL == "" {
		return fmt.Errorf("platforms.polymarket.clob_url is required")
	}

	// Kalshi
	if cfg.Platforms.Kalshi.APIURL == "" {
		return fmt.Errorf("platforms.kalshi.api_url is required")
	}
	if cfg.Platforms.Kalshi.WSURL == "" {
		return fmt.Errorf("platforms.kalshi.ws_url is required")
	}
	if cfg.Platforms.Kalshi.APIKeyID == "" {
		return fmt.Errorf("platforms.kalshi.api_key_id is required")
	}

	return nil
}
