package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"mcpscope/internal/proxy"
	"mcpscope/internal/store"
)

func init() {
	rootCmd.AddCommand(newProxyCmd())
}

func newProxyCmd() *cobra.Command {
	var server string
	var port int
	var transport string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Launch an MCP server subprocess and proxy JSON-RPC traffic",
		RunE: func(cmd *cobra.Command, args []string) error {
			if server == "" {
				return errors.New("--server is required")
			}

			if err := validatePort(port); err != nil {
				return err
			}

			normalizedTransport, err := validateTransport(transport)
			if err != nil {
				return err
			}

			traceStore, err := store.OpenSQLite(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer traceStore.Close()

			return proxy.Run(cmd.Context(), proxy.Config{
				Server:     server,
				ServerName: filepath.Base(server),
				Port:       port,
				Transport:  normalizedTransport,
				Store:      traceStore,
				Stdin:      os.Stdin,
				Stdout:     os.Stdout,
				Stderr:     os.Stderr,
			})
		},
	}

	cmd.Flags().StringVar(&server, "server", "", "Path to the MCP server binary")
	cmd.Flags().IntVar(&port, "port", 4444, "Proxy listen port")
	cmd.Flags().StringVar(&transport, "transport", "stdio", "Proxy transport: stdio or http")
	cmd.Flags().StringVar(&dbPath, "db", "mcpscope.db", "SQLite database path for persisted traces")

	return cmd
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("--port must be between 1 and 65535")
	}

	return nil
}

func validateTransport(transport string) (string, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(transport)); normalized {
	case "stdio", "http":
		return normalized, nil
	default:
		return "", fmt.Errorf("--transport must be either stdio or http")
	}
}
