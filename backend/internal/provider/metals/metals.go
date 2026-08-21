// Package metals fetches live gold/silver spot prices from a
// metals.dev-style JSON API. Prices are parsed via json.Number straight
// into decimals — no float ever touches a price.
package metals

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/service"
)

const Source = "metals.dev"

type Client struct {
	httpClient *http.Client
	apiURL     string
	apiKey     string
	currency   string
}

func NewClient(apiURL, apiKey, currency string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiURL:     apiURL,
		apiKey:     apiKey,
		currency:   currency,
	}
}

type apiResponse struct {
	Status string                 `json:"status"`
	Metals map[string]json.Number `json:"metals"`
}

// FetchLatest returns current gold and silver prices per gram.
func (c *Client) FetchLatest(ctx context.Context) ([]service.MetalPrice, error) {
	u, err := url.Parse(c.apiURL)
	if err != nil {
		return nil, fmt.Errorf("metals api url: %w", err)
	}
	q := u.Query()
	q.Set("api_key", c.apiKey)
	q.Set("currency", c.currency)
	q.Set("unit", "g")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch metal prices: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metals api returned status %d", resp.StatusCode)
	}

	var body apiResponse
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("decode metals response: %w", err)
	}
	if body.Status != "success" {
		return nil, fmt.Errorf("metals api status %q", body.Status)
	}

	now := time.Now()
	var prices []service.MetalPrice
	for _, metal := range []string{"gold", "silver"} {
		num, ok := body.Metals[metal]
		if !ok {
			return nil, fmt.Errorf("metals response missing %s", metal)
		}
		price, err := decimal.NewFromString(num.String())
		if err != nil || !price.IsPositive() {
			return nil, fmt.Errorf("invalid %s price %q", metal, num)
		}
		prices = append(prices, service.MetalPrice{
			Metal:        metal,
			PricePerGram: price,
			Currency:     c.currency,
			Source:       Source,
			FetchedAt:    now,
		})
	}
	return prices, nil
}
