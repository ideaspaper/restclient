package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/variables"
)

func TestExecutor_Execute(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"users": []map[string]any{
					{"id": 1, "name": "John"},
					{"id": 2, "name": "Jane"},
				},
			})
		case "/echo":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Echo back request headers
			headers := make(map[string]string)
			for k, v := range r.Header {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}
			json.NewEncoder(w).Encode(map[string]any{
				"method":  r.Method,
				"path":    r.URL.Path,
				"headers": headers,
			})
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tests := []struct {
		name           string
		request        *models.HttpRequest
		wantStatusCode int
		wantErr        bool
	}{
		{
			name: "simple GET request",
			request: &models.HttpRequest{
				Method:  "GET",
				URL:     server.URL + "/users",
				Headers: map[string]string{},
			},
			wantStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "GET with custom headers",
			request: &models.HttpRequest{
				Method: "GET",
				URL:    server.URL + "/echo",
				Headers: map[string]string{
					"X-Custom-Header": "test-value",
					"Accept":          "application/json",
				},
			},
			wantStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "POST request with body",
			request: &models.HttpRequest{
				Method:  "POST",
				URL:     server.URL + "/echo",
				Headers: map[string]string{"Content-Type": "application/json"},
				RawBody: `{"name": "test"}`,
			},
			wantStatusCode: http.StatusOK,
			wantErr:        false,
		},
		{
			name: "server error response",
			request: &models.HttpRequest{
				Method:  "GET",
				URL:     server.URL + "/error",
				Headers: map[string]string{},
			},
			wantStatusCode: http.StatusInternalServerError,
			wantErr:        false, // Error response is not an error
		},
		{
			name: "not found response",
			request: &models.HttpRequest{
				Method:  "GET",
				URL:     server.URL + "/nonexistent",
				Headers: map[string]string{},
			},
			wantStatusCode: http.StatusNotFound,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				RememberCookies: false,
			}
			varProcessor := variables.NewVariableProcessor()

			exec := New(cfg, varProcessor, Options{
				NoSession: true,
				NoHistory: true,
			})

			result, err := exec.Execute(tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result.Response.StatusCode != tt.wantStatusCode {
					t.Errorf("Execute() StatusCode = %v, want %v", result.Response.StatusCode, tt.wantStatusCode)
				}
			}
		})
	}
}

func TestExecutor_ExecuteWithContext(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			time.Sleep(500 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Run("context cancellation", func(t *testing.T) {
		cfg := &config.Config{
			RememberCookies: false,
		}
		varProcessor := variables.NewVariableProcessor()

		exec := New(cfg, varProcessor, Options{
			NoSession: true,
			NoHistory: true,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		request := &models.HttpRequest{
			Method:  "GET",
			URL:     server.URL + "/slow",
			Headers: map[string]string{},
		}

		_, err := exec.ExecuteWithContext(ctx, request)
		if err == nil {
			t.Error("Expected error due to context cancellation")
		}
	})

	t.Run("successful request with context", func(t *testing.T) {
		cfg := &config.Config{
			RememberCookies: false,
		}
		varProcessor := variables.NewVariableProcessor()

		exec := New(cfg, varProcessor, Options{
			NoSession: true,
			NoHistory: true,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		request := &models.HttpRequest{
			Method:  "GET",
			URL:     server.URL + "/fast",
			Headers: map[string]string{},
		}

		result, err := exec.ExecuteWithContext(ctx, request)
		if err != nil {
			t.Errorf("Execute() unexpected error = %v", err)
			return
		}
		if result.Response.StatusCode != http.StatusOK {
			t.Errorf("Execute() StatusCode = %v, want %v", result.Response.StatusCode, http.StatusOK)
		}
	})
}

func TestExecutor_WithPostScript(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"token": "test-token-123",
			"user":  map[string]any{"id": 1, "name": "John"},
		})
	}))
	defer server.Close()

	tests := []struct {
		name           string
		postScript     string
		wantErr        bool
		wantTestFail   bool
		wantLogsCount  int
		wantTestsCount int
	}{
		{
			name: "simple test that passes",
			postScript: `
				client.test("Status is 200", function() {
					client.assert(response.status === 200);
				});
			`,
			wantErr:        false,
			wantTestFail:   false,
			wantTestsCount: 1,
		},
		{
			name: "test with logging",
			postScript: `
				client.log("Response received");
				client.log("Token: " + response.body.token);
			`,
			wantErr:       false,
			wantLogsCount: 2,
		},
		{
			name: "test that fails",
			postScript: `
				client.test("Status should be 201", function() {
					client.assert(response.status === 201, "Expected 201");
				});
			`,
			wantErr:      true, // Test failure is an error
			wantTestFail: true,
		},
		{
			name: "multiple tests",
			postScript: `
				client.test("Status is 200", function() {
					client.assert(response.status === 200);
				});
				client.test("Has token", function() {
					client.assert(response.body.token !== undefined);
				});
			`,
			wantErr:        false,
			wantTestsCount: 2,
		},
		{
			name: "test with global variable",
			postScript: `
				client.global.set("authToken", response.body.token);
				client.test("Token stored", function() {
					client.assert(client.global.get("authToken") === "test-token-123");
				});
			`,
			wantErr:        false,
			wantTestsCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				RememberCookies: false,
			}
			varProcessor := variables.NewVariableProcessor()

			exec := New(cfg, varProcessor, Options{
				NoSession: true,
				NoHistory: true,
			})

			request := &models.HttpRequest{
				Method:  "GET",
				URL:     server.URL + "/api",
				Headers: map[string]string{},
				Metadata: models.RequestMetadata{
					PostScript: tt.postScript,
				},
			}

			result, err := exec.Execute(request)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.wantLogsCount > 0 && len(result.Logs) != tt.wantLogsCount {
					t.Errorf("Execute() logs count = %v, want %v", len(result.Logs), tt.wantLogsCount)
				}
				if tt.wantTestsCount > 0 && len(result.TestResults) != tt.wantTestsCount {
					t.Errorf("Execute() tests count = %v, want %v", len(result.TestResults), tt.wantTestsCount)
				}
			}
		})
	}
}

