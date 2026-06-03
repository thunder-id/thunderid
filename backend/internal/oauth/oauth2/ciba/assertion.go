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
	"errors"
	"fmt"
	"time"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
)

// assertionClaims represents the claims extracted from the flow assertion JWT.
type assertionClaims struct {
	userID           string
	attributeCacheID string
	completedACR     string
	cibaAuthReqID    string
}

// decodeAttributesFromAssertion decodes the relevant claims from the flow assertion JWT.
// It mirrors the decode logic used by the authorize callback: sub, aci, completed_auth_class, iat.
func decodeAttributesFromAssertion(assertion string) (assertionClaims, time.Time, error) {
	claims := assertionClaims{}

	_, jwtPayload, err := jwt.DecodeJWT(assertion)
	if err != nil {
		return claims, time.Time{}, fmt.Errorf("failed to decode the JWT token: %w", err)
	}

	authTime := time.Time{}
	if iatValue, ok := jwtPayload[oauth2const.ClaimIat]; ok {
		switch v := iatValue.(type) {
		case float64:
			authTime = time.Unix(int64(v), 0)
		case int64:
			authTime = time.Unix(v, 0)
		case int:
			authTime = time.Unix(int64(v), 0)
		default:
			return claims, time.Time{}, errors.New("JWT 'iat' claim has unexpected type")
		}
	}

	if subValue, ok := jwtPayload[oauth2const.ClaimSub]; ok {
		strValue, ok := subValue.(string)
		if !ok {
			return claims, time.Time{}, errors.New("JWT 'sub' claim is not a string")
		}
		claims.userID = strValue
	}

	if aciValue, ok := jwtPayload["aci"]; ok {
		strValue, ok := aciValue.(string)
		if !ok {
			return claims, time.Time{}, errors.New("JWT 'aci' claim is not a string")
		}
		claims.attributeCacheID = strValue
	}

	if acrValue, ok := jwtPayload[oauth2const.ClaimCompletedAuthClass]; ok {
		strValue, ok := acrValue.(string)
		if !ok {
			return claims, time.Time{}, errors.New("JWT 'completed_auth_class' claim is not a string")
		}
		claims.completedACR = strValue
	}

	if cibaValue, ok := jwtPayload[oauth2const.ClaimCIBAAuthReqID]; ok {
		strValue, ok := cibaValue.(string)
		if !ok {
			return claims, time.Time{}, errors.New("JWT 'ciba_auth_req_id' claim is not a string")
		}
		claims.cibaAuthReqID = strValue
	}

	return claims, authTime, nil
}
