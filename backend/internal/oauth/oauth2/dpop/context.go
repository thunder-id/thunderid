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

package dpop

import "context"

// contextKey is a private type for DPoP context value keys to avoid collisions.
type contextKey string

// Context keys for DPoP values propagated across the request pipeline.
const (
	proofKey contextKey = "dpop_proof"
	jktKey   contextKey = "dpop_jkt"
)

// WithProof attaches a raw DPoP proof JWT to the context for downstream verification.
func WithProof(ctx context.Context, proof string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, proofKey, proof)
}

// GetProof returns the raw DPoP proof JWT previously attached via WithProof, or "".
func GetProof(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(proofKey).(string); ok {
		return v
	}
	return ""
}

// WithJkt attaches the verified DPoP proof's JWK thumbprint to the context so grant
// handlers can sender-constrain the issued tokens.
func WithJkt(ctx context.Context, jkt string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, jktKey, jkt)
}

// GetJkt returns the verified DPoP jkt previously attached via WithJkt, or "".
func GetJkt(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(jktKey).(string); ok {
		return v
	}
	return ""
}
