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

package enginebridge

import (
	"context"
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauthauthz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

const parRequestURIPrefix = "urn:ietf:params:oauth:request_uri:"

type defaultRuntimeStore struct {
	stores RuntimeStores
}

func newDefaultRuntimeStore() (thunderidengine.RuntimeStore, error) {
	stores, err := newDefaultInternalRuntimeStores()
	if err != nil {
		return nil, err
	}
	return &defaultRuntimeStore{stores: stores}, nil
}

func newDefaultInternalRuntimeStores() (RuntimeStores, error) {
	flowStore, err := flowexec.NewDefaultContextStoreFromConfig()
	if err != nil {
		return RuntimeStores{}, err
	}
	return RuntimeStores{
		PAR:         par.NewDefaultStore(),
		AuthCode:    oauthauthz.NewDefaultCodeStore(),
		AuthRequest: oauthauthz.NewDefaultRequestStore(),
		FlowContext: flowStore,
	}, nil
}

func (s *defaultRuntimeStore) Store(
	ctx context.Context, parRequest thunderidengine.PushedAuthorizationRequest, expirySeconds int64,
) (string, error) {
	internal, err := par.InternalFromPublic(parRequest)
	if err != nil {
		return "", err
	}
	randomKey, err := s.stores.PAR.Store(ctx, internal, expirySeconds)
	if err != nil {
		return "", err
	}
	return parRequestURIPrefix + randomKey, nil
}

func (s *defaultRuntimeStore) Consume(
	ctx context.Context, requestURI string,
) (thunderidengine.PushedAuthorizationRequest, bool, error) {
	if !strings.HasPrefix(requestURI, parRequestURIPrefix) {
		return thunderidengine.PushedAuthorizationRequest{}, false, nil
	}
	randomKey := strings.TrimPrefix(requestURI, parRequestURIPrefix)
	internal, found, err := s.stores.PAR.Consume(ctx, randomKey)
	if err != nil || !found {
		return thunderidengine.PushedAuthorizationRequest{}, found, err
	}
	pub, err := par.PublicFromInternal(internal)
	return pub, true, err
}

func (s *defaultRuntimeStore) AddRequest(
	ctx context.Context, authRequestContext thunderidengine.AuthRequestContext,
) (string, error) {
	internal, err := oauthauthz.InternalAuthRequestFromPublic(authRequestContext)
	if err != nil {
		return "", err
	}
	return s.stores.AuthRequest.AddRequest(ctx, internal)
}

func (s *defaultRuntimeStore) GetRequest(
	ctx context.Context, key string,
) (bool, thunderidengine.AuthRequestContext, error) {
	found, internal, err := s.stores.AuthRequest.GetRequest(ctx, key)
	if err != nil || !found {
		return found, thunderidengine.AuthRequestContext{}, err
	}
	pub, err := oauthauthz.PublicAuthRequestFromInternal(internal)
	return true, pub, err
}

func (s *defaultRuntimeStore) ClearRequest(ctx context.Context, key string) error {
	return s.stores.AuthRequest.ClearRequest(ctx, key)
}

func (s *defaultRuntimeStore) InsertAuthorizationCode(
	ctx context.Context, code thunderidengine.AuthorizationCode,
) error {
	return s.stores.AuthCode.InsertAuthorizationCode(ctx, oauthauthz.InternalAuthCodeFromPublic(code))
}

func (s *defaultRuntimeStore) ConsumeAuthorizationCode(ctx context.Context, authCodeString string) (bool, error) {
	return s.stores.AuthCode.ConsumeAuthorizationCode(ctx, authCodeString)
}

func (s *defaultRuntimeStore) GetAuthorizationCode(
	ctx context.Context, authCodeString string,
) (*thunderidengine.AuthorizationCode, error) {
	internal, err := s.stores.AuthCode.GetAuthorizationCode(ctx, authCodeString)
	if err != nil || internal == nil {
		return nil, err
	}
	pub := oauthauthz.PublicAuthCodeFromInternal(*internal)
	return &pub, nil
}

func (s *defaultRuntimeStore) StoreFlowContext(
	ctx context.Context, flowContext thunderidengine.FlowContext, expirySeconds int64,
) error {
	return s.stores.FlowContext.StoreFlowContext(
		ctx, flowexec.InternalFlowContextFromPublic(flowContext), expirySeconds)
}

func (s *defaultRuntimeStore) GetFlowContext(
	ctx context.Context, executionID string,
) (*thunderidengine.FlowContext, error) {
	internal, err := s.stores.FlowContext.GetFlowContext(ctx, executionID)
	if err != nil || internal == nil {
		return nil, err
	}
	pub := flowexec.PublicFlowContextFromInternal(*internal)
	return &pub, nil
}

func (s *defaultRuntimeStore) UpdateFlowContext(
	ctx context.Context, flowContext thunderidengine.FlowContext,
) error {
	return s.stores.FlowContext.UpdateFlowContext(ctx, flowexec.InternalFlowContextFromPublic(flowContext))
}

func (s *defaultRuntimeStore) DeleteFlowContext(ctx context.Context, executionID string) error {
	return s.stores.FlowContext.DeleteFlowContext(ctx, executionID)
}
