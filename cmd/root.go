package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Global viper instance
	v *viper.Viper

	// Config flags
	cfgFile     string
	environment string
	verbose     bool
	noColor     bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "restclient",
	Short: "A CLI HTTP client inspired by VS Code REST Client",
	Long: `restclient is a command-line HTTP client that supports .http and .rest files,
environment variables, and request chaining.

Examples:
  # Send a request from a .http file
  restclient send api.http

  # Send a specific request by name
  restclient send api.http --name getUsers

  # List available environments
  restclient env list

  # Switch environment
  restclient env use production`,
	Version: "0.1.0",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Initialize viper
	v = viper.New()

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.restclient/config.json)")
	rootCmd.PersistentFlags().StringVarP(&environment, "env", "e", "", "environment to use")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")

	// Bind flags to viper
	v.BindPFlag("environment", rootCmd.PersistentFlags().Lookup("env"))
	v.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	v.BindPFlag("noColor", rootCmd.PersistentFlags().Lookup("no-color"))
}

// initConfig reads in config file and ENV variables if set
func initConfig() error {
	if cfgFile != "" {
		// Use config file from the flag
		v.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		// Search for config in home directory
		configPath := filepath.Join(home, ".restclient")
		v.AddConfigPath(configPath)
		v.SetConfigType("json")
		v.SetConfigName("config")
	}

	// Environment variables
	v.SetEnvPrefix("RESTCLIENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Set defaults
	v.SetDefault("followRedirect", true)
	v.SetDefault("timeoutInMilliseconds", 0)
	v.SetDefault("rememberCookiesForSubsequentRequests", true)
	v.SetDefault("insecureSSL", false)
	v.SetDefault("proxyStrictSSL", true)
	v.SetDefault("previewOption", "full")
	v.SetDefault("showColors", true)
	v.SetDefault("defaultHeaders", map[string]string{
		"User-Agent": "restclient-cli",
	})

	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		// Config file not found is fine, we'll use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Only return error if it's not "file not found"
			return nil
		}
	}

	return nil
}

// GetViper returns the global viper instance
func GetViper() *viper.Viper {
	return v
}

// GetConfigFile returns the path to the config file in use
func GetConfigFile() string {
	return v.ConfigFileUsed()
}
