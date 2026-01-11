package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aristath/gollama-ui/internal/client"
)

// ToolExecutor executes tool calls from llama.cpp
type ToolExecutor struct {
	searchClient   *client.SearchClient
	newsClient     *client.NewsClient
	sentinelClient *client.SentinelClient
	toolSettings   *ToolSettings
}

// NewToolExecutor creates a new tool executor
func NewToolExecutor(searchClient *client.SearchClient, newsClient *client.NewsClient, sentinelClient *client.SentinelClient, toolSettings *ToolSettings) *ToolExecutor {
	return &ToolExecutor{
		searchClient:   searchClient,
		newsClient:     newsClient,
		sentinelClient: sentinelClient,
		toolSettings:   toolSettings,
	}
}

// ExecuteToolCall executes a single tool call and returns formatted result
func (e *ToolExecutor) ExecuteToolCall(ctx context.Context, name string, arguments string) (string, error) {
	// Parse arguments JSON
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	switch name {
	case "web_search":
		return e.executeWebSearch(ctx, args)
	case "get_news":
		return e.executeGetNews(ctx, args)
	case "analyze_portfolio":
		return e.executeAnalyzePortfolio(ctx, arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// executeWebSearch performs a web search using ddgs
func (e *ToolExecutor) executeWebSearch(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("query parameter is required")
	}

	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	results, err := e.searchClient.Search(ctx, query, maxResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	// Format results for LLM
	var formatted strings.Builder
	formatted.WriteString(fmt.Sprintf("Search results for '%s':\n\n", query))
	for i, result := range results {
		formatted.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, result.Title))
		formatted.WriteString(fmt.Sprintf("   URL: %s\n", result.Href))
		formatted.WriteString(fmt.Sprintf("   %s\n\n", result.Body))
	}

	return formatted.String(), nil
}

// executeGetNews fetches news articles for a topic
func (e *ToolExecutor) executeGetNews(ctx context.Context, args map[string]interface{}) (string, error) {
	topic, ok := args["topic"].(string)
	if !ok || topic == "" {
		topic = "world" // Default
	}

	maxArticles := 10
	if ma, ok := args["max_articles"].(float64); ok {
		maxArticles = int(ma)
	}

	articles, err := e.newsClient.FetchNews(ctx, topic, maxArticles)
	if err != nil {
		return "", fmt.Errorf("failed to fetch news: %w", err)
	}

	// Format results for LLM
	var formatted strings.Builder
	formatted.WriteString(fmt.Sprintf("Latest %s news:\n\n", topic))
	for i, article := range articles {
		formatted.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, article.Title))
		formatted.WriteString(fmt.Sprintf("   Source: %s\n", article.Source))
		formatted.WriteString(fmt.Sprintf("   Published: %s\n", article.Published.Format("Jan 2, 2006 3:04 PM")))
		if article.Description != "" {
			formatted.WriteString(fmt.Sprintf("   %s\n", article.Description))
		}
		formatted.WriteString(fmt.Sprintf("   Read more: %s\n\n", article.Link))
	}

	return formatted.String(), nil
}

