package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ideaspaper/restclient/pkg/config"
	"github.com/ideaspaper/restclient/pkg/history"
	"github.com/ideaspaper/restclient/pkg/models"
	"github.com/ideaspaper/restclient/pkg/variables"
)

var (
	historyLimit int
	historyAll   bool
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View and manage request history",
	Long: `View and manage request history.

Examples:
  # List recent requests
  restclient history list

  # List last 5 requests
  restclient history list --limit 5

  # Show details of a specific request (1-based index)
  restclient history show 1

  # Clear all history
  restclient history clear

  # Search history
  restclient history search "api.example.com"

  # Show history statistics
  restclient history stats

  # Replay a request from history (1-based index)
  restclient history replay 1`,
}

// historyListCmd lists request history
var historyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent requests",
	RunE:  runHistoryList,
}

// historyShowCmd shows details of a history item
var historyShowCmd = &cobra.Command{
	Use:   "show <index>",
	Short: "Show details of a specific request",
	Args:  cobra.ExactArgs(1),
	RunE:  runHistoryShow,
}

// historyClearCmd clears all history
var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all request history",
	RunE:  runHistoryClear,
}

// historySearchCmd searches history
var historySearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search request history",
	Args:  cobra.ExactArgs(1),
	RunE:  runHistorySearch,
}

// historyStatsCmd shows history statistics
var historyStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show history statistics",
	RunE:  runHistoryStats,
}

// historyReplayCmd replays a request from history
var historyReplayCmd = &cobra.Command{
	Use:   "replay <index>",
	Short: "Replay a request from history",
	Args:  cobra.ExactArgs(1),
	RunE:  runHistoryReplay,
}

func init() {
	rootCmd.AddCommand(historyCmd)

	historyCmd.AddCommand(historyListCmd)
	historyCmd.AddCommand(historyShowCmd)
	historyCmd.AddCommand(historyClearCmd)
	historyCmd.AddCommand(historySearchCmd)
	historyCmd.AddCommand(historyStatsCmd)
	historyCmd.AddCommand(historyReplayCmd)

	historyListCmd.Flags().IntVarP(&historyLimit, "limit", "l", 10, "number of items to show")
	historyListCmd.Flags().BoolVarP(&historyAll, "all", "a", false, "show all history items")
}

func runHistoryList(cmd *cobra.Command, args []string) error {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	var items []models.HistoricalHttpRequest
	if historyAll {
		items = histMgr.GetAll()
	} else {
		items = histMgr.GetRecent(historyLimit)
	}

	if len(items) == 0 {
		fmt.Println("No requests in history")
		return nil
	}

	printHeader("Request History:")
	fmt.Println()

	for i, item := range items {
		printHistoryItem(item, i)
	}

	return nil
}

func runHistoryShow(cmd *cobra.Command, args []string) error {
	index := 0
	fmt.Sscanf(args[0], "%d", &index)

	// Convert 1-based user input to 0-based internal index
	index = index - 1

	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	item, err := histMgr.GetByIndex(index)
	if err != nil {
		return err
	}

	// Print request details
	printHeader("Request Details:")
	fmt.Println()

	fmt.Printf("%s %s\n", printMethod(item.Method), item.URL)

	fmt.Printf("Time: %s\n", time.UnixMilli(item.StartTime).Format("2006-01-02 15:04:05"))
	fmt.Println()

	if len(item.Headers) > 0 {
		fmt.Println("Headers:")
		for k, v := range item.Headers {
			fmt.Printf("  %s: %s\n", k, v)
		}
		fmt.Println()
	}

	if item.Body != "" {
		fmt.Println("Body:")
		fmt.Println(item.Body)
	}

	return nil
}

func runHistoryClear(cmd *cobra.Command, args []string) error {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	if err := histMgr.Clear(); err != nil {
		return fmt.Errorf("failed to clear history: %w", err)
	}

	fmt.Println("History cleared")
	return nil
}

func runHistorySearch(cmd *cobra.Command, args []string) error {
	query := args[0]

	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	items := histMgr.Search(query)

	if len(items) == 0 {
		fmt.Printf("No requests matching '%s' found\n", query)
		return nil
	}

	fmt.Printf("Found %d matching requests:\n\n", len(items))

	for i, item := range items {
		printHistoryItem(item, i)
	}

	return nil
}

func runHistoryStats(cmd *cobra.Command, args []string) error {
	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	stats := histMgr.GetStats()

	printHeader("History Statistics:")
	fmt.Println()

	fmt.Printf("Total Requests: %d\n", stats.TotalRequests)
	fmt.Println()

	if len(stats.MethodCounts) > 0 {
		fmt.Println("By Method:")
		for method, count := range stats.MethodCounts {
			fmt.Printf("  %s: %d\n", method, count)
		}
		fmt.Println()
	}

	if len(stats.DomainCounts) > 0 {
		fmt.Println("Top Domains:")
		// Show top 5 domains
		count := 0
		for domain, c := range stats.DomainCounts {
			if count >= 5 {
				break
			}
			fmt.Printf("  %s: %d\n", domain, c)
			count++
		}
		fmt.Println()
	}

	if !stats.OldestRequest.IsZero() {
		fmt.Printf("Oldest Request: %s\n", stats.OldestRequest.Format("2006-01-02 15:04:05"))
		fmt.Printf("Newest Request: %s\n", stats.NewestRequest.Format("2006-01-02 15:04:05"))
	}

	return nil
}

func runHistoryReplay(cmd *cobra.Command, args []string) error {
	index := 0
	fmt.Sscanf(args[0], "%d", &index)

	// Convert 1-based user input to 0-based internal index
	index = index - 1

	histMgr, err := history.NewHistoryManager("")
	if err != nil {
		return fmt.Errorf("failed to load history: %w", err)
	}

	item, err := histMgr.GetByIndex(index)
	if err != nil {
		return err
	}

	// Convert historical request to HttpRequest
	request := &models.HttpRequest{
		Method:  item.Method,
		URL:     item.URL,
		Headers: item.Headers,
		RawBody: item.Body,
	}

	if item.Body != "" {
		request.Body = strings.NewReader(item.Body)
	}

	// Load config and send
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	varProcessor := variables.NewVariableProcessor()
	varProcessor.SetEnvironment(cfg.CurrentEnvironment)
	varProcessor.SetEnvironmentVariables(cfg.EnvironmentVariables)

	// Replay without session - history already contains the Cookie header that was sent
	noSession = true

	return sendRequest("", request, cfg, varProcessor)
}

func printHistoryItem(item models.HistoricalHttpRequest, index int) {
	t := time.UnixMilli(item.StartTime)
	timeStr := t.Format("2006-01-02 15:04:05")

	// Display 1-based index for user-facing output
	displayIndex := index + 1

	fmt.Printf("%s %s %s  %s\n",
		printListIndex(displayIndex),
		printMethod(item.Method),
		truncateString(item.URL, 60),
		printDimText(timeStr))
}

func loadConfig() (*config.Config, error) {
	cfg, err := config.LoadOrCreateConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if environment != "" {
		if err := cfg.SetEnvironment(environment); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
