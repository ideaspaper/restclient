package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/lastfile"
	"github.com/ideaspaper/restclient/pkg/session"
)

var (
	envSessionName string
	envDirPath     string
)

// envCmd represents the env command
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments and variables",
	Long: `Manage environments and variables.

Environments allow you to define variables that can be used in your requests.
Variables are referenced using {{variableName}} syntax.

Environment variables are stored per-session in the session's environments.json file.
Sessions are scoped by directory (based on the .http file location) or by an explicit --session name.

Examples:
  # List all environments (uses last file's session)
  restclient env list

  # Show current environment
  restclient env current

  # Switch to an environment
  restclient env use production

  # Show variables in an environment
  restclient env show production

  # Set a variable
  restclient env set production API_URL https://api.example.com

  # Create a new environment
  restclient env create staging

  # Delete an environment
  restclient env delete staging
  
  # Use a specific named session
  restclient env list --session my-api
  
  # Use a specific directory's session
  restclient env list --dir /path/to/project`,
}

// envListCmd lists all environments
var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environments",
	RunE:  runEnvList,
}

// envCurrentCmd shows the current environment
var envCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show current environment",
	RunE:  runEnvCurrent,
}

// envUseCmd switches to an environment
var envUseCmd = &cobra.Command{
	Use:   "use <environment>",
	Short: "Switch to an environment",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvUse,
}

// envShowCmd shows variables in an environment
var envShowCmd = &cobra.Command{
	Use:   "show [environment]",
	Short: "Show variables in an environment",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runEnvShow,
}

// envSetCmd sets a variable in an environment
var envSetCmd = &cobra.Command{
	Use:   "set <environment> <variable> <value>",
	Short: "Set a variable in an environment",
	Args:  cobra.ExactArgs(3),
	RunE:  runEnvSet,
}

// envUnsetCmd removes a variable from an environment
var envUnsetCmd = &cobra.Command{
	Use:   "unset <environment> <variable>",
	Short: "Remove a variable from an environment",
	Args:  cobra.ExactArgs(2),
	RunE:  runEnvUnset,
}

// envCreateCmd creates a new environment
var envCreateCmd = &cobra.Command{
	Use:   "create <environment>",
	Short: "Create a new environment",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvCreate,
}

// envDeleteCmd deletes an environment
var envDeleteCmd = &cobra.Command{
	Use:   "delete <environment>",
	Short: "Delete an environment",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvDelete,
}

func init() {
	rootCmd.AddCommand(envCmd)

	// Add session/dir flags to all env subcommands
	for _, cmd := range []*cobra.Command{envListCmd, envCurrentCmd, envUseCmd, envShowCmd, envSetCmd, envUnsetCmd, envCreateCmd, envDeleteCmd} {
		cmd.Flags().StringVar(&envSessionName, "session", "", "use named session")
		cmd.Flags().StringVar(&envDirPath, "dir", "", "use session for specific directory")
	}

	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envCurrentCmd)
	envCmd.AddCommand(envUseCmd)
	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envSetCmd)
	envCmd.AddCommand(envUnsetCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envDeleteCmd)
}

// getSessionContext returns the session path and config/store for the current context
func getSessionContext() (string, *session.SessionConfig, *session.EnvironmentStore, error) {
	var httpFilePath string

	if envDirPath != "" {
		// Use the provided directory path - create a fake file path to get the session
		httpFilePath = envDirPath + "/dummy.http"
	} else if envSessionName == "" {
		// Try to use the last file
		lastPath, err := lastfile.Load()
		if err != nil || lastPath == "" {
			// Fall back to current directory
			cwd, err := os.Getwd()
			if err != nil {
				return "", nil, nil, errors.Wrap(err, "failed to get current directory")
			}
			httpFilePath = cwd + "/dummy.http"
		} else {
			httpFilePath = lastPath
		}
	}

	// Create session manager to get the session path
	sessionMgr, err := session.NewSessionManager("", httpFilePath, envSessionName)
	if err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to create session manager")
	}
	sessionPath := sessionMgr.GetSessionPath()

	// Load or create session config
	sessionCfg, err := session.LoadOrCreateSessionConfig(filesystem.Default, sessionPath)
	if err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to load session config")
	}

	// Load or create environment store
	envStore, err := session.LoadOrCreateEnvironmentStore(filesystem.Default, sessionPath)
	if err != nil {
		return "", nil, nil, errors.Wrap(err, "failed to load session environments")
	}

	return sessionPath, sessionCfg, envStore, nil
}

func runEnvList(cmd *cobra.Command, args []string) error {
	sessionPath, sessionCfg, envStore, err := getSessionContext()
	if err != nil {
		return err
	}

	envs := envStore.ListEnvironments()

	printHeader("Available Environments:")
	fmt.Printf("  (session: %s)\n", sessionPath)
	fmt.Println()

	if len(envs) == 0 {
		fmt.Println("  No environments configured")
		fmt.Println()
		fmt.Println("  Create one with: restclient env create <name>")
		return nil
	}

	sort.Strings(envs)

	for _, env := range envs {
		marker := printMarker(env == sessionCfg.Environment.Current)
		fmt.Printf("%s%s\n", marker, env)
	}

	// Show $shared info
	if shared, ok := envStore.GetVariables("$shared"); ok && len(shared) > 0 {
		fmt.Println()
		fmt.Printf("  $shared: %d variables (available in all environments)\n", len(shared))
	}

	return nil
}

func runEnvCurrent(cmd *cobra.Command, args []string) error {
	sessionPath, sessionCfg, _, err := getSessionContext()
	if err != nil {
		return err
	}

	fmt.Printf("Session: %s\n", sessionPath)

	if sessionCfg.Environment.Current == "" {
		fmt.Println("No environment selected")
		fmt.Println()
		fmt.Println("Use: restclient env use <environment>")
	} else {
		fmt.Printf("Current environment: %s\n", sessionCfg.Environment.Current)
	}

	return nil
}

