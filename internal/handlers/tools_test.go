package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aristath/gollama-ui/internal/client"
	"github.com/stretchr/testify/assert"
)

// createTestToolSettings creates a temporary tool settings file for testing
func createTestToolSettings(enableWebSearch, enableFeeds, enableSentinel bool) *ToolSettings {
	tmpFile, _ := os.CreateTemp("", "tool-settings-*.json")
	tmpFile.Close()

	settings := &ToolSettings{
		EnableWebSearch: enableWebSearch,
		EnableFeeds:     enableFeeds,
		EnableSentinel:  enableSentinel,
		configPath:      tmpFile.Name(),
	}

	return settings
}

// cleanupTestSettings removes the temporary settings file
func cleanupTestSettings(ts *ToolSettings) {
	if ts.configPath != "" {
		os.Remove(ts.configPath)
	}
}

func TestToolExecutor_GetAvailableTools_AllDisabled(t *testing.T) {
	settings := createTestToolSettings(false, false, false)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	tools := executor.GetAvailableTools()

	assert.Len(t, tools, 0)
}

func TestToolExecutor_GetAvailableTools_WebSearchOnly(t *testing.T) {
	settings := createTestToolSettings(true, false, false)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	tools := executor.GetAvailableTools()

	assert.Len(t, tools, 1)
	assert.Equal(t, "web_search", tools[0].Function.Name)
}

func TestToolExecutor_GetAvailableTools_SentinelOnly(t *testing.T) {
	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	tools := executor.GetAvailableTools()

	assert.Len(t, tools, 1)
	assert.Equal(t, "analyze_portfolio", tools[0].Function.Name)
	assert.Contains(t, tools[0].Function.Description, "Sentinel")
}

func TestToolExecutor_GetAvailableTools_AllEnabled(t *testing.T) {
	settings := createTestToolSettings(true, true, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	tools := executor.GetAvailableTools()

	// Can be 2 or 3 depending on whether get_news is available (it requires custom feeds)
	assert.GreaterOrEqual(t, len(tools), 2)

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Function.Name] = true
	}

	assert.True(t, toolNames["web_search"])
	assert.True(t, toolNames["analyze_portfolio"])
}

func TestToolExecutor_AnalyzePortfolioTool_Definition(t *testing.T) {
	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	tools := executor.GetAvailableTools()

	assert.Len(t, tools, 1)
	tool := tools[0]
	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "analyze_portfolio", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)

	// Check parameters schema
	params := tool.Function.Parameters
	assert.Equal(t, "object", params["type"])

	properties := params["properties"].(map[string]interface{})
	assert.NotNil(t, properties["query_type"])
	assert.NotNil(t, properties["focus_area"])

	required := params["required"].([]interface{})
	assert.Contains(t, required, "query_type")
}

func TestToolExecutor_ExecuteToolCall_PortfolioOverview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/portfolio/summary" {
			json.NewEncoder(w).Encode(client.PortfolioSummary{
				TotalValue:    50000.00,
				CashBalance:   5000.00,
				PositionCount: 25,
				Allocations: map[string]float64{
					"EU":   0.40,
					"ASIA": 0.30,
					"US":   0.30,
				},
			})
		} else if r.URL.Path == "/api/portfolio/" {
			json.NewEncoder(w).Encode([]client.Position{
				{
					Symbol:          "AAPL",
					Quantity:        10,
					MarketValueEUR:  1610.00,
					Country:         "US",
					StockName:       "Apple Inc.",
				},
			})
		}
	}))
	defer server.Close()

	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient(server.URL)

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	result, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"query_type":"overview"}`)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Portfolio Overview")
	assert.Contains(t, result, "50000.00")
	assert.Contains(t, result, "AAPL")
}

func TestToolExecutor_ExecuteToolCall_UnknownTool(t *testing.T) {
	settings := createTestToolSettings(false, false, false)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	_, err := executor.ExecuteToolCall(ctx, "unknown_tool", "{}")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestToolExecutor_ExecuteToolCall_InvalidArguments(t *testing.T) {
	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	_, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", "invalid json")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid arguments")
}

func TestToolExecutor_ExecuteToolCall_MissingQueryType(t *testing.T) {
	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	_, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"focus_area":"US"}`)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query_type is required")
}

