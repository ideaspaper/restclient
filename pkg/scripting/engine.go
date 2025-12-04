// Package scripting provides a JavaScript execution engine for pre-request
// and post-response scripts, with built-in utilities for assertions, crypto,
// and variable management.
package scripting

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/google/uuid"
	"github.com/ideaspaper/restclient/internal/httputil"
	"github.com/ideaspaper/restclient/pkg/errors"
)

// Engine executes JavaScript scripts for HTTP request/response handling
type Engine struct {
	vm      *goja.Runtime
	context *ScriptContext
}

// TestResult represents the result of a single test
type TestResult struct {
	Name   string
	Passed bool
	Error  string
}

// ScriptResult contains the results of script execution
type ScriptResult struct {
	Tests      []TestResult
	Logs       []string
	Error      error
	GlobalVars map[string]any
}

// NewEngine creates a new scripting engine
func NewEngine() *Engine {
	return &Engine{
		vm: goja.New(),
	}
}

// Execute runs a script with the given context
func (e *Engine) Execute(script string, ctx *ScriptContext) (*ScriptResult, error) {
	return e.ExecuteWithContext(context.Background(), script, ctx)
}

// ExecuteWithContext runs a script with the given context and supports cancellation.
// If the context is canceled or times out, the script execution will be interrupted.
func (e *Engine) ExecuteWithContext(ctx context.Context, script string, scriptCtx *ScriptContext) (*ScriptResult, error) {
	if strings.TrimSpace(script) == "" {
		return &ScriptResult{}, nil
	}

	// Check for context cancellation before starting
	select {
	case <-ctx.Done():
		return nil, errors.Wrapf(ctx.Err(), "script execution canceled")
	default:
	}

	e.context = scriptCtx
	result := &ScriptResult{
		Tests:      []TestResult{},
		Logs:       []string{},
		GlobalVars: make(map[string]any),
	}

	// Set up the runtime
	e.vm = goja.New()
	e.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	// Set up interrupt handler for context cancellation
	go func() {
		<-ctx.Done()
		e.vm.Interrupt("context canceled")
	}()

	// Register global objects
	if err := e.registerGlobals(scriptCtx, result); err != nil {
		return nil, errors.Wrap(err, "failed to register globals")
	}

	// Execute the script
	_, err := e.vm.RunString(script)
	if err != nil {
		// Check if this was due to context cancellation
		select {
		case <-ctx.Done():
			return nil, errors.Wrapf(ctx.Err(), "script execution canceled")
		default:
		}

		if exception, ok := err.(*goja.Exception); ok {
			result.Error = errors.NewScriptError("", exception.String())
		} else {
			result.Error = errors.NewScriptErrorWithCause("", "execution failed", err)
		}
	}

	return result, nil
}

// registerGlobals registers the client, request, and response objects
func (e *Engine) registerGlobals(ctx *ScriptContext, result *ScriptResult) error {
	// Create client object
	client := e.createClientObject(ctx, result)
	e.vm.Set("client", client)

	// Create response object (only for post-request scripts)
	if ctx.Response != nil {
		response := e.createResponseObject(ctx)
		e.vm.Set("response", response)
	}

	// Create request object
	request := e.createRequestObject(ctx)
	e.vm.Set("request", request)

	// Add console.log support
	console := map[string]any{
		"log": func(call goja.FunctionCall) goja.Value {
			args := make([]string, len(call.Arguments))
			for i, arg := range call.Arguments {
				args[i] = arg.String()
			}
			msg := strings.Join(args, " ")
			result.Logs = append(result.Logs, msg)
			return goja.Undefined()
		},
	}
	e.vm.Set("console", console)

	// Register utility functions
	e.registerUtilityFunctions()

	return nil
}

