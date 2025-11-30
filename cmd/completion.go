package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for restclient.

To load completions:

Bash:
  $ source <(restclient completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ restclient completion bash > /etc/bash_completion.d/restclient
  # macOS:
  $ restclient completion bash > $(brew --prefix)/etc/bash_completion.d/restclient

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ restclient completion zsh > "${fpath[1]}/_restclient"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ restclient completion fish | source

  # To load completions for each session, execute once:
  $ restclient completion fish > ~/.config/fish/completions/restclient.fish

PowerShell:
  PS> restclient completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> restclient completion powershell > restclient.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)

	// Add file completions for send command
	sendCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			// Complete .http and .rest files
			return []string{"http", "rest"}, cobra.ShellCompDirectiveFilterFileExt
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Add completions for history subcommands
	historyCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Add completions for env subcommands
	envCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}
