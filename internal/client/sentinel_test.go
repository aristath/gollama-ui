package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSentinelClient(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		expectedURL string
	}{
		{
			name:        "With custom URL",
			baseURL:     "http://custom:9000",
			expectedURL: "http://custom:9000",
		},
		{
			name:        "With empty URL uses default",
			baseURL:     "",
			expectedURL: "http://localhost:8081",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewSentinelClient(tt.baseURL)
			assert.NotNil(t, client)
			assert.Equal(t, tt.expectedURL, client.baseURL)
			assert.NotNil(t, client.httpClient)
			assert.Equal(t, 15*time.Second, client.httpClient.Timeout)
		})
	}
}

func TestSentinelClient_HealthCheck(t *testing.T) {
	t.Run("successful health check", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/health", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"healthy"}`)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		err := client.HealthCheck(ctx)
		assert.NoError(t, err)
	})

	t.Run("health check returns error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		err := client.HealthCheck(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "503")
	})

	t.Run("health check with context timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err := client.HealthCheck(ctx)
		assert.Error(t, err)
	})
}

func TestSentinelClient_GetPortfolioSummary(t *testing.T) {
	t.Run("successful portfolio summary", func(t *testing.T) {
		expectedSummary := PortfolioSummary{
			TotalValue:    50000.00,
			CashBalance:   5000.00,
			PositionCount: 25,
			Allocations: map[string]float64{
				"EU":   0.40,
				"ASIA": 0.30,
				"US":   0.30,
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/portfolio/summary", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(expectedSummary)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		summary, err := client.GetPortfolioSummary(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, summary)
		assert.Equal(t, expectedSummary.TotalValue, summary.TotalValue)
		assert.Equal(t, expectedSummary.CashBalance, summary.CashBalance)
		assert.Equal(t, expectedSummary.PositionCount, summary.PositionCount)
	})

	t.Run("portfolio summary with error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"error":"server error"}`)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		_, err := client.GetPortfolioSummary(ctx)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})
}

func TestSentinelClient_GetPositions(t *testing.T) {
	t.Run("successful position list", func(t *testing.T) {
		positions := []Position{
			{
				Symbol:       "AAPL",
				Quantity:     10,
				AvgPrice:     150.0,
				CurrentPrice: 175.0,
				Currency:     "USD",
				CurrencyRate: 0.92,
				MarketValueEUR: 1610.0,
				StockName:    "Apple Inc.",
				Industry:     "Technology",
				Country:      "US",
			},
			{
				Symbol:       "ASML",
				Quantity:     5,
				AvgPrice:     600.0,
				CurrentPrice: 650.0,
				Currency:     "EUR",
				CurrencyRate: 1.0,
				MarketValueEUR: 3250.0,
				StockName:    "ASML Holding NV",
				Industry:     "Semiconductors",
				Country:      "NL",
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/portfolio/", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(positions)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		result, err := client.GetPositions(ctx)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "AAPL", result[0].Symbol)
		assert.Equal(t, 10.0, result[0].Quantity)
		assert.Equal(t, "ASML", result[1].Symbol)
	})

	t.Run("empty position list", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]Position{})
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		result, err := client.GetPositions(ctx)

		assert.NoError(t, err)
		assert.Len(t, result, 0)
	})
}

func TestSentinelClient_GetAllOpportunities(t *testing.T) {
	t.Run("successful opportunities list", func(t *testing.T) {
		oppsResp := OpportunitiesResponse{
			Data: struct {
				Opportunities []Opportunity          `json:"opportunities"`
				Count         int                    `json:"count"`
				ByCategory    map[string]int         `json:"by_category"`
			}{
				Opportunities: []Opportunity{
					{
						Symbol:   "AAPL",
						ISIN:     "US0378331005",
						Name:     "Apple Inc.",
						Side:     "BUY",
						Quantity: 5,
						Price:    175.0,
						ValueEUR: 805.0,
						Reason:   "Price below 20-day MA",
						Priority: 8.5,
						Category: "opportunity_buys",
					},
				},
				Count: 1,
				ByCategory: map[string]int{
					"opportunity_buys": 1,
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/opportunities/all", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(oppsResp)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		result, err := client.GetAllOpportunities(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 1, result.Data.Count)
		assert.Len(t, result.Data.Opportunities, 1)
		assert.Equal(t, "AAPL", result.Data.Opportunities[0].Symbol)
	})

	t.Run("no opportunities", func(t *testing.T) {
		oppsResp := OpportunitiesResponse{
			Data: struct {
				Opportunities []Opportunity          `json:"opportunities"`
				Count         int                    `json:"count"`
				ByCategory    map[string]int         `json:"by_category"`
			}{
				Opportunities: []Opportunity{},
				Count:         0,
				ByCategory:    map[string]int{},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(oppsResp)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		result, err := client.GetAllOpportunities(ctx)

		assert.NoError(t, err)
		assert.Equal(t, 0, result.Data.Count)
		assert.Len(t, result.Data.Opportunities, 0)
	})
}

func TestSentinelClient_GetPortfolioRisk(t *testing.T) {
	t.Run("successful risk metrics", func(t *testing.T) {
		riskResp := struct {
			Data *RiskMetrics `json:"data"`
		}{
			Data: &RiskMetrics{
				VaR:                  2500.0,
				CVaR:                 3200.0,
				PortfolioVolatility:  0.185,
				SharpeRatio:          1.35,
				SortinoRatio:         1.50,
				MaxDrawdown:          -0.123,
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/snapshots/risk-snapshot", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(riskResp)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		result, err := client.GetPortfolioRisk(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2500.0, result.VaR)
		assert.Equal(t, 1.35, result.SharpeRatio)
	})
}

func TestSentinelClient_GetMarketContext(t *testing.T) {
	t.Run("successful market context", func(t *testing.T) {
		contextResp := struct {
			Data *MarketContext `json:"data"`
		}{
			Data: &MarketContext{
				Regime: struct {
					RawScore      float64 `json:"raw_score"`
					SmoothedScore float64 `json:"smoothed_score"`
					DiscreteRegime string  `json:"discrete_regime"`
				}{
					RawScore:      0.65,
					SmoothedScore: 0.63,
					DiscreteRegime: "bullish",
				},
				AdaptiveWeights: map[string]float64{
					"momentum": 0.45,
					"value":    0.30,
					"quality":  0.25,
				},
				MarketHours: struct {
					Status        string   `json:"status"`
					OpenMarkets   []string `json:"open_markets"`
					ClosedMarkets []string `json:"closed_markets"`
				}{
					Status:        "open",
					OpenMarkets:   []string{"NYSE", "NASDAQ"},
					ClosedMarkets: []string{"XETRA", "LSE"},
				},
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/snapshots/market-context", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(contextResp)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		result, err := client.GetMarketContext(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "bullish", result.Regime.DiscreteRegime)
		assert.Equal(t, 0.45, result.AdaptiveWeights["momentum"])
	})
}

func TestSentinelClient_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{invalid json}`)
	}))
	defer server.Close()

	client := NewSentinelClient(server.URL)
	ctx := context.Background()
	_, err := client.GetPortfolioSummary(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

func TestSentinelClient_NotFoundError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"error":"not found"}`)
	}))
	defer server.Close()

	client := NewSentinelClient(server.URL)
	ctx := context.Background()
	_, err := client.GetPortfolioSummary(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestSentinelClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewSentinelClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetPortfolioSummary(ctx)
	assert.Error(t, err)
}

func TestSentinelClient_CompleteSnapshot(t *testing.T) {
	t.Run("successful complete snapshot", func(t *testing.T) {
		snapshot := CompleteSnapshot{
			Data: struct {
				Portfolio struct {
					TotalValue    float64            `json:"total_value"`
					CashBalances  map[string]float64 `json:"cash_balances"`
					PositionCount int                `json:"position_count"`
				} `json:"portfolio"`
				MarketContext struct {
					RegimeScore     float64            `json:"regime_score"`
					DiscreteRegime  string             `json:"discrete_regime"`
					MarketOpen      bool               `json:"market_open"`
					AdaptiveWeights map[string]float64 `json:"adaptive_weights"`
				} `json:"market_context"`
				Risk map[string]interface{} `json:"risk"`
			}{},
		}
		snapshot.Data.Portfolio.TotalValue = 50000.0
		snapshot.Data.Portfolio.PositionCount = 25
		snapshot.Data.MarketContext.DiscreteRegime = "bullish"

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/snapshots/complete", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(snapshot)
		}))
		defer server.Close()

		client := NewSentinelClient(server.URL)
		ctx := context.Background()
		result, err := client.GetCompleteSnapshot(ctx)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 50000.0, result.Data.Portfolio.TotalValue)
		assert.Equal(t, 25, result.Data.Portfolio.PositionCount)
		assert.Equal(t, "bullish", result.Data.MarketContext.DiscreteRegime)
	})
}
