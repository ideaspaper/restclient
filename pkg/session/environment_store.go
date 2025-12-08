package session

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"sort"

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/pkg/errors"
)

const (
	environmentStoreFileName = "environments.json"
	environmentStoreVersion  = 1
)

// EnvironmentStore manages per-session environment variables.
type EnvironmentStore struct {
	Version              int                          `json:"version"`
	EnvironmentVariables map[string]map[string]string `json:"environmentVariables"`
}

// LoadOrCreateEnvironmentStore loads an existing environment store or recreates it with defaults.
func LoadOrCreateEnvironmentStore(fs filesystem.FileSystem, sessionDir string) (*EnvironmentStore, error) {
	if fs == nil {
		fs = filesystem.Default
	}
	if sessionDir == "" {
		return nil, errors.NewValidationError("sessionDir", "session directory is required")
	}

	if err := fs.MkdirAll(sessionDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to ensure session directory")
	}

	path := filepath.Join(sessionDir, environmentStoreFileName)
	data, err := fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			store := defaultEnvironmentStore()
			if err := writeEnvironmentStore(fs, path, store); err != nil {
				return nil, err
			}
			return store, nil
		}
		return nil, errors.Wrap(err, "failed to read session environments")
	}

	store := &EnvironmentStore{}
	if err := json.Unmarshal(data, store); err != nil {
		store = defaultEnvironmentStore()
		if writeErr := writeEnvironmentStore(fs, path, store); writeErr != nil {
			return nil, writeErr
		}
		return store, nil
	}

	store.ensureIntegrity()
	if store.Version != environmentStoreVersion {
		store.Version = environmentStoreVersion
		if err := writeEnvironmentStore(fs, path, store); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// SaveEnvironmentStore persists the environment store to disk.
func SaveEnvironmentStore(fs filesystem.FileSystem, sessionDir string, store *EnvironmentStore) error {
	if fs == nil {
		fs = filesystem.Default
	}
	if sessionDir == "" {
		return errors.NewValidationError("sessionDir", "session directory is required")
	}
	if store == nil {
		return errors.NewValidationError("store", "environment store is required")
	}

	store.ensureIntegrity()
	store.Version = environmentStoreVersion

	if err := fs.MkdirAll(sessionDir, 0755); err != nil {
		return errors.Wrap(err, "failed to ensure session directory")
	}

	path := filepath.Join(sessionDir, environmentStoreFileName)
	return writeEnvironmentStore(fs, path, store)
}

// ListEnvironments returns all named environments excluding $shared.
func (s *EnvironmentStore) ListEnvironments() []string {
	var envs []string
	for name := range s.EnvironmentVariables {
		if name != "$shared" {
			envs = append(envs, name)
		}
	}
	sort.Strings(envs)
	return envs
}

// HasEnvironment reports whether the environment exists.
func (s *EnvironmentStore) HasEnvironment(name string) bool {
	_, ok := s.EnvironmentVariables[name]
	return ok
}

// AddEnvironment adds a new named environment.
func (s *EnvironmentStore) AddEnvironment(name string, vars map[string]string) error {
	if name == "" {
		return errors.NewValidationError("environment", "name is required")
	}
	if name == "$shared" {
		return errors.NewValidationErrorWithValue("environment", name, "cannot use reserved name")
	}
	if vars == nil {
		vars = make(map[string]string)
	}
	s.EnvironmentVariables[name] = vars
	return nil
}

// RemoveEnvironment removes a named environment.
func (s *EnvironmentStore) RemoveEnvironment(name string) error {
	if name == "$shared" {
		return errors.NewValidationErrorWithValue("environment", name, "cannot remove shared environment")
	}
	if _, ok := s.EnvironmentVariables[name]; !ok {
		return errors.NewValidationErrorWithValue("environment", name, "not found")
	}
	delete(s.EnvironmentVariables, name)
	return nil
}

// SetVariable sets a variable within an environment.
func (s *EnvironmentStore) SetVariable(env, name, value string) error {
	envMap, ok := s.EnvironmentVariables[env]
	if !ok {
		return errors.NewValidationErrorWithValue("environment", env, "not found")
	}
	envMap[name] = value
	return nil
}

// UnsetVariable removes a variable from an environment.
func (s *EnvironmentStore) UnsetVariable(env, name string) error {
	envMap, ok := s.EnvironmentVariables[env]
	if !ok {
		return errors.NewValidationErrorWithValue("environment", env, "not found")
	}
	if _, exists := envMap[name]; !exists {
		return errors.NewValidationErrorWithValue("variable", name, "not found in environment")
	}
	delete(envMap, name)
	return nil
}

// GetVariables returns the raw variables for an environment.
func (s *EnvironmentStore) GetVariables(env string) (map[string]string, bool) {
	vars, ok := s.EnvironmentVariables[env]
	return vars, ok
}

// GetVariable returns a specific variable value.
func (s *EnvironmentStore) GetVariable(env, name string) (string, bool) {
	if envVars, ok := s.EnvironmentVariables[env]; ok {
		val, exists := envVars[name]
		return val, exists
	}
	return "", false
}

// GetEnvironment returns the merged environment map combining $shared and the named environment.
func (s *EnvironmentStore) GetEnvironment(env string) map[string]string {
	result := make(map[string]string)
	if shared, ok := s.EnvironmentVariables["$shared"]; ok {
		maps.Copy(result, shared)
	}
	if env != "" && env != "$shared" {
		if envVars, ok := s.EnvironmentVariables[env]; ok {
			maps.Copy(result, envVars)
		}
	}
	return result
}

func (s *EnvironmentStore) ensureIntegrity() {
	if s.EnvironmentVariables == nil {
		s.EnvironmentVariables = make(map[string]map[string]string)
	}
	if _, ok := s.EnvironmentVariables["$shared"]; !ok {
		s.EnvironmentVariables["$shared"] = make(map[string]string)
	}
	if _, ok := s.EnvironmentVariables["development"]; !ok {
		s.EnvironmentVariables["development"] = make(map[string]string)
	}
}

func defaultEnvironmentStore() *EnvironmentStore {
	return &EnvironmentStore{
		Version: environmentStoreVersion,
		EnvironmentVariables: map[string]map[string]string{
			"$shared":     {},
			"development": {},
		},
	}
}

func writeEnvironmentStore(fs filesystem.FileSystem, path string, store *EnvironmentStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal environment store")
	}
	if err := fs.WriteFile(path, data, 0600); err != nil {
		return errors.Wrap(err, "failed to write session environments")
	}
	return nil
}
