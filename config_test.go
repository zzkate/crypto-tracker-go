package main

import "testing"

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"APP_API_KEY", "PRICE_CURRENCY", "PORT", "CMC_API_KEY", "CMC_BASE_URL",
		"CMC_TIMEOUT_MS", "SYNC_INTERVAL_MS", "DATABASE_URL",
		"CMC_BATCH_SIZE", "CMC_MAX_RETRIES", "CMC_RETRY_BACKOFF_MS",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_API_KEY", "secret")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want 3000", cfg.Port)
	}
	if cfg.CmcBaseURL != "https://pro-api.coinmarketcap.com" {
		t.Errorf("CmcBaseURL = %q", cfg.CmcBaseURL)
	}
	if cfg.CmcTimeoutMS != 5000 {
		t.Errorf("CmcTimeoutMS = %d", cfg.CmcTimeoutMS)
	}
	if cfg.SyncIntervalMS != 60000 {
		t.Errorf("SyncIntervalMS = %d", cfg.SyncIntervalMS)
	}
	if cfg.Currency != "USD" {
		t.Errorf("Currency = %q", cfg.Currency)
	}
	if cfg.DatabaseURL != "postgres://crypto:crypto@localhost:5432/crypto?sslmode=disable" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.CmcBatchSize != 100 || cfg.CmcMaxRetries != 2 || cfg.CmcRetryBackoff != 200 {
		t.Errorf("batch/retry defaults = %d/%d/%d", cfg.CmcBatchSize, cfg.CmcMaxRetries, cfg.CmcRetryBackoff)
	}
	if cfg.CmcAPIKey != "" {
		t.Errorf("CmcAPIKey should be empty by default")
	}
}

func TestLoadConfigMissingAPIKey(t *testing.T) {
	clearConfigEnv(t)
	if _, err := LoadConfig(); err == nil {
		t.Fatal("want error without APP_API_KEY")
	}
}

func TestLoadConfigInvalidCurrency(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_API_KEY", "secret")
	for _, v := range []string{"usd", "U S", "US DOLLAR", "USDT "} {
		t.Setenv("PRICE_CURRENCY", v)
		if _, err := LoadConfig(); err == nil {
			t.Errorf("PRICE_CURRENCY=%q should error", v)
		}
	}
}

func TestLoadConfigInvalidPort(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_API_KEY", "secret")
	for _, v := range []string{"0", "-1", "abc"} {
		t.Setenv("PORT", v)
		if _, err := LoadConfig(); err == nil {
			t.Errorf("PORT=%q should error", v)
		}
	}
}

func TestLoadConfigCustom(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_API_KEY", "secret")
	t.Setenv("PORT", "8080")
	t.Setenv("PRICE_CURRENCY", "EUR")
	t.Setenv("CMC_API_KEY", "cmc-key")
	t.Setenv("CMC_BASE_URL", "https://example.com/")
	t.Setenv("CMC_TIMEOUT_MS", "10000")
	t.Setenv("SYNC_INTERVAL_MS", "30000")
	t.Setenv("DATABASE_URL", "postgres://x:y@h/db")
	t.Setenv("CMC_BATCH_SIZE", "50")
	t.Setenv("CMC_MAX_RETRIES", "5")
	t.Setenv("CMC_RETRY_BACKOFF_MS", "500")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 8080 || cfg.Currency != "EUR" || cfg.CmcAPIKey != "cmc-key" {
		t.Errorf("custom = %+v", cfg)
	}
	if cfg.CmcBaseURL != "https://example.com" {
		t.Errorf("CmcBaseURL should be trimmed: %q", cfg.CmcBaseURL)
	}
	if cfg.CmcTimeoutMS != 10000 || cfg.SyncIntervalMS != 30000 {
		t.Errorf("timeouts = %d/%d", cfg.CmcTimeoutMS, cfg.SyncIntervalMS)
	}
	if cfg.DatabaseURL != "postgres://x:y@h/db" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.CmcBatchSize != 50 || cfg.CmcMaxRetries != 5 || cfg.CmcRetryBackoff != 500 {
		t.Errorf("batch/retry = %d/%d/%d", cfg.CmcBatchSize, cfg.CmcMaxRetries, cfg.CmcRetryBackoff)
	}
}

func TestLoadConfigZeroRetriesAllowed(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_API_KEY", "secret")
	t.Setenv("CMC_MAX_RETRIES", "0")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CmcMaxRetries != 0 {
		t.Errorf("CmcMaxRetries = %d, want 0", cfg.CmcMaxRetries)
	}
}

func TestLoadConfigNegativeRetries(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("APP_API_KEY", "secret")
	t.Setenv("CMC_MAX_RETRIES", "-1")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("want error for negative CMC_MAX_RETRIES")
	}
}
