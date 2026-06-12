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
	}

	data, err := json.Marshal(&authUser)
	s.NoError(err)

	var restored AuthUser
	err = json.Unmarshal(data, &restored)
	s.NoError(err)

	s.NotNil(restored.entityReferenceToken)
	s.NotNil(restored.entityReference)
	s.Equal("user-123", restored.entityReference.EntityID)
	s.Equal("person", restored.entityReference.EntityCategory)
	s.Equal("customer", restored.entityReference.EntityType)
	s.Equal("ou-456", restored.entityReference.OUID)

	s.Equal("secret-token", restored.attributeToken)
	s.NotNil(restored.attributes)
	s.Equal("test@example.com", restored.attributes.Attributes["email"].Value)
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_ZeroValue() {
	var a AuthUser
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_EmptyAuthUser() {
	a := AuthUser{}
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_WithEntityRefTokenAndAttrToken() {
	a := AuthUser{
		entityReferenceToken: map[string]interface{}{"userID": "user-123"},
		attributeToken:       "tok",
	}
	s.True(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_WithEntityRefAndAttributes() {
	a := AuthUser{
		entityReference: &authnprovidercm.EntityReference{EntityID: "user-123"},
		attributes:      &authnprovidercm.AttributesResponse{},
	}
	s.True(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_OnlyEntityRef() {
	a := AuthUser{
		entityReference: &authnprovidercm.EntityReference{EntityID: "user-123"},
	}
	s.False(a.IsAuthenticated())
}

func (s *ModelTestSuite) TestAuthUserIsAuthenticated_OnlyAttributes() {
	a := AuthUser{
		attributes: &authnprovidercm.AttributesResponse{},
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
	s.Nil(restored.entityReferenceToken)
	s.Nil(restored.entityReference)
	s.Nil(restored.attributeToken)
	s.Nil(restored.attributes)
}
