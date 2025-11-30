package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const (
	// Module path for go install
	modulePath = "github.com/ideaspaper/restclient"
)

var (
	updateVersion string
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update restclient to the latest version",
	Long: `Update restclient to the latest version using go install.

Examples:
  # Update to the latest version
  restclient update

  # Update to a specific version
  restclient update --version v1.0.0`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVar(&updateVersion, "version", "latest", "version to install (e.g., v1.0.0, latest)")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	currentVersion := rootCmd.Version

	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Printf("Updating to: %s\n", updateVersion)
	fmt.Println()

	// Build the module path with version
	moduleWithVersion := fmt.Sprintf("%s@%s", modulePath, updateVersion)

	fmt.Printf("Running: go install %s\n", moduleWithVersion)
	fmt.Println()

	// Run go install
	goCmd := exec.Command("go", "install", moduleWithVersion)
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr

	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	fmt.Println()
	successColor := color.New(color.FgGreen, color.Bold)
	successColor.Printf("Successfully updated restclient to %s!\n", updateVersion)

	return nil
}
