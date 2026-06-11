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
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AssertionTestSuite struct {
	suite.Suite
}

func TestAssertionTestSuite(t *testing.T) {
	suite.Run(t, new(AssertionTestSuite))
}

// buildCIBAAssertion constructs a minimal unsigned JWT from a payload map.
func buildCIBAAssertion(payload map[string]interface{}) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payloadBytes, _ := json.Marshal(payload)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + payloadEnc + ".sig"
}

func (suite *AssertionTestSuite) TestDecodeAttributesFromAssertion_CIBAAuthReqIDWrongType_ReturnsError() {
	assertion := buildCIBAAssertion(map[string]interface{}{
		"sub":              "user-1",
		"ciba_auth_req_id": 42,
	})

	_, _, err := decodeAttributesFromAssertion(assertion)
	suite.Error(err)
	suite.Contains(err.Error(), "ciba_auth_req_id")
}

func (suite *AssertionTestSuite) TestDecodeAttributesFromAssertion_MissingAuthorizedPermissions_NoError() {
	// authorized_permissions is optional — absence should not cause an error and the
	// field should default to the empty string.
	assertion := buildCIBAAssertion(map[string]interface{}{
		"sub":              "user-1",
		"ciba_auth_req_id": "auth-req-123",
	})

	claims, _, err := decodeAttributesFromAssertion(assertion)
	suite.NoError(err)
	suite.Equal("user-1", claims.userID)
	suite.Equal("auth-req-123", claims.cibaAuthReqID)
	suite.Empty(claims.authorizedPermissions)
}
