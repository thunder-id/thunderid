// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// upstreamReadTimeout bounds reading one upstream's tools or instructions, so an upstream that accepts
// the connection and never answers is left out rather than holding the listing. A tool call is not
// bounded by it: it runs as long as the caller's request does.
var upstreamReadTimeout = 10 * time.Second

const (
	methodInitialize = "initialize"
	methodDiscover   = "server/discover"
	methodListTools  = "tools/list"
	methodCallTool   = "tools/call"
)

// upstreams serves the tools of other MCP servers beside this one's own, so a client connects to one
// server for both. A deployment that adds its own tools, such as one that knows environments this
// server does not, runs them as an upstream rather than as a second server its clients connect to.
//
// Nothing is read from an upstream until a client asks, because an upstream answers only a caller
// with a token: listing the tools reads each upstream's with the caller's token, and a call to an
// upstream tool is forwarded with it. An upstream tool named like one of this server's own is not
// served, so it cannot stand in for one.
//
// What an upstream tells a client about using its tools, its instructions, follows this server's own,
// so a client learns how the tools work together, such as deploying what it configured, from the one
// server it connects to.
type upstreams struct {
	endpoints []string
	// local are the names of this server's own tools.
	local  map[string]bool
	client *http.Client
	logger *log.Logger

	mu sync.RWMutex
	// owners is which upstream serves each tool, as the last listing found.
	owners map[string]string
	// instructions are each upstream's instructions, kept once read: they describe the upstream, not
	// the caller.
	instructions map[string]string
}

func newUpstreams(endpoints []string, local map[string]bool) *upstreams {
	return &upstreams{
		endpoints: endpoints,
		local:     local,
		client:    &http.Client{},
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "MCPUpstreams")),
		owners:    map[string]string{},

		instructions: map[string]string{},
	}
}

// middleware adds the upstream tools to a listing and forwards a call to one.
func (u *upstreams) middleware(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		switch method {
		case methodInitialize, methodDiscover:
			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}
			added := u.instructionsFor(ctx, authorizationOf(req))
			switch answer := result.(type) {
			case *mcpsdk.InitializeResult:
				answer.Instructions = joinInstructions(answer.Instructions, added)
			case *mcpsdk.DiscoverResult:
				answer.Instructions = joinInstructions(answer.Instructions, added)
			}
			return result, nil
		case methodListTools:
			result, err := next(ctx, method, req)
			if err != nil {
				return result, err
			}
			listed, ok := result.(*mcpsdk.ListToolsResult)
			// The upstream tools follow this server's own, on the last page.
			if !ok || listed.NextCursor != "" {
				return result, nil
			}
			listed.Tools = append(listed.Tools, u.list(ctx, authorizationOf(req))...)
			return listed, nil
		case methodCallTool:
			call, ok := req.(*mcpsdk.CallToolRequest)
			if !ok || call.Params == nil || u.local[call.Params.Name] {
				return next(ctx, method, req)
			}
			authorization := authorizationOf(req)
			endpoint, found := u.ownerOf(call.Params.Name)
			if !found {
				// A client may call a tool it learned of in an earlier session; the listing finds its upstream.
				u.list(ctx, authorization)
				if endpoint, found = u.ownerOf(call.Params.Name); !found {
					return next(ctx, method, req)
				}
			}
			return u.call(ctx, endpoint, authorization, call.Params)
		default:
			return next(ctx, method, req)
		}
	}
}

// list reads every upstream's tools with the caller's token and records which upstream serves each.
// An upstream that cannot be read is left out of the listing, so the server's own tools are still
// served while it is down.
func (u *upstreams) list(ctx context.Context, authorization string) []*mcpsdk.Tool {
	var tools []*mcpsdk.Tool
	owners := map[string]string{}
	for _, endpoint := range u.endpoints {
		listed, err := u.listOne(ctx, endpoint, authorization)
		if err != nil {
			u.logger.Warn(ctx, "Failed to list an upstream MCP server's tools", log.String("upstream", endpoint),
				log.Error(err))
			continue
		}
		for _, tool := range listed {
			if u.local[tool.Name] || owners[tool.Name] != "" {
				continue
			}
			owners[tool.Name] = endpoint
			tools = append(tools, tool)
		}
	}
	u.mu.Lock()
	for name, endpoint := range owners {
		u.owners[name] = endpoint
	}
	u.mu.Unlock()
	return tools
}

