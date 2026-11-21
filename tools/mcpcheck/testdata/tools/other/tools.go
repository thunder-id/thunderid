package other

// Imports no MCP package, so the Tool literal is someone else's type.
type Tool struct{ Name string }

var notMCP = Tool{Name: "thunderid_not_mcp"}