// executeAnalyzePortfolio handles portfolio analysis requests
func (e *ToolExecutor) executeAnalyzePortfolio(ctx context.Context, arguments string) (string, error) {
	var args struct {
		QueryType string `json:"query_type"`
		FocusArea string `json:"focus_area"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.QueryType == "" {
		return "", fmt.Errorf("query_type is required")
	}

	switch args.QueryType {
	case "overview":
		return e.executePortfolioOverview(ctx)
	case "opportunities":
		return e.executeOpportunitiesAnalysis(ctx)
	case "risk":
		return e.executeRiskAnalysis(ctx)
	case "market_context":
		return e.executeMarketContextAnalysis(ctx)
	case "full_analysis":
		return e.executeFullAnalysis(ctx, args.FocusArea)
	default:
		return "", fmt.Errorf("unknown query_type: %s", args.QueryType)
	}
}

// executePortfolioOverview returns portfolio overview
func (e *ToolExecutor) executePortfolioOverview(ctx context.Context) (string, error) {
	summary, err := e.sentinelClient.GetPortfolioSummary(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get portfolio summary: %w", err)
	}

	positions, err := e.sentinelClient.GetPositions(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get positions: %w", err)
	}

	var result strings.Builder
	result.WriteString("# 📊 Portfolio Overview\n\n")
	result.WriteString(fmt.Sprintf("**Total Value:** €%.2f\n", summary.TotalValue))
	result.WriteString(fmt.Sprintf("**Cash Balance:** €%.2f\n", summary.CashBalance))
	result.WriteString(fmt.Sprintf("**Number of Positions:** %d\n\n", summary.PositionCount))

	result.WriteString("## Allocation\n")
	for region, pct := range summary.Allocations {
		result.WriteString(fmt.Sprintf("- %s: %.1f%%\n", region, pct*100))
	}

	if len(positions) > 0 {
		result.WriteString("\n## Top Holdings\n")
		// Sort positions by market value (descending) and show top 5
		topCount := 5
		if len(positions) < topCount {
			topCount = len(positions)
		}

		for i := 0; i < topCount && i < len(positions); i++ {
			pos := positions[i]
			pctOfPortfolio := (pos.MarketValueEUR / summary.TotalValue) * 100
			result.WriteString(fmt.Sprintf("%d. **%s** (%s): €%.2f (%.1f%%)\n",
				i+1, pos.Symbol, pos.Country, pos.MarketValueEUR, pctOfPortfolio))
		}
	}

	return result.String(), nil
}

// executeOpportunitiesAnalysis returns trading opportunities
func (e *ToolExecutor) executeOpportunitiesAnalysis(ctx context.Context) (string, error) {
	opps, err := e.sentinelClient.GetAllOpportunities(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get opportunities: %w", err)
	}

	recs, err := e.sentinelClient.GetRecommendations(ctx)
	if err != nil {
		// Non-fatal, continue without recommendations
		recs = nil
	}

	var result strings.Builder
	result.WriteString("# 🎯 Trading Opportunities\n\n")
	result.WriteString(fmt.Sprintf("**Total Opportunities:** %d\n\n", opps.Data.Count))

	if len(opps.Data.ByCategory) > 0 {
		result.WriteString("## By Category\n")
		for category, count := range opps.Data.ByCategory {
			result.WriteString(fmt.Sprintf("- %s: %d\n", strings.Title(strings.ReplaceAll(category, "_", " ")), count))
		}
		result.WriteString("\n")
	}

	if len(opps.Data.Opportunities) > 0 {
		result.WriteString("## Top Priority Opportunities\n")
		topCount := 5
		if len(opps.Data.Opportunities) < topCount {
			topCount = len(opps.Data.Opportunities)
		}

		for i := 0; i < topCount; i++ {
			opp := opps.Data.Opportunities[i]
			result.WriteString(fmt.Sprintf("%d. **%s %s**: %v @ €%.2f (Priority: %.1f)\n",
				i+1, opp.Side, opp.Symbol, opp.Quantity, opp.Price, opp.Priority))
			result.WriteString(fmt.Sprintf("   Reason: %s\n", opp.Reason))
		}
	}

	if recs != nil && len(recs.Data.Recommendations) > 0 {
		result.WriteString("\n## Planner Recommendations\n")
		result.WriteString(fmt.Sprintf("- %d recommendation(s) available\n", len(recs.Data.Recommendations)))
	}

	return result.String(), nil
}

// executeRiskAnalysis returns portfolio risk metrics
func (e *ToolExecutor) executeRiskAnalysis(ctx context.Context) (string, error) {
	risk, err := e.sentinelClient.GetPortfolioRisk(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get risk metrics: %w", err)
	}

	deviations, err := e.sentinelClient.GetAllocationDeviations(ctx)
	if err != nil {
		// Non-fatal, continue without deviations
		deviations = nil
	}

	var result strings.Builder
	result.WriteString("# ⚠️ Risk Metrics\n\n")

	if risk.VaR > 0 {
		result.WriteString(fmt.Sprintf("**Value at Risk (95%%):** €%.2f\n", risk.VaR))
	}
	if risk.CVaR > 0 {
		result.WriteString(fmt.Sprintf("**Conditional VaR:** €%.2f\n", risk.CVaR))
	}
	if risk.PortfolioVolatility > 0 {
		result.WriteString(fmt.Sprintf("**Portfolio Volatility:** %.2f%% annualized\n", risk.PortfolioVolatility*100))
	}
	if risk.SharpeRatio != 0 {
		result.WriteString(fmt.Sprintf("**Sharpe Ratio:** %.2f\n", risk.SharpeRatio))
	}
	if risk.MaxDrawdown < 0 {
		result.WriteString(fmt.Sprintf("**Max Drawdown:** %.2f%%\n", risk.MaxDrawdown*100))
	}

	if deviations != nil && len(deviations.Allocations) > 0 {
		result.WriteString("\n## Allocation vs Targets\n")
		for region, dev := range deviations.Allocations {
			status := "✓"
			if dev.Deviation > 0.02 {
				status = "⚠️"
			}
			result.WriteString(fmt.Sprintf("%s %s: %.1f%% (target: %.1f%%, deviation: %+.1f%%)\n",
				status, region, dev.Current*100, dev.Target*100, dev.Deviation*100))
		}
		result.WriteString(fmt.Sprintf("\n**Status:** %s\n", deviations.Status))
	}

	return result.String(), nil
}

// executeMarketContextAnalysis returns market regime and context
func (e *ToolExecutor) executeMarketContextAnalysis(ctx context.Context) (string, error) {
	context, err := e.sentinelClient.GetMarketContext(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get market context: %w", err)
	}

	var result strings.Builder
	result.WriteString("# 📈 Market Context\n\n")

	result.WriteString(fmt.Sprintf("**Market Regime:** %s\n", strings.ToUpper(context.Regime.DiscreteRegime)))
	result.WriteString(fmt.Sprintf("**Regime Score:** %.2f/1.0\n\n", context.Regime.RawScore))

	if len(context.AdaptiveWeights) > 0 {
		result.WriteString("## Adaptive Strategy Weights\n")
		for strategy, weight := range context.AdaptiveWeights {
			result.WriteString(fmt.Sprintf("- %s: %.1f%%\n", strings.Title(strategy), weight*100))
		}
		result.WriteString("\n")
	}

	if context.MarketHours.Status != "" {
		result.WriteString(fmt.Sprintf("**Market Status:** %s\n", context.MarketHours.Status))
		if len(context.MarketHours.OpenMarkets) > 0 {
			result.WriteString(fmt.Sprintf("**Open Markets:** %s\n", strings.Join(context.MarketHours.OpenMarkets, ", ")))
		}
		if len(context.MarketHours.ClosedMarkets) > 0 {
			result.WriteString(fmt.Sprintf("**Closed Markets:** %s\n", strings.Join(context.MarketHours.ClosedMarkets, ", ")))
		}
	}

	return result.String(), nil
}

// executeFullAnalysis returns complete portfolio analysis
func (e *ToolExecutor) executeFullAnalysis(ctx context.Context, focusArea string) (string, error) {
	var result strings.Builder
	result.WriteString("# 📊 Complete Portfolio Analysis\n\n")

	// Get overview
	overview, err := e.executePortfolioOverview(ctx)
	if err == nil {
		result.WriteString("## Portfolio State\n")
		result.WriteString(overview)
		result.WriteString("\n")
	}

	// Get opportunities
	opps, err := e.executeOpportunitiesAnalysis(ctx)
	if err == nil {
		result.WriteString("## Trading Opportunities\n")
		result.WriteString(opps)
		result.WriteString("\n")
	}

	// Get risk
	risk, err := e.executeRiskAnalysis(ctx)
	if err == nil {
		result.WriteString("## Risk Assessment\n")
		result.WriteString(risk)
		result.WriteString("\n")
	}

	// Get market context
	mktCtx, err := e.executeMarketContextAnalysis(ctx)
	if err == nil {
		result.WriteString("## Market Context\n")
		result.WriteString(mktCtx)
		result.WriteString("\n")
	}

	return result.String(), nil
}

// GetAvailableTools returns tool definitions for llama.cpp (only enabled tools)
func (e *ToolExecutor) GetAvailableTools() []client.Tool {
	settings := e.toolSettings.Get()
	tools := []client.Tool{}

	// Add web_search tool if enabled
	if settings.EnableWebSearch {
		tools = append(tools, client.Tool{
			Type: "function",
			Function: client.Function{
				Name:        "web_search",
				Description: "Search the web for current information. Use this when you need up-to-date information or facts not in your training data.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "The search query to find information about",
						},
						"max_results": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of search results to return (default 5)",
						},
					},
					"required": []string{"query"},
				},
			},
		})
	}

	// Add get_news tool if enabled
	if settings.EnableFeeds {
		topics := e.newsClient.GetAvailableTopics()
		var toolDescription string
		var topicDescription string

		if len(topics) == 0 {
			toolDescription = "Get latest news articles. No feeds are currently configured."
			topicDescription = "News topic (no feeds configured - add feeds in settings)"
		} else {
			topicDescription = fmt.Sprintf("Must be one of: %s. Use the exact topic name as shown.", strings.Join(topics, ", "))
			toolDescription = fmt.Sprintf("Get latest news articles. Available topics: %s. Call this tool once per topic if you need multiple categories.", strings.Join(topics, ", "))
		}

		tools = append(tools, client.Tool{
			Type: "function",
			Function: client.Function{
				Name:        "get_news",
				Description: toolDescription,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"topic": map[string]interface{}{
							"type":        "string",
							"description": topicDescription,
						},
						"max_articles": map[string]interface{}{
							"type":        "integer",
							"description": "Maximum number of articles to return (default 10)",
						},
					},
					"required": []string{"topic"},
				},
			},
		})
	}

	// Add analyze_portfolio tool if enabled
	if settings.EnableSentinel {
		tools = append(tools, client.Tool{
			Type: "function",
			Function: client.Function{
				Name:        "analyze_portfolio",
				Description: "Analyze the Sentinel portfolio management system to get current portfolio state, trading opportunities, risk metrics, and market context. Use this to answer questions about portfolio health, performance, allocation, or to suggest next actions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query_type": map[string]interface{}{
							"type":        "string",
							"description": "Type of analysis to perform: 'overview' for portfolio summary, 'opportunities' for trade suggestions, 'risk' for risk metrics, 'market_context' for market regime, 'full_analysis' for comprehensive snapshot",
							"enum":        []interface{}{"overview", "opportunities", "risk", "market_context", "full_analysis"},
						},
						"focus_area": map[string]interface{}{
							"type":        "string",
							"description": "Optional: specific area to focus on (e.g., 'US allocation', 'technology sector', 'high volatility positions')",
						},
					},
					"required": []interface{}{"query_type"},
				},
			},
		})
	}

	return tools
}
