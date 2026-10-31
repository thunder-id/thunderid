package app

import "github.com/modelcontextprotocol/go-sdk/mcp"

func register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_list_apps",
		Description: "List apps.",
	}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_" +
			"create_app",
		Description: "Create an app.",
	}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "thunderid_search_apps"}, nil)
}
