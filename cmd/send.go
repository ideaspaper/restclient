package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ideaspaper/restclient/internal/stringutil"
	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/executor"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/parser"
	"github.com/ideaspaper/restclient/pkg/tui"
	"github.com/ideaspaper/restclient/pkg/variables"
)

// RequestItem implements tui.Item for HTTP requests
type RequestItem struct {
	Request *models.HttpRequest
	Index   int // 0-based index
}

// FilterValue returns the string used for fuzzy matching
func (r RequestItem) FilterValue() string {
	name := r.Request.Metadata.Name
	if name == "" {
		name = fmt.Sprintf("request %d", r.Index+1)
	}
	// Include method, URL, and name for matching (not index)
	return fmt.Sprintf("%s %s %s", r.Request.Method, r.Request.URL, name)
}

// Title returns the main display text (method and URL)
func (r RequestItem) Title() string {
	return fmt.Sprintf("%s %s", r.Request.Method, stringutil.Truncate(r.Request.URL, 50))
}

// Description returns the request name
func (r RequestItem) Description() string {
	name := r.Request.Metadata.Name
	if name == "" {
		return fmt.Sprintf("(unnamed request %d)", r.Index+1)
	}
	return name
}

var (
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
	strictMode   bool
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
	sendCmd.Flags().BoolVar(&strictMode, "strict", false, "error on duplicate @name values instead of warning")
}

