package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ideaspaper/restclient/pkg/models"
)

func TestNewHistoryManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	if hm == nil {
		t.Fatal("HistoryManager should not be nil")
	}

	// Check that history file path is set
	expectedPath := filepath.Join(tmpDir, historyFileName)
	if hm.historyPath != expectedPath {
		t.Errorf("historyPath should be %s, got %s", expectedPath, hm.historyPath)
	}
}

func TestNewHistoryManager_DefaultDir(t *testing.T) {
	// Test with empty directory (should use default)
	hm, err := NewHistoryManager("")
	if err != nil {
		t.Fatalf("NewHistoryManager with empty dir failed: %v", err)
	}

	if hm == nil {
		t.Fatal("HistoryManager should not be nil")
	}
}

func TestHistoryManager_Add(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	request := &models.HttpRequest{
		Method:  "GET",
		URL:     "https://api.example.com/users",
		Headers: map[string]string{"Authorization": "Bearer token"},
		RawBody: "",
	}

	if err := hm.Add(request); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	items := hm.GetAll()
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}

	if items[0].Method != "GET" {
		t.Errorf("Method should be GET, got %s", items[0].Method)
	}
	if items[0].URL != "https://api.example.com/users" {
		t.Errorf("URL mismatch")
	}
}

func TestHistoryManager_Add_MaxItems(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	// Add more than maxHistoryItems
	for i := 0; i < maxHistoryItems+10; i++ {
		request := &models.HttpRequest{
			Method: "GET",
			URL:    "https://api.example.com/item/" + string(rune('A'+i%26)),
		}
		if err := hm.Add(request); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	items := hm.GetAll()
	if len(items) != maxHistoryItems {
		t.Errorf("Expected %d items (max), got %d", maxHistoryItems, len(items))
	}
}

func TestHistoryManager_GetRecent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	// Add 5 items
	for i := 0; i < 5; i++ {
		request := &models.HttpRequest{
			Method: "GET",
			URL:    "https://api.example.com/item/" + string(rune('A'+i)),
		}
		hm.Add(request)
	}

	// Get 3 recent
	recent := hm.GetRecent(3)
	if len(recent) != 3 {
		t.Errorf("Expected 3 items, got %d", len(recent))
	}

	// Get more than available
	all := hm.GetRecent(100)
	if len(all) != 5 {
		t.Errorf("Expected 5 items, got %d", len(all))
	}

	// Get 0 or negative
	none := hm.GetRecent(0)
	if len(none) != 5 {
		t.Errorf("GetRecent(0) should return all items")
	}
}

func TestHistoryManager_GetByIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	// Add items
	hm.Add(&models.HttpRequest{Method: "GET", URL: "https://example.com/1"})
	hm.Add(&models.HttpRequest{Method: "POST", URL: "https://example.com/2"})
	hm.Add(&models.HttpRequest{Method: "PUT", URL: "https://example.com/3"})

	// Get by valid index (0 = most recent)
	item, err := hm.GetByIndex(0)
	if err != nil {
		t.Fatalf("GetByIndex(0) failed: %v", err)
	}
	if item.Method != "PUT" {
		t.Errorf("Most recent should be PUT, got %s", item.Method)
	}

	// Get by invalid index
	_, err = hm.GetByIndex(-1)
	if err == nil {
		t.Error("GetByIndex(-1) should fail")
	}

	_, err = hm.GetByIndex(100)
	if err == nil {
		t.Error("GetByIndex(100) should fail")
	}
}

func TestHistoryManager_Search(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	hm.Add(&models.HttpRequest{Method: "GET", URL: "https://api.example.com/users"})
	hm.Add(&models.HttpRequest{Method: "POST", URL: "https://api.example.com/users"})
	hm.Add(&models.HttpRequest{Method: "GET", URL: "https://api.example.com/orders"})
	hm.Add(&models.HttpRequest{Method: "DELETE", URL: "https://other.com/resource"})

	// Search by URL
	results := hm.Search("users")
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'users', got %d", len(results))
	}

	// Search by method
	results = hm.Search("GET")
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'GET', got %d", len(results))
	}

	// Search case-insensitive
	results = hm.Search("delete")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'delete', got %d", len(results))
	}

	// Search with no results
	results = hm.Search("nonexistent")
	if len(results) != 0 {
		t.Errorf("Expected 0 results for 'nonexistent', got %d", len(results))
	}
}

func TestHistoryManager_Clear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	hm.Add(&models.HttpRequest{Method: "GET", URL: "https://example.com"})
	hm.Add(&models.HttpRequest{Method: "POST", URL: "https://example.com"})

	if len(hm.GetAll()) != 2 {
		t.Error("Should have 2 items before clear")
	}

	if err := hm.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if len(hm.GetAll()) != 0 {
		t.Error("Should have 0 items after clear")
	}
}

func TestHistoryManager_Remove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	hm.Add(&models.HttpRequest{Method: "GET", URL: "https://example.com/1"})
	hm.Add(&models.HttpRequest{Method: "POST", URL: "https://example.com/2"})
	hm.Add(&models.HttpRequest{Method: "PUT", URL: "https://example.com/3"})

	// Remove middle item (index 1)
	if err := hm.Remove(1); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	items := hm.GetAll()
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}

	// Verify correct item was removed
	for _, item := range items {
		if item.URL == "https://example.com/2" {
			t.Error("Item at index 1 should have been removed")
		}
	}

	// Remove with invalid index
	if err := hm.Remove(-1); err == nil {
		t.Error("Remove(-1) should fail")
	}
	if err := hm.Remove(100); err == nil {
		t.Error("Remove(100) should fail")
	}
}

