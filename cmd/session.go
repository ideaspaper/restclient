package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ideaspaper/restclient/pkg/session"
)

var (
	// Session command flags
	clearCookies   bool
	clearVariables bool
	clearAllFlag   bool
	sessionDir     string
)

// sessionCmd represents the session command
var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage session data (cookies and variables)",
	Long: `Manage session data including cookies and script variables.

Sessions are scoped by directory (based on .http file location) by default,
or can be named explicitly using the --session flag with send command.

Examples:
  # Show current session data
  restclient session show

  # Show session for a specific directory
  restclient session show --dir ./api

  # Show a named session
  restclient session show --session my-api

  # Clear all session data for current directory
  restclient session clear

  # Clear only cookies
  restclient session clear --cookies

  # Clear only variables
  restclient session clear --variables

  # Clear all sessions
  restclient session clear --all

  # List all sessions
  restclient session list`,
}

// sessionShowCmd shows session data
var sessionShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show session data",
	Long:  `Show cookies and variables stored in the session.`,
	RunE:  runSessionShow,
}

// sessionClearCmd clears session data
var sessionClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear session data",
	Long:  `Clear cookies and/or variables from the session.`,
	RunE:  runSessionClear,
}

// sessionListCmd lists all sessions
var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Long:  `List all stored sessions (both named and directory-based).`,
	RunE:  runSessionList,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionClearCmd)
	sessionCmd.AddCommand(sessionListCmd)

	// Flags for show command
	sessionShowCmd.Flags().StringVar(&sessionName, "session", "", "named session to show")
	sessionShowCmd.Flags().StringVar(&sessionDir, "dir", "", "directory to show session for (defaults to current directory)")

	// Flags for clear command
	sessionClearCmd.Flags().StringVar(&sessionName, "session", "", "named session to clear")
	sessionClearCmd.Flags().StringVar(&sessionDir, "dir", "", "directory to clear session for (defaults to current directory)")
	sessionClearCmd.Flags().BoolVar(&clearCookies, "cookies", false, "clear only cookies")
	sessionClearCmd.Flags().BoolVar(&clearVariables, "variables", false, "clear only variables")
	sessionClearCmd.Flags().BoolVar(&clearAllFlag, "all", false, "clear all sessions")
}

func runSessionShow(cmd *cobra.Command, args []string) error {
	// Determine the http file path for session scoping
	httpFilePath := ""
	if sessionDir != "" {
		// Use specified directory
		absPath, err := filepath.Abs(sessionDir)
		if err != nil {
			return fmt.Errorf("failed to resolve directory: %w", err)
		}
		httpFilePath = filepath.Join(absPath, "dummy.http")
	} else if sessionName == "" {
		// Use current directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		httpFilePath = filepath.Join(cwd, "dummy.http")
	}

	sessionMgr, err := session.NewSessionManager("", httpFilePath, sessionName)
	if err != nil {
		return fmt.Errorf("failed to initialize session: %w", err)
	}

	if err := sessionMgr.Load(); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No session data found.")
			return nil
		}
		// Continue even if one file doesn't exist
	}

	// Print session path
	printHeader("Session Path:")
	fmt.Printf("  %s\n\n", sessionMgr.GetSessionPath())

	// Print cookies
	cookies := sessionMgr.GetAllCookies()
	printHeader("Cookies:")

	if len(cookies) == 0 {
		fmt.Println("  (none)")
	} else {
		for host, hostCookies := range cookies {
			fmt.Printf("  %s:\n", host)
			for _, cookie := range hostCookies {
				fmt.Printf("    %s = %s\n", cookie.Name, truncateValue(cookie.Value, 50))
				if !cookie.Expires.IsZero() {
					fmt.Printf("      expires: %s\n", cookie.Expires.Format("2006-01-02 15:04:05"))
				}
			}
		}
	}

	fmt.Println()

	// Print variables
	variables := sessionMgr.GetAllVariables()
	printHeader("Variables:")

	if len(variables) == 0 {
		fmt.Println("  (none)")
	} else {
		for name, value := range variables {
			valueStr := formatValue(value)
			fmt.Printf("  %s = %s\n", name, truncateValue(valueStr, 60))
		}
	}

	return nil
}

func runSessionClear(cmd *cobra.Command, args []string) error {
	// Clear all sessions
	if clearAllFlag {
		if err := session.ClearAllSessions(""); err != nil {
			return fmt.Errorf("failed to clear all sessions: %w", err)
		}
		fmt.Println("All sessions cleared.")
		return nil
	}

	// Determine the http file path for session scoping
	httpFilePath := ""
	if sessionDir != "" {
		absPath, err := filepath.Abs(sessionDir)
		if err != nil {
			return fmt.Errorf("failed to resolve directory: %w", err)
		}
		httpFilePath = filepath.Join(absPath, "dummy.http")
	} else if sessionName == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		httpFilePath = filepath.Join(cwd, "dummy.http")
	}

	sessionMgr, err := session.NewSessionManager("", httpFilePath, sessionName)
	if err != nil {
		return fmt.Errorf("failed to initialize session: %w", err)
	}

	// Load existing data first
	sessionMgr.Load()

	// Determine what to clear
	if clearCookies && !clearVariables {
		sessionMgr.ClearCookies()
		if err := sessionMgr.SaveCookies(); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}
		fmt.Println("Cookies cleared.")
	} else if clearVariables && !clearCookies {
		sessionMgr.ClearVariables()
		if err := sessionMgr.SaveVariables(); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}
		fmt.Println("Variables cleared.")
	} else {
		// Clear everything and delete the session directory
		if err := sessionMgr.Delete(); err != nil {
			return fmt.Errorf("failed to delete session: %w", err)
		}
		fmt.Println("Session cleared.")
	}

	return nil
}

func runSessionList(cmd *cobra.Command, args []string) error {
	sessions, err := session.ListAllSessions("")
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	printHeader("Sessions:")

	for _, s := range sessions {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			sessionType := parts[0]
			sessionID := parts[1]

			if sessionType == "named" {
				fmt.Printf("  %s %s\n", printDimText("[named]"), sessionID)
			} else {
				fmt.Printf("  %s %s\n", printDimText("[dir]  "), sessionID)
			}
		} else {
			fmt.Printf("  %s\n", s)
		}
	}

	return nil
}

// truncateValue truncates a string to maxLen and adds ellipsis
func truncateValue(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatValue formats a value for display
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case map[string]interface{}, []interface{}:
		bytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(bytes)
	default:
		return fmt.Sprintf("%v", val)
	}
}