func runSend(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	if environment != "" {
		if err := cfg.SetEnvironment(environment); err != nil {
			return err
		}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to read file")
	}

	fileVarsResult := variables.ParseFileVariablesWithDuplicates(string(content))
	fileVars := fileVarsResult.Variables

	if len(fileVarsResult.Duplicates) > 0 {
		for _, dup := range fileVarsResult.Duplicates {
			msg := fmt.Sprintf("duplicate file variable '@%s': value '%s' overwritten with '%s'",
				dup.Name, dup.OldValue, dup.NewValue)
			if useColors() {
				warnColor.Fprintf(os.Stderr, "Warning: %s\n", msg)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
			}
		}
	}

	varProcessor := variables.NewVariableProcessor()
	varProcessor.SetEnvironment(cfg.CurrentEnvironment)
	varProcessor.SetEnvironmentVariables(cfg.EnvironmentVariables)
	varProcessor.SetFileVariables(fileVars)
	varProcessor.SetCurrentDir(filepath.Dir(filePath))
	varProcessor.SetPromptHandler(promptHandler)

	httpParser := parser.NewHttpRequestParser(string(content), cfg.DefaultHeaders, filepath.Dir(filePath))
	parseResult := httpParser.ParseAllWithWarnings()
	requests := parseResult.Requests

	duplicates := parser.FindDuplicateNames(requests)
	if len(duplicates) > 0 {
		errColor := color.New(color.FgRed)

		for name, dupes := range duplicates {
			var details []string
			for _, d := range dupes {
				details = append(details, fmt.Sprintf("request %d: %s %s", d.Index+1, d.Method, d.URL))
			}
			msg := fmt.Sprintf("duplicate @name '%s' found in %d requests:\n", name, len(dupes))
			for _, detail := range details {
				msg += fmt.Sprintf("  - %s\n", detail)
			}
			msg += "First match will be used when selecting by name."

			if strictMode {
				if useColors() {
					errColor.Fprintf(os.Stderr, "Error: %s\n", msg)
				} else {
					fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
				}
			} else {
				if useColors() {
					warnColor.Fprintf(os.Stderr, "Warning: %s\n", msg)
				} else {
					fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
				}
			}
		}

		if strictMode {
			return errors.NewValidationError("@name", "duplicate @name values found (use without --strict to continue with warnings)")
		}
	}

	if verbose && len(parseResult.Warnings) > 0 {
		for _, w := range parseResult.Warnings {
			if useColors() {
				warnColor.Fprintf(os.Stderr, "Warning: block %d: %s\n", w.BlockIndex+1, w.Message)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: block %d: %s\n", w.BlockIndex+1, w.Message)
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	if len(requests) == 0 {
		return errors.NewValidationError("requests", "no requests found in file")
	}

	var request *models.HttpRequest
	if requestName != "" {
		for _, req := range requests {
			if req.Name == requestName || req.Metadata.Name == requestName {
				request = req
				break
			}
		}
		if request == nil {
			return errors.NewValidationErrorWithValue("request name", requestName, "request not found")
		}
	} else if cmd.Flags().Changed("index") {
		internalIndex := requestIndex - 1
		if internalIndex < 0 || internalIndex >= len(requests) {
			return errors.NewValidationErrorWithValue("request index", fmt.Sprintf("%d", requestIndex), fmt.Sprintf("out of range (1-%d)", len(requests)))
		}
		request = requests[internalIndex]
	} else {
		if len(requests) > 1 {
			request, err = selectRequest(requests)
			if err != nil {
				if errors.Is(err, errors.ErrCanceled) {
					return nil
				}
				return err
			}
			fmt.Println() // Blank line after selection
		} else {
			request = requests[0]
		}
	}

	for _, pv := range request.Metadata.Prompts {
		value, err := promptHandler(pv.Name, pv.Description, pv.IsPassword)
		if err != nil {
			return errors.Wrapf(err, "failed to get input for %s", pv.Name)
		}
		varProcessor.SetFileVariables(map[string]string{pv.Name: value})
	}

	// Execute pre-request script before variable processing so scripts can set variables
	if request.Metadata.PreScript != "" {
		// Create context with cancellation support for interrupt signals
		ctx, cancel := context.WithCancel(context.Background())

		// Handle interrupt signal (Ctrl+C)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigChan
			if verbose {
				fmt.Fprintln(os.Stderr, "\nInterrupt received, cancelling pre-script...")
			}
			cancel()
		}()

		result, err := executor.ExecutePreScriptWithContext(ctx, request.Metadata.PreScript, cfg, request, varProcessor)

		// Clean up signal handling
		signal.Stop(sigChan)
		cancel()

		if err != nil {
			// Check for context cancellation
			if ctx.Err() == context.Canceled {
				return errors.Wrap(errors.ErrCanceled, "pre-script cancelled")
			}
			return err
		}

		for _, log := range result.Logs {
			fmt.Printf("[pre-script] %s\n", log)
		}
	}

	var varErr error
	request.URL, varErr = varProcessor.Process(request.URL)
	if varErr != nil {
		return errors.Wrap(varErr, "failed to process variables in URL")
	}
	for k, v := range request.Headers {
		request.Headers[k], varErr = varProcessor.Process(v)
		if varErr != nil {
			return errors.Wrapf(varErr, "failed to process variables in header %s", k)
		}
	}
	if request.RawBody != "" {
		request.RawBody, varErr = varProcessor.Process(request.RawBody)
		if varErr != nil {
			return errors.Wrap(varErr, "failed to process variables in body")
		}
		request.Body = strings.NewReader(request.RawBody)
	}

	if !skipValidate {
		validation := request.Validate()
		if !validation.IsValid() {
			if useColors() {
				errorColor.Fprintln(os.Stderr, "Request validation failed:")
			} else {
				fmt.Fprintln(os.Stderr, "Request validation failed:")
			}
			for _, e := range validation.Errors {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", e.Field, e.Message)
			}
			return errors.NewValidationError("request", validation.Error())
		}
	}

	if dryRun {
		return printDryRun(filePath, request, cfg)
	}

	return sendRequest(filePath, request, cfg, varProcessor)
}

func selectRequest(requests []*models.HttpRequest) (*models.HttpRequest, error) {
	items := make([]tui.Item, len(requests))
	for i, req := range requests {
		items[i] = RequestItem{Request: req, Index: i}
	}

	_, selectedIndex, err := tui.Run(items, useColors())
	if err != nil {
		return nil, err
	}

	return requests[selectedIndex], nil
}

func promptHandler(name, description string, isPassword bool) (string, error) {
	prompt := name
	if description != "" {
		prompt = fmt.Sprintf("%s (%s)", name, description)
	}

	fmt.Printf("Enter value for %s: ", prompt)

	if isPassword {
		// Read password without echo
		bytePassword, err := term.ReadPassword(int(os.Stdin.Fd()))
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
