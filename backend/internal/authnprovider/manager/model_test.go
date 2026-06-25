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

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
)

type ModelTestSuite struct {
	suite.Suite
}

func TestModelTestSuite(t *testing.T) {
	suite.Run(t, new(ModelTestSuite))
}

func (s *ModelTestSuite) TestAuthUserMarshalUnmarshal() {
	authUser := AuthUser{
		state: map[providerName]authState{
			"default": {
				entityReferenceToken: map[string]interface{}{"userID": "user-123"},
				entityReference: &authnprovidercm.EntityReference{
					EntityID:       "user-123",
					EntityCategory: "person",
					EntityType:     "customer",
					OUID:           "ou-456",
				},
				attributeToken: "secret-token",
				attributes: &authnprovidercm.AttributesResponse{
					Attributes: map[string]*authnprovidercm.AttributeResponse{
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

	s.NotNil(restoredState.entityReferenceToken)
	s.NotNil(restoredState.entityReference)
	s.Equal("user-123", restoredState.entityReference.EntityID)
	s.Equal("person", restoredState.entityReference.EntityCategory)
	s.Equal("customer", restoredState.entityReference.EntityType)
	s.Equal("ou-456", restoredState.entityReference.OUID)

	s.Equal("secret-token", restoredState.attributeToken)
	s.NotNil(restoredState.attributes)
	s.Equal("test@example.com", restoredState.attributes.Attributes["email"].Value)
}

func (s *ModelTestSuite) TestAuthUserMarshalUnmarshal_MultipleProviders() {
	authUser := AuthUser{
		state: map[providerName]authState{
			"provider-a": {
				entityReference: &authnprovidercm.EntityReference{EntityID: "user-a"},
				attributes:      &authnprovidercm.AttributesResponse{},
			},
			"provider-b": {
				entityReferenceToken: map[string]interface{}{"userID": "user-b"},
				attributeToken:       "tok-b",
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
	s.NotNil(stateA.entityReference)
	s.Equal("user-a", stateA.entityReference.EntityID)
	s.NotNil(stateA.attributes)

	stateB, ok := restored.state["provider-b"]
	s.True(ok)
	s.NotNil(stateB.entityReferenceToken)
	s.Equal("tok-b", stateB.attributeToken)
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_ZeroValue() {
	var a AuthUser
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_EmptyAuthUser() {
	a := AuthUser{}
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_EmptyStateMap() {
	a := AuthUser{state: map[providerName]authState{}}
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_WithEntityRefTokenAndAttrToken() {
	a := AuthUser{
		state: map[providerName]authState{
			"default": {
				entityReferenceToken: map[string]interface{}{"userID": "user-123"},
				attributeToken:       "tok",
			},
		},
	}
	s.True(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_WithEntityRefAndAttributes() {
	a := AuthUser{
		state: map[providerName]authState{
			"default": {
				entityReference: &authnprovidercm.EntityReference{EntityID: "user-123"},
				attributes:      &authnprovidercm.AttributesResponse{},
			},
		},
	}
	s.True(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_OnlyEntityRef() {
	a := AuthUser{
		state: map[providerName]authState{
			"default": {
				entityReference: &authnprovidercm.EntityReference{EntityID: "user-123"},
			},
		},
	}
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_OnlyAttributes() {
	a := AuthUser{
		state: map[providerName]authState{
			"default": {
				attributes: &authnprovidercm.AttributesResponse{},
			},
		},
	}
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_AllProvidersValid() {
	a := AuthUser{
		state: map[providerName]authState{
			"provider-a": {
				entityReference: &authnprovidercm.EntityReference{EntityID: "user-123"},
				attributes:      &authnprovidercm.AttributesResponse{},
			},
			"provider-b": {
				entityReferenceToken: map[string]interface{}{"userID": "user-123"},
				attributeToken:       "tok",
			},
		},
	}
	s.True(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_OneProviderInvalid() {
	a := AuthUser{
		state: map[providerName]authState{
			"provider-a": {
				entityReference: &authnprovidercm.EntityReference{EntityID: "user-123"},
				attributes:      &authnprovidercm.AttributesResponse{},
			},
			"provider-b": {
				entityReference: &authnprovidercm.EntityReference{EntityID: "user-123"},
			},
		},
	}
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserMarshalEmpty() {
	authUser := AuthUser{}

	data, err := json.Marshal(&authUser)
	s.NoError(err)
	s.NotEmpty(data)

	var restored AuthUser
	err = json.Unmarshal(data, &restored)
	s.NoError(err)
	s.Empty(restored.state)
}
