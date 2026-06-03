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

package idp

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
)

// singleProfileMapping builds an attribute configuration that resolves to userType with a single
// user-type-attributes entry carrying the given claim mappings.
func singleProfileMapping(userType string, mappings []AttributeMapping) *AttributeConfiguration {
	return &AttributeConfiguration{
		UserTypeResolution:        &UserTypeResolution{Default: userType},
		UserTypeAttributeMappings: []UserTypeAttributeMapping{{UserType: userType, Attributes: mappings}},
	}
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_NilMapping_OK() {
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), &IDPDTO{})
	s.Nil(svcErr)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_Valid() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}, {Attribute: "email"}},
			(*serviceerror.ServiceError)(nil))

	idp := &IDPDTO{AttributeConfiguration: singleProfileMapping("person", []AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
		{ExternalAttribute: "address.email", LocalAttribute: "email"},
	})}

	s.Nil(s.idpService.validateAttributeConfiguration(context.Background(), idp))
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_EmptyUserType() {
	idp := &IDPDTO{AttributeConfiguration: &AttributeConfiguration{
		UserTypeAttributeMappings: []UserTypeAttributeMapping{{
			Attributes: []AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}},
		}},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_EmptyMappings() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}}, (*serviceerror.ServiceError)(nil))
	idp := &IDPDTO{AttributeConfiguration: singleProfileMapping("person", nil)}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.Nil(svcErr)
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_OneSourceToMultipleTargets() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
		Return([]entitytype.AttributeInfo{{Attribute: "email"}, {Attribute: "contactEmail"}},
			(*serviceerror.ServiceError)(nil))

	idp := &IDPDTO{AttributeConfiguration: singleProfileMapping("person", []AttributeMapping{
		{ExternalAttribute: "email", LocalAttribute: "email"},
		{ExternalAttribute: "email", LocalAttribute: "contactEmail"},
	})}

	s.Nil(s.idpService.validateAttributeConfiguration(context.Background(), idp))
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DuplicateTarget() {
	idp := &IDPDTO{AttributeConfiguration: singleProfileMapping("person", []AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
		{ExternalAttribute: "first_name", LocalAttribute: "firstName"},
	})}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "more than once")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_DuplicateUserType() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
		Return([]entitytype.AttributeInfo{{Attribute: "firstName"}}, (*serviceerror.ServiceError)(nil))
	idp := &IDPDTO{AttributeConfiguration: &AttributeConfiguration{
		UserTypeResolution: &UserTypeResolution{Default: "person"},
		UserTypeAttributeMappings: []UserTypeAttributeMapping{
			{
				UserType:   "person",
				Attributes: []AttributeMapping{{ExternalAttribute: "given_name", LocalAttribute: "firstName"}},
			},
			{
				UserType:   "person",
				Attributes: []AttributeMapping{{ExternalAttribute: "family_name", LocalAttribute: "lastName"}},
			},
		},
	}}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "configured more than once")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_TargetNotInSchema() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "person", false, true, false).
		Return([]entitytype.AttributeInfo{{Attribute: "email"}}, (*serviceerror.ServiceError)(nil))

	idp := &IDPDTO{AttributeConfiguration: singleProfileMapping("person", []AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
	})}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
	s.Contains(svcErr.ErrorDescription.DefaultValue, "not an attribute")
}

func (s *IDPServiceTestSuite) TestValidateAttributeConfiguration_UnknownUserType() {
	s.mockET.On("GetAttributes", mock.Anything, entitytype.TypeCategoryUser, "ghost", false, true, false).
		Return([]entitytype.AttributeInfo(nil), &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType, Code: "ETS-1004",
			ErrorDescription: core.I18nMessage{DefaultValue: "entity type not found"},
		})

	idp := &IDPDTO{AttributeConfiguration: singleProfileMapping("ghost", []AttributeMapping{
		{ExternalAttribute: "given_name", LocalAttribute: "firstName"},
	})}
	svcErr := s.idpService.validateAttributeConfiguration(context.Background(), idp)
	s.NotNil(svcErr)
	s.Equal(ErrorInvalidAttributeConfiguration.Code, svcErr.Code)
}
