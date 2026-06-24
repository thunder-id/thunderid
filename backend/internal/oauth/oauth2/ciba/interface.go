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

package ciba

import (
	"context"
	"time"
)

// CIBARequestStoreInterface defines the interface for CIBA authentication request storage.
type CIBARequestStoreInterface interface {
	Add(ctx context.Context, request *CIBAAuthRequest) error
	GetByID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error)
	MarkAuthenticated(ctx context.Context, authReqID, userID, authorizedScopes, attributeCacheID,
		completedACR string, authTime time.Time) error
	MarkConsumed(ctx context.Context, authReqID string) (bool, error)
	UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error
	UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error
}
