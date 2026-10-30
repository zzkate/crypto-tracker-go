package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var currencyRe = regexp.MustCompile(`^[A-Z]{2,10}$`)

type Config struct {
	Port            int
	AppAPIKey       string
	CmcAPIKey       string
	CmcBaseURL      string
	CmcTimeoutMS    int
	SyncIntervalMS  int
	Currency        string
	DatabaseURL     string
	CmcBatchSize    int
	CmcMaxRetries   int
	CmcRetryBackoff int
}

func positiveInt(env, name string, fallback int) (int, error) {
	if env == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(env)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}

func nonNegativeInt(env, name string, fallback int) (int, error) {
	if env == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(env)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return n, nil
}

func LoadConfig() (Config, error) {
	cfg := Config{}
	var err error

	cfg.AppAPIKey = os.Getenv("APP_API_KEY")
	if cfg.AppAPIKey == "" {
		return cfg, fmt.Errorf("APP_API_KEY is required")
	}

	cfg.Currency = os.Getenv("PRICE_CURRENCY")
	if cfg.Currency == "" {
		cfg.Currency = "USD"
	}
	if !currencyRe.MatchString(cfg.Currency) {
		return cfg, fmt.Errorf("PRICE_CURRENCY must be a currency code like USD")
	}

	if cfg.Port, err = positiveInt(os.Getenv("PORT"), "PORT", 3000); err != nil {
		return cfg, err
	}
	cfg.CmcAPIKey = os.Getenv("CMC_API_KEY")
	cfg.CmcBaseURL = strings.TrimRight(os.Getenv("CMC_BASE_URL"), "/")
	if cfg.CmcBaseURL == "" {
		cfg.CmcBaseURL = "https://pro-api.coinmarketcap.com"
	}
	if cfg.CmcTimeoutMS, err = positiveInt(os.Getenv("CMC_TIMEOUT_MS"), "CMC_TIMEOUT_MS", 5000); err != nil {
		return cfg, err
	}
	if cfg.SyncIntervalMS, err = positiveInt(os.Getenv("SYNC_INTERVAL_MS"), "SYNC_INTERVAL_MS", 60000); err != nil {
		return cfg, err
	}
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = "postgres://crypto:crypto@localhost:5432/crypto?sslmode=disable"
	}
	if cfg.CmcBatchSize, err = positiveInt(os.Getenv("CMC_BATCH_SIZE"), "CMC_BATCH_SIZE", 100); err != nil {
		return cfg, err
	}
	if cfg.CmcMaxRetries, err = nonNegativeInt(os.Getenv("CMC_MAX_RETRIES"), "CMC_MAX_RETRIES", 2); err != nil {
		return cfg, err
	}
	if cfg.CmcRetryBackoff, err = positiveInt(os.Getenv("CMC_RETRY_BACKOFF_MS"), "CMC_RETRY_BACKOFF_MS", 200); err != nil {
		return cfg, err
	}

	return cfg, nil
}
