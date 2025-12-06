// Package secrets provides secure storage for environment variables and secrets.
// This file is stored separately from config.json and should be gitignored.
package secrets

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"

	"github.com/ideaspaper/restclient/internal/paths"
	"github.com/ideaspaper/restclient/pkg/errors"
)

const (
	secretsFileName = "secrets.json"
)

// Store manages environment variables in a separate secrets file
type Store struct {
	// EnvironmentVariables maps environment names to their variables
	EnvironmentVariables map[string]map[string]string `json:"environmentVariables"`

	// path is the file path for the secrets file
	path string `json:"-"`
}

// NewStore creates a new Store instance
func NewStore() *Store {
	return &Store{
		EnvironmentVariables: map[string]map[string]string{
			"$shared": {},
		},
	}
}

// Load loads secrets from the default path
func Load() (*Store, error) {
	secretsDir, err := paths.AppDataDir("")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get secrets directory")
	}

	return LoadFromDir(secretsDir)
}

// LoadFromDir loads secrets from a specific directory
func LoadFromDir(dir string) (*Store, error) {
	secretsPath := filepath.Join(dir, secretsFileName)

	store := NewStore()
	store.path = secretsPath

	// If file doesn't exist, return empty store
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return store, nil
	}

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read secrets file")
	}

	if err := json.Unmarshal(data, store); err != nil {
		return nil, errors.Wrap(err, "failed to parse secrets file")
	}

	// Ensure maps are initialized
	if store.EnvironmentVariables == nil {
		store.EnvironmentVariables = make(map[string]map[string]string)
	}
	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		store.EnvironmentVariables["$shared"] = make(map[string]string)
	}

	return store, nil
}

// LoadFromFile loads secrets from a specific file path
func LoadFromFile(filePath string) (*Store, error) {
	store := NewStore()
	store.path = filePath

	// If file doesn't exist, return empty store
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return store, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read secrets file")
	}

	if err := json.Unmarshal(data, store); err != nil {
		return nil, errors.Wrap(err, "failed to parse secrets file")
	}

	// Ensure maps are initialized
	if store.EnvironmentVariables == nil {
		store.EnvironmentVariables = make(map[string]map[string]string)
	}
	if _, ok := store.EnvironmentVariables["$shared"]; !ok {
		store.EnvironmentVariables["$shared"] = make(map[string]string)
	}

	return store, nil
}

// Save saves the secrets to the file
func (s *Store) Save() error {
	if s.path == "" {
		secretsDir, err := paths.AppDataDir("")
		if err != nil {
			return errors.Wrap(err, "failed to get secrets directory")
		}
		s.path = filepath.Join(secretsDir, secretsFileName)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return errors.Wrap(err, "failed to create secrets directory")
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal secrets")
	}

	// Write with restrictive permissions (owner read/write only)
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return errors.Wrap(err, "failed to write secrets file")
	}

	return nil
}

// GetEnvironment returns the combined environment variables for the given environment name.
// It merges $shared variables with the specified environment, with environment values taking precedence.
func (s *Store) GetEnvironment(envName string) map[string]string {
	result := make(map[string]string)

	// Copy shared environment first
	if shared, ok := s.EnvironmentVariables["$shared"]; ok {
		maps.Copy(result, shared)
	}

	// Overlay specified environment
	if envName != "" && envName != "$shared" {
		if env, ok := s.EnvironmentVariables[envName]; ok {
			maps.Copy(result, env)
		}
	}

	return result
}

// ListEnvironments returns a list of available environment names (excluding $shared)
func (s *Store) ListEnvironments() []string {
	var envs []string
	for name := range s.EnvironmentVariables {
		if name != "$shared" {
			envs = append(envs, name)
		}
	}
	return envs
}

// HasEnvironment checks if an environment exists
func (s *Store) HasEnvironment(name string) bool {
	_, ok := s.EnvironmentVariables[name]
	return ok
}

// AddEnvironment adds a new environment
func (s *Store) AddEnvironment(name string, vars map[string]string) error {
	if name == "$shared" {
		return errors.NewValidationErrorWithValue("environment", "$shared", "cannot use reserved name")
	}
	if vars == nil {
		vars = make(map[string]string)
	}
	s.EnvironmentVariables[name] = vars
	return nil
}

// RemoveEnvironment removes an environment
func (s *Store) RemoveEnvironment(name string) error {
	if name == "$shared" {
		return errors.NewValidationErrorWithValue("environment", "$shared", "cannot remove shared environment")
	}
	if _, ok := s.EnvironmentVariables[name]; !ok {
		return errors.NewValidationErrorWithValue("environment", name, "not found")
	}
	delete(s.EnvironmentVariables, name)
	return nil
}

// SetVariable sets a variable in an environment
func (s *Store) SetVariable(env, name, value string) error {
	if _, ok := s.EnvironmentVariables[env]; !ok {
		return errors.NewValidationErrorWithValue("environment", env, "not found")
	}
	s.EnvironmentVariables[env][name] = value
	return nil
}

// GetVariable gets a variable from a specific environment
func (s *Store) GetVariable(env, name string) (string, bool) {
	if envVars, ok := s.EnvironmentVariables[env]; ok {
		val, exists := envVars[name]
		return val, exists
	}
	return "", false
}

// UnsetVariable removes a variable from an environment
func (s *Store) UnsetVariable(env, name string) error {
	if envVars, ok := s.EnvironmentVariables[env]; !ok {
		return errors.NewValidationErrorWithValue("environment", env, "not found")
	} else if _, exists := envVars[name]; !exists {
		return errors.NewValidationErrorWithValue("variable", name, "not found in environment")
	}
	delete(s.EnvironmentVariables[env], name)
	return nil
}

// GetVariables returns all variables for a specific environment
func (s *Store) GetVariables(env string) (map[string]string, bool) {
	vars, ok := s.EnvironmentVariables[env]
	return vars, ok
}

// Path returns the current secrets file path
func (s *Store) Path() string {
	return s.path
}

// DefaultSecretsPath returns the path to the default secrets file
func DefaultSecretsPath() (string, error) {
	dir, err := paths.AppDataDir("")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, secretsFileName), nil
}
