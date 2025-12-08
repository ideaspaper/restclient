package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ideaspaper/restclient/internal/filesystem"
	"github.com/ideaspaper/restclient/internal/stringutil"
	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/errors"
	"github.com/ideaspaper/restclient/pkg/history"
	"github.com/ideaspaper/restclient/pkg/lastfile"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/session"
	"github.com/ideaspaper/restclient/pkg/tui"
	"github.com/ideaspaper/restclient/pkg/variables"
)

// HistoryItem implements tui.Item for historical requests
type HistoryItem struct {
	Request models.HistoricalHttpRequest
	Index   int // 0-based index
}

// FilterValue returns the string used for fuzzy matching
func (h HistoryItem) FilterValue() string {
	t := time.UnixMilli(h.Request.StartTime)
	timeStr := t.Format("2006-01-02 15:04:05")
	// Include method, URL, and timestamp for matching (not index)
	return fmt.Sprintf("%s %s %s", h.Request.Method, h.Request.URL, timeStr)
}

// Title returns the main display text (method and URL)
func (h HistoryItem) Title() string {
	return fmt.Sprintf("%s %s", h.Request.Method, stringutil.Truncate(h.Request.URL, 50))
}

// Description returns the timestamp
func (h HistoryItem) Description() string {
	t := time.UnixMilli(h.Request.StartTime)
	return t.Format("2006-01-02 15:04:05")
}

// String returns formatted string for display: [index] title  description
func (h HistoryItem) String() string {
	return fmt.Sprintf("[%d] %s  %s", h.Index+1, h.Title(), h.Description())
}

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View and manage request history",
	Long: `View and manage request history.

Examples:
  # Show details of a specific request (1-based index)
  restclient history show 1

  # Interactive selection to show request details
  restclient history show

  # Clear all history
  restclient history clear

  # Show history statistics
  restclient history stats

  # Replay a request from history (1-based index)
  restclient history replay 1

  # Interactive selection to replay a request
  restclient history replay`,
}

// historyShowCmd shows details of a history item
var historyShowCmd = &cobra.Command{
	Use:   "show [index]",
	Short: "Show details of a specific request",
	Long: `Show details of a specific request from history.

If no index is provided, an interactive selector will be shown.

Examples:
  # Show request at index 1
  restclient history show 1

  # Interactive selection
  restclient history show`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHistoryShow,
}

// historyClearCmd clears all history
var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all request history",
	RunE:  runHistoryClear,
}

// historyStatsCmd shows history statistics
var historyStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show history statistics",
	RunE:  runHistoryStats,
}

// historyReplayCmd replays a request from history
var historyReplayCmd = &cobra.Command{
	Use:   "replay [index]",
	Short: "Replay a request from history",
	Long: `Replay a request from history.

If no index is provided, an interactive selector will be shown.

Examples:
  # Replay request at index 1
  restclient history replay 1

  # Interactive selection
  restclient history replay`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHistoryReplay,
}

func init() {
	rootCmd.AddCommand(historyCmd)

	historyCmd.AddCommand(historyShowCmd)
	historyCmd.AddCommand(historyClearCmd)
	historyCmd.AddCommand(historyStatsCmd)
	historyCmd.AddCommand(historyReplayCmd)
}

func runHistoryShow(cmd *cobra.Command, args []string) error {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return errors.Wrap(err, "failed to load history")
	}

	var item models.HistoricalHttpRequest

	if len(args) == 0 {
		items := histMgr.GetAll()
		if len(items) == 0 {
			fmt.Println("No requests in history")
			return nil
		}

		selectedItem, err := selectHistoryItem(items)
		if err != nil {
			if errors.Is(err, errors.ErrCanceled) {
				return nil
			}
			return err
		}
		item = *selectedItem
		fmt.Println() // Blank line after selection
	} else {
		index := 0
		fmt.Sscanf(args[0], "%d", &index)

		// Convert 1-based user input to 0-based internal index
		index = index - 1

		itemPtr, err := histMgr.GetByIndex(index)
		if err != nil {
			return err
		}
		item = *itemPtr
	}

	printHeader("Request Details:")
	fmt.Println()

	formatter := newHistoryFormatter()
	fmt.Print(formatter.FormatDetails(item))

	return nil
}

func runHistoryClear(cmd *cobra.Command, args []string) error {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return errors.Wrap(err, "failed to load history")
	}

	if err := histMgr.Clear(); err != nil {
		return errors.Wrap(err, "failed to clear history")
	}

	fmt.Println("History cleared")
	return nil
}

