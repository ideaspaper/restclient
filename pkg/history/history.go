package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ideaspaper/restclient/pkg/models"
)

const (
	maxHistoryItems = 50
	historyFileName = "request_history.json"
)

// HistoryManager manages request history
type HistoryManager struct {
	historyPath string
	items       []models.HistoricalHttpRequest
}

// NewHistoryManager creates a new history manager
func NewHistoryManager(dataDir string) (*HistoryManager, error) {
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".restclient")
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	hm := &HistoryManager{
		historyPath: filepath.Join(dataDir, historyFileName),
		items:       make([]models.HistoricalHttpRequest, 0),
	}

	// Load existing history
	if err := hm.load(); err != nil {
		// If file doesn't exist, that's fine
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load history: %w", err)
		}
	}

	return hm, nil
}

// Add adds a request to history
func (hm *HistoryManager) Add(request *models.HttpRequest) error {
	item := models.HistoricalHttpRequest{
		Method:    request.Method,
		URL:       request.URL,
		Headers:   request.Headers,
		Body:      request.RawBody,
		StartTime: time.Now().UnixMilli(),
	}

	// Add to beginning
	hm.items = append([]models.HistoricalHttpRequest{item}, hm.items...)

	// Trim to max items
	if len(hm.items) > maxHistoryItems {
		hm.items = hm.items[:maxHistoryItems]
	}

	return hm.save()
}

// GetAll returns all history items
func (hm *HistoryManager) GetAll() []models.HistoricalHttpRequest {
	return hm.items
}

// GetRecent returns the most recent n items
func (hm *HistoryManager) GetRecent(n int) []models.HistoricalHttpRequest {
	if n <= 0 || n > len(hm.items) {
		return hm.items
	}
	return hm.items[:n]
}

// GetByIndex returns a history item by index (0 = most recent)
func (hm *HistoryManager) GetByIndex(index int) (*models.HistoricalHttpRequest, error) {
	if index < 0 || index >= len(hm.items) {
		return nil, fmt.Errorf("index out of range: %d (0-%d)", index, len(hm.items)-1)
	}
	return &hm.items[index], nil
}

// Search searches history by URL or method
func (hm *HistoryManager) Search(query string) []models.HistoricalHttpRequest {
	var results []models.HistoricalHttpRequest
	for _, item := range hm.items {
		if containsIgnoreCase(item.URL, query) || containsIgnoreCase(item.Method, query) {
			results = append(results, item)
		}
	}
	return results
}

// Clear clears all history
func (hm *HistoryManager) Clear() error {
	hm.items = make([]models.HistoricalHttpRequest, 0)
	return hm.save()
}

// Remove removes a history item by index
func (hm *HistoryManager) Remove(index int) error {
	if index < 0 || index >= len(hm.items) {
		return fmt.Errorf("index out of range: %d", index)
	}

	hm.items = slices.Delete(hm.items, index, index+1)
	return hm.save()
}

// load loads history from file
func (hm *HistoryManager) load() error {
	data, err := os.ReadFile(hm.historyPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &hm.items)
}

// save saves history to file
func (hm *HistoryManager) save() error {
	data, err := json.MarshalIndent(hm.items, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history: %w", err)
	}

	return os.WriteFile(hm.historyPath, data, 0644)
}

// FormatHistoryItem formats a history item for display
func FormatHistoryItem(item models.HistoricalHttpRequest, index int) string {
	t := time.UnixMilli(item.StartTime)
	return fmt.Sprintf("[%d] %s %s - %s",
		index,
		item.Method,
		truncateURL(item.URL, 60),
		t.Format("2006-01-02 15:04:05"))
}

// truncateURL truncates a URL to max length
func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// extractDomain extracts domain from URL
func extractDomain(urlStr string) string {
	// Simple domain extraction
	start := 0
	if idx := strings.Index(urlStr, "://"); idx >= 0 {
		start = idx + 3
	}

	end := len(urlStr)
	for i := start; i < len(urlStr); i++ {
		if urlStr[i] == '/' || urlStr[i] == ':' || urlStr[i] == '?' {
			end = i
			break
		}
	}

	return urlStr[start:end]
}

// HistoryStats returns statistics about the history
type HistoryStats struct {
	TotalRequests int
	MethodCounts  map[string]int
	DomainCounts  map[string]int
	OldestRequest time.Time
	NewestRequest time.Time
}

// GetStats returns statistics about the history
func (hm *HistoryManager) GetStats() HistoryStats {
	stats := HistoryStats{
		TotalRequests: len(hm.items),
		MethodCounts:  make(map[string]int),
		DomainCounts:  make(map[string]int),
	}

	for _, item := range hm.items {
		stats.MethodCounts[item.Method]++

		// Extract domain from URL
		domain := extractDomain(item.URL)
		stats.DomainCounts[domain]++

		t := time.UnixMilli(item.StartTime)
		if stats.OldestRequest.IsZero() || t.Before(stats.OldestRequest) {
			stats.OldestRequest = t
		}
		if stats.NewestRequest.IsZero() || t.After(stats.NewestRequest) {
			stats.NewestRequest = t
		}
	}

	return stats
}

// SortByTime sorts history items by time (newest first)
func SortByTime(items []models.HistoricalHttpRequest) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].StartTime > items[j].StartTime
	})
}

// SortByURL sorts history items by URL alphabetically
func SortByURL(items []models.HistoricalHttpRequest) {
	sort.Slice(items, func(i, j int) bool {
		return items[i].URL < items[j].URL
	})
}