func TestToolExecutor_ExecuteToolCall_UnknownQueryType(t *testing.T) {
	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient("http://localhost:8081")

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	_, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"query_type":"invalid"}`)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown query_type")
}

func TestToolExecutor_ExecuteToolCall_PortfolioOpportunities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/opportunities/all" {
			json.NewEncoder(w).Encode(client.OpportunitiesResponse{
				Data: struct {
					Opportunities []client.Opportunity          `json:"opportunities"`
					Count         int                    `json:"count"`
					ByCategory    map[string]int         `json:"by_category"`
				}{
					Opportunities: []client.Opportunity{
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
						},
					},
					Count: 1,
					ByCategory: map[string]int{
						"opportunity_buys": 1,
					},
				},
			})
		} else if r.URL.Path == "/api/planning/recommendations" {
			json.NewEncoder(w).Encode(client.RecommendationsResponse{
				Data: struct {
					Recommendations []interface{} `json:"recommendations"`
				}{
					Recommendations: []interface{}{},
				},
			})
		}
	}))
	defer server.Close()

	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient(server.URL)

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	result, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"query_type":"opportunities"}`)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Trading Opportunities")
	assert.Contains(t, result, "AAPL")
	assert.Contains(t, result, "BUY")
}

func TestToolExecutor_ExecuteToolCall_PortfolioRisk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/snapshots/risk-snapshot" {
			json.NewEncoder(w).Encode(struct {
				Data *client.RiskMetrics `json:"data"`
			}{
				Data: &client.RiskMetrics{
					VaR:                 2500.0,
					CVaR:                3200.0,
					PortfolioVolatility: 0.185,
					SharpeRatio:         1.35,
					MaxDrawdown:         -0.123,
				},
			})
		} else if r.URL.Path == "/api/allocation/deviations" {
			json.NewEncoder(w).Encode(client.AllocationDeviations{
				Allocations: map[string]struct {
					Current   float64 `json:"current"`
					Target    float64 `json:"target"`
					Deviation float64 `json:"deviation"`
				}{
					"US": {
						Current:   0.32,
						Target:    0.30,
						Deviation: 0.02,
					},
				},
				Status: "Minor rebalancing suggested",
			})
		}
	}))
	defer server.Close()

	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient(server.URL)

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	result, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"query_type":"risk"}`)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Risk Metrics")
	assert.Contains(t, result, "2500.00")
	assert.Contains(t, result, "1.35")
	assert.Contains(t, result, "18.50%")
}

func TestToolExecutor_ExecuteToolCall_MarketContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/snapshots/market-context" {
			json.NewEncoder(w).Encode(struct {
				Data *client.MarketContext `json:"data"`
			}{
				Data: &client.MarketContext{
					Regime: struct {
						RawScore       float64 `json:"raw_score"`
						SmoothedScore  float64 `json:"smoothed_score"`
						DiscreteRegime string  `json:"discrete_regime"`
					}{
						RawScore:       0.65,
						SmoothedScore:  0.63,
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
			})
		}
	}))
	defer server.Close()

	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient(server.URL)

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	result, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"query_type":"market_context"}`)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Market Context")
	assert.Contains(t, result, "BULLISH")
	assert.Contains(t, result, "0.65")
	assert.Contains(t, result, "Momentum")
}

