package guide

import sdk "github.com/modelcontextprotocol/go-sdk/mcp"

func register(server *sdk.Server) {
	sdk.AddTool(server, &sdk.Tool{Name: "thunderid_integrate_guide"}, nil)
}
