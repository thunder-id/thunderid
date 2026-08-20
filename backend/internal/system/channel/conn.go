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
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// wsConn wraps a coder/websocket connection. coder/websocket permits only one concurrent writer, so
// writeMessage serializes all writes behind a mutex; reads happen only from a single read loop.
type wsConn struct {
	ws      *websocket.Conn
	writeMu sync.Mutex
}

func newWSConn(ws *websocket.Conn, readLimit int64) *wsConn {
	if readLimit > 0 {
		ws.SetReadLimit(readLimit)
	}
	return &wsConn{ws: ws}
}

func (c *wsConn) writeMessage(ctx context.Context, v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return wsjson.Write(ctx, c.ws, v)
}

func (c *wsConn) readMessage(ctx context.Context, v any) error {
	return wsjson.Read(ctx, c.ws, v)
}

func (c *wsConn) ping(ctx context.Context) error {
	return c.ws.Ping(ctx)
}

func (c *wsConn) close(code websocket.StatusCode, reason string) error {
	return c.ws.Close(code, reason)
}

func (c *wsConn) closeNow() error {
	return c.ws.CloseNow()
}
