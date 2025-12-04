// Package executor provides request execution functionality.
// It handles the core logic of sending HTTP requests with session management,
// scripting support, and history tracking.
package executor

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/ideaspaper/restclient/pkg/client"
	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/history"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/scripting"
	"github.com/ideaspaper/restclient/pkg/session"
	"github.com/ideaspaper/restclient/pkg/variables"
)

// Options configures request execution behavior
type Options struct {
	// HTTPFilePath is the path to the .http file (used for session scoping)
	HTTPFilePath string
	// SessionName is an optional named session override
	SessionName string
	// NoSession disables session management (cookies and variables)
	NoSession bool
	// NoHistory disables saving to history
	NoHistory bool
	// Verbose enables verbose output
	Verbose bool
	// LogFunc is called with log messages (optional)
	LogFunc func(format string, args ...any)
}

// Result contains the execution result
type Result struct {
	Response    *models.HttpResponse
	TestResults []scripting.TestResult
	Logs        []string
}

// Executor handles HTTP request execution
type Executor struct {
	config       *config.Config
	varProcessor *variables.VariableProcessor
	options      Options
}

// New creates a new Executor
func New(cfg *config.Config, varProcessor *variables.VariableProcessor, opts Options) *Executor {
	return &Executor{
		config:       cfg,
		varProcessor: varProcessor,
		options:      opts,
	}
}

// Execute sends an HTTP request with full session and scripting support.
// It uses context.Background(). For cancellation support, use ExecuteWithContext.
func (e *Executor) Execute(request *models.HttpRequest) (*Result, error) {
	return e.ExecuteWithContext(context.Background(), request)
}

// ExecuteWithContext sends an HTTP request with context support for cancellation and timeouts.
func (e *Executor) ExecuteWithContext(ctx context.Context, request *models.HttpRequest) (*Result, error) {
	result := &Result{}

	var sessionMgr *session.SessionManager
	if !e.options.NoSession && e.config.RememberCookies {
		var err error
		sessionMgr, err = session.NewSessionManager("", e.options.HTTPFilePath, e.options.SessionName)
		if err != nil {
			e.log("Warning: failed to initialize session: %v", err)
		} else {
			if err := sessionMgr.Load(); err != nil {
				e.log("Warning: failed to load session: %v", err)
			}

			// Load session variables into processor
			for name, value := range sessionMgr.GetAllVariables() {
				if strVal, ok := value.(string); ok {
					e.varProcessor.SetFileVariables(map[string]string{name: strVal})
				} else {
					e.varProcessor.SetFileVariables(map[string]string{name: fmt.Sprintf("%v", value)})
				}
			}
		}
	}

	// Create HTTP client
	clientCfg := e.config.ToClientConfig()
	if request.Metadata.NoRedirect {
		clientCfg.FollowRedirects = false
	}

	httpClient, err := client.NewHttpClient(clientCfg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP client")
	}

	// Load cookies from session
	if sessionMgr != nil && !request.Metadata.NoCookieJar {
		cookies := sessionMgr.GetCookiesForURL(request.URL)
		if len(cookies) > 0 {
			httpClient.SetCookies(request.URL, cookies)
		}
	}

	// Send request with context
	resp, err := httpClient.SendWithContext(ctx, request)
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, errors.Wrap(ctx.Err(), "request cancelled")
		}
		return nil, errors.NewRequestErrorWithURL("send", request.Method, request.URL, err)
	}
	result.Response = resp

	// Save cookies to session
	if sessionMgr != nil && !request.Metadata.NoCookieJar {
		responseCookies := httpClient.GetCookies(request.URL)
		if len(responseCookies) > 0 {
			sessionMgr.SetCookiesFromResponse(request.URL, responseCookies)
		}
	}

	// Save to history
	if !e.options.NoHistory {
		e.saveToHistory(request, sessionMgr)
	}

	// Store result for request variable references
	if request.Name != "" || request.Metadata.Name != "" {
		name := request.Name
		if name == "" {
			name = request.Metadata.Name
		}
		e.varProcessor.SetRequestResult(name, variables.RequestResult{
			StatusCode: resp.StatusCode,
			Headers:    resp.Headers,
			Body:       resp.Body,
		})
	}

	// Execute post-response script with context
	if request.Metadata.PostScript != "" {
		scriptResult, err := e.executePostScript(ctx, request, resp, sessionMgr)
		if err != nil {
			return result, err
		}
		result.Logs = scriptResult.Logs
		result.TestResults = scriptResult.Tests

		// Check for test failures
		for _, test := range scriptResult.Tests {
			if !test.Passed {
				return result, errors.NewScriptError("test", fmt.Sprintf("'%s' failed: %s", test.Name, test.Error))
			}
		}
	}

	// Save session
	if sessionMgr != nil {
		if err := sessionMgr.Save(); err != nil {
			e.log("Warning: failed to save session: %v", err)
		}
	}

	return result, nil
}

