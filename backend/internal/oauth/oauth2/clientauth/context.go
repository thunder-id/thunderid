/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package clientauth

import (
	"context"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
)

type contextKey string

// OAuthClientKey is the context key for storing authenticated OAuth client info.
var OAuthClientKey contextKey = "oauth_client"

// OAuthClientInfo contains authenticated client information.
type OAuthClientInfo struct {
	ClientID     string
	ClientSecret string
	OAuthApp     *inboundmodel.OAuthClient
}

// withOAuthClient adds OAuth client information to the context.
func withOAuthClient(ctx context.Context, client *OAuthClientInfo) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, OAuthClientKey, client)
}

// GetOAuthClient retrieves OAuth client information from the context.
func GetOAuthClient(ctx context.Context) *OAuthClientInfo {
	if ctx == nil {
		return nil
	}

	if client, ok := ctx.Value(OAuthClientKey).(*OAuthClientInfo); ok {
		return client
	}

	return nil
}
