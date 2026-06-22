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

package oauth

import (
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// RuntimeStores bundles the runtime persistence backends required by the OAuth
// services. The composition root selects the SQL or Redis implementation for each
// store and passes them in, so the oauth package only depends on the store
// interfaces and never transitively links the SQL database drivers.
type RuntimeStores struct {
	// JTI is the JWT jti replay cache used by DPoP and client-assertion validation.
	JTI jti.JTIStoreInterface
	// CIBA is the backchannel authentication request store.
	CIBA ciba.CIBARequestStoreInterface
	// PAR is the pushed authorization request store.
	PAR par.PARStoreInterface
	// AuthzCode is the authorization code store.
	AuthzCode authz.AuthorizationCodeStoreInterface
	// AuthzRequest is the authorization request context store.
	AuthzRequest authz.AuthorizationRequestStoreInterface
	// AuthzTransactioner spans the authorization code/request store operations.
	AuthzTransactioner transaction.Transactioner
}
