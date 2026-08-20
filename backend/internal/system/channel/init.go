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
	"net/http"
	"strings"
)

// InitializeServer builds the Control Plane channel server and, when enabled, registers its
// WebSocket route on mux. It returns the server (whose Close the caller invokes at shutdown), or nil
// when disabled. An empty cfg.Path defaults to "/cp/connect", and a path missing its leading slash is
// normalized so it cannot be misparsed as a host pattern by http.ServeMux.
func InitializeServer(mux *http.ServeMux, cfg ServerConfig, tokens TokenStore) *Server {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Path == "" {
		cfg.Path = "/cp/connect"
	}
	if !strings.HasPrefix(cfg.Path, "/") {
		cfg.Path = "/" + cfg.Path
	}
	s := NewServer(cfg, serverVerifier(cfg, tokens))
	mux.HandleFunc("GET "+cfg.Path, s.HandleConnect)
	return s
}

// serverVerifier picks how a handshake is authenticated. Issued per-Data-Plane tokens win over the
// configured shared one, because they are the only form that tells the server which Data Plane
// connected.
func serverVerifier(cfg ServerConfig, tokens TokenStore) Verifier {
	if tokens != nil {
		return newStoredTokenVerifier(tokens)
	}
	return newTokenVerifier(cfg.AuthToken)
}

// InitializeClient builds the Data Plane channel client with the Import, Ping and secret-store
// handlers registered, or returns nil when disabled. The caller owns Start/Stop.
//
// A nil store registers no secret handlers, so a Data Plane that serves no store of its own answers
// method-not-found rather than pretending to hold secrets.
func InitializeClient(cfg ClientConfig, runner ImportRunner, store SecretStore) *Client {
	if !cfg.Enabled {
		return nil
	}
	router := NewRouter()
	RegisterDataPlaneMethods(router, runner, cfg.ID)
	RegisterSecretMethods(router, store)
	return NewClient(cfg, router)
}
