package cmd

import (
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/mcp"
	"github.com/fadhlidev/snapdock/pkg/types"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the SnapDock MCP server",
	Long: `Start the SnapDock Model Context Protocol (MCP) server.
This server allows AI agents to interact with SnapDock to manage Docker container snapshots.
The server communicates over standard I/O (stdin/stdout).`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	socketPath, _ := cmd.Flags().GetString("socket")
	
	srv := mcp.NewServer(types.SnapDockVersion, socketPath)
	return srv.Start()
}
