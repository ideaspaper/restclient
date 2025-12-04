package cmd

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSendCommand_PrintsResolvedURL(t *testing.T) {
	v.Set("showColors", false)
	defer v.Set("showColors", true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	httpFile := filepath.Join(tempDir, "test.http")
	content := `@baseUrl = ` + server.URL + `

GET {{baseUrl}}/users
`
	if err := os.WriteFile(httpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write http file: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	oldArgs := os.Args
	os.Args = []string{"restclient", "send", httpFile, "--no-history"}

	rootCmd.SetArgs([]string{"send", httpFile, "--no-history"})
	err := rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout
	os.Args = oldArgs

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Fatalf("command failed: %v\nOutput: %s", err, output)
	}

	expectedPrefix := "GET " + server.URL + "/users"
	if !strings.Contains(output, expectedPrefix) {
		t.Errorf("output should contain resolved URL\nExpected to contain: %s\nGot: %s", expectedPrefix, output)
	}

	if strings.Contains(output, "{{baseUrl}}") {
		t.Errorf("output should not contain unresolved variables\nGot: %s", output)
	}
}

func TestSendCommand_PrintsResolvedURL_MultipleVariables(t *testing.T) {
	v.Set("showColors", false)
	defer v.Set("showColors", true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	httpFile := filepath.Join(tempDir, "test.http")
	content := `@baseUrl = ` + server.URL + `
@version = v2
@resource = users

GET {{baseUrl}}/{{version}}/{{resource}}
`
	if err := os.WriteFile(httpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write http file: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	rootCmd.SetArgs([]string{"send", httpFile, "--no-history"})
	err := rootCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if err != nil {
		t.Fatalf("command failed: %v\nOutput: %s", err, output)
	}

	expectedURL := "GET " + server.URL + "/v2/users"
	if !strings.Contains(output, expectedURL) {
		t.Errorf("output should contain fully resolved URL\nExpected to contain: %s\nGot: %s", expectedURL, output)
	}

	if strings.Contains(output, "{{") {
		t.Errorf("output should not contain any unresolved variables\nGot: %s", output)
	}
}
