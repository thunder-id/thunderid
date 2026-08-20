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

	"github.com/thunder-id/thunderid/internal/secretstore"
)

// Secret-store RPC method names.
//
// A Data Plane's secret store is reached over this channel rather than over its management API,
// because the Data Plane is the side that dials out: it needs no inbound endpoint, and the Control
// Plane needs no credential for it.
const (
	MethodSecretPut   = "Secret.Put"
	MethodSecretNames = "Secret.Names"
	MethodSecretGet   = "Secret.Get"
)

// SecretStore is the subset of the Data Plane's secret store the channel serves. It is satisfied by
// *secretstore.Store.
type SecretStore interface {
	Put(ctx context.Context, secret secretstore.Secret) error
	Get(ctx context.Context, name string) (secretstore.Secret, bool)
	All(ctx context.Context) map[string]secretstore.Secret
	Names(ctx context.Context) []string
}

// secretNamesResult lists what the store holds without any value: enough to tell whether a
// configuration's credentials are all present, without shipping them to something that only needs to
// know they exist.
type secretNamesResult struct {
	Names []string                    `json:"names"`
	Kinds map[string]secretstore.Kind `json:"kinds"`
}

// secretGetParams names the secret to read.
type secretGetParams struct {
	Name string `json:"name"`
}

// secretGetResult carries one secret, with Found distinguishing "not held" from an error.
type secretGetResult struct {
	Found bool   `json:"found"`
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
}

// RegisterSecretMethods registers the Data Plane's secret-store handlers on the router. A Data Plane
// with no store of its own registers nothing, so a Control Plane calling one gets a method-not-found
// rather than a silent success.
func RegisterSecretMethods(router *Router, store SecretStore) {
	if store == nil {
		return
	}

	router.Register(MethodSecretPut, func(ctx context.Context, params json.RawMessage) (json.RawMessage, *Error) {
		var secret secretstore.Secret
		if err := json.Unmarshal(params, &secret); err != nil {
			return nil, NewError(CodeInvalidParams, "invalid secret: "+err.Error())
		}
		if err := store.Put(ctx, secret); err != nil {
			return nil, NewError(CodeInvalidParams, err.Error())
		}
		// The stored value is not echoed back.
		return marshalResult(secret.Redacted())
	})

	router.Register(MethodSecretNames, func(ctx context.Context, _ json.RawMessage) (json.RawMessage, *Error) {
		all := store.All(ctx)
		kinds := make(map[string]secretstore.Kind, len(all))
		for name, secret := range all {
			kinds[name] = secret.Kind
		}
		return marshalResult(secretNamesResult{Names: store.Names(ctx), Kinds: kinds})
	})

	router.Register(MethodSecretGet, func(ctx context.Context, params json.RawMessage) (json.RawMessage, *Error) {
		var p secretGetParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, NewError(CodeInvalidParams, "invalid params: "+err.Error())
		}
		secret, ok := store.Get(ctx, p.Name)
		if !ok {
			return marshalResult(secretGetResult{Found: false})
		}
		return marshalResult(secretGetResult{Found: true, Kind: string(secret.Kind), Value: secret.Value})
	})
}

// marshalResult encodes a handler's reply, turning an encoding failure into an RPC error.
func marshalResult(v any) (json.RawMessage, *Error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, NewError(CodeInternalError, err.Error())
	}
	return raw, nil
}

// DataPlaneSecrets is the Data Plane secret store as the Control Plane uses it. Control Plane code
// depends on this rather than on the channel server, so it needs neither the Data Plane id nor the
// transport, and can be given a test double instead.
type DataPlaneSecrets interface {
	// PutSecret stores or replaces one secret on the Data Plane.
	PutSecret(ctx context.Context, secret secretstore.Secret) error
	// SecretNames lists what the Data Plane holds, with each name's kind and no values.
	SecretNames(ctx context.Context) ([]string, map[string]string, error)
	// GetSecret reads one secret. The boolean reports whether the Data Plane holds it.
	GetSecret(ctx context.Context, name string) (kind, value string, found bool, err error)
}

// CallPutSecret stores a secret on the given Data Plane.
func (s *Server) CallPutSecret(ctx context.Context, dpID string, secret secretstore.Secret) error {
	_, err := s.CallMethod(ctx, dpID, MethodSecretPut, secret)
	return err
}

// CallSecretNames lists the names and kinds the given Data Plane holds.
func (s *Server) CallSecretNames(ctx context.Context, dpID string) ([]string, map[string]string, error) {
	raw, err := s.CallMethod(ctx, dpID, MethodSecretNames, nil)
	if err != nil {
		return nil, nil, err
	}
	var result secretNamesResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, nil, err
	}
	kinds := make(map[string]string, len(result.Kinds))
	for name, kind := range result.Kinds {
		kinds[name] = string(kind)
	}
	return result.Names, kinds, nil
}

// CallGetSecret reads one secret from the given Data Plane.
func (s *Server) CallGetSecret(ctx context.Context, dpID, name string) (string, string, bool, error) {
	raw, err := s.CallMethod(ctx, dpID, MethodSecretGet, secretGetParams{Name: name})
	if err != nil {
		return "", "", false, err
	}
	var result secretGetResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", "", false, err
	}
	return result.Kind, result.Value, result.Found, nil
}

// SecretClient is the DataPlaneSecrets backed by the channel server, bound to a single Data Plane.
// The id is resolved on each call rather than at construction, so a client stays usable across that
// Data Plane's reconnects.
type SecretClient struct {
	server *Server
	dpID   string
}

// NewSecretClient binds a secret client to one Data Plane id.
func NewSecretClient(server *Server, dpID string) *SecretClient {
	return &SecretClient{server: server, dpID: dpID}
}

// DataPlaneID reports the Data Plane this client targets.
func (c *SecretClient) DataPlaneID() string {
	return c.dpID
}

// PutSecret stores a secret on this client's Data Plane.
func (c *SecretClient) PutSecret(ctx context.Context, secret secretstore.Secret) error {
	return c.server.CallPutSecret(ctx, c.dpID, secret)
}

// SecretNames lists what this client's Data Plane holds.
func (c *SecretClient) SecretNames(ctx context.Context) ([]string, map[string]string, error) {
	return c.server.CallSecretNames(ctx, c.dpID)
}

// GetSecret reads one secret from this client's Data Plane.
func (c *SecretClient) GetSecret(ctx context.Context, name string) (string, string, bool, error) {
	return c.server.CallGetSecret(ctx, c.dpID, name)
}
