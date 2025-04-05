package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/Dragon-Huang0403/slack-dump-mcp/pkg/slackdump"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"Dump Slack Threads",
		"0.1.0",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	dumpSlackThreadTool := mcp.NewTool("dump_slack_thread",
		mcp.WithDescription("Dump a Slack Thread"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The URL of the Slack thread to dump. for example: https://slack.com/archives/C0123456789/p1234567890"),
		),
	)

	// Add the calculator handler
	s.AddTool(dumpSlackThreadTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url := request.Params.Arguments["url"].(string)
		dump, err := slackdump.Dump(ctx, url)
		if err != nil {
			return nil, err
		}

		return mcp.NewToolResultText(strings.Join(dump, "\n")), nil
	})

	// Start the server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
