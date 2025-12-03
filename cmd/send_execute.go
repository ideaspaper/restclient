package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/ideaspaper/restclient/pkg/client"
	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/history"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/output"
	"github.com/ideaspaper/restclient/pkg/scripting"
	"github.com/ideaspaper/restclient/pkg/session"
	"github.com/ideaspaper/restclient/pkg/variables"
)

// sendRequest sends an HTTP request with session management and scripting support
func sendRequest(httpFilePath string, request *models.HttpRequest, cfg *config.Config, varProcessor *variables.VariableProcessor) error {
	var sessionMgr *session.SessionManager
	if !noSession && cfg.RememberCookies {
		var err error
		sessionMgr, err = session.NewSessionManager("", httpFilePath, sessionName)
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to initialize session: %v\n", err)
			}
		} else {
			if err := sessionMgr.Load(); err != nil && verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to load session: %v\n", err)
			}

			for name, value := range sessionMgr.GetAllVariables() {
				if strVal, ok := value.(string); ok {
					varProcessor.SetFileVariables(map[string]string{name: strVal})
				} else {
					varProcessor.SetFileVariables(map[string]string{name: fmt.Sprintf("%v", value)})
				}
			}
		}
	}

	clientCfg := cfg.ToClientConfig()

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

	resp, err := httpClient.Send(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if sessionMgr != nil && !request.Metadata.NoCookieJar {
		responseCookies := httpClient.GetCookies(request.URL)
		if len(responseCookies) > 0 {
			sessionMgr.SetCookiesFromResponse(request.URL, responseCookies)
		}
	}

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

	// Store result for request variable references (e.g., {{requestName.response.body}})
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

		for _, log := range result.Logs {
			fmt.Printf("[script] %s\n", log)
		}

		if len(result.Tests) > 0 {
			fmt.Println()
			printTestResults(result.Tests)
		}

		applyScriptGlobalVars(varProcessor, result.GlobalVars)

		if sessionMgr != nil {
			for name, value := range result.GlobalVars {
				sessionMgr.SetVariable(name, value)
			}
		}

		if result.Error != nil {
			return fmt.Errorf("post-response script failed: %w", result.Error)
		}

		for _, test := range result.Tests {
			if !test.Passed {
				return fmt.Errorf("test '%s' failed: %s", test.Name, test.Error)
			}
		}
	}

	if sessionMgr != nil {
		if err := sessionMgr.Save(); err != nil && verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
		}
	}

	formatter := output.NewFormatter(useColors())
	return displayResponse(resp, formatter)
}

// setupScriptContext creates a script context with environment variables from config
func setupScriptContext(cfg *config.Config, request *models.HttpRequest, resp *models.HttpResponse) *scripting.ScriptContext {
	scriptCtx := scripting.NewScriptContext()
	scriptCtx.SetRequest(request)

	if resp != nil {
		scriptCtx.SetResponse(resp)
	}

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

// printTestResults prints script test results
func printTestResults(tests []scripting.TestResult) {
	passColor := color.New(color.FgGreen)
	failColor := color.New(color.FgRed)

	fmt.Println("Test Results:")
	for _, test := range tests {
		if test.Passed {
			if useColors() {
				fmt.Printf("  %s %s\n", passColor.Sprint("✓"), test.Name)
			} else {
				fmt.Printf("  [PASS] %s\n", test.Name)
			}
		} else {
			if useColors() {
				fmt.Printf("  %s %s: %s\n", failColor.Sprint("✗"), test.Name, test.Error)
			} else {
				fmt.Printf("  [FAIL] %s: %s\n", test.Name, test.Error)
			}
		}
	}
}

// displayResponse formats and displays an HTTP response
func displayResponse(resp *models.HttpResponse, formatter *output.Formatter) error {
	if outputFile != "" {
		if err := os.WriteFile(outputFile, resp.BodyBuffer, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Printf("Response saved to %s\n", outputFile)
		if showBody {
			return nil
		}
	}

	if showBody {
		fmt.Println(formatter.FormatBody(resp))
		return nil
	}

	if showHeaders {
		fmt.Println(formatter.FormatHeaders(resp))
		return nil
	}

	fmt.Println(formatter.FormatResponse(resp))
	return nil
}

// printRequestInfo prints verbose request information
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

// printDryRun displays a dry run of the request without sending it
func printDryRun(httpFilePath string, request *models.HttpRequest, cfg *config.Config) error {
	colorsEnabled := useColors() && cfg.ShowColors
	formatter := output.NewFormatter(colorsEnabled)

	fmt.Println(formatter.FormatInfo("=== DRY RUN (request will not be sent) ==="))
	fmt.Println()

	c := color.New(color.FgGreen, color.Bold)
	if colorsEnabled {
		c.Printf("%s %s\n", request.Method, request.URL)
	} else {
		fmt.Printf("%s %s\n", request.Method, request.URL)
	}
	fmt.Println()

	headerColor := color.New(color.FgCyan)
	fmt.Println("Headers:")
	for k, v := range request.Headers {
		if colorsEnabled {
			fmt.Printf("  %s: %s\n", headerColor.Sprint(k), v)
		} else {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	if len(cfg.DefaultHeaders) > 0 {
		fmt.Println()
		fmt.Println("Default Headers (from config):")
		for k, v := range cfg.DefaultHeaders {
			// Skip if already set in request
			if _, exists := request.Headers[k]; exists {
				continue
			}
			if colorsEnabled {
				fmt.Printf("  %s: %s\n", headerColor.Sprint(k), v)
			} else {
				fmt.Printf("  %s: %s\n", k, v)
			}
		}
	}

	if !noSession && cfg.RememberCookies && !request.Metadata.NoCookieJar {
		sessionMgr, err := session.NewSessionManager("", httpFilePath, sessionName)
		if err == nil {
			if err := sessionMgr.Load(); err == nil {
				parsedURL, err := url.Parse(request.URL)
				if err == nil {
					cookies := sessionMgr.GetCookiesForURL(parsedURL.String())
					if len(cookies) > 0 {
						fmt.Println()
						fmt.Println("Session Cookies (from previous requests):")
						for _, cookie := range cookies {
							if colorsEnabled {
								fmt.Printf("  %s = %s\n", headerColor.Sprint(cookie.Name), cookie.Value)
							} else {
								fmt.Printf("  %s = %s\n", cookie.Name, cookie.Value)
							}
						}
					}
				}

				vars := sessionMgr.GetAllVariables()
				if len(vars) > 0 {
					fmt.Println()
					fmt.Println("Session Variables:")
					for k, v := range vars {
						if colorsEnabled {
							fmt.Printf("  %s = %s\n", headerColor.Sprint(k), v)
						} else {
							fmt.Printf("  %s = %s\n", k, v)
						}
					}
				}
			}
		}
	}

	if request.RawBody != "" {
		fmt.Println()
		fmt.Println("Body:")
		fmt.Println(request.RawBody)
	}

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
