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
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const serverLoggerComponent = "ChannelServer"

// serverConn is one accepted Data Plane connection on the Control Plane. It tracks in-flight
// CallMethod requests by id so responses can be matched to their waiters.
type serverConn struct {
	*wsConn
	dpID         string
	instance     string
	lastSeenNano atomic.Int64
	pendingMu    sync.Mutex
	pending      map[string]chan *Response
}

func newServerConn(wc *wsConn, dpID, instance string) *serverConn {
	sc := &serverConn{
		wsConn: wc, dpID: dpID, instance: instance, pending: make(map[string]chan *Response),
	}
	sc.touch()
	return sc
}

func (sc *serverConn) ID() string          { return sc.dpID }
func (sc *serverConn) Instance() string    { return sc.instance }
func (sc *serverConn) LastSeen() time.Time { return time.Unix(0, sc.lastSeenNano.Load()) }
func (sc *serverConn) Close(reason string) {
	_ = sc.wsConn.close(websocket.StatusNormalClosure, reason)
}
func (sc *serverConn) CloseNow() { _ = sc.wsConn.closeNow() }
func (sc *serverConn) touch()    { sc.lastSeenNano.Store(time.Now().UnixNano()) }

func (sc *serverConn) registerPending(id string) chan *Response {
	ch := make(chan *Response, 1)
	sc.pendingMu.Lock()
	sc.pending[id] = ch
	sc.pendingMu.Unlock()
	return ch
}

func (sc *serverConn) unregisterPending(id string) {
	sc.pendingMu.Lock()
	delete(sc.pending, id)
	sc.pendingMu.Unlock()
}

func (sc *serverConn) deliver(resp *Response) {
	sc.pendingMu.Lock()
	ch, ok := sc.pending[resp.ID]
	sc.pendingMu.Unlock()
	if ok {
		select {
		case ch <- resp:
		default:
		}
	}
}

// failAllPending unblocks every waiting caller when the connection drops.
func (sc *serverConn) failAllPending() {
	sc.pendingMu.Lock()
	defer sc.pendingMu.Unlock()
	for id, ch := range sc.pending {
		resp := &Response{JSONRPC: Version, ID: id, Error: NewError(CodeInternalError, "data plane connection closed")}
		select {
		case ch <- resp:
		default:
		}
	}
}

// Server is the Control Plane end of the channel: it accepts Data Plane connections and issues
// reverse JSON-RPC requests to them.
type Server struct {
	cfg      ServerConfig
	verifier Verifier
	registry *Registry[*serverConn]
	logger   *log.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewServer builds a channel server. Its base context is cancelled by Close to stop all read loops.
func NewServer(cfg ServerConfig, verifier Verifier) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:      cfg,
		verifier: verifier,
		registry: NewRegistry[*serverConn](),
		logger:   log.GetLogger().With(log.String(log.LoggerKeyComponentName, serverLoggerComponent)),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// HandleConnect authenticates the handshake, upgrades to WebSocket, registers the connection, and
// runs the response read loop until the connection closes or the server shuts down.
func (s *Server) HandleConnect(w http.ResponseWriter, r *http.Request) {
	dpID := r.Header.Get(HeaderDataPlaneID)
	authenticated, err := s.verifier.Verify(r)
	if err != nil {
		s.logger.Warn(r.Context(), "Rejected data plane handshake",
			log.String("remoteAddr", r.RemoteAddr), log.String("dpID", dpID), log.Error(err))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	// A credential that proves an identity decides which Data Plane this is. Only a shared token,
	// which proves none, leaves the claim in the header to be taken at face value.
	if authenticated != "" {
		dpID = authenticated
	}
	if dpID == "" {
		http.Error(w, errMissingDataPlaneID.Error(), http.StatusBadRequest)
		return
	}

	// Which replica of that Data Plane this connection belongs to. A Data Plane runs as several pods
	// and every one of them dials, so without this they would be one identity and each new connection
	// would evict the last. An older Data Plane that sends none is treated as a single replica.
	instance := r.Header.Get(HeaderDataPlaneInstance)
	if instance == "" {
		instance = defaultInstance
	}

	// sc is assigned only after Accept returns, but OnPingReceived can fire as soon as the
	// connection is accepted, so it must tolerate a nil sc.
	var sc *serverConn
	ws, acceptErr := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The Data Plane is a non-browser client and sends no Origin header, and coder/websocket's
		// default origin check already passes an absent Origin header, so no override is needed here.
		OnPingReceived: func(_ context.Context, _ []byte) bool {
			if sc != nil {
				sc.touch()
			}
			return true
		},
	})
	if acceptErr != nil {
		return // Accept has already written the error response.
	}
	sc = newServerConn(newWSConn(ws, s.cfg.ReadLimit), dpID, instance)
	s.registry.Register(sc)
	s.logger.Info(s.ctx, "Data plane connected",
		log.String("dpID", dpID), log.String("instance", instance))

	defer func() {
		s.registry.Unregister(dpID, sc)
		sc.failAllPending()
		_ = sc.closeNow()
		s.logger.Info(s.ctx, "Data plane disconnected",
			log.String("dpID", dpID), log.String("instance", instance))
	}()

	for {
		var resp Response
		if err := sc.readMessage(s.ctx, &resp); err != nil {
			if s.ctx.Err() == nil {
				s.logger.Debug(s.ctx, "Data plane read loop ended", log.String("dpID", dpID), log.Error(err))
			}
			return
		}
		sc.touch()
		sc.deliver(&resp)
	}
}

// CallMethod sends a JSON-RPC request to the given Data Plane and waits for its tagged response. It
// fails fast with ErrDataPlaneNotConnected when the Data Plane is not connected.
func (s *Server) CallMethod(ctx context.Context, dpID, method string, params any) (json.RawMessage, error) {
	sc, ok := s.registry.Get(dpID)
	if !ok {
		return nil, ErrDataPlaneNotConnected
	}

	// Apply the configured RPC timeout as a fallback only when the caller set no deadline.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && s.cfg.RPCTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.RPCTimeout)
		defer cancel()
	}

	// A nil params argument (for example Agent.Ping) must marshal to a nil json.RawMessage so the
	// Params field is omitted from the frame entirely, rather than marshaling to the literal null.
	var paramsRaw json.RawMessage
	if params != nil {
		raw, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		paramsRaw = raw
	}
	id := utils.GenerateUUID()
	respCh := sc.registerPending(id)
	defer sc.unregisterPending(id)

	if err := sc.writeMessage(ctx, Request{JSONRPC: Version, ID: id, Method: method, Params: paramsRaw}); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

// Connections returns a snapshot of connected Data Planes for observability/health.
func (s *Server) Connections() []ConnInfo {
	return s.registry.List()
}

// Close closes every active connection concurrently, each with a normal closure so its Data Plane
// receives a proper close frame, then cancels the base context to stop any remaining read loops.
// Connections are closed in parallel so that shutdown time does not scale with the number of
// connected Data Planes (each graceful close can take up to several seconds against an unresponsive
// peer). Safe to call repeatedly: a later call simply closes an emptier registry and re-cancels an
// already-cancelled context.
func (s *Server) Close() {
	var wg sync.WaitGroup
	for _, c := range s.registry.entries() {
		wg.Add(1)
		go func(c *serverConn) {
			defer wg.Done()
			c.Close("control plane shutting down")
		}(c)
	}
	wg.Wait()
	s.cancel()
}
