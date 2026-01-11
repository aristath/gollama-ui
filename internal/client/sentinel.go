package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SentinelClient queries the Sentinel portfolio management API
type SentinelClient struct {
	baseURL    string
	httpClient *http.Client
}

// Data structures for Sentinel API responses

type CompleteSnapshot struct {
	Data struct {
		Portfolio struct {
			TotalValue      float64            `json:"total_value"`
			CashBalances    map[string]float64 `json:"cash_balances"`
			PositionCount   int                `json:"position_count"`
		} `json:"portfolio"`
		MarketContext struct {
			RegimeScore     float64            `json:"regime_score"`
			DiscreteRegime  string             `json:"discrete_regime"`
			MarketOpen      bool               `json:"market_open"`
			AdaptiveWeights map[string]float64 `json:"adaptive_weights"`
		} `json:"market_context"`
		Risk map[string]interface{} `json:"risk"`
	} `json:"data"`
	Metadata struct {
		Timestamp  string `json:"timestamp"`
		SnapshotID int64  `json:"snapshot_id"`
	} `json:"metadata"`
}

type PortfolioSummary struct {
	TotalValue   float64            `json:"total_value"`
	CashBalance  float64            `json:"cash_balance"`
	Allocations  map[string]float64 `json:"allocations"`
	PositionCount int               `json:"position_count"`
}

type Position struct {
	Symbol            string  `json:"symbol"`
	Quantity          float64 `json:"quantity"`
	AvgPrice          float64 `json:"avg_price"`
	CurrentPrice      float64 `json:"current_price"`
	Currency          string  `json:"currency"`
	CurrencyRate      float64 `json:"currency_rate"`
	MarketValueEUR    float64 `json:"market_value_eur"`
	LastUpdated       string  `json:"last_updated"`
	StockName         string  `json:"stock_name"`
	Industry          string  `json:"industry"`
	Country           string  `json:"country"`
	FullExchangeName  string  `json:"fullExchangeName"`
}

type OpportunitiesResponse struct {
	Data struct {
		Opportunities []Opportunity          `json:"opportunities"`
		Count         int                    `json:"count"`
		ByCategory    map[string]int         `json:"by_category"`
	} `json:"data"`
	Metadata struct {
		Timestamp string `json:"timestamp"`
	} `json:"metadata"`
}

type Opportunity struct {
	Symbol    string  `json:"symbol"`
	ISIN      string  `json:"isin"`
	Name      string  `json:"name"`
	Side      string  `json:"side"`
	Quantity  float64 `json:"quantity"`
	Price     float64 `json:"price"`
	ValueEUR  float64 `json:"value_eur"`
	Currency  string  `json:"currency"`
	Reason    string  `json:"reason"`
	Priority  float64 `json:"priority"`
	Category  string  `json:"category"`
}

type RecommendationsResponse struct {
	Data struct {
		Recommendations []interface{} `json:"recommendations"`
	} `json:"data"`
	Metadata struct {
		Timestamp string `json:"timestamp"`
	} `json:"metadata"`
}

type RiskMetrics struct {
	VaR                float64 `json:"var"`
	CVaR               float64 `json:"cvar"`
	PortfolioVolatility float64 `json:"portfolio_volatility"`
	SharpeRatio        float64 `json:"sharpe_ratio"`
	SortinoRatio       float64 `json:"sortino_ratio"`
	MaxDrawdown        float64 `json:"max_drawdown"`
}

type AllocationDeviations struct {
	Allocations map[string]struct {
		Current  float64 `json:"current"`
		Target   float64 `json:"target"`
		Deviation float64 `json:"deviation"`
	} `json:"allocations"`
	Status string `json:"status"`
}

type MarketContext struct {
	Regime struct {
		RawScore     float64 `json:"raw_score"`
		SmoothedScore float64 `json:"smoothed_score"`
		DiscreteRegime string `json:"discrete_regime"`
	} `json:"regime"`
	AdaptiveWeights map[string]float64 `json:"adaptive_weights"`
	MarketHours struct {
		Status        string   `json:"status"`
		OpenMarkets   []string `json:"open_markets"`
		ClosedMarkets []string `json:"closed_markets"`
	} `json:"market_hours"`
}

