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

// Package dataplane reaches a data plane over the channel it dials the control plane on, presenting
// it as the environment manager's DataPlanes.
package dataplane

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
	envmgrservice "github.com/thunder-id/thunderid/internal/envmgr/service"
	"github.com/thunder-id/thunderid/internal/envmgr/thunder"
	"github.com/thunder-id/thunderid/internal/secretstore"
	"github.com/thunder-id/thunderid/internal/system/channel"
	"github.com/thunder-id/thunderid/internal/system/importer"
)

// ChannelDataPlanes reaches data planes over the connections they hold open to this server.
//
// A data plane is deployed where nothing can reach it, so it dials out and keeps that connection
// alive. Everything the control plane sends it travels back down that connection: there is no
// management URL to call and no client credential to hold. A data plane that is not connected cannot
// be reached at all, which is why For refuses rather than queueing.
type ChannelDataPlanes struct {
	server *channel.Server
}

// New builds a ChannelDataPlanes over the given channel server.
func New(server *channel.Server) *ChannelDataPlanes {
	return &ChannelDataPlanes{server: server}
}

// For returns the named data plane, or an error when it holds no live connection.
func (p *ChannelDataPlanes) For(dataPlaneID string) (envmgrservice.DataPlane, error) {
	if p.server == nil {
		return nil, fmt.Errorf("this server serves no data plane connections")
	}
	if !p.connected(dataPlaneID) {
		return nil, fmt.Errorf("data plane %s is not connected", dataPlaneID)
	}
	return &channelDataPlane{server: p.server, id: dataPlaneID}, nil
}

// Status reports whether the named data plane is connected, and when it was last heard from.
func (p *ChannelDataPlanes) Status(dataPlaneID string) model.DataPlaneStatus {
	if p.server == nil {
		return model.DataPlaneStatus{}
	}
	for _, conn := range p.server.Connections() {
		if conn.ID == dataPlaneID {
			return model.DataPlaneStatus{Connected: true, LastSeen: conn.LastSeen}
		}
	}
	return model.DataPlaneStatus{}
}

func (p *ChannelDataPlanes) connected(dataPlaneID string) bool {
	return p.Status(dataPlaneID).Connected
}

// channelDataPlane is one data plane, addressed by the id it presented when it connected. The id is
// resolved on each call rather than held as a connection, so it stays usable across reconnects.
type channelDataPlane struct {
	server *channel.Server
	id     string
}

// Import applies a bundle on the data plane.
//
// The request is re-encoded rather than copied field by field: the two request types are the same
// wire document, one as the environment manager builds it and one as the import service reads it, and
// converting through JSON keeps a field added to both from being silently dropped here.
func (d *channelDataPlane) Import(ctx context.Context,
	req thunder.ImportRequest) (*thunder.ImportResponse, error) {
	var importReq importer.ImportRequest
	if err := reencode(req, &importReq); err != nil {
		return nil, fmt.Errorf("failed to prepare the import: %w", err)
	}
	resp, err := d.server.CallImport(ctx, d.id, &importReq)
	if err != nil {
		return nil, err
	}
	return toThunderResponse(resp)
}

// reencode converts between two types that describe the same JSON document.
func reencode(from, to any) error {
	raw, err := json.Marshal(from)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, to)
}

// PutSecret writes one secret into the data plane's own store.
func (d *channelDataPlane) PutSecret(ctx context.Context, name string,
	body map[string]interface{}) error {
	secret, err := toSecret(name, body)
	if err != nil {
		return err
	}
	return d.server.CallPutSecret(ctx, d.id, secret)
}

// SecretNames lists what the data plane holds, without any value.
func (d *channelDataPlane) SecretNames(ctx context.Context) ([]string, error) {
	names, _, err := d.server.CallSecretNames(ctx, d.id)
	return names, err
}

// toSecret converts the generic body the capture path builds into the stored form. The body is
// already the document the secret store accepts, so it is re-encoded rather than mapped by hand.
func toSecret(name string, body map[string]interface{}) (secretstore.Secret, error) {
	var secret secretstore.Secret
	if err := reencode(body, &secret); err != nil {
		return secretstore.Secret{}, fmt.Errorf("failed to read the secret: %w", err)
	}
	secret.Name = name
	return secret, nil
}

// toThunderResponse converts the import service's response into the shape the environment manager
// reports, which is the same JSON the HTTP import returns.
func toThunderResponse(resp *importer.ImportResponse) (*thunder.ImportResponse, error) {
	if resp == nil {
		return nil, nil
	}
	var out thunder.ImportResponse
	if err := reencode(resp, &out); err != nil {
		return nil, fmt.Errorf("failed to read the import result: %w", err)
	}
	return &out, nil
}
