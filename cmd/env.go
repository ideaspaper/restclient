package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/secrets"
)

// envCmd represents the env command
var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments and variables",
	Long: `Manage environments and variables.

Environments allow you to define variables that can be used in your requests.
Variables are referenced using {{variableName}} syntax.

Environment variables are stored in a separate secrets file (~/.restclient/secrets.json)
that should NOT be committed to version control.

Examples:
  # List all environments
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
  restclient env delete staging`,
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

	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envCurrentCmd)
	envCmd.AddCommand(envUseCmd)
	envCmd.AddCommand(envShowCmd)
	envCmd.AddCommand(envSetCmd)
	envCmd.AddCommand(envUnsetCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envDeleteCmd)
}

func runEnvList(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	store, err := secrets.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load secrets")
	}

	envs := store.ListEnvironments()

	printHeader("Available Environments:")
	fmt.Println()

	if len(envs) == 0 {
		fmt.Println("  No environments configured")
		fmt.Println()
		fmt.Println("  Create one with: restclient env create <name>")
		return nil
	}

	sort.Strings(envs)

	for _, env := range envs {
		marker := printMarker(env == cfg.CurrentEnvironment)
		fmt.Printf("%s%s\n", marker, env)
	}

	// Show $shared info
	if shared, ok := store.GetVariables("$shared"); ok && len(shared) > 0 {
		fmt.Println()
		fmt.Printf("  $shared: %d variables (available in all environments)\n", len(shared))
	}

	return nil
}

func runEnvCurrent(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	if cfg.CurrentEnvironment == "" {
		fmt.Println("No environment selected")
		fmt.Println()
		fmt.Println("Use: restclient env use <environment>")
	} else {
		fmt.Printf("Current environment: %s\n", cfg.CurrentEnvironment)
	}

	return nil
}

func runEnvUse(cmd *cobra.Command, args []string) error {
	envName := args[0]

	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	store, err := secrets.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load secrets")
	}

	// Validate that the environment exists in secrets
	if !store.HasEnvironment(envName) && envName != "$shared" {
		return errors.NewValidationErrorWithValue("environment", envName, "not found")
	}

	cfg.CurrentEnvironment = envName
	if err := cfg.Save(); err != nil {
		return errors.Wrap(err, "failed to save config")
	}

	fmt.Printf("Switched to environment: %s\n", envName)
	return nil
}

func runEnvShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	store, err := secrets.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load secrets")
	}

	envName := cfg.CurrentEnvironment
	if len(args) > 0 {
		envName = args[0]
	}

	if envName == "" {
		// Show $shared
		envName = "$shared"
	}

	vars, ok := store.GetVariables(envName)
	if !ok && envName != "$shared" {
		return errors.NewValidationErrorWithValue("environment", envName, "environment not found")
	}

	// Show $shared first if showing a specific environment
	if envName != "$shared" {
		if shared, ok := store.GetVariables("$shared"); ok && len(shared) > 0 {
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

	store, err := secrets.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load secrets")
	}

	if err := store.SetVariable(envName, varName, varValue); err != nil {
		return err
	}

	if err := store.Save(); err != nil {
		return errors.Wrap(err, "failed to save secrets")
	}

	fmt.Printf("Set %s=%s in environment '%s'\n", varName, maskValue(varValue), envName)
	return nil
}

func runEnvUnset(cmd *cobra.Command, args []string) error {
	envName := args[0]
	varName := args[1]

	store, err := secrets.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load secrets")
	}

	if err := store.UnsetVariable(envName, varName); err != nil {
		return err
	}

	if err := store.Save(); err != nil {
		return errors.Wrap(err, "failed to save secrets")
	}

	fmt.Printf("Removed %s from environment '%s'\n", varName, envName)
	return nil
}

func runEnvCreate(cmd *cobra.Command, args []string) error {
	envName := args[0]

	store, err := secrets.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load secrets")
	}

	if store.HasEnvironment(envName) {
		return errors.NewValidationErrorWithValue("environment", envName, "environment already exists")
	}

	if err := store.AddEnvironment(envName, nil); err != nil {
		return err
	}

	if err := store.Save(); err != nil {
		return errors.Wrap(err, "failed to save secrets")
	}

	fmt.Printf("Created environment: %s\n", envName)
	fmt.Println()
	fmt.Printf("To use it: restclient env use %s\n", envName)
	return nil
}

func runEnvDelete(cmd *cobra.Command, args []string) error {
	envName := args[0]

	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	store, err := secrets.Load()
	if err != nil {
		return errors.Wrap(err, "failed to load secrets")
	}

	if err := store.RemoveEnvironment(envName); err != nil {
		return err
	}

	if err := store.Save(); err != nil {
		return errors.Wrap(err, "failed to save secrets")
	}

	// Clear current environment if it was the deleted one
	if cfg.CurrentEnvironment == envName {
		cfg.CurrentEnvironment = ""
		if err := cfg.Save(); err != nil {
			return errors.Wrap(err, "failed to save config")
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
