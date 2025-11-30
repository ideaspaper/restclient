package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ideaspaper/restclient/pkg/client"
	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/history"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/output"
	"github.com/ideaspaper/restclient/pkg/parser"
	"github.com/ideaspaper/restclient/pkg/variables"
)

var (
	// Send command flags
	requestName    string
	requestIndex   int
	showHeaders    bool
	showBody       bool
	outputFile     string
	noHistory      bool
	followRedirect *bool
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send <file.http|file.rest> [flags]",
	Short: "Send HTTP request from a .http or .rest file",
	Long: `Send an HTTP request defined in a .http or .rest file.

The file can contain one or multiple requests separated by ###.
You can select a specific request by name (using @name) or by index.

Examples:
  # Send the first request in the file
  restclient send api.http

  # Send a request by name
  restclient send api.http --name getUsers

  # Send request by index (0-based)
  restclient send api.http --index 2

  # Only show response headers
  restclient send api.http --headers

  # Only show response body
  restclient send api.http --body

  # Save response to file
  restclient send api.http --output response.json`,
	Args: cobra.ExactArgs(1),
	RunE: runSend,
}

func init() {
	rootCmd.AddCommand(sendCmd)

	// Send command flags
	sendCmd.Flags().StringVarP(&requestName, "name", "n", "", "request name (from @name metadata)")
	sendCmd.Flags().IntVarP(&requestIndex, "index", "i", 0, "request index (0-based)")
	sendCmd.Flags().BoolVar(&showHeaders, "headers", false, "only show response headers")
	sendCmd.Flags().BoolVar(&showBody, "body", false, "only show response body")
	sendCmd.Flags().StringVarP(&outputFile, "output", "o", "", "save response body to file")
	sendCmd.Flags().BoolVar(&noHistory, "no-history", false, "don't save request to history")
}

func runSend(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Load config
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override environment if specified
	if environment != "" {
		if err := cfg.SetEnvironment(environment); err != nil {
			return err
		}
	}

	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse file variables
	fileVars := variables.ParseFileVariables(string(content))

	// Create variable processor
	varProcessor := variables.NewVariableProcessor()
	varProcessor.SetEnvironment(cfg.CurrentEnvironment)
	varProcessor.SetEnvironmentVariables(cfg.EnvironmentVariables)
	varProcessor.SetFileVariables(fileVars)
	varProcessor.SetCurrentDir(filepath.Dir(filePath))
	varProcessor.SetPromptHandler(promptHandler)

	// Parse requests
	httpParser := parser.NewHttpRequestParser(string(content), cfg.DefaultHeaders, filepath.Dir(filePath))
	requests, err := httpParser.ParseAll()
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	if len(requests) == 0 {
		return fmt.Errorf("no requests found in file")
	}

	// Select request
	var request *models.HttpRequest
	if requestName != "" {
		// Find by name
		for _, req := range requests {
			if req.Name == requestName || req.Metadata.Name == requestName {
				request = req
				break
			}
		}
		if request == nil {
			return fmt.Errorf("request with name '%s' not found", requestName)
		}
	} else if cmd.Flags().Changed("index") {
		if requestIndex < 0 || requestIndex >= len(requests) {
			return fmt.Errorf("request index %d out of range (0-%d)", requestIndex, len(requests)-1)
		}
		request = requests[requestIndex]
	} else {
		// If multiple requests, show selection
		if len(requests) > 1 {
			request, err = selectRequest(requests)
			if err != nil {
				return err
			}
		} else {
			request = requests[0]
		}
	}

	// Handle prompt variables
	for _, pv := range request.Metadata.Prompts {
		value, err := promptHandler(pv.Name, pv.Description, pv.IsPassword)
		if err != nil {
			return fmt.Errorf("failed to get input for %s: %w", pv.Name, err)
		}
		varProcessor.SetFileVariables(map[string]string{pv.Name: value})
	}

	// Process variables in URL, headers, and body
	request.URL, _ = varProcessor.Process(request.URL)
	for k, v := range request.Headers {
		request.Headers[k], _ = varProcessor.Process(v)
	}
	if request.RawBody != "" {
		request.RawBody, _ = varProcessor.Process(request.RawBody)
		request.Body = strings.NewReader(request.RawBody)
	}

	// Send request
	return sendRequest(request, cfg, varProcessor)
}

func sendRequest(request *models.HttpRequest, cfg *config.Config, varProcessor *variables.VariableProcessor) error {
	// Create HTTP client
	clientCfg := cfg.ToClientConfig()

	// Handle per-request no-redirect
	if request.Metadata.NoRedirect {
		clientCfg.FollowRedirects = false
	}

	httpClient, err := client.NewHttpClient(clientCfg)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Print request info if verbose
	if verbose {
		printRequestInfo(request)
	}

	// Send the request
	resp, err := httpClient.Send(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	// Save to history if not disabled
	if !noHistory {
		histMgr, err := history.NewHistoryManager("")
		if err == nil {
			histMgr.Add(request)
		}
	}

	// Store result for request variables
	if request.Name != "" || request.Metadata.Name != "" {
		name := request.Name
		if name == "" {
			name = request.Metadata.Name
		}
		varProcessor.SetRequestResult(name, variables.RequestResult{
			StatusCode: resp.StatusCode,
			Headers:    resp.Headers,
			Body:       resp.Body,
		})
	}

	// Format and display response
	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	return displayResponse(resp, formatter)
}

func displayResponse(resp *models.HttpResponse, formatter *output.Formatter) error {
	// Save to file if specified
	if outputFile != "" {
		if err := os.WriteFile(outputFile, resp.BodyBuffer, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Response saved to %s\n", outputFile)
		if showBody {
			return nil
		}
	}

	// Determine what to show
	if showBody {
		fmt.Println(formatter.FormatBody(resp))
		return nil
	}

	if showHeaders {
		fmt.Println(formatter.FormatHeaders(resp))
		return nil
	}

	// Show full response
	fmt.Println(formatter.FormatResponse(resp))
	return nil
}

func printRequestInfo(request *models.HttpRequest) {
	c := color.New(color.FgCyan)
	c.Printf("=> %s %s\n", request.Method, request.URL)
	for k, v := range request.Headers {
		fmt.Printf("   %s: %s\n", k, v)
	}
	if request.RawBody != "" {
		fmt.Println()
		fmt.Println(request.RawBody)
	}
	fmt.Println()
}

func selectRequest(requests []*models.HttpRequest) (*models.HttpRequest, error) {
	fmt.Println("Multiple requests found. Select one:")
	fmt.Println()

	for i, req := range requests {
		name := req.Metadata.Name
		if name == "" {
			name = fmt.Sprintf("(unnamed request %d)", i+1)
		}
		fmt.Printf("  [%d] %s %s - %s\n", i, req.Method, truncateString(req.URL, 50), name)
	}

	fmt.Println()
	fmt.Print("Enter request number: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	input = strings.TrimSpace(input)
	index, err := strconv.Atoi(input)
	if err != nil || index < 0 || index >= len(requests) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	return requests[index], nil
}

func promptHandler(name, description string, isPassword bool) (string, error) {
	prompt := name
	if description != "" {
		prompt = fmt.Sprintf("%s (%s)", name, description)
	}

	fmt.Printf("Enter value for %s: ", prompt)

	if isPassword {
		// Read password without echo
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println() // New line after password input
		if err != nil {
			return "", err
		}
		return string(bytePassword), nil
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(input), nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
