package main

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [command]",
	Short: "Generate shell completion scripts",
	Long:  `Generate shell completion scripts for bash, zsh, fish, or powershell.`,
}

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generate bash completion script",
	Long: `Generate bash completion script.

To load completions in your current shell session:
  source <(lazyspeed completion bash)

To load completions for each session, add to your ~/.bashrc:
  echo 'source <(lazyspeed completion bash)' >> ~/.bashrc`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return rootCmd.GenBashCompletionV2(os.Stdout, true)
	},
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate zsh completion script",
	Long: `Generate zsh completion script.

To load completions in your current shell session:
  source <(lazyspeed completion zsh)

To load completions for each session, add to your ~/.zshrc:
  echo 'source <(lazyspeed completion zsh)' >> ~/.zshrc

Or place the output in your fpath:
  lazyspeed completion zsh > "${fpath[1]}/_lazyspeed"`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return rootCmd.GenZshCompletion(os.Stdout)
	},
}

var completionFishCmd = &cobra.Command{
	Use:   "fish",
	Short: "Generate fish completion script",
	Long: `Generate fish completion script.

To load completions in your current shell session:
  lazyspeed completion fish | source

To load completions for each session:
  lazyspeed completion fish > ~/.config/fish/completions/lazyspeed.fish`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return rootCmd.GenFishCompletion(os.Stdout, true)
	},
}

var completionPowershellCmd = &cobra.Command{
	Use:   "powershell",
	Short: "Generate powershell completion script",
	Long: `Generate powershell completion script.

To load completions in your current shell session:
  lazyspeed completion powershell | Out-String | Invoke-Expression

To load completions for each session, add the output to your PowerShell profile.`,
	Args: cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
	},
}

func init() {
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)
	completionCmd.AddCommand(completionFishCmd)
	completionCmd.AddCommand(completionPowershellCmd)
	rootCmd.AddCommand(completionCmd)
}