func TestHistoryManager_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create and add items
	hm1, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	hm1.Add(&models.HttpRequest{Method: "GET", URL: "https://example.com/persistent"})

	// Create new manager (simulating restart)
	hm2, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager (2) failed: %v", err)
	}

	items := hm2.GetAll()
	if len(items) != 1 {
		t.Errorf("Expected 1 persisted item, got %d", len(items))
	}
	if items[0].URL != "https://example.com/persistent" {
		t.Error("Persisted URL mismatch")
	}
}

func TestHistoryManager_GetStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "restclient-history-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hm, err := NewHistoryManager(tmpDir)
	if err != nil {
		t.Fatalf("NewHistoryManager failed: %v", err)
	}

	hm.Add(&models.HttpRequest{Method: "GET", URL: "https://api.example.com/users"})
	hm.Add(&models.HttpRequest{Method: "GET", URL: "https://api.example.com/orders"})
	hm.Add(&models.HttpRequest{Method: "POST", URL: "https://api.example.com/users"})
	hm.Add(&models.HttpRequest{Method: "DELETE", URL: "https://other.com/resource"})

	stats := hm.GetStats()

	if stats.TotalRequests != 4 {
		t.Errorf("TotalRequests should be 4, got %d", stats.TotalRequests)
	}

	if stats.MethodCounts["GET"] != 2 {
		t.Errorf("GET count should be 2, got %d", stats.MethodCounts["GET"])
	}
	if stats.MethodCounts["POST"] != 1 {
		t.Errorf("POST count should be 1, got %d", stats.MethodCounts["POST"])
	}
	if stats.MethodCounts["DELETE"] != 1 {
		t.Errorf("DELETE count should be 1, got %d", stats.MethodCounts["DELETE"])
	}

	if stats.DomainCounts["api.example.com"] != 3 {
		t.Errorf("api.example.com count should be 3, got %d", stats.DomainCounts["api.example.com"])
	}
	if stats.DomainCounts["other.com"] != 1 {
		t.Errorf("other.com count should be 1, got %d", stats.DomainCounts["other.com"])
	}
}

func TestFormatHistoryItem(t *testing.T) {
	item := models.HistoricalHttpRequest{
		Method:    "GET",
		URL:       "https://api.example.com/users",
		StartTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli(),
	}

	// Index 5 (0-based) should display as 6 (1-based)
	formatted := FormatHistoryItem(item, 5)

	if formatted == "" {
		t.Error("FormatHistoryItem should return non-empty string")
	}

	// Should contain 1-based index, method, URL
	if !containsIgnoreCase(formatted, "[6]") {
		t.Error("Should contain 1-based index [6]")
	}
	if !containsIgnoreCase(formatted, "GET") {
		t.Error("Should contain method GET")
	}
	if !containsIgnoreCase(formatted, "example.com") {
		t.Error("Should contain part of URL")
	}
}

func TestTruncateURL(t *testing.T) {
	tests := []struct {
		url    string
		maxLen int
	}{
		{"https://short.com", 50},
		{"https://very-long-url-that-exceeds-the-limit.example.com/path/to/resource", 30},
		{"abc", 10},
		{"abcdefghij", 10},
		{"abcdefghijk", 10},
	}

	for _, tt := range tests {
		got := truncateURL(tt.url, tt.maxLen)
		if len(got) > tt.maxLen {
			t.Errorf("truncateURL(%q, %d) length %d exceeds maxLen", tt.url, tt.maxLen, len(got))
		}
		if len(tt.url) > tt.maxLen && !strings.HasSuffix(got, "...") {
			t.Errorf("truncateURL(%q, %d) should end with '...'", tt.url, tt.maxLen)
		}
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/path", "example.com"},
		{"http://api.example.com:8080/resource", "api.example.com"},
		{"https://example.com?query=1", "example.com"},
		{"example.com/path", "example.com"},
	}

	for _, tt := range tests {
		got := extractDomain(tt.url)
		if got != tt.want {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestSortByTime(t *testing.T) {
	items := []models.HistoricalHttpRequest{
		{URL: "oldest", StartTime: 1000},
		{URL: "newest", StartTime: 3000},
		{URL: "middle", StartTime: 2000},
	}

	SortByTime(items)

	if items[0].URL != "newest" {
		t.Errorf("First item should be newest, got %s", items[0].URL)
	}
	if items[1].URL != "middle" {
		t.Errorf("Second item should be middle, got %s", items[1].URL)
	}
	if items[2].URL != "oldest" {
		t.Errorf("Third item should be oldest, got %s", items[2].URL)
	}
}

func TestSortByURL(t *testing.T) {
	items := []models.HistoricalHttpRequest{
		{URL: "https://z.com"},
		{URL: "https://a.com"},
		{URL: "https://m.com"},
	}

	SortByURL(items)

	if items[0].URL != "https://a.com" {
		t.Errorf("First item should be a.com, got %s", items[0].URL)
	}
	if items[1].URL != "https://m.com" {
		t.Errorf("Second item should be m.com, got %s", items[1].URL)
	}
	if items[2].URL != "https://z.com" {
		t.Errorf("Third item should be z.com, got %s", items[2].URL)
	}
}
