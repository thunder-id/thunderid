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

package security

import (
	"context"
	"testing"
)

// NewSecurityContextForTest creates a new immutable SecurityContext.
// Used for testing purposes.
func NewSecurityContextForTest(userID, ouID, token string,
	permissions []string, attributes map[string]interface{}) *SecurityContext {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return &SecurityContext{
		subject:     userID,
		ouID:        ouID,
		token:       token,
		permissions: permissions,
		attributes:  attributes,
	}
}

// WithSecurityContextTest adds security context to the request context.
// Used for testing purposes.
func WithSecurityContextTest(ctx context.Context, authCtx *SecurityContext) context.Context {
	if !testing.Testing() {
		panic("only for tests!")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, securityContextKey, authCtx)
}
