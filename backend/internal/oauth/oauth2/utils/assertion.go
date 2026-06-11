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

package utils

import (
	"errors"
	"fmt"
	"time"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// FlowAssertionClaims holds the common claims extracted from a flow assertion JWT.
// Both the authorization code callback and the CIBA callback decode these base claims;
// each path then reads its own additional claims from the same payload.
type FlowAssertionClaims struct {
	UserID           string
	AttributeCacheID string
	CompletedACR     string
	AuthTime         time.Time
}

// DecodeFlowAssertionClaims decodes the common flow assertion claims from a JWT string.
// It extracts sub (user ID), aci (attribute cache ID), completed_auth_class (completed ACR),
// and iat (authentication time). The raw JWT payload is also returned so callers can extract
// grant-type-specific claims (e.g. ciba_auth_req_id for CIBA, authorized_permissions for auth code).
func DecodeFlowAssertionClaims(assertion string) (FlowAssertionClaims, map[string]interface{}, error) {
	claims := FlowAssertionClaims{}

	_, jwtPayload, err := jwt.DecodeJWT(assertion)
	if err != nil {
		return claims, nil, fmt.Errorf("failed to decode the JWT token: %w", err)
	}

	if iatValue, ok := jwtPayload["iat"]; ok {
		switch v := iatValue.(type) {
		case float64:
			claims.AuthTime = time.Unix(int64(v), 0)
		case int64:
			claims.AuthTime = time.Unix(v, 0)
		case int:
			claims.AuthTime = time.Unix(int64(v), 0)
		default:
			return claims, nil, errors.New("JWT 'iat' claim has unexpected type")
		}
	}

	if subValue, ok := jwtPayload[oauth2const.ClaimSub]; ok {
		strValue, ok := subValue.(string)
		if !ok {
			return claims, nil, errors.New("JWT 'sub' claim is not a string")
		}
		claims.UserID = strValue
	}

	if aciValue, ok := jwtPayload["aci"]; ok {
		strValue, ok := aciValue.(string)
		if !ok {
			return claims, nil, errors.New("JWT 'aci' claim is not a string")
		}
		claims.AttributeCacheID = strValue
	}

	if acrValue, ok := jwtPayload[oauth2const.ClaimCompletedAuthClass]; ok {
		strValue, ok := acrValue.(string)
		if !ok {
			return claims, nil, errors.New("JWT 'completed_auth_class' claim is not a string")
		}
		claims.CompletedACR = strValue
	}

	return claims, jwtPayload, nil
}
