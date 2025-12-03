package cmd

import (
	"bufio"
	"fmt"
	"net/url"
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
	"github.com/ideaspaper/restclient/pkg/scripting"
	"github.com/ideaspaper/restclient/pkg/session"
	"github.com/ideaspaper/restclient/pkg/variables"
)

var (
	// Send command flags
	requestName  string
	requestIndex int
	showHeaders  bool
	showBody     bool
	outputFile   string
	noHistory    bool
	dryRun       bool
	skipValidate bool
	sessionName  string
	noSession    bool
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

  # Send request by index (1-based)
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
	sendCmd.Flags().IntVarP(&requestIndex, "index", "i", 0, "request index (1-based)")
	sendCmd.Flags().BoolVar(&showHeaders, "headers", false, "only show response headers")
	sendCmd.Flags().BoolVar(&showBody, "body", false, "only show response body")
	sendCmd.Flags().StringVarP(&outputFile, "output", "o", "", "save response body to file")
	sendCmd.Flags().BoolVar(&noHistory, "no-history", false, "don't save request to history")
	sendCmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview request without sending")
	sendCmd.Flags().BoolVar(&skipValidate, "skip-validate", false, "skip request validation")
	sendCmd.Flags().StringVar(&sessionName, "session", "", "use named session instead of directory-based session")
	sendCmd.Flags().BoolVar(&noSession, "no-session", false, "don't load or save session state (cookies and variables)")
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

	// Parse requests with warnings
	httpParser := parser.NewHttpRequestParser(string(content), cfg.DefaultHeaders, filepath.Dir(filePath))
	parseResult := httpParser.ParseAllWithWarnings()
	requests := parseResult.Requests

	// Display parsing warnings in verbose mode
	if verbose && len(parseResult.Warnings) > 0 {
		warnColor := color.New(color.FgYellow)
		for _, w := range parseResult.Warnings {
			if noColor || !cfg.ShowColors {
				fmt.Fprintf(os.Stderr, "Warning: block %d: %s\n", w.BlockIndex+1, w.Message)
			} else {
				warnColor.Fprintf(os.Stderr, "Warning: block %d: %s\n", w.BlockIndex+1, w.Message)
			}
		}
		fmt.Fprintln(os.Stderr)
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
		// Convert 1-based user input to 0-based internal index
		internalIndex := requestIndex - 1
		if internalIndex < 0 || internalIndex >= len(requests) {
			return fmt.Errorf("request index %d out of range (1-%d)", requestIndex, len(requests))
		}
		request = requests[internalIndex]
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

	// Execute pre-request script BEFORE variable processing
	// This allows scripts to set variables that can be used in the request
	if request.Metadata.PreScript != "" {
		scriptCtx := setupScriptContext(cfg, request, nil)

		engine := scripting.NewEngine()
		result, err := engine.Execute(request.Metadata.PreScript, scriptCtx)
		if err != nil {
			return fmt.Errorf("pre-request script error: %w", err)
		}
		if result.Error != nil {
			return fmt.Errorf("pre-request script failed: %w", result.Error)
		}

		// Print script logs
		for _, log := range result.Logs {
			fmt.Printf("[pre-script] %s\n", log)
		}

		// Apply global variables set by script to variable processor
		applyScriptGlobalVars(varProcessor, result.GlobalVars)
	}

	// Process variables in URL, headers, and body
	var varErr error
	request.URL, varErr = varProcessor.Process(request.URL)
	if varErr != nil {
		return fmt.Errorf("failed to process variables in URL: %w", varErr)
	}
	for k, v := range request.Headers {
		request.Headers[k], varErr = varProcessor.Process(v)
		if varErr != nil {
			return fmt.Errorf("failed to process variables in header %s: %w", k, varErr)
		}
	}
	if request.RawBody != "" {
		request.RawBody, varErr = varProcessor.Process(request.RawBody)
		if varErr != nil {
			return fmt.Errorf("failed to process variables in body: %w", varErr)
		}
		request.Body = strings.NewReader(request.RawBody)
	}

	// Validate request before sending (unless --skip-validate is set)
	if !skipValidate {
		validation := request.Validate()
		if !validation.IsValid() {
			errColor := color.New(color.FgRed)
			if noColor || !cfg.ShowColors {
				fmt.Fprintln(os.Stderr, "Request validation failed:")
			} else {
				errColor.Fprintln(os.Stderr, "Request validation failed:")
			}
			for _, e := range validation.Errors {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", e.Field, e.Message)
			}
			return fmt.Errorf("request validation failed: %s", validation.Error())
		}
	}

	// Dry run - just print the request without sending
	if dryRun {
		return printDryRun(filePath, request, cfg)
	}

	// Send request
	return sendRequest(filePath, request, cfg, varProcessor)
}

func sendRequest(httpFilePath string, request *models.HttpRequest, cfg *config.Config, varProcessor *variables.VariableProcessor) error {
	// Initialize session manager (unless disabled)
	var sessionMgr *session.SessionManager
	if !noSession && cfg.RememberCookies {
		var err error
		sessionMgr, err = session.NewSessionManager("", httpFilePath, sessionName)
		if err != nil {
			// Non-fatal: just log warning and continue without session
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to initialize session: %v\n", err)
			}
		} else {
			// Load existing session data
			if err := sessionMgr.Load(); err != nil && verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to load session: %v\n", err)
			}

			// Inject session variables into variable processor
			for name, value := range sessionMgr.GetAllVariables() {
				if strVal, ok := value.(string); ok {
					varProcessor.SetFileVariables(map[string]string{name: strVal})
				} else {
					varProcessor.SetFileVariables(map[string]string{name: fmt.Sprintf("%v", value)})
				}
			}
		}
	}

	// Create HTTP client
	clientCfg := cfg.ToClientConfig()

	// Handle per-request no-redirect
	if request.Metadata.NoRedirect {
		clientCfg.FollowRedirects = false
	}

	// Note: We don't set clientCfg.RememberCookies = false for @no-cookie-jar
	// because the HTTP client's internal cookie jar is still needed for the
	// request to work correctly. Instead, we just skip loading/saving cookies
	// to/from the session (handled below with NoCookieJar checks).

	httpClient, err := client.NewHttpClient(clientCfg)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Load cookies from session into HTTP client
	if sessionMgr != nil && !request.Metadata.NoCookieJar {
		cookies := sessionMgr.GetCookiesForURL(request.URL)
		if len(cookies) > 0 {
			httpClient.SetCookies(request.URL, cookies)
		}
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

	// Save cookies from response to session
	if sessionMgr != nil && !request.Metadata.NoCookieJar {
		responseCookies := httpClient.GetCookies(request.URL)
		if len(responseCookies) > 0 {
			sessionMgr.SetCookiesFromResponse(request.URL, responseCookies)
		}
	}

	// Save to history if not disabled
	if !noHistory {
		histMgr, err := history.NewHistoryManager("")
		if err == nil {
			// Create a copy of request with actual Cookie header that was sent
			historyRequest := *request
			historyRequest.Headers = make(map[string]string)
			for k, v := range request.Headers {
				historyRequest.Headers[k] = v
			}
			// Add Cookie header if cookies were sent
			if sessionMgr != nil && !request.Metadata.NoCookieJar {
				cookies := sessionMgr.GetCookiesForURL(request.URL)
				if len(cookies) > 0 {
					var cookieParts []string
					for _, c := range cookies {
						cookieParts = append(cookieParts, c.Name+"="+c.Value)
					}
					historyRequest.Headers["Cookie"] = strings.Join(cookieParts, "; ")
				}
			}
			histMgr.Add(&historyRequest)
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

	// Execute post-response script if present
	if request.Metadata.PostScript != "" {
		scriptCtx := setupScriptContext(cfg, request, resp)

		// Load existing session variables into script context
		if sessionMgr != nil {
			for name, value := range sessionMgr.GetAllVariables() {
				scriptCtx.SetGlobalVar(name, value)
			}
		}

		engine := scripting.NewEngine()
		result, err := engine.Execute(request.Metadata.PostScript, scriptCtx)
		if err != nil {
			return fmt.Errorf("post-response script error: %w", err)
		}

		// Print script logs
		for _, log := range result.Logs {
			fmt.Printf("[script] %s\n", log)
		}

		// Print test results
		if len(result.Tests) > 0 {
			fmt.Println()
			printTestResults(result.Tests, !noColor && cfg.ShowColors)
		}

		// Apply global variables set by script to variable processor
		applyScriptGlobalVars(varProcessor, result.GlobalVars)

		// Save script globals to session
		if sessionMgr != nil {
			for name, value := range result.GlobalVars {
				sessionMgr.SetVariable(name, value)
			}
		}

		// Check for script errors after processing
		if result.Error != nil {
			return fmt.Errorf("post-response script failed: %w", result.Error)
		}

		// Check if any tests failed
		for _, test := range result.Tests {
			if !test.Passed {
				return fmt.Errorf("test '%s' failed: %s", test.Name, test.Error)
			}
		}
	}

	// Save session to disk
	if sessionMgr != nil {
		if err := sessionMgr.Save(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
		}
	}

	// Format and display response
	formatter := output.NewFormatter(!noColor && cfg.ShowColors)
	return displayResponse(resp, formatter)
}

func printTestResults(tests []scripting.TestResult, useColors bool) {
	passColor := color.New(color.FgGreen)
	failColor := color.New(color.FgRed)

	fmt.Println("Test Results:")
	for _, test := range tests {
		if test.Passed {
			if useColors {
				fmt.Printf("  %s %s\n", passColor.Sprint("✓"), test.Name)
			} else {
				fmt.Printf("  [PASS] %s\n", test.Name)
			}
		} else {
			if useColors {
				fmt.Printf("  %s %s: %s\n", failColor.Sprint("✗"), test.Name, test.Error)
			} else {
				fmt.Printf("  [FAIL] %s: %s\n", test.Name, test.Error)
			}
		}
	}
}

// setupScriptContext creates a script context with environment variables from config
func setupScriptContext(cfg *config.Config, request *models.HttpRequest, resp *models.HttpResponse) *scripting.ScriptContext {
	scriptCtx := scripting.NewScriptContext()
	scriptCtx.SetRequest(request)

	if resp != nil {
		scriptCtx.SetResponse(resp)
	}

	// Copy environment variables to script context
	if cfg.EnvironmentVariables != nil {
		if shared, ok := cfg.EnvironmentVariables["$shared"]; ok {
			for k, v := range shared {
				scriptCtx.SetEnvVar(k, v)
			}
		}
		if current, ok := cfg.EnvironmentVariables[cfg.CurrentEnvironment]; ok {
			for k, v := range current {
				scriptCtx.SetEnvVar(k, v)
			}
		}
	}

	return scriptCtx
}

// applyScriptGlobalVars applies global variables from script result to variable processor
func applyScriptGlobalVars(varProcessor *variables.VariableProcessor, globalVars map[string]interface{}) {
	for k, v := range globalVars {
		if strVal, ok := v.(string); ok {
			varProcessor.SetFileVariables(map[string]string{k: strVal})
		} else {
			varProcessor.SetFileVariables(map[string]string{k: fmt.Sprintf("%v", v)})
		}
	}
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

func printDryRun(httpFilePath string, request *models.HttpRequest, cfg *config.Config) error {
	formatter := output.NewFormatter(!noColor && cfg.ShowColors)

	// Print header
	fmt.Println(formatter.FormatInfo("=== DRY RUN (request will not be sent) ==="))
	fmt.Println()

	// Print request line
	c := color.New(color.FgGreen, color.Bold)
	if noColor || !cfg.ShowColors {
		fmt.Printf("%s %s\n", request.Method, request.URL)
	} else {
		c.Printf("%s %s\n", request.Method, request.URL)
	}
	fmt.Println()

	// Print headers
	headerColor := color.New(color.FgCyan)
	fmt.Println("Headers:")
	for k, v := range request.Headers {
		if noColor || !cfg.ShowColors {
			fmt.Printf("  %s: %s\n", k, v)
		} else {
			fmt.Printf("  %s: %s\n", headerColor.Sprint(k), v)
		}
	}

	// Print default headers that would be added
	if len(cfg.DefaultHeaders) > 0 {
		fmt.Println()
		fmt.Println("Default Headers (from config):")
		for k, v := range cfg.DefaultHeaders {
			// Skip if already set in request
			if _, exists := request.Headers[k]; exists {
				continue
			}
			if noColor || !cfg.ShowColors {
				fmt.Printf("  %s: %s\n", k, v)
			} else {
				fmt.Printf("  %s: %s\n", headerColor.Sprint(k), v)
			}
		}
	}

	// Print session cookies that would be sent
	if !noSession && cfg.RememberCookies && !request.Metadata.NoCookieJar {
		sessionMgr, err := session.NewSessionManager("", httpFilePath, sessionName)
		if err == nil {
			if err := sessionMgr.Load(); err == nil {
				// Parse URL to get host
				parsedURL, err := url.Parse(request.URL)
				if err == nil {
					cookies := sessionMgr.GetCookiesForURL(parsedURL.String())
					if len(cookies) > 0 {
						fmt.Println()
						fmt.Println("Session Cookies (from previous requests):")
						for _, cookie := range cookies {
							if noColor || !cfg.ShowColors {
								fmt.Printf("  %s = %s\n", cookie.Name, cookie.Value)
							} else {
								fmt.Printf("  %s = %s\n", headerColor.Sprint(cookie.Name), cookie.Value)
							}
						}
					}
				}

				// Print session variables
				vars := sessionMgr.GetAllVariables()
				if len(vars) > 0 {
					fmt.Println()
					fmt.Println("Session Variables:")
					for k, v := range vars {
						if noColor || !cfg.ShowColors {
							fmt.Printf("  %s = %s\n", k, v)
						} else {
							fmt.Printf("  %s = %s\n", headerColor.Sprint(k), v)
						}
					}
				}
			}
		}
	}

	// Print body if present
	if request.RawBody != "" {
		fmt.Println()
		fmt.Println("Body:")
		fmt.Println(request.RawBody)
	}

	// Print metadata
	fmt.Println()
	fmt.Println("Request Settings:")
	fmt.Printf("  Follow Redirects: %v\n", cfg.FollowRedirects && !request.Metadata.NoRedirect)
	fmt.Printf("  Use Cookie Jar: %v\n", cfg.RememberCookies && !request.Metadata.NoCookieJar)
	if cfg.TimeoutMs > 0 {
		fmt.Printf("  Timeout: %dms\n", cfg.TimeoutMs)
	}
	if cfg.Proxy != "" {
		fmt.Printf("  Proxy: %s\n", cfg.Proxy)
	}

	return nil
}

func selectRequest(requests []*models.HttpRequest) (*models.HttpRequest, error) {
	fmt.Println("Multiple requests found. Select one:")
	fmt.Println()

	for i, req := range requests {
		name := req.Metadata.Name
		if name == "" {
			name = fmt.Sprintf("(unnamed request %d)", i+1)
		}
		// Display 1-based index for user-facing output
		fmt.Printf("  [%d] %s %s - %s\n", i+1, req.Method, truncateString(req.URL, 50), name)
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
	if err != nil || index < 1 || index > len(requests) {
		return nil, fmt.Errorf("invalid selection: %s", input)
	}

	// Convert 1-based user input to 0-based internal index
	return requests[index-1], nil
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
