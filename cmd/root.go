package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
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
cURL commands, environment variables, and request chaining.

Examples:
  # Send a request from a .http file
  restclient send api.http

  # Send a specific request by name
  restclient send api.http --name getUsers

  # Send a cURL command
  restclient curl 'curl -X GET https://api.example.com/users'

  # List available environments
  restclient env list

  # Switch environment
  restclient env use production`,
	Version: "0.1.0",
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.restclient/config.json)")
	rootCmd.PersistentFlags().StringVarP(&environment, "env", "e", "", "environment to use")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
}