// createClientObject creates the client JavaScript object
func (e *Engine) createClientObject(ctx *ScriptContext, result *ScriptResult) map[string]any {
	global := &GlobalStorage{
		vars:    ctx.GlobalVars,
		headers: make(map[string]string),
	}

	return map[string]any{
		// client.test(name, func)
		"test": func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 2 {
				return goja.Undefined()
			}

			testName := call.Arguments[0].String()
			testFunc, ok := goja.AssertFunction(call.Arguments[1])
			if !ok {
				result.Tests = append(result.Tests, TestResult{
					Name:   testName,
					Passed: false,
					Error:  "second argument must be a function",
				})
				return goja.Undefined()
			}

			// Execute the test function
			_, err := testFunc(goja.Undefined())
			if err != nil {
				result.Tests = append(result.Tests, TestResult{
					Name:   testName,
					Passed: false,
					Error:  err.Error(),
				})
			} else {
				result.Tests = append(result.Tests, TestResult{
					Name:   testName,
					Passed: true,
				})
			}
			return goja.Undefined()
		},

		// client.assert(condition, message)
		"assert": func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 1 {
				panic(e.vm.ToValue("assert requires at least one argument"))
			}

			condition := call.Arguments[0].ToBoolean()
			if !condition {
				message := "Assertion failed"
				if len(call.Arguments) > 1 {
					message = call.Arguments[1].String()
				}
				panic(e.vm.ToValue(message))
			}
			return goja.Undefined()
		},

		// client.log(text)
		"log": func(call goja.FunctionCall) goja.Value {
			args := make([]string, len(call.Arguments))
			for i, arg := range call.Arguments {
				args[i] = arg.String()
			}
			msg := strings.Join(args, " ")
			result.Logs = append(result.Logs, msg)
			return goja.Undefined()
		},

		// client.global
		"global": map[string]any{
			// client.global.set(name, value)
			"set": func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 2 {
					return goja.Undefined()
				}
				name := call.Arguments[0].String()
				value := call.Arguments[1].Export()
				global.Set(name, value)
				result.GlobalVars[name] = value
				return goja.Undefined()
			},

			// client.global.get(name)
			"get": func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 1 {
					return goja.Undefined()
				}
				name := call.Arguments[0].String()
				value := global.Get(name)
				if value == nil {
					return goja.Undefined()
				}
				return e.vm.ToValue(value)
			},

			// client.global.isEmpty()
			"isEmpty": func(call goja.FunctionCall) goja.Value {
				return e.vm.ToValue(global.IsEmpty())
			},

			// client.global.clear(name)
			"clear": func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 1 {
					return goja.Undefined()
				}
				name := call.Arguments[0].String()
				global.Clear(name)
				delete(result.GlobalVars, name)
				return goja.Undefined()
			},

			// client.global.clearAll()
			"clearAll": func(call goja.FunctionCall) goja.Value {
				global.ClearAll()
				result.GlobalVars = make(map[string]any)
				return goja.Undefined()
			},
		},
	}
}

// createResponseObject creates the response JavaScript object
func (e *Engine) createResponseObject(ctx *ScriptContext) map[string]any {
	resp := ctx.Response

	// Parse body as JSON if possible
	var bodyObj any
	if err := json.Unmarshal([]byte(resp.Body), &bodyObj); err != nil {
		bodyObj = resp.Body // Use string if not valid JSON
	}

	contentType := resp.ContentType()

	return map[string]any{
		"status":     resp.StatusCode,
		"statusText": resp.StatusMessage,
		"body":       bodyObj,
		"contentType": map[string]any{
			"mimeType": getMimeType(contentType),
			"charset":  getCharset(contentType),
		},
		"headers": map[string]any{
			// response.headers.valueOf(name)
			"valueOf": func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 1 {
					return goja.Null()
				}
				name := call.Arguments[0].String()
				value := resp.GetHeader(name)
				if value == "" {
					return goja.Null()
				}
				return e.vm.ToValue(value)
			},
			// response.headers.valuesOf(name)
			"valuesOf": func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 1 {
					return e.vm.ToValue([]string{})
				}
				name := call.Arguments[0].String()
				for k, v := range resp.Headers {
					if strings.EqualFold(k, name) {
						return e.vm.ToValue(v)
					}
				}
				return e.vm.ToValue([]string{})
			},
		},
	}
}

// createRequestObject creates the request JavaScript object
func (e *Engine) createRequestObject(ctx *ScriptContext) map[string]any {
	req := ctx.Request

	// Build headers array
	var headersArray []map[string]any
	for name, value := range req.Headers {
		headersArray = append(headersArray, map[string]any{
			"name":  name,
			"value": value,
		})
	}

	return map[string]any{
		"method": req.Method,
		"url":    req.URL,
		"body":   req.RawBody,
		"headers": map[string]any{
			"all": headersArray,
			// request.headers.findByName(name)
			"findByName": func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 1 {
					return goja.Null()
				}
				name := call.Arguments[0].String()
				if val, ok := httputil.GetHeader(req.Headers, name); ok {
					return e.vm.ToValue(val)
				}
				return goja.Null()
			},
		},
		"environment": map[string]any{
			// request.environment.get(name)
			"get": func(call goja.FunctionCall) goja.Value {
				if len(call.Arguments) < 1 {
					return goja.Null()
				}
				name := call.Arguments[0].String()
				value := ctx.GetEnvVar(name)
				if value == "" {
					return goja.Null()
				}
				return e.vm.ToValue(value)
			},
		},
	}
}

