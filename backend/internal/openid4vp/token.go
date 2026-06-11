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

package openid4vp

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// resultTokenIssuer issues a signed result token for a completed verification.
type resultTokenIssuer interface {
	issueResultToken(ctx context.Context, rpID string, rs *RequestState, validitySeconds int64) (string, error)
}

// jwtResultTokenIssuer signs result tokens with the server's JWT service so
// the token is verifiable against Thunder's published JWKS.
type jwtResultTokenIssuer struct {
	jwt      jwt.JWTServiceInterface
	issuer   string
	clientID string
}

func newJWTresultTokenIssuer(svc jwt.JWTServiceInterface, issuer, clientID string) resultTokenIssuer {
	return &jwtResultTokenIssuer{jwt: svc, issuer: issuer, clientID: clientID}
}

func (i *jwtResultTokenIssuer) issueResultToken(
	ctx context.Context, rpID string, rs *RequestState, validitySeconds int64,
) (string, error) {
	if rs == nil {
		return "", fmt.Errorf("%w: request state is required to issue a result token", ErrPolicy)
	}
	if rs.Status != StatusCompleted || rs.Result == nil {
		return "", fmt.Errorf("%w: result token can only be issued for completed verifications", ErrPolicy)
	}
	if i.jwt == nil {
		return "", fmt.Errorf("%w: jwt service is not configured", ErrPolicy)
	}

	claims := map[string]interface{}{
		"aud":             rpID,
		"txn":             rs.State,
		"definition_id":   rs.DefinitionID,
		"subject":         rs.Result.Subject,
		"verified_claims": rs.Result.Claims,
		"verifier":        i.clientID,
	}

	token, _, svcErr := i.jwt.GenerateJWT(ctx, rs.Result.Subject, i.issuer, validitySeconds, claims, "JWT", "")
	if svcErr != nil {
		return "", fmt.Errorf("failed to sign result token: %s", svcErr.Code)
	}
	return token, nil
}
