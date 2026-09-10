// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"
	"time"

	flowcm "github.com/thunder-id/thunderid/internal/flow/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
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

// FlowErrorAssertionClaims holds the claims of a flow error assertion, minted by the flow service
// when an OAuth-initiated flow terminates in failure.
type FlowErrorAssertionClaims struct {
	AuthorizationRequestID string
	ErrorType              string
	Description            string
}

// DecodeFlowErrorAssertionClaims decodes a flow error assertion JWT. Callers must verify the
// signature and the authorization request binding before acting on the claims.
func DecodeFlowErrorAssertionClaims(assertion string) (FlowErrorAssertionClaims, error) {
	claims := FlowErrorAssertionClaims{}

	_, jwtPayload, err := jwt.DecodeJWT(assertion)
	if err != nil {
		return claims, fmt.Errorf("failed to decode the JWT token: %w", err)
	}

	claims.AuthorizationRequestID, _ = jwtPayload[flowcm.ClaimAuthorizationRequestID].(string)
	claims.ErrorType, _ = jwtPayload[flowcm.ClaimFlowErrorType].(string)
	claims.Description, _ = jwtPayload[flowcm.ClaimFlowErrorDescription].(string)

	return claims, nil
}

// DecodeFlowAssertionClaims decodes the common flow assertion claims from a JWT string.
// It extracts sub (user ID), aci (attribute cache ID), completed_auth_class (completed ACR),
// and iat (authentication time). The raw JWT payload is also returned so callers can extract
// grant-type-specific claims (e.g. auth_req_id for CIBA, authorized_permissions for auth code).
func DecodeFlowAssertionClaims(assertion string) (FlowAssertionClaims, map[string]interface{}, error) {
	claims := FlowAssertionClaims{}

	_, jwtPayload, err := jwt.DecodeJWT(assertion)
	if err != nil {
		return claims, nil, fmt.Errorf("failed to decode the JWT token: %w", err)
	}

	if iatValue, ok := jwtPayload["iat"]; ok {
		iat, ok := sysutils.ToInt64(iatValue)
		if !ok {
			return claims, nil, errors.New("JWT 'iat' claim has unexpected type")
		}
		claims.AuthTime = time.Unix(iat, 0)
	}

	// auth_time, when the flow supplies it, is when the subject authenticated, which on the SSO
	// path predates this assertion. It therefore wins over the iat fallback above.
	if authTimeValue, ok := jwtPayload[oauth2const.ClaimAuthTime]; ok {
		authTime, ok := sysutils.ToInt64(authTimeValue)
		if !ok {
			return claims, nil, errors.New("JWT 'auth_time' claim has unexpected type")
		}
		claims.AuthTime = time.Unix(authTime, 0)
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