func runEnvUse(cmd *cobra.Command, args []string) error {
	envName := args[0]

	sessionPath, sessionCfg, envStore, err := getSessionContext()
	if err != nil {
		return err
	}

	// Validate that the environment exists
	if !envStore.HasEnvironment(envName) && envName != "$shared" {
		return errors.NewValidationErrorWithValue("environment", envName, "not found")
	}

	sessionCfg.Environment.Current = envName
	if err := session.SaveSessionConfig(filesystem.Default, sessionPath, sessionCfg); err != nil {
		return errors.Wrap(err, "failed to save session config")
	}

	fmt.Printf("Switched to environment: %s\n", envName)
	return nil
}

func runEnvShow(cmd *cobra.Command, args []string) error {
	_, sessionCfg, envStore, err := getSessionContext()
	if err != nil {
		return err
	}

	envName := sessionCfg.Environment.Current
	if len(args) > 0 {
		envName = args[0]
	}

	if envName == "" {
		// Show $shared
		envName = "$shared"
	}

	vars, ok := envStore.GetVariables(envName)
	if !ok && envName != "$shared" {
		return errors.NewValidationErrorWithValue("environment", envName, "environment not found")
	}

	// Show $shared first if showing a specific environment
	if envName != "$shared" {
		if shared, ok := envStore.GetVariables("$shared"); ok && len(shared) > 0 {
			printHeader("$shared variables:")
			printVariables(shared)
			fmt.Println()
		}
	}

	printHeader(fmt.Sprintf("Variables in '%s':", envName))
	if len(vars) == 0 {
		fmt.Println("  (no variables)")
	} else {
		printVariables(vars)
	}

	return nil
}

func runEnvSet(cmd *cobra.Command, args []string) error {
	envName := args[0]
	varName := args[1]
	varValue := args[2]

	sessionPath, _, envStore, err := getSessionContext()
	if err != nil {
		return err
	}

	if err := envStore.SetVariable(envName, varName, varValue); err != nil {
		return err
	}

	if err := session.SaveEnvironmentStore(filesystem.Default, sessionPath, envStore); err != nil {
		return errors.Wrap(err, "failed to save session environments")
	}

	fmt.Printf("Set %s=%s in environment '%s'\n", varName, maskValue(varValue), envName)
	return nil
}

func runEnvUnset(cmd *cobra.Command, args []string) error {
	envName := args[0]
	varName := args[1]

	sessionPath, _, envStore, err := getSessionContext()
	if err != nil {
		return err
	}

	if err := envStore.UnsetVariable(envName, varName); err != nil {
		return err
	}

	if err := session.SaveEnvironmentStore(filesystem.Default, sessionPath, envStore); err != nil {
		return errors.Wrap(err, "failed to save session environments")
	}

	fmt.Printf("Removed %s from environment '%s'\n", varName, envName)
	return nil
}

func runEnvCreate(cmd *cobra.Command, args []string) error {
	envName := args[0]

	sessionPath, _, envStore, err := getSessionContext()
	if err != nil {
		return err
	}

	if envStore.HasEnvironment(envName) {
		return errors.NewValidationErrorWithValue("environment", envName, "environment already exists")
	}

	if err := envStore.AddEnvironment(envName, nil); err != nil {
		return err
	}

	if err := session.SaveEnvironmentStore(filesystem.Default, sessionPath, envStore); err != nil {
		return errors.Wrap(err, "failed to save session environments")
	}

	fmt.Printf("Created environment: %s\n", envName)
	fmt.Println()
	fmt.Printf("To use it: restclient env use %s\n", envName)
	return nil
}

func runEnvDelete(cmd *cobra.Command, args []string) error {
	envName := args[0]

	sessionPath, sessionCfg, envStore, err := getSessionContext()
	if err != nil {
		return err
	}

	if err := envStore.RemoveEnvironment(envName); err != nil {
		return err
	}

	if err := session.SaveEnvironmentStore(filesystem.Default, sessionPath, envStore); err != nil {
		return errors.Wrap(err, "failed to save session environments")
	}

	// Clear current environment if it was the deleted one
	if sessionCfg.Environment.Current == envName {
		sessionCfg.Environment.Current = ""
		if err := session.SaveSessionConfig(filesystem.Default, sessionPath, sessionCfg); err != nil {
			return errors.Wrap(err, "failed to save session config")
		}
	}

	fmt.Printf("Deleted environment: %s\n", envName)
	return nil
}

func printVariables(vars map[string]string) {
	var names []string
	for name := range vars {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := vars[name]
		displayValue := maskValueByName(name, value)
		printKeyValue(name, displayValue)
	}
}

func maskValue(value string) string {
	if len(value) > 50 {
		return value[:47] + "..."
	}

	return value
}

// maskValueByName masks the value if the variable name suggests it's sensitive
func maskValueByName(name, value string) string {
	lowerName := strings.ToLower(name)
	if strings.Contains(lowerName, "password") ||
		strings.Contains(lowerName, "secret") ||
		strings.Contains(lowerName, "token") ||
		strings.Contains(lowerName, "key") ||
		strings.Contains(lowerName, "api_key") ||
		strings.Contains(lowerName, "apikey") ||
		strings.Contains(lowerName, "credential") ||
		strings.Contains(lowerName, "auth") {
		if len(value) > 4 {
			return value[:4] + strings.Repeat("*", len(value)-4)
		}
		return strings.Repeat("*", len(value))
	}

	return maskValue(value)
}
