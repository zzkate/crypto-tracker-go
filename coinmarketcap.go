package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type cmcError struct {
	Status  int
	Message string
}

func (e *cmcError) Error() string { return e.Message }

type cmcClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func newCmcClient(apiKey, baseURL string, timeoutMS int) *cmcClient {
	return &cmcClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond},
	}
}

func (c *cmcClient) fetchPrices(ctx context.Context, symbols []string, currency string) (map[string]float64, error) {
	unique := dedupUpper(symbols)
	if len(unique) == 0 {
		return map[string]float64{}, nil
	}

	u := c.baseURL + "/v1/cryptocurrency/quotes/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("symbol", strings.Join(unique, ","))
	q.Set("convert", currency)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-CMC_PRO_API_KEY", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &cmcError{
			Status:  resp.StatusCode,
			Message: fmt.Sprintf("CoinMarketCap request failed with status %d", resp.StatusCode),
		}
	}

	var payload struct {
		Data map[string]struct {
			Quote map[string]struct {
				Price *float64 `json:"price"`
			} `json:"quote"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	result := map[string]float64{}
	for symbol, entry := range payload.Data {
		if quote, ok := entry.Quote[currency]; ok && quote.Price != nil {
			result[strings.ToUpper(symbol)] = *quote.Price
		}
	}
	return result, nil
}

func dedupUpper(symbols []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range symbols {
		u := strings.ToUpper(s)
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}
