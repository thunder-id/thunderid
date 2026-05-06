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

package manager

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestAuthUserMarshalUnmarshal() {
	authUser := AuthUser{
		userState: ProviderUserStateExists,
		authHistory: []*authResult{
			{
				authenticator: "password",
				isVerified:    true,
				runtimeAttributes: map[string]interface{}{
					"email": "test@example.com",
				},
			},
		},
		userHistory: []*providerUserResult{
			{
				userID:   "user-123",
				userType: "customer",
				ouID:     "ou-456",
				attributes: map[string]interface{}{
					"name": "Test User",
				},
				isValuesIncluded: true,
				token:            "secret-token",
			},
		},
	}

	// Marshal
	data, err := json.Marshal(&authUser)
	s.NoError(err)

	// Unmarshal into a new AuthUser
	var restored AuthUser
	err = json.Unmarshal(data, &restored)
	s.NoError(err)

	// User state round-trips correctly
	s.Equal(ProviderUserStateExists, restored.userState)

	// Auth history round-trips correctly
	s.Require().Len(restored.authHistory, 1)
	ar := restored.authHistory[0]
	s.Equal("password", ar.authenticator)
	s.True(ar.isVerified)
	s.NotNil(ar.runtimeAttributes)
	s.Equal("test@example.com", ar.runtimeAttributes["email"])

	// User history round-trips correctly
	s.Require().Len(restored.userHistory, 1)
	ur := restored.userHistory[0]
	s.Equal("user-123", ur.userID)
	s.Equal("customer", ur.userType)
	s.Equal("ou-456", ur.ouID)
	s.NotNil(ur.attributes)
	s.Equal("Test User", ur.attributes["name"])
	s.True(ur.isValuesIncluded)
	s.Equal("secret-token", ur.token)
}

func (s *ModelTestSuite) TestAuthUserIsSet_ZeroValue() {
	var a AuthUser
	s.False(a.IsSet())
}

func (s *ModelTestSuite) TestAuthUserIsSet_EmptyAuthUser() {
	a := AuthUser{}
	s.False(a.IsSet())
}

func (s *ModelTestSuite) TestAuthUserIsSet_WithUserHistory() {
	a := AuthUser{}
	a.userHistory = []*providerUserResult{
		{userID: "user-123", userType: "customer", ouID: "ou-456"},
	}
	s.True(a.IsSet())
}

func (s *ModelTestSuite) TestAuthUserIsSet_WithOnlyAuthHistory() {
	a := AuthUser{}
	a.authHistory = []*authResult{
		{authenticator: "password"},
	}
	s.True(a.IsSet())
}

func (s *ModelTestSuite) TestAuthUserMarshalNilAuthHistory() {
	// An empty AuthUser must marshal and unmarshal without panicking
	authUser := AuthUser{}

	data, err := json.Marshal(&authUser)
	s.NoError(err)
	s.NotEmpty(data)

	var restored AuthUser
	err = json.Unmarshal(data, &restored)
	s.NoError(err)
	s.Empty(restored.userHistory)
	s.Empty(restored.authHistory)
}
