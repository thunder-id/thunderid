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

package envmgr

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/thunder-id/thunderid/internal/envmgr/service"
	"github.com/thunder-id/thunderid/internal/envmgr/store"
	"github.com/thunder-id/thunderid/internal/envmgr/thunder"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// registry hands each deployment its own environment manager, each scoped to its own rows in the
// environment database.
//
// One store per deployment means a caller cannot name another deployment's environment at all: every
// query carries the deployment, so the id simply does not resolve in the store the request is served
// from. That is the same guarantee the row-scoped resources get.
type registry struct {
	hasher       service.SecretHasher
	workspaceURL string
	dataPlanes   service.DataPlanes
	tokenIssuer  service.DataPlaneTokenIssuer
	sealer       service.SecretSealer

	mu      sync.Mutex
	servers map[string]*Server
}

func newRegistry(hasher service.SecretHasher) *registry {
	return &registry{hasher: hasher, servers: make(map[string]*Server)}
}

// serverFor returns the deployment's environment manager, building it on first use.
func (r *registry) serverFor(ctx context.Context) (*Server, error) {
	return r.serverForID(deployment.ResolveDefault(ctx))
}

// storeKeyFor returns the store an environment belongs in.
//
// An organization's environments share one store, because promotion is a relationship between them:
// each has to see the others to be promoted to, and a credential captured in one has to reach that
// one's data plane. A deployment id names its organization ("<org>:<env>"), so the organization is
// the key. An id that names no organization is its own store, which is what a deployment provisioned
// before organizations existed keeps.
func storeKeyFor(deploymentID string) string {
	org, _, found := strings.Cut(deploymentID, ":")
	if !found || strings.TrimSpace(org) == "" {
		return deploymentID
	}
	return org
}

// serverForID returns a named deployment's environment manager. It exists for the capture path, which
// knows the deployment the credential belongs to without a request to resolve it from.
func (r *registry) serverForID(id string) (*Server, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("no deployment in context, so the environment manager cannot be scoped")
	}
	id = storeKeyFor(id)

	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.servers[id]; ok {
		return existing, nil
	}

	st, err := store.New(id)
	if err != nil {
		return nil, err
	}
	svc := service.New(st, func(baseURL string, creds thunder.Credentials, insecure bool) service.ThunderClient {
		return thunder.New(baseURL, creds, insecure)
	})
	svc.SetSecretHasher(r.hasher)
	svc.SetSecretSealer(r.sealer)
	svc.SetWorkspaceURL(r.workspaceURL)
	svc.SetOrganization(id)
	svc.SetDataPlanes(r.dataPlanes)
	svc.SetDataPlaneTokenIssuer(r.tokenIssuer)
	server := New(svc)
	r.servers[id] = server
	return server, nil
}

// CreateEnvironment registers an environment in the store its deployment belongs to, so a tenant
// appears in its organization's promotion chain without a second call to set it up.
func (r *registry) CreateEnvironment(ctx context.Context, deploymentID string,
	in service.CreateEnvironmentInput) (service.CreateEnvironmentResult, error) {
	server, err := r.serverForID(deploymentID)
	if err != nil {
		return service.CreateEnvironmentResult{}, err
	}
	return server.svc.CreateEnvironment(ctx, in)
}

// SetDataPlaneTokenIssuer installs what mints the credential a data plane connects with. It is set
// after the fact because it is built alongside the channel server.
func (r *registry) SetDataPlaneTokenIssuer(issuer service.DataPlaneTokenIssuer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokenIssuer = issuer
	for _, server := range r.servers {
		server.svc.SetDataPlaneTokenIssuer(issuer)
	}
}

// SetSecretSealer installs what encrypts a credential queued for a data plane. It is set after the
// fact because the server's crypto is built after the environment manager is mounted.
func (r *registry) SetSecretSealer(sealer service.SecretSealer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sealer = sealer
	for _, server := range r.servers {
		server.svc.SetSecretSealer(sealer)
	}
}

// DeliverPending carries out queued work for a data plane, if this pod holds its connection. The
// data plane id names its organization, which is the store the work was queued in.
func (r *registry) DeliverPending(ctx context.Context, dataPlaneID string) error {
	server, err := r.serverForID(dataPlaneID)
	if err != nil {
		return err
	}
	return server.svc.DeliverNext(ctx, dataPlaneID)
}

// SetDataPlanes installs the connections this server reaches data planes over. It is set after the
// fact because the channel server is built after the environment manager is mounted.
func (r *registry) SetDataPlanes(planes service.DataPlanes) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dataPlanes = planes
	for _, server := range r.servers {
		server.svc.SetDataPlanes(planes)
	}
}

// SetWorkspaceURL installs the address of the control plane these managers run in, which is the
// organization workspace a capture reads. It is set after the fact because it is resolved from the
// server's configuration after the environment manager is mounted.
func (r *registry) SetWorkspaceURL(baseURL string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaceURL = baseURL
	for _, server := range r.servers {
		server.svc.SetWorkspaceURL(baseURL)
	}
}

func (r *registry) handler(pick func(*Server) http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		server, err := r.serverFor(req.Context())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		pick(server)(w, req)
	}
}

// CaptureSecret hands a captured credential to the deployment's environment manager, which routes it to
// every data plane registered for that deployment.
//
// This is the same work the HTTP capture route does, reached without a request: the control plane and
// the environment manager are one process here, so a self-call over HTTP would only add an authenticated
// round trip to itself.
func (r *registry) CaptureSecret(ctx context.Context, deploymentID, name string,
	body map[string]interface{}) (int, error) {
	server, err := r.serverForID(deploymentID)
	if err != nil {
		return 0, err
	}
	return server.svc.CaptureSecretForTenant(ctx, deploymentID, name, body)
}