// GlobalStorage manages global variables across requests
type GlobalStorage struct {
	vars    map[string]any
	headers map[string]string
}

// Set stores a variable
func (g *GlobalStorage) Set(name string, value any) {
	if g.vars == nil {
		g.vars = make(map[string]any)
	}
	g.vars[name] = value
}

// Get retrieves a variable
func (g *GlobalStorage) Get(name string) any {
	if g.vars == nil {
		return nil
	}
	return g.vars[name]
}

// IsEmpty checks if the storage is empty
func (g *GlobalStorage) IsEmpty() bool {
	return len(g.vars) == 0
}

// Clear removes a variable
func (g *GlobalStorage) Clear(name string) {
	if g.vars != nil {
		delete(g.vars, name)
	}
}

// ClearAll removes all variables
func (g *GlobalStorage) ClearAll() {
	g.vars = make(map[string]any)
}

// Helper functions
func getMimeType(contentType string) string {
	if contentType == "" {
		return ""
	}
	parts := strings.Split(contentType, ";")
	return strings.TrimSpace(parts[0])
}

func getCharset(contentType string) string {
	if contentType == "" {
		return ""
	}
	parts := strings.Split(contentType, ";")
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "charset=") {
			return strings.TrimPrefix(part, "charset=")
		}
	}
	return ""
}

// registerUtilityFunctions registers built-in utility functions
func (e *Engine) registerUtilityFunctions() {
	// $uuid - Generate a random UUID v4
	e.vm.Set("$uuid", func(call goja.FunctionCall) goja.Value {
		return e.vm.ToValue(uuid.New().String())
	})

	// $timestamp - Get current Unix timestamp in milliseconds
	e.vm.Set("$timestamp", func(call goja.FunctionCall) goja.Value {
		return e.vm.ToValue(time.Now().UnixMilli())
	})

	// $isoTimestamp - Get current ISO 8601 timestamp
	e.vm.Set("$isoTimestamp", func(call goja.FunctionCall) goja.Value {
		return e.vm.ToValue(time.Now().UTC().Format(time.RFC3339))
	})

	// $randomInt - Generate a random integer between min and max (inclusive)
	e.vm.Set("$randomInt", func(call goja.FunctionCall) goja.Value {
		min := 0
		max := 1000
		if len(call.Arguments) >= 1 {
			min = int(call.Arguments[0].ToInteger())
		}
		if len(call.Arguments) >= 2 {
			max = int(call.Arguments[1].ToInteger())
		}
		if min > max {
			min, max = max, min
		}
		return e.vm.ToValue(rand.Intn(max-min+1) + min)
	})

	// $base64 - Encode string to base64
	e.vm.Set("$base64", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		input := call.Arguments[0].String()
		encoded := base64.StdEncoding.EncodeToString([]byte(input))
		return e.vm.ToValue(encoded)
	})

	// $base64Decode - Decode base64 string
	e.vm.Set("$base64Decode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		input := call.Arguments[0].String()
		decoded, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			return goja.Undefined()
		}
		return e.vm.ToValue(string(decoded))
	})

	// $md5 - Generate MD5 hash
	e.vm.Set("$md5", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		input := call.Arguments[0].String()
		hash := md5.Sum([]byte(input))
		return e.vm.ToValue(hex.EncodeToString(hash[:]))
	})

	// $sha256 - Generate SHA256 hash
	e.vm.Set("$sha256", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		input := call.Arguments[0].String()
		hash := sha256.Sum256([]byte(input))
		return e.vm.ToValue(hex.EncodeToString(hash[:]))
	})

	// $sha512 - Generate SHA512 hash
	e.vm.Set("$sha512", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			return goja.Undefined()
		}
		input := call.Arguments[0].String()
		hash := sha512.Sum512([]byte(input))
		return e.vm.ToValue(hex.EncodeToString(hash[:]))
	})

	// $randomString - Generate a random alphanumeric string
	e.vm.Set("$randomString", func(call goja.FunctionCall) goja.Value {
		length := 16
		if len(call.Arguments) >= 1 {
			length = int(call.Arguments[0].ToInteger())
			if length <= 0 {
				length = 16
			}
		}
		const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		result := make([]byte, length)
		for i := range result {
			result[i] = charset[rand.Intn(len(charset))]
		}
		return e.vm.ToValue(string(result))
	})

	// $guid - Alias for $uuid
	e.vm.Set("$guid", func(call goja.FunctionCall) goja.Value {
		return e.vm.ToValue(uuid.New().String())
	})
}
