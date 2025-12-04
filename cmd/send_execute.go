package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/executor"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/output"
	"github.com/ideaspaper/restclient/pkg/scripting"
	"github.com/ideaspaper/restclient/pkg/session"
	"github.com/ideaspaper/restclient/pkg/variables"
)

// sendRequest sends an HTTP request with session management and scripting support
func sendRequest(httpFilePath string, request *models.HttpRequest, cfg *config.Config, varProcessor *variables.VariableProcessor) error {
	// Create context with cancellation support for interrupt signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if verbose {
			fmt.Fprintln(os.Stderr, "\nInterrupt received, cancelling request...")
		}
		cancel()
	}()
	defer signal.Stop(sigChan)

	// Apply timeout from config if set
	if cfg.TimeoutMs > 0 {
		var timeoutCancel context.CancelFunc
		ctx, timeoutCancel = context.WithTimeout(ctx, time.Duration(cfg.TimeoutMs)*time.Millisecond)
		defer timeoutCancel()
	}

	opts := executor.Options{
		HTTPFilePath: httpFilePath,
		SessionName:  sessionName,
		NoSession:    noSession,
		NoHistory:    noHistory,
		Verbose:      verbose,
		LogFunc: func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, format+"\n", args...)
		},
	}

	exec := executor.New(cfg, varProcessor, opts)

	// Print request info if verbose
	if verbose {
		printRequestInfo(request)
	}

	result, err := exec.ExecuteWithContext(ctx, request)
	if err != nil {
		// Check for context cancellation
		if ctx.Err() == context.Canceled {
			return errors.Wrap(errors.ErrCanceled, "request cancelled")
		}
		if ctx.Err() == context.DeadlineExceeded {
			return errors.Wrap(errors.ErrTimeout, "request timed out")
		}
		return err
	}

	// Print logs from scripts
	for _, log := range result.Logs {
		fmt.Printf("[script] %s\n", log)
	}

	// Print test results if any
	if len(result.TestResults) > 0 {
		fmt.Println()
		printTestResults(result.TestResults)
	}

	formatter := output.NewFormatter(useColors())
	return displayResponse(result.Response, formatter)
}

// printTestResults prints script test results
func printTestResults(tests []scripting.TestResult) {
	fmt.Println("Test Results:")
	for _, test := range tests {
		if test.Passed {
			if useColors() {
				fmt.Printf("  %s %s\n", successColor.Sprint("✓"), test.Name)
			} else {
				fmt.Printf("  [PASS] %s\n", test.Name)
			}
		} else {
			if useColors() {
				fmt.Printf("  %s %s: %s\n", errorColor.Sprint("✗"), test.Name, test.Error)
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
			return errors.Wrap(err, "failed to write output file")
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
	if useColors() {
		headerColor.Printf("=> %s %s\n", request.Method, request.URL)
	} else {
		fmt.Printf("=> %s %s\n", request.Method, request.URL)
	}
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

	printMethodURL(request.Method, request.URL)
	fmt.Println()

	fmt.Println("Headers:")
	for k, v := range request.Headers {
		fmt.Printf("  %s: %s\n", formatKey(k), v)
	}

	if len(cfg.DefaultHeaders) > 0 {
		fmt.Println()
		fmt.Println("Default Headers (from config):")
		for k, v := range cfg.DefaultHeaders {
			// Skip if already set in request
			if _, exists := request.Headers[k]; exists {
				continue
			}
			fmt.Printf("  %s: %s\n", formatKey(k), v)
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
							fmt.Printf("  %s = %s\n", formatKey(cookie.Name), cookie.Value)
						}
					}
				}

				vars := sessionMgr.GetAllVariables()
				if len(vars) > 0 {
					fmt.Println()
					fmt.Println("Session Variables:")
					for k, v := range vars {
						fmt.Printf("  %s = %s\n", formatKey(k), v)
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
