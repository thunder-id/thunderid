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

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
	"github.com/thunder-id/thunderid/internal/envmgr/thunder"
)

// DataPlane is one data plane as this service uses it.
//
// Every one of these calls travels the connection the data plane itself opened to the control plane.
// A data plane is deployed where nothing can reach it: it has no inbound management endpoint the
// control plane could call, and the control plane holds no credential for it. Naming the deployment
// is all an environment records.
type DataPlane interface {
	Import(ctx context.Context, req thunder.ImportRequest) (*thunder.ImportResponse, error)
	PutSecret(ctx context.Context, name string, body map[string]interface{}) error
	SecretNames(ctx context.Context) ([]string, error)
}

// DataPlanes resolves the data plane an environment names.
type DataPlanes interface {
	// For returns the data plane, or an error when it is not connected. The two are not worth
	// distinguishing to a caller: nothing can be done with a data plane that is not there.
	For(dataPlaneID string) (DataPlane, error)
	// Status reports whether a data plane is connected, for showing an operator what a promotion or
	// an apply would find before they start one.
	Status(dataPlaneID string) model.DataPlaneStatus
}

// SetDataPlanes installs the connections this service reaches data planes over. It is separate from
// New because the channel server is built after this service.
func (s *Service) SetDataPlanes(planes DataPlanes) {
	s.dataPlanes = planes
}

// dataPlaneFor returns the data plane an environment applies to.
//
// The error names the environment rather than the id, because that is what the operator asked for
// and an unnamed or disconnected data plane is something they can act on.
func (s *Service) dataPlaneFor(env model.Environment) (DataPlane, error) {
	id := strings.TrimSpace(env.Target.DataPlaneID)
	if id == "" {
		return nil, fmt.Errorf("%w: %s names no data plane", ErrValidation, env.Name)
	}
	if s.dataPlanes == nil {
		return nil, fmt.Errorf("this server hosts no data plane connections, so %s cannot be reached",
			env.Name)
	}
	plane, err := s.dataPlanes.For(id)
	if err != nil {
		return nil, fmt.Errorf("%s is not reachable: %w", env.Name, err)
	}
	return plane, nil
}

// DataPlaneStatus reports whether an environment's data plane is connected.
func (s *Service) DataPlaneStatus(env model.Environment) model.DataPlaneStatus {
	id := strings.TrimSpace(env.Target.DataPlaneID)
	if id == "" || s.dataPlanes == nil {
		return model.DataPlaneStatus{}
	}
	return s.dataPlanes.Status(id)
}

// DataPlaneTokenIssuer mints the credential a data plane presents when it connects to this control
// plane. It is supplied by the server, which owns where those are kept.
type DataPlaneTokenIssuer interface {
	// Issue returns a new token for the data plane, replacing any previous one. It is readable only
	// here: what is kept is encrypted, and nothing hands it back afterwards.
	Issue(ctx context.Context, dataPlaneID, deploymentID string) (string, error)
}

// SetDataPlaneTokenIssuer installs what mints data plane tokens. Without one, an environment is
// registered with no token, which is how a deployment using a single shared token behaves.
func (s *Service) SetDataPlaneTokenIssuer(issuer DataPlaneTokenIssuer) {
	s.tokens = issuer
}
