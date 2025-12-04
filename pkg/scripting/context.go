package scripting

import (
	"fmt"

	"github.com/ideaspaper/restclient/pkg/models"
)

// ScriptContext holds all the context needed for script execution
type ScriptContext struct {
	Request    *models.HttpRequest
	Response   *models.HttpResponse
	GlobalVars map[string]any
	EnvVars    map[string]string
}

// NewScriptContext creates a new script context
func NewScriptContext() *ScriptContext {
	return &ScriptContext{
		GlobalVars: make(map[string]any),
		EnvVars:    make(map[string]string),
	}
}

// SetRequest sets the request in the context
func (c *ScriptContext) SetRequest(req *models.HttpRequest) {
	c.Request = req
}

// SetResponse sets the response in the context
func (c *ScriptContext) SetResponse(resp *models.HttpResponse) {
	c.Response = resp
}

// SetGlobalVar sets a global variable
func (c *ScriptContext) SetGlobalVar(name string, value any) {
	if c.GlobalVars == nil {
		c.GlobalVars = make(map[string]any)
	}
	c.GlobalVars[name] = value
}

// GetGlobalVar gets a global variable
func (c *ScriptContext) GetGlobalVar(name string) any {
	if c.GlobalVars == nil {
		return nil
	}
	return c.GlobalVars[name]
}

// SetEnvVar sets an environment variable
func (c *ScriptContext) SetEnvVar(name, value string) {
	if c.EnvVars == nil {
		c.EnvVars = make(map[string]string)
	}
	c.EnvVars[name] = value
}

// GetEnvVar gets an environment variable
func (c *ScriptContext) GetEnvVar(name string) string {
	if c.EnvVars == nil {
		return ""
	}
	return c.EnvVars[name]
}

// MergeGlobalVars merges the given global vars into the context
func (c *ScriptContext) MergeGlobalVars(vars map[string]any) {
	if c.GlobalVars == nil {
		c.GlobalVars = make(map[string]any)
	}
	for k, v := range vars {
		c.GlobalVars[k] = v
	}
}

// GetGlobalVarAsString gets a global variable as a string
func (c *ScriptContext) GetGlobalVarAsString(name string) string {
	val := c.GetGlobalVar(name)
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
