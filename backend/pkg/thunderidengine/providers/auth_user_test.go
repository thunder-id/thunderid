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

package providers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AuthUserTestSuite struct {
	suite.Suite
}

func TestAuthUserTestSuite(t *testing.T) {
	suite.Run(t, new(AuthUserTestSuite))
}

func (s *AuthUserTestSuite) TestAuthUserMarshalUnmarshal() {
	authUser := AuthUser{
		state: map[string]AuthState{
			"default": {
				EntityReferenceToken: map[string]interface{}{"userID": "user-123"},
				EntityReference: &EntityReference{
					EntityID:       "user-123",
					EntityCategory: "person",
					EntityType:     "customer",
					OUID:           "ou-456",
				},
				AttributeToken: "secret-token",
				Attributes: &AttributesResponse{
					Attributes: map[string]*AttributeResponse{
						"email": {Value: "test@example.com"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(&authUser)
	s.NoError(err)

	var restored AuthUser
	err = json.Unmarshal(data, &restored)
	s.NoError(err)

	s.Len(restored.state, 1)
	restoredState, ok := restored.state["default"]
	s.True(ok)

	s.NotNil(restoredState.EntityReferenceToken)
	s.NotNil(restoredState.EntityReference)
	s.Equal("user-123", restoredState.EntityReference.EntityID)
	s.Equal("person", restoredState.EntityReference.EntityCategory)
	s.Equal("customer", restoredState.EntityReference.EntityType)
	s.Equal("ou-456", restoredState.EntityReference.OUID)

	s.Equal("secret-token", restoredState.AttributeToken)
	s.NotNil(restoredState.Attributes)
	s.Equal("test@example.com", restoredState.Attributes.Attributes["email"].Value)
}

func (s *AuthUserTestSuite) TestAuthUserMarshalUnmarshal_MultipleProviders() {
	authUser := AuthUser{
		state: map[string]AuthState{
			"provider-a": {
				EntityReference: &EntityReference{EntityID: "user-a"},
				Attributes:      &AttributesResponse{},
			},
			"provider-b": {
				EntityReferenceToken: map[string]interface{}{"userID": "user-b"},
				AttributeToken:       "tok-b",
			},
		},
	}

	data, err := json.Marshal(&authUser)
	s.NoError(err)

	var restored AuthUser
	err = json.Unmarshal(data, &restored)
	s.NoError(err)

	s.Len(restored.state, 2)

	stateA, ok := restored.state["provider-a"]
	s.True(ok)
	s.NotNil(stateA.EntityReference)
	s.Equal("user-a", stateA.EntityReference.EntityID)
	s.NotNil(stateA.Attributes)

	stateB, ok := restored.state["provider-b"]
	s.True(ok)
	s.NotNil(stateB.EntityReferenceToken)
	s.Equal("tok-b", stateB.AttributeToken)
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_ZeroValue() {
	var a AuthUser
	s.False(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_EmptyAuthUser() {
	a := AuthUser{}
	s.False(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_EmptyStateMap() {
	a := AuthUser{state: map[string]AuthState{}}
	s.False(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_WithEntityRefTokenAndAttrToken() {
	a := AuthUser{
		state: map[string]AuthState{
			"default": {
				EntityReferenceToken: map[string]interface{}{"userID": "user-123"},
				AttributeToken:       "tok",
			},
		},
	}
	s.True(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_WithEntityRefAndAttributes() {
	a := AuthUser{
		state: map[string]AuthState{
			"default": {
				EntityReference: &EntityReference{EntityID: "user-123"},
				Attributes:      &AttributesResponse{},
			},
		},
	}
	s.True(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_OnlyEntityRef() {
	a := AuthUser{
		state: map[string]AuthState{
			"default": {
				EntityReference: &EntityReference{EntityID: "user-123"},
			},
		},
	}
	s.False(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_OnlyAttributes() {
	a := AuthUser{
		state: map[string]AuthState{
			"default": {
				Attributes: &AttributesResponse{},
			},
		},
	}
	s.False(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_AllProvidersValid() {
	a := AuthUser{
		state: map[string]AuthState{
			"provider-a": {
				EntityReference: &EntityReference{EntityID: "user-123"},
				Attributes:      &AttributesResponse{},
			},
			"provider-b": {
				EntityReferenceToken: map[string]interface{}{"userID": "user-123"},
				AttributeToken:       "tok",
			},
		},
	}
	s.True(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserIsAuthenticated_OneProviderInvalid() {
	a := AuthUser{
		state: map[string]AuthState{
			"provider-a": {
				EntityReference: &EntityReference{EntityID: "user-123"},
				Attributes:      &AttributesResponse{},
			},
			"provider-b": {
				EntityReference: &EntityReference{EntityID: "user-123"},
			},
		},
	}
	s.False(a.IsAuthenticated())
}

func (s *AuthUserTestSuite) TestAuthUserMarshalEmpty() {
	authUser := AuthUser{}

	data, err := json.Marshal(&authUser)
	s.NoError(err)
	s.NotEmpty(data)

	var restored AuthUser
	err = json.Unmarshal(data, &restored)
	s.NoError(err)
	s.Empty(restored.state)
}

func (s *AuthUserTestSuite) TestAuthUser_StateFor_AndSetStateFor() {
	var au AuthUser
	_, ok := au.StateFor("default")
	s.False(ok, "empty AuthUser should have no state")

	au.SetStateFor("default", AuthState{EntityReferenceToken: "tok"})
	got, ok := au.StateFor("default")
	s.True(ok)
	s.Equal("tok", got.EntityReferenceToken)

	au.SetStateFor("acme", AuthState{AttributeToken: "atok"})
	s.ElementsMatch([]string{"acme", "default"}, au.ProviderNames())
}

func (s *AuthUserTestSuite) TestAuthUser_ProviderNames_Sorted() {
	var au AuthUser
	au.SetStateFor("zeta", AuthState{})
	au.SetStateFor("alpha", AuthState{})
	au.SetStateFor("mu", AuthState{})
	s.Equal([]string{"alpha", "mu", "zeta"}, au.ProviderNames())
}
