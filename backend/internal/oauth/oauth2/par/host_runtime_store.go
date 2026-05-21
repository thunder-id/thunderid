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

package par

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type hostRuntimeStore struct {
	host thunderidengine.RuntimeStore
}

// NewStoreFromRuntime adapts a host RuntimeStore to the internal PAR store interface.
func NewStoreFromRuntime(host thunderidengine.RuntimeStore) StoreInterface {
	return &hostRuntimeStore{host: host}
}

func (s *hostRuntimeStore) Store(
	ctx context.Context, request pushedAuthorizationRequest, expirySeconds int64,
) (string, error) {
	pub, err := toPublicPARRequest(request)
	if err != nil {
		return "", err
	}
	requestURI, err := s.host.Store(ctx, pub, expirySeconds)
	if err != nil {
		return "", err
	}
	return randomKeyFromRequestURI(requestURI)
}

func (s *hostRuntimeStore) Consume(
	ctx context.Context, randomKey string,
) (pushedAuthorizationRequest, bool, error) {
	pub, found, err := s.host.Consume(ctx, requestURIPrefix+randomKey)
	if err != nil {
		return pushedAuthorizationRequest{}, false, err
	}
	if !found {
		return pushedAuthorizationRequest{}, false, nil
	}
	internal, err := fromPublicPARRequest(pub)
	if err != nil {
		return pushedAuthorizationRequest{}, false, err
	}
	return internal, true, nil
}

// PublicFromInternal converts an internal stored PAR request to the host model.
func PublicFromInternal(request pushedAuthorizationRequest) (thunderidengine.PushedAuthorizationRequest, error) {
	return toPublicPARRequest(request)
}

// InternalFromPublic converts a host PAR request to the internal stored model.
func InternalFromPublic(pub thunderidengine.PushedAuthorizationRequest) (pushedAuthorizationRequest, error) {
	return fromPublicPARRequest(pub)
}

func toPublicPARRequest(request pushedAuthorizationRequest) (thunderidengine.PushedAuthorizationRequest, error) {
	params, err := marshalOAuthParameters(request.OAuthParameters)
	if err != nil {
		return thunderidengine.PushedAuthorizationRequest{}, err
	}
	return thunderidengine.PushedAuthorizationRequest{
		ClientID:        request.ClientID,
		OAuthParameters: params,
	}, nil
}

func fromPublicPARRequest(pub thunderidengine.PushedAuthorizationRequest) (pushedAuthorizationRequest, error) {
	params, err := unmarshalOAuthParameters(pub.OAuthParameters)
	if err != nil {
		return pushedAuthorizationRequest{}, err
	}
	return pushedAuthorizationRequest{
		ClientID:        pub.ClientID,
		OAuthParameters: params,
	}, nil
}

func marshalOAuthParameters(params oauth2model.OAuthParameters) (thunderidengine.OAuthParameters, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal oauth parameters: %w", err)
	}
	return thunderidengine.OAuthParameters(raw), nil
}

func randomKeyFromRequestURI(requestURI string) (string, error) {
	if !strings.HasPrefix(requestURI, requestURIPrefix) {
		return "", fmt.Errorf("invalid request_uri prefix")
	}
	randomKey := strings.TrimPrefix(requestURI, requestURIPrefix)
	if randomKey == "" {
		return "", fmt.Errorf("empty request_uri key")
	}
	return randomKey, nil
}

func unmarshalOAuthParameters(raw thunderidengine.OAuthParameters) (oauth2model.OAuthParameters, error) {
	if len(raw) == 0 {
		return oauth2model.OAuthParameters{}, nil
	}
	var params oauth2model.OAuthParameters
	if err := json.Unmarshal(raw, &params); err != nil {
		return oauth2model.OAuthParameters{}, fmt.Errorf("unmarshal oauth parameters: %w", err)
	}
	return params, nil
}
