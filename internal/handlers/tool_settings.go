package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// ToolSettings manages which tools are enabled/disabled
type ToolSettings struct {
	EnableWebSearch bool            `json:"enable_web_search"`
	EnableFeeds     bool            `json:"enable_feeds"`
	EnableSentinel  bool            `json:"enable_sentinel"`
	configPath      string
	mu              sync.RWMutex
}

// NewToolSettings creates a new tool settings manager
func NewToolSettings(configPath string) *ToolSettings {
	settings := &ToolSettings{
		EnableWebSearch: false, // Disabled by default - user must explicitly enable
		EnableFeeds:     false, // Disabled by default - user must explicitly enable
		configPath:      configPath,
	}

	// Load existing settings from file if it exists
	settings.Load()

	return settings
}

// Load reads tool settings from file
func (ts *ToolSettings) Load() error {
	if ts.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(ts.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, use defaults
		}
		return fmt.Errorf("failed to read tool settings: %w", err)
	}

	var settings ToolSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to parse tool settings: %w", err)
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.EnableWebSearch = settings.EnableWebSearch
	ts.EnableFeeds = settings.EnableFeeds
	ts.EnableSentinel = settings.EnableSentinel

	return nil
}

// Save persists tool settings to file
func (ts *ToolSettings) Save() error {
	if ts.configPath == "" {
		return fmt.Errorf("no config path set for saving settings")
	}

	// Ensure directory exists
	dir := ""
	for i := len(ts.configPath) - 1; i >= 0; i-- {
		if ts.configPath[i] == '/' {
			dir = ts.configPath[:i]
			break
		}
	}

	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	ts.mu.RLock()
	settings := ToolSettings{
		EnableWebSearch: ts.EnableWebSearch,
		EnableFeeds:     ts.EnableFeeds,
		EnableSentinel:  ts.EnableSentinel,
	}
	ts.mu.RUnlock()

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.WriteFile(ts.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// Get returns a copy of current settings
func (ts *ToolSettings) Get() ToolSettings {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ToolSettings{
		EnableWebSearch: ts.EnableWebSearch,
		EnableFeeds:     ts.EnableFeeds,
		EnableSentinel:  ts.EnableSentinel,
	}
}

// Set updates tool settings
func (ts *ToolSettings) Set(webSearch, feeds, sentinel bool) error {
	ts.mu.Lock()
	ts.EnableWebSearch = webSearch
	ts.EnableFeeds = feeds
	ts.EnableSentinel = sentinel
	ts.mu.Unlock()

	return ts.Save()
}
