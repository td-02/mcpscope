package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:           "mcpscope",
	Short:         "MCP utility CLI",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