func TestApplyScriptGlobalVars(t *testing.T) {
	tests := []struct {
		name       string
		globalVars map[string]any
		wantVars   map[string]string
	}{
		{
			name: "string values",
			globalVars: map[string]any{
				"token": "abc123",
				"user":  "john",
			},
			wantVars: map[string]string{
				"token": "abc123",
				"user":  "john",
			},
		},
		{
			name: "mixed types",
			globalVars: map[string]any{
				"token":  "abc123",
				"count":  42,
				"active": true,
			},
			wantVars: map[string]string{
				"token":  "abc123",
				"count":  "42",
				"active": "true",
			},
		},
		{
			name:       "empty map",
			globalVars: map[string]any{},
			wantVars:   map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			varProcessor := variables.NewVariableProcessor()
			ApplyScriptGlobalVars(varProcessor, tt.globalVars)

			for k, want := range tt.wantVars {
				got, err := varProcessor.Process("{{" + k + "}}")
				if err != nil {
					t.Errorf("Process() error = %v", err)
					continue
				}
				if got != want {
					t.Errorf("Variable %s = %v, want %v", k, got, want)
				}
			}
		})
	}
}

func TestExecutePreScript(t *testing.T) {
	tests := []struct {
		name      string
		script    string
		wantErr   bool
		wantLogs  int
		checkVars map[string]string
	}{
		{
			name: "simple logging",
			script: `
				client.log("Preparing request");
			`,
			wantErr:  false,
			wantLogs: 1,
		},
		{
			name: "set global variable",
			script: `
				client.global.set("timestamp", "12345");
			`,
			wantErr: false,
			checkVars: map[string]string{
				"timestamp": "12345",
			},
		},
		{
			name: "multiple operations",
			script: `
				client.log("Starting");
				client.global.set("requestId", "req-123");
				client.global.set("version", "v1");
				client.log("Done");
			`,
			wantErr:  false,
			wantLogs: 2,
			checkVars: map[string]string{
				"requestId": "req-123",
				"version":   "v1",
			},
		},
		{
			name:    "syntax error",
			script:  `client.log("unclosed string`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			varProcessor := variables.NewVariableProcessor()
			request := &models.HttpRequest{
				Method:  "GET",
				URL:     "https://example.com/api",
				Headers: map[string]string{},
			}

			result, err := ExecutePreScript(tt.script, cfg, request, varProcessor)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecutePreScript() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.wantLogs > 0 && len(result.Logs) != tt.wantLogs {
					t.Errorf("ExecutePreScript() logs = %v, want %v", len(result.Logs), tt.wantLogs)
				}

				for k, want := range tt.checkVars {
					got, err := varProcessor.Process("{{" + k + "}}")
					if err != nil {
						t.Errorf("Process() error = %v", err)
						continue
					}
					if got != want {
						t.Errorf("Variable %s = %v, want %v", k, got, want)
					}
				}
			}
		})
	}
}

func TestExecutePreScriptWithContext(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		cfg := &config.Config{}
		varProcessor := variables.NewVariableProcessor()
		request := &models.HttpRequest{
			Method:  "GET",
			URL:     "https://example.com/api",
			Headers: map[string]string{},
		}

		// Create already cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// This script has an infinite loop, but context is already cancelled
		script := `
			while(true) {
				// This would hang forever without context cancellation
			}
		`

		_, err := ExecutePreScriptWithContext(ctx, script, cfg, request, varProcessor)
		if err == nil {
			t.Error("Expected error due to context cancellation")
		}
	})
}

func TestExecutor_RequestResultStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-Id", "req-123")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":    1,
			"token": "auth-token",
		})
	}))
	defer server.Close()

	cfg := &config.Config{
		RememberCookies: false,
	}
	varProcessor := variables.NewVariableProcessor()

	exec := New(cfg, varProcessor, Options{
		NoSession: true,
		NoHistory: true,
	})

	request := &models.HttpRequest{
		Method:  "GET",
		URL:     server.URL + "/api",
		Headers: map[string]string{},
		Name:    "loginRequest",
	}

	_, err := exec.Execute(request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify request result is stored and can be referenced
	tests := []struct {
		variable string
		contains string
	}{
		{
			variable: "{{loginRequest.response.headers.X-Request-Id}}",
			contains: "req-123",
		},
		{
			variable: "{{loginRequest.response.body.$.token}}",
			contains: "auth-token",
		},
	}

	for _, tt := range tests {
		got, err := varProcessor.Process(tt.variable)
		if err != nil {
			t.Errorf("Process(%s) error = %v", tt.variable, err)
			continue
		}
		if got != tt.contains {
			t.Errorf("Process(%s) = %v, want %v", tt.variable, got, tt.contains)
		}
	}
}
