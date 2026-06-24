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

package authz

import (
	"context"
)

// AuthorizationRequestStoreInterface defines the interface for authorization request storage.
type AuthorizationRequestStoreInterface interface {
	AddRequest(ctx context.Context, value AuthRequestContext) (string, error)
	GetRequest(ctx context.Context, key string) (bool, AuthRequestContext, error)
	ClearRequest(ctx context.Context, key string) error
}

// AuthorizationCodeStoreInterface defines the interface for managing authorization codes.
type AuthorizationCodeStoreInterface interface {
	InsertAuthorizationCode(ctx context.Context, authzCode AuthorizationCode) error
	ConsumeAuthorizationCode(ctx context.Context, authCode string) (bool, error)
	GetAuthorizationCode(ctx context.Context, authCode string) (*AuthorizationCode, error)
}