// executePostScript runs the post-response script
func (e *Executor) executePostScript(ctx context.Context, request *models.HttpRequest, resp *models.HttpResponse, sessionMgr *session.SessionManager) (*scripting.ScriptResult, error) {
	scriptCtx := e.setupScriptContext(request, resp)

	// Load session variables into script context
	if sessionMgr != nil {
		for name, value := range sessionMgr.GetAllVariables() {
			scriptCtx.SetGlobalVar(name, value)
		}
	}

	engine := scripting.NewEngine()
	result, err := engine.ExecuteWithContext(ctx, request.Metadata.PostScript, scriptCtx)
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, errors.Wrap(ctx.Err(), "script execution cancelled")
		}
		return nil, errors.NewScriptErrorWithCause("post-response", "script error", err)
	}

	// Apply global vars to processor
	ApplyScriptGlobalVars(e.varProcessor, result.GlobalVars)

	// Save to session
	if sessionMgr != nil {
		for name, value := range result.GlobalVars {
			sessionMgr.SetVariable(name, value)
		}
	}

	if result.Error != nil {
		return result, errors.NewScriptErrorWithCause("post-response", "script failed", result.Error)
	}

	return result, nil
}

// setupScriptContext creates a script context with environment variables
func (e *Executor) setupScriptContext(request *models.HttpRequest, resp *models.HttpResponse) *scripting.ScriptContext {
	scriptCtx := scripting.NewScriptContext()
	scriptCtx.SetRequest(request)

	if resp != nil {
		scriptCtx.SetResponse(resp)
	}

	if e.config.EnvironmentVariables != nil {
		if shared, ok := e.config.EnvironmentVariables["$shared"]; ok {
			for k, v := range shared {
				scriptCtx.SetEnvVar(k, v)
			}
		}
		if current, ok := e.config.EnvironmentVariables[e.config.CurrentEnvironment]; ok {
			for k, v := range current {
				scriptCtx.SetEnvVar(k, v)
			}
		}
	}

	return scriptCtx
}

// saveToHistory saves the request to history
func (e *Executor) saveToHistory(request *models.HttpRequest, sessionMgr *session.SessionManager) {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return
	}

	// Create a copy of request with actual Cookie header that was sent
	historyRequest := *request
	historyRequest.Headers = make(map[string]string)
	maps.Copy(historyRequest.Headers, request.Headers)

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

// log outputs a log message if LogFunc is configured
func (e *Executor) log(format string, args ...any) {
	if e.options.Verbose && e.options.LogFunc != nil {
		e.options.LogFunc(format, args...)
	}
}

// ApplyScriptGlobalVars applies global variables from script result to variable processor
func ApplyScriptGlobalVars(varProcessor *variables.VariableProcessor, globalVars map[string]any) {
	for k, v := range globalVars {
		if strVal, ok := v.(string); ok {
			varProcessor.SetFileVariables(map[string]string{k: strVal})
		} else {
			varProcessor.SetFileVariables(map[string]string{k: fmt.Sprintf("%v", v)})
		}
	}
}

// ExecutePreScript executes a pre-request script.
// It uses context.Background(). For cancellation support, use ExecutePreScriptWithContext.
func ExecutePreScript(script string, cfg *config.Config, request *models.HttpRequest, varProcessor *variables.VariableProcessor) (*scripting.ScriptResult, error) {
	return ExecutePreScriptWithContext(context.Background(), script, cfg, request, varProcessor)
}

// ExecutePreScriptWithContext executes a pre-request script with context support.
func ExecutePreScriptWithContext(ctx context.Context, script string, cfg *config.Config, request *models.HttpRequest, varProcessor *variables.VariableProcessor) (*scripting.ScriptResult, error) {
	scriptCtx := scripting.NewScriptContext()
	scriptCtx.SetRequest(request)

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

	engine := scripting.NewEngine()
	result, err := engine.ExecuteWithContext(ctx, script, scriptCtx)
	if err != nil {
		// Check for context cancellation
		if ctx.Err() != nil {
			return nil, errors.Wrap(ctx.Err(), "script execution cancelled")
		}
		return nil, errors.NewScriptErrorWithCause("pre-request", "script error", err)
	}

	if result.Error != nil {
		return nil, errors.NewScriptErrorWithCause("pre-request", "script failed", result.Error)
	}

	ApplyScriptGlobalVars(varProcessor, result.GlobalVars)

	return result, nil
}