// NewSentinelClient creates a new Sentinel portfolio client
func NewSentinelClient(baseURL string) *SentinelClient {
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	return &SentinelClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// HealthCheck verifies Sentinel is accessible
func (sc *SentinelClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", sc.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	req.Header.Set("User-Agent", "gollama-ui/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := sc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetCompleteSnapshot fetches the complete portfolio snapshot
func (sc *SentinelClient) GetCompleteSnapshot(ctx context.Context) (*CompleteSnapshot, error) {
	url := fmt.Sprintf("%s/api/snapshots/complete", sc.baseURL)
	snapshot := &CompleteSnapshot{}
	_, err := sc.getJSON(ctx, url, snapshot)
	return snapshot, err
}

// GetPortfolioSummary fetches portfolio summary
func (sc *SentinelClient) GetPortfolioSummary(ctx context.Context) (*PortfolioSummary, error) {
	url := fmt.Sprintf("%s/api/portfolio/summary", sc.baseURL)
	summary := &PortfolioSummary{}
	_, err := sc.getJSON(ctx, url, summary)
	return summary, err
}

// GetPositions fetches all portfolio positions
func (sc *SentinelClient) GetPositions(ctx context.Context) ([]Position, error) {
	url := fmt.Sprintf("%s/api/portfolio/", sc.baseURL)
	resp := make([]Position, 0)
	_, err := sc.getJSON(ctx, url, &resp)
	return resp, err
}

// GetAllOpportunities fetches all trading opportunities
func (sc *SentinelClient) GetAllOpportunities(ctx context.Context) (*OpportunitiesResponse, error) {
	url := fmt.Sprintf("%s/api/opportunities/all", sc.baseURL)
	opps := &OpportunitiesResponse{}
	_, err := sc.getJSON(ctx, url, opps)
	return opps, err
}

// GetRecommendations fetches planning recommendations
func (sc *SentinelClient) GetRecommendations(ctx context.Context) (*RecommendationsResponse, error) {
	url := fmt.Sprintf("%s/api/planning/recommendations", sc.baseURL)
	recs := &RecommendationsResponse{}
	_, err := sc.getJSON(ctx, url, recs)
	return recs, err
}

// GetPortfolioRisk fetches portfolio risk metrics
func (sc *SentinelClient) GetPortfolioRisk(ctx context.Context) (*RiskMetrics, error) {
	url := fmt.Sprintf("%s/api/snapshots/risk-snapshot", sc.baseURL)
	resp := &struct {
		Data *RiskMetrics `json:"data"`
	}{}

	_, err := sc.getJSON(ctx, url, resp)
	if err != nil {
		return nil, err
	}

	if resp.Data == nil {
		return &RiskMetrics{}, nil
	}

	return resp.Data, nil
}

// GetAllocationDeviations fetches allocation deviations from targets
func (sc *SentinelClient) GetAllocationDeviations(ctx context.Context) (*AllocationDeviations, error) {
	url := fmt.Sprintf("%s/api/allocation/deviations", sc.baseURL)
	deviations := &AllocationDeviations{}
	_, err := sc.getJSON(ctx, url, deviations)
	return deviations, err
}

// GetMarketContext fetches current market context and regime
func (sc *SentinelClient) GetMarketContext(ctx context.Context) (*MarketContext, error) {
	url := fmt.Sprintf("%s/api/snapshots/market-context", sc.baseURL)
	resp := &struct {
		Data *MarketContext `json:"data"`
	}{}

	_, err := sc.getJSON(ctx, url, resp)
	if err != nil {
		return nil, err
	}

	if resp.Data == nil {
		return &MarketContext{}, nil
	}

	return resp.Data, nil
}

// getJSON is a helper to make GET requests and parse JSON responses
func (sc *SentinelClient) getJSON(ctx context.Context, url string, result interface{}) (interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "gollama-ui/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := sc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sentinel returned status %d: %s", resp.StatusCode, string(body))
	}

	// Limit response size to 50MB
	limitedReader := io.LimitReader(resp.Body, 50*1024*1024)
	if err := json.NewDecoder(limitedReader).Decode(result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result, nil
}
