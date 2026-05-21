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

package authz

import (
	"context"
	"encoding/json"
	"fmt"

	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type hostAuthRequestStore struct {
	host thunderidengine.RuntimeStore
}

type hostAuthCodeStore struct {
	host thunderidengine.RuntimeStore
}

// NewRequestStoreFromRuntime adapts a host RuntimeStore for authorization request persistence.
func NewRequestStoreFromRuntime(host thunderidengine.RuntimeStore) RequestStoreInterface {
	return &hostAuthRequestStore{host: host}
}

// NewCodeStoreFromRuntime adapts a host RuntimeStore for authorization code persistence.
func NewCodeStoreFromRuntime(host thunderidengine.RuntimeStore) AuthorizationCodeStoreInterface {
	return &hostAuthCodeStore{host: host}
}

func (s *hostAuthRequestStore) AddRequest(ctx context.Context, value authRequestContext) (string, error) {
	pub, err := toPublicAuthRequest(value)
	if err != nil {
		return "", err
	}
	return s.host.AddRequest(ctx, pub)
}

func (s *hostAuthRequestStore) GetRequest(
	ctx context.Context, key string,
) (bool, authRequestContext, error) {
	found, pub, err := s.host.GetRequest(ctx, key)
	if err != nil {
		return false, authRequestContext{}, err
	}
	if !found {
		return false, authRequestContext{}, nil
	}
	internal, err := fromPublicAuthRequest(pub)
	if err != nil {
		return false, authRequestContext{}, err
	}
	return true, internal, nil
}

func (s *hostAuthRequestStore) ClearRequest(ctx context.Context, key string) error {
	return s.host.ClearRequest(ctx, key)
}

func (s *hostAuthCodeStore) InsertAuthorizationCode(ctx context.Context, authzCode AuthorizationCode) error {
	return s.host.InsertAuthorizationCode(ctx, toPublicAuthorizationCode(authzCode))
}

func (s *hostAuthCodeStore) ConsumeAuthorizationCode(ctx context.Context, authCode string) (bool, error) {
	return s.host.ConsumeAuthorizationCode(ctx, authCode)
}

func (s *hostAuthCodeStore) GetAuthorizationCode(
	ctx context.Context, authCode string,
) (*AuthorizationCode, error) {
	pub, err := s.host.GetAuthorizationCode(ctx, authCode)
	if err != nil {
		return nil, err
	}
	if pub == nil {
		return nil, nil
	}
	internal := fromPublicAuthorizationCode(*pub)
	return &internal, nil
}

// PublicAuthCodeFromInternal converts an internal authorization code to the host model.
func PublicAuthCodeFromInternal(code AuthorizationCode) thunderidengine.AuthorizationCode {
	return toPublicAuthorizationCode(code)
}

// InternalAuthCodeFromPublic converts a host authorization code to the internal model.
func InternalAuthCodeFromPublic(pub thunderidengine.AuthorizationCode) AuthorizationCode {
	return fromPublicAuthorizationCode(pub)
}

// PublicAuthRequestFromInternal converts an internal authorization request to the host model.
func PublicAuthRequestFromInternal(value authRequestContext) (thunderidengine.AuthRequestContext, error) {
	return toPublicAuthRequest(value)
}

// InternalAuthRequestFromPublic converts a host authorization request to the internal model.
func InternalAuthRequestFromPublic(pub thunderidengine.AuthRequestContext) (authRequestContext, error) {
	return fromPublicAuthRequest(pub)
}

func toPublicAuthRequest(value authRequestContext) (thunderidengine.AuthRequestContext, error) {
	params, err := marshalOAuthParameters(value.OAuthParameters)
	if err != nil {
		return thunderidengine.AuthRequestContext{}, err
	}
	return thunderidengine.AuthRequestContext{OAuthParameters: params}, nil
}

func fromPublicAuthRequest(pub thunderidengine.AuthRequestContext) (authRequestContext, error) {
	params, err := unmarshalOAuthParameters(pub.OAuthParameters)
	if err != nil {
		return authRequestContext{}, err
	}
	return authRequestContext{OAuthParameters: params}, nil
}

func marshalOAuthParameters(params oauth2model.OAuthParameters) (thunderidengine.OAuthParameters, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal oauth parameters: %w", err)
	}
	return thunderidengine.OAuthParameters(raw), nil
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

func toPublicAuthorizationCode(code AuthorizationCode) thunderidengine.AuthorizationCode {
	claimsRaw, _ := json.Marshal(code.ClaimsRequest)
	return thunderidengine.AuthorizationCode{
		CodeID:              code.CodeID,
		Code:                code.Code,
		ClientID:            code.ClientID,
		RedirectURI:         code.RedirectURI,
		AuthorizedUserID:    code.AuthorizedUserID,
		AttributeCacheID:    code.AttributeCacheID,
		TimeCreated:         code.TimeCreated,
		ExpiryTime:          code.ExpiryTime,
		Scopes:              code.Scopes,
		State:               code.State,
		CodeChallenge:       code.CodeChallenge,
		CodeChallengeMethod: code.CodeChallengeMethod,
		Resources:           code.Resources,
		Nonce:               code.Nonce,
		CompletedACR:        code.CompletedACR,
		ClaimsRequestJSON:   claimsRaw,
	}
}

func fromPublicAuthorizationCode(pub thunderidengine.AuthorizationCode) AuthorizationCode {
	var claims *oauth2model.ClaimsRequest
	if len(pub.ClaimsRequestJSON) > 0 {
		_ = json.Unmarshal(pub.ClaimsRequestJSON, &claims)
	}
	return AuthorizationCode{
		CodeID:              pub.CodeID,
		Code:                pub.Code,
		ClientID:            pub.ClientID,
		RedirectURI:         pub.RedirectURI,
		AuthorizedUserID:    pub.AuthorizedUserID,
		AttributeCacheID:    pub.AttributeCacheID,
		TimeCreated:         pub.TimeCreated,
		ExpiryTime:          pub.ExpiryTime,
		Scopes:              pub.Scopes,
		State:               pub.State,
		CodeChallenge:       pub.CodeChallenge,
		CodeChallengeMethod: pub.CodeChallengeMethod,
		Resources:           pub.Resources,
		Nonce:               pub.Nonce,
		CompletedACR:        pub.CompletedACR,
		ClaimsRequest:       claims,
	}
}