func (u *upstreams) listOne(ctx context.Context, endpoint, authorization string) ([]*mcpsdk.Tool, error) {
	ctx, cancel := context.WithTimeout(ctx, upstreamReadTimeout)
	defer cancel()
	session, err := u.connect(ctx, endpoint, authorization, upstreamReadTimeout)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()
	var tools []*mcpsdk.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

// instructionsFor is every upstream's instructions, in the order the upstreams are configured. One not
// read yet is read with the caller's token; one that cannot be read is left out until a later client
// connects.
func (u *upstreams) instructionsFor(ctx context.Context, authorization string) []string {
	var all []string
	for _, endpoint := range u.endpoints {
		u.mu.RLock()
		text, known := u.instructions[endpoint]
		u.mu.RUnlock()
		if !known {
			read, err := u.readInstructions(ctx, endpoint, authorization)
			if err != nil {
				u.logger.Warn(ctx, "Failed to read an upstream MCP server's instructions",
					log.String("upstream", endpoint), log.Error(err))
				continue
			}
			u.mu.Lock()
			u.instructions[endpoint] = read
			u.mu.Unlock()
			text = read
		}
		if text != "" {
			all = append(all, text)
		}
	}
	return all
}

func (u *upstreams) readInstructions(ctx context.Context, endpoint, authorization string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, upstreamReadTimeout)
	defer cancel()
	session, err := u.connect(ctx, endpoint, authorization, upstreamReadTimeout)
	if err != nil {
		return "", err
	}
	defer func() { _ = session.Close() }()
	if result := session.InitializeResult(); result != nil {
		return strings.TrimSpace(result.Instructions), nil
	}
	return "", nil
}

// joinInstructions puts the upstreams' instructions after the server's own, a paragraph apart.
func joinInstructions(own string, added []string) string {
	parts := make([]string, 0, len(added)+1)
	if own = strings.TrimSpace(own); own != "" {
		parts = append(parts, own)
	}
	parts = append(parts, added...)
	return strings.Join(parts, "\n\n")
}

func (u *upstreams) ownerOf(name string) (string, bool) {
	u.mu.RLock()
	defer u.mu.RUnlock()
	endpoint, ok := u.owners[name]
	return endpoint, ok
}

// call forwards a tool call to the upstream that serves the tool and answers with what it answered.
func (u *upstreams) call(ctx context.Context, endpoint, authorization string,
	params *mcpsdk.CallToolParamsRaw) (*mcpsdk.CallToolResult, error) {
	session, err := u.connect(ctx, endpoint, authorization, 0)
	if err != nil {
		u.logger.Warn(ctx, "Failed to reach an upstream MCP server", log.String("upstream", endpoint), log.Error(err))
		return toolFailure(fmt.Sprintf("the tool %s is not available right now", params.Name)), nil
	}
	defer func() { _ = session.Close() }()
	return session.CallTool(ctx, &mcpsdk.CallToolParams{Name: params.Name, Arguments: params.Arguments})
}

// connect opens a session with an upstream as the caller. A timeout bounds each HTTP request of the
// session, the notice the client sends when a request is cancelled included; zero leaves them
// unbounded, as a tool call is.
func (u *upstreams) connect(ctx context.Context, endpoint, authorization string,
	timeout time.Duration) (*mcpsdk.ClientSession, error) {
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "thunderid-mcp", Version: "1.0.0"}, nil)
	transport := &asCaller{base: u.client.Transport, authorization: authorization}
	return client.Connect(ctx, &mcpsdk.StreamableClientTransport{
		Endpoint: endpoint,
		// The caller's token is added below the client's redirect handling, so a redirect would carry it
		// to wherever the upstream points. It is not followed: the upstream answered with a redirect,
		// and is left out as one that cannot be read.
		HTTPClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}, nil)
}

// asCaller sends each request with the caller's Authorization header.
type asCaller struct {
	base          http.RoundTripper
	authorization string
}

func (a *asCaller) RoundTrip(req *http.Request) (*http.Response, error) {
	base := a.base
	if base == nil {
		base = http.DefaultTransport
	}
	if a.authorization == "" {
		return base.RoundTrip(req)
	}
	forwarded := req.Clone(req.Context())
	forwarded.Header.Set("Authorization", a.authorization)
	return base.RoundTrip(forwarded)
}

// authorizationOf is the Authorization header the caller's request carried, which the guard already
// verified.
func authorizationOf(req mcpsdk.Request) string {
	extra := req.GetExtra()
	if extra == nil || extra.Header == nil {
		return ""
	}
	return strings.TrimSpace(extra.Header.Get("Authorization"))
}

func toolFailure(message string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{IsError: true, Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: message}}}
}
