package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"mcpscope/internal/store"
)

func init() {
	rootCmd.AddCommand(newExportCmd())
}

func newExportCmd() *cobra.Command {
	var dbPath string
	var outputPath string
	var environment string
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export persisted traces as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath = effectiveString(cmd, "db", dbPath, loadedConfig.Proxy.DB)
			environment = defaultEnvironment(effectiveString(cmd, "environment", environment, loadedConfig.Environment))

			traceStore, err := store.OpenSQLite(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer traceStore.Close()

			traces, err := traceStore.Query(cmd.Context(), store.QueryFilter{
				Environment: environment,
				Limit:       limit,
			})
			if err != nil {
				return err
			}

			encoded, err := json.MarshalIndent(traces, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal export: %w", err)
			}
			if outputPath == "" {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
				return err
			}
			return os.WriteFile(outputPath, append(encoded, '\n'), 0o644)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "mcpscope.db", "SQLite database path for persisted traces")
	cmd.Flags().StringVar(&outputPath, "output", "", "Path to write exported traces")
	cmd.Flags().StringVar(&environment, "environment", "default", "Environment to export")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum number of traces to export")

	return cmd
}