func runHistoryStats(cmd *cobra.Command, args []string) error {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return errors.Wrap(err, "failed to load history")
	}

	stats := histMgr.GetStats()

	printHeader("History Statistics:")
	fmt.Println()

	formatter := newHistoryFormatter()
	fmt.Print(formatter.FormatStats(stats))

	return nil
}

func runHistoryReplay(cmd *cobra.Command, args []string) error {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return errors.Wrap(err, "failed to load history")
	}

	var item models.HistoricalHttpRequest

	if len(args) == 0 {
		items := histMgr.GetAll()
		if len(items) == 0 {
			fmt.Println("No requests in history")
			return nil
		}

		selectedItem, err := selectHistoryItem(items)
		if err != nil {
			if errors.Is(err, errors.ErrCanceled) {
				return nil
			}
			return err
		}
		item = *selectedItem
		fmt.Println()
	} else {
		index := 0
		fmt.Sscanf(args[0], "%d", &index)
		index = index - 1

		itemPtr, err := histMgr.GetByIndex(index)
		if err != nil {
			return err
		}
		item = *itemPtr
		fmt.Println() // Blank line before output
	}

	request := &models.HttpRequest{
		Method:  item.Method,
		URL:     item.URL,
		Headers: item.Headers,
		RawBody: item.Body,
	}

	if item.Body != "" {
		request.Body = strings.NewReader(item.Body)
	}

	cfg, sessionCfg, envStore, err := loadConfig()
	if err != nil {
		return err
	}
	// Silence unused variable warning - cfg is for CLI preferences (not used in replay)
	_ = cfg

	varProcessor := variables.NewVariableProcessor()
	varProcessor.SetEnvironment(sessionCfg.CurrentEnvironment())

	// Load environment variables from per-session environment store
	if envStore != nil {
		varProcessor.SetEnvironmentVariables(envStore.EnvironmentVariables)
	}

	// History already contains the Cookie header that was sent, no session needed
	noSession = true

	fmt.Printf("%s %s\n\n", printMethod(request.Method), request.URL)

	return sendRequest("", request, sessionCfg, varProcessor, envStore)
}

// selectHistoryItem shows an interactive selector for history items
func selectHistoryItem(items []models.HistoricalHttpRequest) (*models.HistoricalHttpRequest, error) {
	tuiItems := make([]tui.Item, len(items))
	for i, item := range items {
		tuiItems[i] = HistoryItem{Request: item, Index: i}
	}

	_, selectedIndex, err := tui.Run(tuiItems, useColors())
	if err != nil {
		return nil, err
	}

	return &items[selectedIndex], nil
}

func printHistoryItem(item models.HistoricalHttpRequest, index int) {
	formatter := newHistoryFormatter()
	fmt.Println(formatter.FormatItem(item, index))
}

func loadConfig() (*config.Config, *session.SessionConfig, *session.EnvironmentStore, error) {
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to load config")
	}

	// Try to get session context from lastfile
	var envStore *session.EnvironmentStore
	var sessionCfg *session.SessionConfig
	lastPath, _ := lastfile.Load()
	if lastPath != "" {
		sessionMgr, err := session.NewSessionManager("", lastPath, sessionName)
		if err == nil {
			sessionPath := sessionMgr.GetSessionPath()
			sessionCfg, err = session.LoadOrCreateSessionConfig(filesystem.Default, sessionPath)
			if err != nil {
				sessionCfg = session.DefaultSessionConfig()
			}
			envStore, err = session.LoadOrCreateEnvironmentStore(filesystem.Default, sessionPath)
			if err != nil {
				envStore = nil // Fall back to no environment store
			}
		}
	}

	// Default session config if none loaded
	if sessionCfg == nil {
		sessionCfg = session.DefaultSessionConfig()
	}

	if environment != "" {
		// Validate the environment exists in session
		if envStore == nil {
			return nil, nil, nil, errors.NewValidationError("environment", "no session context available (run 'send' first)")
		}
		if !envStore.HasEnvironment(environment) && environment != "$shared" {
			return nil, nil, nil, errors.NewValidationErrorWithValue("environment", environment, "not found")
		}
		sessionCfg.SetCurrentEnvironment(environment)
	}

	return cfg, sessionCfg, envStore, nil
}

// newHistoryFormatter creates a history formatter with color support
func newHistoryFormatter() *history.Formatter {
	if useColors() {
		return &history.Formatter{
			FormatIndex:  printListIndex,
			FormatMethod: printMethod,
			FormatTime:   printDimText,
		}
	}
	return history.DefaultFormatter()
}
