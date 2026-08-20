/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package channel

import (
	"context"
	"encoding/json"
	"sync"
)

// HandlerFunc handles one JSON-RPC method. params is the raw request params; the return values are
// the raw result or a JSON-RPC error (exactly one is non-nil).
type HandlerFunc func(ctx context.Context, params json.RawMessage) (json.RawMessage, *Error)

// Router maps JSON-RPC method names to handlers on the Data Plane.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]HandlerFunc
}

// NewRouter creates an empty router.
func NewRouter() *Router {
	return &Router{handlers: make(map[string]HandlerFunc)}
}

// Register binds a handler to a method name. A later registration for the same method replaces the
// earlier one.
func (r *Router) Register(method string, h HandlerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[method] = h
}

// Dispatch routes req to its handler and builds the response frame. Unknown methods return a
// MethodNotFound error response.
func (r *Router) Dispatch(ctx context.Context, req Request) Response {
	r.mu.RLock()
	h, ok := r.handlers[req.Method]
	r.mu.RUnlock()
	if !ok {
		notFound := NewError(CodeMethodNotFound, "method not found: "+req.Method)
		return Response{JSONRPC: Version, ID: req.ID, Error: notFound}
	}
	result, rpcErr := h(ctx, req.Params)
	if rpcErr != nil {
		return Response{JSONRPC: Version, ID: req.ID, Error: rpcErr}
	}
	return Response{JSONRPC: Version, ID: req.ID, Result: result}
}
