// Package history provides request history management with persistence,
// filtering, and display formatting capabilities.
package history

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/internal/paths"
	"github.com/ideaspaper/restclient/internal/stringutil"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/models"
)

const (
	maxHistoryItems = 50
	historyFileName = "request_history.json"
)

// HistoryManager manages request history
type HistoryManager struct {
	fs          filesystem.FileSystem
	historyPath string
	items       []models.HistoricalHttpRequest
}

// NewHistoryManager creates a new history manager
func NewHistoryManager(dataDir string) (*HistoryManager, error) {
	return NewHistoryManagerWithFS(filesystem.Default, dataDir)
}

// NewHistoryManagerWithFS creates a new history manager with a custom file system.
// This is primarily useful for testing.
func NewHistoryManagerWithFS(fs filesystem.FileSystem, dataDir string) (*HistoryManager, error) {
	if fs == nil {
		fs = filesystem.Default
	}

	if dataDir == "" {
		appDir, err := paths.AppDataDir("")
		if err != nil {
			return nil, errors.Wrap(err, "failed to get app data directory")
		}
		dataDir = appDir
	}

	if err := fs.MkdirAll(dataDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create data directory")
	}

	h := &HistoryManager{
		fs:          fs,
		historyPath: filepath.Join(dataDir, historyFileName),
		items:       make([]models.HistoricalHttpRequest, 0),
	}

	if err := h.load(); err != nil {
		if !isNotExist(err) {
			return nil, errors.Wrap(err, "failed to load history")
		}
	}

	return h, nil
}

// isNotExist checks if the error indicates a file does not exist.
// Works with both os.PathError and fs.PathError.
func isNotExist(err error) bool {
	return os.IsNotExist(err) || errors.Is(err, fs.ErrNotExist)
}

// Add adds a request to history
func (h *HistoryManager) Add(request *models.HttpRequest) error {
	item := models.HistoricalHttpRequest{
		Method:    request.Method,
		URL:       request.URL,
		Headers:   request.Headers,
		Body:      request.RawBody,
		StartTime: time.Now().UnixMilli(),
	}

	h.items = append([]models.HistoricalHttpRequest{item}, h.items...)

	if len(h.items) > maxHistoryItems {
		h.items = h.items[:maxHistoryItems]
	}

	return h.save()
}

// GetAll returns all history items
func (h *HistoryManager) GetAll() []models.HistoricalHttpRequest {
	return h.items
}

// GetRecent returns the most recent n items
func (h *HistoryManager) GetRecent(n int) []models.HistoricalHttpRequest {
	if n <= 0 || n > len(h.items) {
		return h.items
	}
	return h.items[:n]
}

// GetByIndex returns a history item by index (0-based internally, but error message shows 1-based for user)
func (h *HistoryManager) GetByIndex(index int) (*models.HistoricalHttpRequest, error) {
	if index < 0 || index >= len(h.items) {
		return nil, errors.NewValidationError("index", fmt.Sprintf("out of range: valid range is 1-%d", len(h.items)))
	}
	return &h.items[index], nil
}

// Clear clears all history
func (h *HistoryManager) Clear() error {
	h.items = make([]models.HistoricalHttpRequest, 0)
	return h.save()
}

// Remove removes a history item by index
func (h *HistoryManager) Remove(index int) error {
	if index < 0 || index >= len(h.items) {
		return errors.NewValidationError("index", fmt.Sprintf("out of range: %d", index))
	}

	h.items = slices.Delete(h.items, index, index+1)
	return h.save()
}

// load loads history from file
func (h *HistoryManager) load() error {
	data, err := h.fs.ReadFile(h.historyPath)
	if err != nil {
		if isNotExist(err) {
			return err
		}
		return errors.Wrap(err, "failed to read history file")
	}

	if err := json.Unmarshal(data, &h.items); err != nil {
		return errors.Wrap(err, "failed to parse history file")
	}
	return nil
}

// save saves history to file
func (h *HistoryManager) save() error {
	data, err := json.MarshalIndent(h.items, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal history")
	}

	return h.fs.WriteFile(h.historyPath, data, 0644)
}

// FormatHistoryItem formats a history item for display (1-based index for users)
func FormatHistoryItem(item models.HistoricalHttpRequest, index int) string {
	t := time.UnixMilli(item.StartTime)
	return fmt.Sprintf("[%d] %s %s - %s",
		index+1,
		item.Method,
		truncateURL(item.URL, 60),
		t.Format("2006-01-02 15:04:05"))
}

// truncateURL truncates a URL to max length
func truncateURL(url string, maxLen int) string {
	return stringutil.Truncate(url, maxLen)
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
func (h *HistoryManager) GetStats() HistoryStats {
	stats := HistoryStats{
		TotalRequests: len(h.items),
		MethodCounts:  make(map[string]int),
		DomainCounts:  make(map[string]int),
	}

	for _, item := range h.items {
		stats.MethodCounts[item.Method]++

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

// Formatter handles history display formatting
type Formatter struct {
	// FormatIndex formats the index number (e.g., "[1]")
	FormatIndex func(index int) string
	// FormatMethod formats the HTTP method
	FormatMethod func(method string) string
	// FormatTime formats the timestamp in dim/muted style
	FormatTime func(timeStr string) string
}

// DefaultFormatter returns a formatter with no colors
func DefaultFormatter() *Formatter {
	return &Formatter{
		FormatIndex:  func(i int) string { return fmt.Sprintf("[%d]", i) },
		FormatMethod: func(m string) string { return m },
		FormatTime:   func(t string) string { return t },
	}
}

// FormatItem formats a history item for display
func (f *Formatter) FormatItem(item models.HistoricalHttpRequest, index int) string {
	t := time.UnixMilli(item.StartTime)
	timeStr := t.Format("2006-01-02 15:04:05")

	// Display 1-based index for user-facing output
	displayIndex := index + 1

	return fmt.Sprintf("%s %s %s  %s",
		f.FormatIndex(displayIndex),
		f.FormatMethod(item.Method),
		truncateURL(item.URL, 60),
		f.FormatTime(timeStr))
}

// FormatDetails formats full details of a history item
func (f *Formatter) FormatDetails(item models.HistoricalHttpRequest) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s %s\n", f.FormatMethod(item.Method), item.URL))
	sb.WriteString(fmt.Sprintf("Time: %s\n", time.UnixMilli(item.StartTime).Format("2006-01-02 15:04:05")))

	if len(item.Headers) > 0 {
		sb.WriteString("\nHeaders:\n")
		for k, v := range item.Headers {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	if item.Body != "" {
		sb.WriteString("\nBody:\n")
		sb.WriteString(item.Body)
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatStats formats history statistics
func (f *Formatter) FormatStats(stats HistoryStats) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Total Requests: %d\n\n", stats.TotalRequests))

	if len(stats.MethodCounts) > 0 {
		sb.WriteString("By Method:\n")
		for method, count := range stats.MethodCounts {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", method, count))
		}
		sb.WriteString("\n")
	}

	if len(stats.DomainCounts) > 0 {
		sb.WriteString("Top Domains:\n")
		count := 0
		for domain, c := range stats.DomainCounts {
			if count >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %s: %d\n", domain, c))
			count++
		}
		sb.WriteString("\n")
	}

	if !stats.OldestRequest.IsZero() {
		sb.WriteString(fmt.Sprintf("Oldest Request: %s\n", stats.OldestRequest.Format("2006-01-02 15:04:05")))
		sb.WriteString(fmt.Sprintf("Newest Request: %s\n", stats.NewestRequest.Format("2006-01-02 15:04:05")))
	}

	return sb.String()
}
