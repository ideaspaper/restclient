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

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/internal/stringutil"
	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/executor"
	"github.com/ideaspaper/restclient/pkg/lastfile"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/parser"
	"github.com/ideaspaper/restclient/pkg/session"
	"github.com/ideaspaper/restclient/pkg/tui"
	"github.com/ideaspaper/restclient/pkg/userinput"
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

// String returns formatted string for display: [index] title  description
func (r RequestItem) String() string {
	return fmt.Sprintf("[%d] %s  %s", r.Index+1, r.Title(), r.Description())
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
	Use:   "send [file.http|file.rest] [flags]",
	Short: "Send HTTP request from a .http or .rest file",
	Long: `Send an HTTP request defined in a .http or .rest file.

The file can contain one or multiple requests separated by ###.
You can select a specific request by name (using @name) or by index.

If no file is specified, the last used file will be used automatically.
The last file path is stored in ~/.restclient/lastfile.

Examples:
  # Send the first request in the file
  restclient send api.http

  # Send from the last used file
  restclient send

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
	Args: cobra.MaximumNArgs(1),
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
	filePath, err := resolveRequestFilePath(cmd, args)
	if err != nil {
		return err
	}

	cfg, sessionCfg, envStore, err := loadSendConfig(filePath)
	if err != nil {
		return err
	}

	content, err := readRequestFile(filePath)
	if err != nil {
		return err
	}

	varProcessor := buildVariableProcessor(sessionCfg, filePath, content, envStore)

	requests, parseWarnings, err := parseRequestsFromContent(content, sessionCfg, filePath)
	if err != nil {
		return err
	}

	printParseWarnings(parseWarnings)

	request, err := selectRequestForSend(cmd, requests)
	if err != nil {
		if errors.Is(err, errors.ErrCanceled) {
			return nil
		}
		return err
	}

	printRequestWarnings(request)

	if err := processSessionInputs(request, filePath); err != nil {
		if errors.Is(err, errors.ErrCanceled) {
			return nil
		}
		return err
	}

	if err := applyPromptVariables(request, varProcessor); err != nil {
		return err
	}

	if err := runPreRequestScript(request, sessionCfg, varProcessor, envStore); err != nil {
		if errors.Is(err, errors.ErrCanceled) {
			return nil
		}
		return err
	}

	if err := processRequestVariables(request, varProcessor); err != nil {
		return err
	}

	if err := validateRequest(request); err != nil {
		return err
	}

	if dryRun {
		return printDryRun(filePath, request, cfg, sessionCfg)
	}

	fmt.Printf("%s %s\n\n", printMethod(request.Method), request.URL)

	return sendRequest(filePath, request, sessionCfg, varProcessor, envStore)
}

func resolveRequestFilePath(cmd *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}

	lastPath, err := lastfile.Load()
	if err != nil {
		return "", err
	}
	if lastPath == "" {
		return "", cmd.Help()
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "Using last file: %s\n", lastPath)
	}
	return lastPath, nil
}

func loadSendConfig(filePath string) (*config.Config, *session.SessionConfig, *session.EnvironmentStore, error) {
	// Load global config (CLI preferences only)
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to load config")
	}

	// Get session path for this file
	sessionMgr, err := session.NewSessionManager("", filePath, sessionName)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to create session manager")
	}
	sessionPath := sessionMgr.GetSessionPath()

	// Load or create session config (HTTP behavior)
	sessionCfg, err := session.LoadOrCreateSessionConfig(filesystem.Default, sessionPath)
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to load session config: %v\n", err)
		}
		// Use default session config if loading fails
		sessionCfg = session.DefaultSessionConfig()
	}

	// Load or create session environment store
	envStore, err := session.LoadOrCreateEnvironmentStore(filesystem.Default, sessionPath)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to load session environments")
	}

	// Override environment from command line flag
	if environment != "" {
		if !envStore.HasEnvironment(environment) && environment != "$shared" {
			return nil, nil, nil, errors.NewValidationErrorWithValue("environment", environment, "not found")
		}
		sessionCfg.SetCurrentEnvironment(environment)
	}

	return cfg, sessionCfg, envStore, nil
}

func readRequestFile(filePath string) ([]byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	if saveErr := lastfile.Save(filePath); saveErr != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to save last file path: %v\n", saveErr)
		}
	}

	return content, nil
}

func buildVariableProcessor(sessionCfg *session.SessionConfig, filePath string, content []byte, envStore *session.EnvironmentStore) *variables.VariableProcessor {
	fileVarsResult := variables.ParseFileVariablesWithDuplicates(string(content))

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
	varProcessor.SetEnvironment(sessionCfg.CurrentEnvironment())

	// Load environment variables from session environment store
	if envStore != nil {
		varProcessor.SetEnvironmentVariables(envStore.EnvironmentVariables)
	}

	varProcessor.SetFileVariables(fileVarsResult.Variables)
	varProcessor.SetCurrentDir(filepath.Dir(filePath))
	varProcessor.SetPromptHandler(promptHandler)

	return varProcessor
}

func parseRequestsFromContent(content []byte, sessionCfg *session.SessionConfig, filePath string) ([]*models.HttpRequest, []parser.ParseWarning, error) {
	httpParser := parser.NewHttpRequestParser(string(content), sessionCfg.DefaultHeaders(), filepath.Dir(filePath))
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
			return nil, nil, errors.NewValidationError("@name", "duplicate @name values found (use without --strict to continue with warnings)")
		}
	}

	if len(requests) == 0 {
		return nil, nil, errors.NewValidationError("requests", "no requests found in file")
	}

	return requests, parseResult.Warnings, nil
}

func printParseWarnings(warnings []parser.ParseWarning) {
	if !verbose || len(warnings) == 0 {
		return
	}

	for _, w := range warnings {
		if useColors() {
			warnColor.Fprintf(os.Stderr, "Warning: block %d: %s\n", w.BlockIndex+1, w.Message)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: block %d: %s\n", w.BlockIndex+1, w.Message)
		}
	}

	fmt.Fprintln(os.Stderr)
}

func selectRequestForSend(cmd *cobra.Command, requests []*models.HttpRequest) (*models.HttpRequest, error) {
	var request *models.HttpRequest
	var selectedIndex int

	if requestName != "" {
		for i, req := range requests {
			if req.Name == requestName || req.Metadata.Name == requestName {
				request = req
				selectedIndex = i
				break
			}
		}
		if request == nil {
			return nil, errors.NewValidationErrorWithValue("request name", requestName, "request not found")
		}
		item := RequestItem{Request: request, Index: selectedIndex}
		fmt.Printf("\n%s\n\n", item.String())
		return request, nil
	}

	if cmd.Flags().Changed("index") {
		internalIndex := requestIndex - 1
		if internalIndex < 0 || internalIndex >= len(requests) {
			return nil, errors.NewValidationErrorWithValue("request index", fmt.Sprintf("%d", requestIndex), fmt.Sprintf("out of range (1-%d)", len(requests)))
		}
		request = requests[internalIndex]
		selectedIndex = internalIndex
		item := RequestItem{Request: request, Index: selectedIndex}
		fmt.Printf("\n%s\n\n", item.String())
		return request, nil
	}

	if len(requests) > 1 {
		selected, err := selectRequest(requests)
		if err != nil {
			return nil, err
		}
		fmt.Println() // Blank line after selection
		return selected, nil
	}

	return requests[0], nil
}

func printRequestWarnings(request *models.HttpRequest) {
	if len(request.Warnings) == 0 {
		return
	}

	for _, w := range request.Warnings {
		if useColors() {
			warnColor.Fprintf(os.Stderr, "Warning: %s\n", w)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
		}
	}
	fmt.Fprintln(os.Stderr)
}

func processSessionInputs(request *models.HttpRequest, filePath string) error {
	if noSession {
		return nil
	}

	sessionMgr, err := session.NewSessionManager("", filePath, sessionName)
	if err != nil {
		return errors.Wrap(err, "failed to create session manager")
	}
	if err := sessionMgr.Load(); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to load session: %v\n", err)
		}
	}

	inputPrompter := userinput.NewPrompter(nil, true, useColors())

	if inputPrompter.HasPatterns(request.URL) {
		result, err := inputPrompter.ProcessURL(request.URL)
		if err != nil {
			if errors.Is(err, errors.ErrCanceled) {
				return errors.ErrCanceled
			}
			return errors.Wrap(err, "failed to process user input variables in URL")
		}
		request.URL = result.URL
	}

	urlKey := inputPrompter.GenerateKey(request.URL)

	for k, v := range request.Headers {
		if inputPrompter.HasPatterns(v) {
			processedValue, err := inputPrompter.ProcessContent(v, urlKey)
			if err != nil {
				if errors.Is(err, errors.ErrCanceled) {
					return errors.ErrCanceled
				}
				return errors.Wrapf(err, "failed to process user input variables in header %s", k)
			}
			request.Headers[k] = processedValue
		}
	}

	if request.RawBody != "" && inputPrompter.HasPatterns(request.RawBody) {
		processedBody, err := inputPrompter.ProcessContent(request.RawBody, urlKey)
		if err != nil {
			if errors.Is(err, errors.ErrCanceled) {
				return errors.ErrCanceled
			}
			return errors.Wrap(err, "failed to process user input variables in body")
		}
		request.RawBody = processedBody
		request.Body = strings.NewReader(request.RawBody)
	}

	for i, part := range request.MultipartParts {
		if part.IsFile {
			if inputPrompter.HasPatterns(part.FilePath) {
				processedPath, err := inputPrompter.ProcessContent(part.FilePath, urlKey)
				if err != nil {
					if errors.Is(err, errors.ErrCanceled) {
						return errors.ErrCanceled
					}
					return errors.Wrapf(err, "failed to process user input variables in multipart file path %s", part.Name)
				}
				request.MultipartParts[i].FilePath = processedPath
			}
			continue
		}

		if inputPrompter.HasPatterns(part.Value) {
			processedValue, err := inputPrompter.ProcessContent(part.Value, urlKey)
			if err != nil {
				if errors.Is(err, errors.ErrCanceled) {
					return errors.ErrCanceled
				}
				return errors.Wrapf(err, "failed to process user input variables in multipart field %s", part.Name)
			}
			request.MultipartParts[i].Value = processedValue
		}
	}

	if err := sessionMgr.Save(); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
		}
	}

	return nil
}

func applyPromptVariables(request *models.HttpRequest, varProcessor *variables.VariableProcessor) error {
	for _, pv := range request.Metadata.Prompts {
		value, err := promptHandler(pv.Name, pv.Description, pv.IsPassword)
		if err != nil {
			return errors.Wrapf(err, "failed to get input for %s", pv.Name)
		}
		varProcessor.SetFileVariables(map[string]string{pv.Name: value})
	}
	return nil
}

func runPreRequestScript(request *models.HttpRequest, sessionCfg *session.SessionConfig, varProcessor *variables.VariableProcessor, envStore *session.EnvironmentStore) error {
	if request.Metadata.PreScript == "" {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	go func() {
		<-sigChan
		if verbose {
			fmt.Fprintln(os.Stderr, "\nInterrupt received, cancelling pre-script...")
		}
		cancel()
	}()

	result, err := executor.ExecutePreScriptWithContext(ctx, request.Metadata.PreScript, sessionCfg, request, varProcessor, envStore)
	if err != nil {
		if ctx.Err() == context.Canceled {
			return errors.Wrap(errors.ErrCanceled, "pre-script cancelled")
		}
		return err
	}

	for _, log := range result.Logs {
		fmt.Printf("[pre-script] %s\n", log)
	}

	return nil
}

func processRequestVariables(request *models.HttpRequest, varProcessor *variables.VariableProcessor) error {
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

	return nil
}

func validateRequest(request *models.HttpRequest) error {
	if skipValidate {
		return nil
	}

	validation := request.Validate()
	if validation.IsValid() {
		return nil
	}

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
