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

	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type clientBridge struct {
	provider thunderidengine.ClientProvider
}

func newClientBridge(provider thunderidengine.ClientProvider) *clientBridge {
	return &clientBridge{provider: provider}
}

func (b *clientBridge) CreateInboundClient(
	_ context.Context, _ *inboundmodel.InboundClient, _ *inboundmodel.Certificate,
	_ *inboundmodel.OAuthProfile, _ bool, _ string,
) error {
	return errBridgeNotImplemented
}

func (b *clientBridge) GetInboundClientByEntityID(_ context.Context, _ string) (*inboundmodel.InboundClient, error) {
	return nil, errBridgeNotImplemented
}

func (b *clientBridge) GetInboundClientList(_ context.Context) ([]inboundmodel.InboundClient, error) {
	return nil, errBridgeNotImplemented
}

func (b *clientBridge) UpdateInboundClient(
	_ context.Context, _ *inboundmodel.InboundClient, _ *inboundmodel.Certificate,
	_ *inboundmodel.OAuthProfile, _ bool, _ string, _ string,
) error {
	return errBridgeNotImplemented
}

func (b *clientBridge) DeleteInboundClient(_ context.Context, _ string) error {
	return errBridgeNotImplemented
}

func (b *clientBridge) Validate(
	_ context.Context, _ *inboundmodel.InboundClient, _ *inboundmodel.OAuthProfile, _ bool,
) error {
	return errBridgeNotImplemented
}

func (b *clientBridge) ResolveInboundAuthProfileHandles(
	_ context.Context, _ *inboundmodel.InboundAuthProfile,
) error {
	return errBridgeNotImplemented
}

func (b *clientBridge) GetOAuthProfileByEntityID(_ context.Context, _ string) (*inboundmodel.OAuthProfile, error) {
	return nil, errBridgeNotImplemented
}

func (b *clientBridge) GetOAuthClientByClientID(
	ctx context.Context, clientID string,
) (*inboundmodel.OAuthClient, error) {
	if b.provider == nil {
		return nil, errBridgeNotImplemented
	}
	client, err := b.provider.GetOAuthClientByClientID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return toInboundOAuthClient(client), nil
}

func (b *clientBridge) IsDeclarative(_ context.Context, _ string) bool {
	return false
}

func (b *clientBridge) LoadDeclarativeResources(_ context.Context, _ inboundmodel.DeclarativeLoaderConfig) error {
	return errBridgeNotImplemented
}

func (b *clientBridge) GetCertificate(
	_ context.Context, _ cert.CertificateReferenceType, _ string,
) (*inboundmodel.Certificate, *inboundclient.CertOperationError) {
	return nil, &inboundclient.CertOperationError{}
}

var _ inboundclient.InboundClientServiceInterface = (*clientBridge)(nil)