func TestToolExecutor_ExecuteToolCall_FullAnalysis(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/portfolio/summary":
			json.NewEncoder(w).Encode(client.PortfolioSummary{
				TotalValue:    50000.00,
				CashBalance:   5000.00,
				PositionCount: 25,
				Allocations: map[string]float64{
					"EU": 0.40,
				},
			})
		case "/api/portfolio/":
			json.NewEncoder(w).Encode([]client.Position{})
		case "/api/opportunities/all":
			json.NewEncoder(w).Encode(client.OpportunitiesResponse{
				Data: struct {
					Opportunities []client.Opportunity          `json:"opportunities"`
					Count         int                    `json:"count"`
					ByCategory    map[string]int         `json:"by_category"`
				}{
					Opportunities: []client.Opportunity{},
					Count:         0,
					ByCategory:    map[string]int{},
				},
			})
		case "/api/planning/recommendations":
			json.NewEncoder(w).Encode(client.RecommendationsResponse{
				Data: struct {
					Recommendations []interface{} `json:"recommendations"`
				}{
					Recommendations: []interface{}{},
				},
			})
		case "/api/snapshots/risk-snapshot":
			json.NewEncoder(w).Encode(struct {
				Data *client.RiskMetrics `json:"data"`
			}{
				Data: &client.RiskMetrics{
					VaR:                 2500.0,
					PortfolioVolatility: 0.185,
					SharpeRatio:         1.35,
				},
			})
		case "/api/allocation/deviations":
			json.NewEncoder(w).Encode(client.AllocationDeviations{
				Allocations: map[string]struct {
					Current   float64 `json:"current"`
					Target    float64 `json:"target"`
					Deviation float64 `json:"deviation"`
				}{},
				Status: "Ok",
			})
		case "/api/snapshots/market-context":
			json.NewEncoder(w).Encode(struct {
				Data *client.MarketContext `json:"data"`
			}{
				Data: &client.MarketContext{
					Regime: struct {
						RawScore       float64 `json:"raw_score"`
						SmoothedScore  float64 `json:"smoothed_score"`
						DiscreteRegime string  `json:"discrete_regime"`
					}{
						DiscreteRegime: "bullish",
					},
					AdaptiveWeights: map[string]float64{},
					MarketHours: struct {
						Status        string   `json:"status"`
						OpenMarkets   []string `json:"open_markets"`
						ClosedMarkets []string `json:"closed_markets"`
					}{
						Status: "open",
					},
				},
			})
		}
	}))
	defer server.Close()

	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient(server.URL)

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	result, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"query_type":"full_analysis"}`)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Complete Portfolio Analysis")
	assert.Contains(t, result, "Portfolio State")
	assert.Contains(t, result, "Risk Assessment")
	assert.Contains(t, result, "Market Context")
}

func TestToolExecutor_ExecuteToolCall_FullAnalysisWithFocusArea(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/portfolio/summary":
			json.NewEncoder(w).Encode(client.PortfolioSummary{
				TotalValue:    50000.00,
				CashBalance:   5000.00,
				PositionCount: 25,
				Allocations: map[string]float64{
					"EU": 0.40,
				},
			})
		case "/api/portfolio/":
			json.NewEncoder(w).Encode([]client.Position{})
		case "/api/opportunities/all":
			json.NewEncoder(w).Encode(client.OpportunitiesResponse{
				Data: struct {
					Opportunities []client.Opportunity          `json:"opportunities"`
					Count         int                    `json:"count"`
					ByCategory    map[string]int         `json:"by_category"`
				}{
					Opportunities: []client.Opportunity{},
					Count:         0,
					ByCategory:    map[string]int{},
				},
			})
		case "/api/planning/recommendations":
			json.NewEncoder(w).Encode(client.RecommendationsResponse{
				Data: struct {
					Recommendations []interface{} `json:"recommendations"`
				}{
					Recommendations: []interface{}{},
				},
			})
		case "/api/snapshots/risk-snapshot":
			json.NewEncoder(w).Encode(struct {
				Data *client.RiskMetrics `json:"data"`
			}{
				Data: &client.RiskMetrics{
					VaR:                 2500.0,
					PortfolioVolatility: 0.185,
					SharpeRatio:         1.35,
				},
			})
		case "/api/allocation/deviations":
			json.NewEncoder(w).Encode(client.AllocationDeviations{
				Allocations: map[string]struct {
					Current   float64 `json:"current"`
					Target    float64 `json:"target"`
					Deviation float64 `json:"deviation"`
				}{},
				Status: "Ok",
			})
		case "/api/snapshots/market-context":
			json.NewEncoder(w).Encode(struct {
				Data *client.MarketContext `json:"data"`
			}{
				Data: &client.MarketContext{
					Regime: struct {
						RawScore       float64 `json:"raw_score"`
						SmoothedScore  float64 `json:"smoothed_score"`
						DiscreteRegime string  `json:"discrete_regime"`
					}{
						DiscreteRegime: "bullish",
					},
					AdaptiveWeights: map[string]float64{},
					MarketHours: struct {
						Status        string   `json:"status"`
						OpenMarkets   []string `json:"open_markets"`
						ClosedMarkets []string `json:"closed_markets"`
					}{
						Status: "open",
					},
				},
			})
		}
	}))
	defer server.Close()

	settings := createTestToolSettings(false, false, true)
	defer cleanupTestSettings(settings)

	searchClient := client.NewSearchClient("http://localhost:8000")
	newsClient := client.NewNewsClient("")
	sentinelClient := client.NewSentinelClient(server.URL)

	executor := NewToolExecutor(searchClient, newsClient, sentinelClient, settings)
	ctx := context.Background()

	result, err := executor.ExecuteToolCall(ctx, "analyze_portfolio", `{"query_type":"full_analysis","focus_area":"US allocation"}`)

	assert.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Complete Portfolio Analysis")
}
