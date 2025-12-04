package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ideaspaper/restclient/pkg/models"
)

func TestHistoryReplay_PrintsURL(t *testing.T) {
	v.Set("showColors", false)
	defer v.Set("showColors", true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	historyFile := filepath.Join(tempDir, "request_history.json")

	historyItems := []models.HistoricalHttpRequest{
		{
			Method:    "GET",
			URL:       server.URL + "/api/users",
			Headers:   map[string]string{"Accept": "application/json"},
			StartTime: time.Now().UnixMilli(),
		},
	}
	data, _ := json.Marshal(historyItems)
	if err := os.WriteFile(historyFile, data, 0644); err != nil {
		t.Fatalf("failed to write history file: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	restclientDir := filepath.Join(tempDir, ".restclient")
	os.MkdirAll(restclientDir, 0755)
	os.Rename(historyFile, filepath.Join(restclientDir, "request_history.json"))

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"history", "replay", "1"})
	err := rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Fatalf("command failed: %v\nOutput: %s", err, output)
	}

	expectedURL := "GET " + server.URL + "/api/users"
	if !strings.Contains(output, expectedURL) {
		t.Errorf("output should contain URL being replayed\nExpected to contain: %s\nGot: %s", expectedURL, output)
	}
}

func TestHistoryReplay_PrintsURL_POST(t *testing.T) {
	v.Set("showColors", false)
	defer v.Set("showColors", true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 1}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	restclientDir := filepath.Join(tempDir, ".restclient")
	os.MkdirAll(restclientDir, 0755)

	historyItems := []models.HistoricalHttpRequest{
		{
			Method:    "POST",
			URL:       server.URL + "/api/items",
			Headers:   map[string]string{"Content-Type": "application/json"},
			Body:      `{"name": "test"}`,
			StartTime: time.Now().UnixMilli(),
		},
	}
	data, _ := json.Marshal(historyItems)
	if err := os.WriteFile(filepath.Join(restclientDir, "request_history.json"), data, 0644); err != nil {
		t.Fatalf("failed to write history file: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", origHome)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"history", "replay", "1"})
	err := rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Fatalf("command failed: %v\nOutput: %s", err, output)
	}

	expectedURL := "POST " + server.URL + "/api/items"
	if !strings.Contains(output, expectedURL) {
		t.Errorf("output should contain POST URL being replayed\nExpected to contain: %s\nGot: %s", expectedURL, output)
	}
}
