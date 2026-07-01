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

package consent

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/consentmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

type ConsentEnforcerServiceTestSuite struct {
	suite.Suite
	mockConsentSvc *consentmock.ConsentServiceInterfaceMock
	mockJWTSvc     *jwtmock.JWTServiceInterfaceMock
	service        *consentEnforcerService
}

func TestConsentEnforcerServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentEnforcerServiceTestSuite))
}

func (s *ConsentEnforcerServiceTestSuite) SetupSuite() {
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
}

func (s *ConsentEnforcerServiceTestSuite) SetupTest() {
	s.mockConsentSvc = consentmock.NewConsentServiceInterfaceMock(s.T())
	s.mockJWTSvc = jwtmock.NewJWTServiceInterfaceMock(s.T())
	s.service = &consentEnforcerService{
		consentService: s.mockConsentSvc,
		jwtService:     s.mockJWTSvc,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ConsentEnforcerService")),
	}
}

func (s *ConsentEnforcerServiceTestSuite) TestNewConsentEnforcerService() {
	svc := newConsentEnforcerService(s.mockConsentSvc, s.mockJWTSvc)
	s.NotNil(svc)
}

// ResolveConsent tests

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_ConsentDisabled() {
	s.mockConsentSvc.On("IsEnabled").Return(false)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.Nil(svcErr)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_ListPurposesClientError() {
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4001",
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(nil, clientErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentPurposeFetchFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_ListPurposesServerError() {
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5001",
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(nil, serverErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_NoPurposesConfigured() {
	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return([]consent.ConsentPurpose{}, nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.Nil(svcErr)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_SearchConsentsClientError() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
			},
		},
	}
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4002",
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return(nil, clientErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentSearchFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_SearchConsentsServerError() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
			},
		},
	}
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5002",
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return(nil, serverErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_AllConsentsActive() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
			},
		},
	}
	existingConsents := []providers.Consent{
		{
			ID:      "consent-1",
			GroupID: "app1",
			Purposes: []providers.ConsentPurposeItem{
				{
					Name: "app:app1:attrs",
					Elements: []providers.ConsentElementApproval{
						{Name: "email", IsUserApproved: true},
					},
				},
			},
		},
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return(existingConsents, nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.Nil(svcErr)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_ForceRepromptIgnoresExistingConsent() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
			},
		},
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	// forceReprompt is honored: existing active consent is ignored, the element is prompted again,
	// and SearchConsents is never called.
	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, true, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Equal("app:app1:attrs", result.Purposes[0].PurposeName)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Essential)
	s.NotEmpty(result.SessionToken)
	s.mockConsentSvc.AssertNotCalled(s.T(), "SearchConsents", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_PromptNeeded() {
	purposes := []consent.ConsentPurpose{
		{
			ID:          "purpose-1",
			Namespace:   providers.NamespaceAttribute,
			Name:        "app:app1:attrs",
			Description: "Test purpose",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
				{Name: "phone", IsMandatory: false},
			},
		},
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, []string{"phone"}, nil, nil, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Equal("app:app1:attrs", result.Purposes[0].PurposeName)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "phone"}}, result.Purposes[0].Optional)
	s.NotEmpty(result.SessionToken)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_RequiredAttributesFilter() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
				{Name: "phone", IsMandatory: false},
				{Name: "address", IsMandatory: false},
			},
		},
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	// Only request "email" — "phone" and "address" should be filtered out
	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Essential)
	s.Empty(result.Purposes[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_UserProfileFilter() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
				{Name: "phone", IsMandatory: false},
			},
		},
	}
	availableAttributes := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {},
		},
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	// Both "email" and "phone" are requested as optional; the user-profile filter must
	// drop "phone" because it is not present in availableAttributes.
	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		nil, []string{"email", "phone"}, nil, availableAttributes, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Empty(result.Purposes[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_PartialConsentsExist() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
				{Name: "phone", IsMandatory: false},
			},
		},
	}
	existingConsents := []providers.Consent{
		{
			ID: "consent-1",
			Purposes: []providers.ConsentPurposeItem{
				{
					Name: "app:app1:attrs",
					Elements: []providers.ConsentElementApproval{
						{Name: "email", IsUserApproved: true},
					},
				},
			},
		},
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return(existingConsents, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, []string{"phone"}, nil, nil, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Empty(result.Purposes[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "phone"}}, result.Purposes[0].Optional)
}

// RecordConsent tests

// buildTestSessionToken creates a fake JWT with the given consent session payload embedded.
// The token is structured as a valid 3-part JWT so DecodeJWTPayload can parse it.
// VerifyJWT is mocked to pass, so the signature is a placeholder.
func buildTestSessionToken(purposes []consentSessionPurpose) string {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)

	sessionData := consentSessionData{Purposes: purposes}
	sessionJSON, _ := json.Marshal(sessionData)
	payload := map[string]interface{}{
		consentSessionClaimKey: json.RawMessage(sessionJSON),
	}
	payloadJSON, _ := json.Marshal(payload)

	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + ".fake-sig"
}

func buildSessionTokenWithPayload(payload map[string]interface{}) string {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + ".fake-sig"
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_CreateConsentSessionTokenFails() {
	purposes := []consent.ConsentPurpose{
		{
			ID:        "purpose-1",
			Namespace: providers.NamespaceAttribute,
			Name:      "app:app1:attrs",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
			},
		},
	}

	s.mockConsentSvc.On("IsEnabled").Return(true)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").
		Return(purposes, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
		})

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestCreateConsentSessionToken_GenerateJWTFails() {
	promptData := &providers.ConsentPromptData{
		Purposes: []providers.ConsentPurposePrompt{{PurposeName: "purpose-1",
			Essential: []providers.PromptElement{{Name: "email"}}}},
	}

	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
		})

	token, err := s.service.createConsentSessionToken(context.Background(), promptData)

	s.Empty(token)
	s.Error(err)
	s.Contains(err.Error(), "failed to generate consent session token")
}

func (s *ConsentEnforcerServiceTestSuite) TestVerifyAndDecodeConsentSession_DecodePayloadFails() {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	token := base64.RawURLEncoding.EncodeToString(headerJSON) + ".invalid-payload.signature"

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, token, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	result, err := s.service.verifyAndDecodeConsentSession(context.Background(), token)

	s.Nil(result)
	s.Error(err)
}

func (s *ConsentEnforcerServiceTestSuite) TestVerifyAndDecodeConsentSession_MissingClaim() {
	token := buildSessionTokenWithPayload(map[string]interface{}{"sub": "user1"})

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, token, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	result, err := s.service.verifyAndDecodeConsentSession(context.Background(), token)

	s.Nil(result)
	s.Error(err)
	s.Contains(err.Error(), "missing consent session claim")
}

func (s *ConsentEnforcerServiceTestSuite) TestVerifyAndDecodeConsentSession_InvalidClaimFormat() {
	token := buildSessionTokenWithPayload(map[string]interface{}{consentSessionClaimKey: "invalid"})

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, token, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	result, err := s.service.verifyAndDecodeConsentSession(context.Background(), token)

	s.Nil(result)
	s.Error(err)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_SessionTokenInvalid() {
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true},
		},
	}
	s.mockJWTSvc.On("VerifyJWT", mock.Anything, "bad-token", consentSessionTokenAudience, mock.Anything).
		Return(&tidcommon.ServiceError{
			Code:  "JWT-5001",
			Error: tidcommon.I18nMessage{Key: "error.test.invalid_token", DefaultValue: "Invalid token"},
		})

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, "bad-token", 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentSessionInvalid.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_MissingPurpose_TreatedAsDenied() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
		{PurposeName: "purpose2", Essential: []string{"phone"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
			// purpose2 is missing — should be filled in as denied
		},
	}
	createdConsent := &providers.Consent{
		ID: "consent-filled",
		Purposes: []providers.ConsentPurposeItem{
			{Name: "purpose1", Elements: []providers.ConsentElementApproval{
				{Name: "email", IsUserApproved: true},
			}},
			{Name: "purpose2", Elements: []providers.ConsentElementApproval{
				{Name: "phone", IsUserApproved: true}, // essential overridden to approved
			}},
		},
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "purpose1", Elements: []consent.PurposeElement{{Name: "email"}}},
		{ID: "p2", Name: "purpose2", Elements: []consent.PurposeElement{{Name: "phone"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			// Verify purpose2 was added with phone element
			for _, p := range req.Purposes {
				if p.Name == "purpose2" {
					for _, e := range p.Elements {
						if e.Name == "phone" {
							return true
						}
					}
				}
			}
			return false
		})).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	// HasEssentialDenial: phone (essential in purpose2) was implicitly denied
	s.Equal(ErrorEssentialConsentDenied.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_SearchFails_ClientError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4002",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return(nil, clientErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentSearchFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_SearchFails_ServerError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5002",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return(nil, serverErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_NoExisting_CreateSuccess() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "app:app1:attrs", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "app:app1:attrs",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
				},
			},
		},
	}
	createdConsent := &providers.Consent{
		ID:      "consent-new",
		GroupID: "app1",
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "app:app1:attrs",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
		},
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "app:app1:attrs", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Equal("consent-new", result.ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_NoExisting_CreateFails_ClientError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4003",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "purpose1", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, clientErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentCreateFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_NoExisting_CreateFails_ServerError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5003",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "purpose1", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, serverErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_ExistingConsent_UpdateSuccess() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "app:app1:attrs", Essential: []string{"email"}, Optional: []string{"phone"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "app:app1:attrs",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "phone", Approved: true},
				},
			},
		},
	}
	existingConsent := providers.Consent{
		ID:      "consent-existing",
		GroupID: "app1",
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "app:app1:attrs",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
		},
	}
	updatedConsent := &providers.Consent{
		ID:      "consent-existing",
		GroupID: "app1",
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "app:app1:attrs",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{existingConsent}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "app:app1:attrs", Elements: []consent.PurposeElement{
			{Name: "email"}, {Name: "phone"},
		}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("UpdateConsent", mock.Anything, "ou1", "consent-existing",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(updatedConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Equal("consent-existing", result.ID)
	s.Len(result.Purposes[0].Elements, 2)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_ExistingConsent_UpdateFails_ClientError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	existingConsent := providers.Consent{ID: "consent-existing"}
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4004",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{existingConsent}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "purpose1", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("UpdateConsent", mock.Anything, "ou1", "consent-existing",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, clientErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentUpdateFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_ExistingConsent_UpdateFails_ServerError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	existingConsent := providers.Consent{ID: "consent-existing"}
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5004",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{existingConsent}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "purpose1", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("UpdateConsent", mock.Anything, "ou1", "consent-existing",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, serverErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_WithValidityPeriod() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	createdConsent := &providers.Consent{ID: "consent-timed"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "purpose1", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			return req.ValidityTime > 0
		})).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 3600, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Equal("consent-timed", result.ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_ZeroValidityPeriod() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	createdConsent := &providers.Consent{ID: "consent-no-expiry"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "purpose1", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			return req.ValidityTime == 0
		})).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(svcErr)
	s.NotNil(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_EssentialDenied_ReturnsError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "app:app1:attrs", Essential: []string{"email"}, Optional: []string{"phone"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "app:app1:attrs",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: false}, // user denies essential
					{Name: "phone", Approved: false},
				},
			},
		},
	}
	// The consent record should reflect the user's actual decisions (email denied)
	createdConsent := &providers.Consent{
		ID:      "consent-essential-deny",
		GroupID: "app1",
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "app:app1:attrs",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: false},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "app:app1:attrs", Elements: []consent.PurposeElement{
			{Name: "email"}, {Name: "phone"},
		}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			// Verify that email element is NOT overridden — stays denied
			for _, p := range req.Purposes {
				for _, e := range p.Elements {
					if e.Name == "email" {
						return !e.IsUserApproved
					}
				}
			}
			return false
		})).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorEssentialConsentDenied.Code, svcErr.Code)
	// Verify consent was still persisted (CreateConsent was called)
	s.mockConsentSvc.AssertCalled(s.T(), "CreateConsent", mock.Anything, "ou1", mock.Anything)
}

// TestRecordConsent_NoExisting_StaleElementFiltered verifies that a first-time consent submission
// drops element decisions for elements no longer in the current purpose definition (e.g. a purpose
// was re-versioned between the prompt and the submission).
func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_NoExisting_StaleElementFiltered() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "app:app1:attrs", Optional: []string{"email", "legacy_attr"}},
	})
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "app:app1:attrs",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "legacy_attr", Approved: true},
				},
			},
		},
	}
	createdConsent := &providers.Consent{ID: "consent-filtered"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, "ou1",
		mock.AnythingOfType("*consent.ConsentSearchFilter")).Return([]providers.Consent{}, nil)
	// Current purpose definition no longer contains legacy_attr.
	s.mockConsentSvc.On("ListConsentPurposes", mock.Anything, "ou1", "app1").Return([]consent.ConsentPurpose{
		{ID: "p1", Name: "app:app1:attrs", Elements: []consent.PurposeElement{{Name: "email"}}},
	}, (*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("CreateConsent", mock.Anything, "ou1",
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			if len(req.Purposes) != 1 || req.Purposes[0].Name != "app:app1:attrs" {
				return false
			}
			if len(req.Purposes[0].Elements) != 1 {
				return false
			}
			return req.Purposes[0].Elements[0].Name == "email"
		})).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Equal("consent-filtered", result.ID)
}

// buildConsentedElementSet tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentedElementSet_Empty() {
	result := buildConsentedElementSet([]providers.Consent{})
	s.Empty(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentedElementSet_ApprovedElements() {
	consents := []providers.Consent{
		{
			Purposes: []providers.ConsentPurposeItem{
				{
					Name: "purpose1",
					Elements: []providers.ConsentElementApproval{
						{Name: "email", IsUserApproved: true},
						{Name: "phone", IsUserApproved: false},
					},
				},
			},
		},
	}

	result := buildConsentedElementSet(consents)

	s.True(result["purpose1:email"])
	s.False(result["purpose1:phone"])
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentedElementSet_MultipleConsents() {
	consents := []providers.Consent{
		{
			Purposes: []providers.ConsentPurposeItem{
				{Name: "purpose1", Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				}},
			},
		},
		{
			Purposes: []providers.ConsentPurposeItem{
				{Name: "purpose2", Elements: []providers.ConsentElementApproval{
					{Name: "phone", IsUserApproved: true},
				}},
			},
		},
	}

	result := buildConsentedElementSet(consents)

	s.True(result["purpose1:email"])
	s.True(result["purpose2:phone"])
	s.Len(result, 2)
}

// buildUserAttributeSet tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildUserAttributeSet_Nil() {
	result := buildUserAttributeSet(nil)
	s.Nil(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildUserAttributeSet_Empty() {
	available := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{},
	}

	result := buildUserAttributeSet(available)
	s.Nil(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildUserAttributeSet_WithAttributes() {
	available := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {},
			"phone": {},
		},
	}

	result := buildUserAttributeSet(available)

	s.NotNil(result)
	s.True(result["email"])
	s.True(result["phone"])
	s.Len(result, 2)
}

// buildPurposePrompts tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_AllNeedConsent() {
	purposes := []consent.ConsentPurpose{
		{
			ID:          "p1",
			Namespace:   providers.NamespaceAttribute,
			Name:        "purpose1",
			Description: "Test purpose",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
				{Name: "phone", IsMandatory: false},
			},
		},
	}

	result := buildPurposePrompts(purposes, nil, []string{"email", "phone"}, map[string]bool{}, nil, nil)

	s.Len(result, 1)
	s.Equal("purpose1", result[0].PurposeName)
	s.Equal("p1", result[0].PurposeID)
	s.Equal("Test purpose", result[0].Description)
	s.Empty(result[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "email"}, {Name: "phone"}}, result[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_AllAlreadyConsented() {
	purposes := []consent.ConsentPurpose{
		{
			Namespace: providers.NamespaceAttribute,
			Name:      "purpose1",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
			},
		},
	}
	consentedElements := map[string]bool{"purpose1:email": true}

	// "email" is requested but already consented; the prompt builder must drop it.
	result := buildPurposePrompts(purposes, []string{"email"}, nil, consentedElements, nil, nil)

	s.Empty(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_RequiredAttributesFilter() {
	purposes := []consent.ConsentPurpose{
		{
			Namespace: providers.NamespaceAttribute,
			Name:      "purpose1",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
				{Name: "phone", IsMandatory: false},
			},
		},
	}

	result := buildPurposePrompts(purposes, []string{"email"}, nil, map[string]bool{}, nil, nil)

	s.Len(result, 1)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result[0].Essential)
	s.Empty(result[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_UserProfileFilter() {
	purposes := []consent.ConsentPurpose{
		{
			Namespace: providers.NamespaceAttribute,
			Name:      "purpose1",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
				{Name: "phone", IsMandatory: false},
			},
		},
	}
	userAttributeSet := map[string]bool{"email": true}

	// Both elements are requested; the user-profile filter must drop "phone" since it is
	// not in availableAttributes.
	result := buildPurposePrompts(purposes, nil, []string{"email", "phone"}, map[string]bool{},
		userAttributeSet, nil)

	s.Len(result, 1)
	s.Empty(result[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_NoMatchingElements() {
	purposes := []consent.ConsentPurpose{
		{
			Namespace: providers.NamespaceAttribute,
			Name:      "purpose1",
			Elements: []consent.PurposeElement{
				{Name: "email", IsMandatory: true},
			},
		},
	}

	// email is filtered out by required attributes
	result := buildPurposePrompts(purposes, []string{"phone"}, nil, map[string]bool{}, nil, nil)

	s.Empty(result)
}

// mergeConsentPurposes tests

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_NoExisting() {
	incoming := []providers.ConsentPurposeItem{
		{
			Name: "purpose1",
			Elements: []providers.ConsentElementApproval{
				{Name: "email", IsUserApproved: true},
			},
		},
	}
	valid := map[string]map[string]bool{"purpose1": {"email": true}}

	result := mergeConsentPurposes(nil, incoming, valid)

	s.Len(result, 1)
	s.Equal("purpose1", result[0].Name)
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_NewElementAddedToExistingPurpose() {
	existing := []providers.ConsentPurposeItem{
		{
			Name: "purpose1",
			Elements: []providers.ConsentElementApproval{
				{Name: "email", IsUserApproved: true},
			},
		},
	}
	incoming := []providers.ConsentPurposeItem{
		{
			Name: "purpose1",
			Elements: []providers.ConsentElementApproval{
				{Name: "phone", IsUserApproved: true},
			},
		},
	}
	valid := map[string]map[string]bool{"purpose1": {"email": true, "phone": true}}

	result := mergeConsentPurposes(existing, incoming, valid)

	s.Len(result, 1)
	s.Len(result[0].Elements, 2)
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_NewDecisionOverridesExisting() {
	existing := []providers.ConsentPurposeItem{
		{
			Name: "purpose1",
			Elements: []providers.ConsentElementApproval{
				{Name: "email", IsUserApproved: false},
			},
		},
	}
	incoming := []providers.ConsentPurposeItem{
		{
			Name: "purpose1",
			Elements: []providers.ConsentElementApproval{
				{Name: "email", IsUserApproved: true},
			},
		},
	}
	valid := map[string]map[string]bool{"purpose1": {"email": true}}

	result := mergeConsentPurposes(existing, incoming, valid)

	s.Len(result, 1)
	s.Len(result[0].Elements, 1)
	s.True(result[0].Elements[0].IsUserApproved)
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_ExistingPurposePreserved() {
	existing := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
		}},
		{Name: "purpose2", Elements: []providers.ConsentElementApproval{
			{Name: "address", IsUserApproved: true},
		}},
	}
	incoming := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
		}},
	}
	valid := map[string]map[string]bool{
		"purpose1": {"email": true},
		"purpose2": {"address": true},
	}

	result := mergeConsentPurposes(existing, incoming, valid)

	s.Len(result, 2)
	purposeNames := make([]string, 0, 2)
	for _, p := range result {
		purposeNames = append(purposeNames, p.Name)
	}
	s.Contains(purposeNames, "purpose1")
	s.Contains(purposeNames, "purpose2")
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_NewPurposeAdded() {
	existing := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
		}},
	}
	incoming := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
		}},
		{Name: "purpose-new", Elements: []providers.ConsentElementApproval{
			{Name: "phone", IsUserApproved: true},
		}},
	}
	valid := map[string]map[string]bool{
		"purpose1":    {"email": true},
		"purpose-new": {"phone": true},
	}

	result := mergeConsentPurposes(existing, incoming, valid)

	s.Len(result, 2)
	purposeNames := make([]string, 0, 2)
	for _, p := range result {
		purposeNames = append(purposeNames, p.Name)
	}
	s.Contains(purposeNames, "purpose1")
	s.Contains(purposeNames, "purpose-new")
}

// TestMergeConsentPurposes_StaleElementDropped covers the case where the existing record
// holds a consent decision for an element that has since been removed from its purpose.
// The stale element must be filtered out of the merged result.
func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_StaleElementDropped() {
	existing := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
			{Name: "legacy", IsUserApproved: true},
		}},
	}
	incoming := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "phone", IsUserApproved: true},
		}},
	}
	valid := map[string]map[string]bool{"purpose1": {"email": true, "phone": true}}

	result := mergeConsentPurposes(existing, incoming, valid)

	s.Len(result, 1)
	s.Len(result[0].Elements, 2)
	names := []string{result[0].Elements[0].Name, result[0].Elements[1].Name}
	s.Contains(names, "email")
	s.Contains(names, "phone")
	s.NotContains(names, "legacy")
}

// TestMergeConsentPurposes_DeletedPurposeDropped covers the case where an existing purpose
// no longer exists upstream. The whole purpose entry should be removed from the merged result.
func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_DeletedPurposeDropped() {
	existing := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
		}},
		{Name: "purpose-gone", Elements: []providers.ConsentElementApproval{
			{Name: "old", IsUserApproved: true},
		}},
	}
	valid := map[string]map[string]bool{"purpose1": {"email": true}}

	result := mergeConsentPurposes(existing, nil, valid)

	s.Len(result, 1)
	s.Equal("purpose1", result[0].Name)
}

// buildConsentElementApprovals tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentElementApprovals_Empty() {
	session := &consentSessionData{}
	decisions := &providers.ConsentDecisions{Purposes: []providers.PurposeDecision{}}

	result := buildConsentElementApprovals(session, decisions)

	s.Empty(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentElementApprovals_MultipleDecisions() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "purpose1", Optional: []string{"email", "phone"}},
		},
	}
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "purpose1",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "phone", Approved: false},
				},
			},
		},
	}

	result := buildConsentElementApprovals(session, decisions)

	s.Len(result, 1)
	s.Equal("purpose1", result[0].Name)
	s.Len(result[0].Elements, 2)
	s.True(result[0].Elements[0].IsUserApproved)
	s.False(result[0].Elements[1].IsUserApproved)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentElementApprovals_DropsUnpromptedPurposeAndElement() {
	// Privilege-escalation defense: extras in the submission that weren't in the signed session
	// must be dropped from the persisted record.
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "purpose1", Optional: []string{"email"}},
		},
	}
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
				{Name: "phone", Approved: true}, // not prompted — dropped
			}},
			{PurposeName: "forged", Elements: []providers.ElementDecision{ // not prompted — dropped
				{Name: "admin", Approved: true},
			}},
		},
	}

	result := buildConsentElementApprovals(session, decisions)

	s.Len(result, 1)
	s.Equal("purpose1", result[0].Name)
	s.Len(result[0].Elements, 1)
	s.Equal("email", result[0].Elements[0].Name)
}

// purposeElementKey tests

func (s *ConsentEnforcerServiceTestSuite) TestPurposeElementKey() {
	key := purposeElementKey("purpose1", "email")
	s.Equal("purpose1:email", key)
}

// fillMissingDecisions tests

func (s *ConsentEnforcerServiceTestSuite) TestFillMissingDecisions_AllPresent() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p1", Essential: []string{"email"}},
		},
	}
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Approved: true, Elements: []providers.ElementDecision{{Name: "email", Approved: true}}},
		},
	}
	fillMissingDecisions(session, decisions)
	s.Len(decisions.Purposes, 1) // no change
}

func (s *ConsentEnforcerServiceTestSuite) TestFillMissingDecisions_MissingPurposeAdded() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p1", Essential: []string{"email"}},
			{PurposeName: "p2", Essential: []string{"phone"}, Optional: []string{"address"}},
		},
	}
	decisions := &providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Approved: true, Elements: []providers.ElementDecision{{Name: "email", Approved: true}}},
		},
	}
	fillMissingDecisions(session, decisions)

	s.Len(decisions.Purposes, 2)
	added := decisions.Purposes[1]
	s.Equal("p2", added.PurposeName)
	s.False(added.Approved)
	s.Len(added.Elements, 2)
	s.Equal("phone", added.Elements[0].Name)
	s.False(added.Elements[0].Approved)
	s.Equal("address", added.Elements[1].Name)
	s.False(added.Elements[1].Approved)
}

// applyPermissionsPurpose tests
//
// Empty-permission and consent-disabled short-circuits live in ResolveConsent (the caller),
// not in this helper, and are exercised by the ResolveConsent tests.

func (s *ConsentEnforcerServiceTestSuite) TestApplyPermissionsPurpose_EmptyPermissionsReturnsInputUnchanged() {
	input := []consent.ConsentPurpose{
		{ID: "attr-p", Namespace: providers.NamespaceAttribute, Name: "App 1"},
	}
	out, svcErr := s.service.applyPermissionsPurpose(context.Background(), input, "ou1", "app1", "App 1", nil)
	s.Nil(svcErr)
	s.Equal(input, out)
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyPermissionsPurpose_CreatesPurposeWhenMissing() {
	perms := []string{"booking:read", "booking:write"}
	input := []consent.ConsentPurpose{
		{ID: "attr-p", Namespace: providers.NamespaceAttribute, Name: "App 1"},
	}
	s.mockConsentSvc.On("CreateConsentPurpose", mock.Anything, "ou1",
		mock.MatchedBy(func(in *consent.ConsentPurposeInput) bool {
			return in.GroupID == "app1" &&
				in.Name == consent.PermissionsPurposeName("app1") &&
				len(in.Elements) == 2
		})).Return(&consent.ConsentPurpose{ID: "perm-p", Namespace: providers.NamespacePermission}, nil)

	out, svcErr := s.service.applyPermissionsPurpose(context.Background(), input, "ou1", "app1", "App 1", perms)
	s.Nil(svcErr)
	s.Len(out, 2)
	s.Equal("perm-p", out[1].ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyPermissionsPurpose_NoopWhenPurposeAlreadyHasAllElements() {
	perms := []string{"booking:read"}
	input := []consent.ConsentPurpose{
		{
			ID:        "perm-p",
			Namespace: providers.NamespacePermission,
			Name:      "permissions:app1",
			Elements: []consent.PurposeElement{
				{Name: "booking:read", Namespace: providers.NamespacePermission},
			},
		},
	}
	out, svcErr := s.service.applyPermissionsPurpose(context.Background(), input, "ou1", "app1", "App 1", perms)
	s.Nil(svcErr)
	s.Equal(input, out)
	s.mockConsentSvc.AssertNotCalled(s.T(), "UpdateConsentPurpose", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
	s.mockConsentSvc.AssertNotCalled(s.T(), "CreateConsentPurpose", mock.Anything, mock.Anything, mock.Anything)
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyPermissionsPurpose_UpdatesPurposeWhenNewElementsAppear() {
	perms := []string{"booking:read", "booking:cancel"}
	input := []consent.ConsentPurpose{
		{
			ID:        "perm-p",
			Namespace: providers.NamespacePermission,
			Name:      "permissions:app1",
			Elements: []consent.PurposeElement{
				{Name: "booking:read", Namespace: providers.NamespacePermission},
			},
		},
	}
	s.mockConsentSvc.On("UpdateConsentPurpose", mock.Anything, "ou1", "perm-p",
		mock.MatchedBy(func(in *consent.ConsentPurposeInput) bool {
			if in.Name != consent.PermissionsPurposeName("app1") {
				return false
			}
			names := map[string]bool{}
			for _, e := range in.Elements {
				names[e.Name] = true
			}
			return names["booking:read"] && names["booking:cancel"]
		})).Return(&consent.ConsentPurpose{ID: "perm-p", Namespace: providers.NamespacePermission}, nil)

	out, svcErr := s.service.applyPermissionsPurpose(context.Background(), input, "ou1", "app1", "App 1", perms)
	s.Nil(svcErr)
	s.Len(out, 1)
	s.Equal("perm-p", out[0].ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyPermissionsPurpose_PropagatesCreatePurposeClientError() {
	perms := []string{"booking:read"}
	s.mockConsentSvc.On("CreateConsentPurpose", mock.Anything, "ou1", mock.Anything).
		Return(nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "X"})

	_, svcErr := s.service.applyPermissionsPurpose(
		context.Background(), []consent.ConsentPurpose{}, "ou1", "app1", "App 1", perms,
	)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentPurposeCreateFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyPermissionsPurpose_IgnoresAttributePurposeForOwner() {
	perms := []string{"booking:read"}
	// Only an attribute purpose for this app — applyPermissionsPurpose must treat as missing and create.
	input := []consent.ConsentPurpose{
		{
			ID:        "attr-p",
			Namespace: providers.NamespaceAttribute,
			Name:      "App 1",
			Elements: []consent.PurposeElement{
				{Name: "email", Namespace: providers.NamespaceAttribute},
			},
		},
	}
	s.mockConsentSvc.On("CreateConsentPurpose", mock.Anything, "ou1",
		mock.MatchedBy(func(in *consent.ConsentPurposeInput) bool {
			return in.Name == consent.PermissionsPurposeName("app1")
		})).Return(&consent.ConsentPurpose{ID: "perm-new", Namespace: providers.NamespacePermission}, nil)

	out, svcErr := s.service.applyPermissionsPurpose(context.Background(), input, "ou1", "app1", "App 1", perms)
	s.Nil(svcErr)
	s.Len(out, 2)
	s.Equal("perm-new", out[1].ID)
}

// Helper-level tests

func (s *ConsentEnforcerServiceTestSuite) TestPermissionsPurposeName() {
	s.Equal("permissions:app1", consent.PermissionsPurposeName("app1"))
	s.Equal("permissions:", consent.PermissionsPurposeName(""))
}

func (s *ConsentEnforcerServiceTestSuite) TestFilterPermissionPurposes() {
	input := []consent.ConsentPurpose{
		{ID: "1", Namespace: providers.NamespaceAttribute},
		{ID: "2", Namespace: providers.NamespacePermission},
		{ID: "3", Namespace: ""}, // purpose with no recognized prefix — skipped
		{ID: "4", Namespace: providers.NamespacePermission},
	}
	got := consent.FilterPermissionPurposes(input)
	s.Len(got, 2)
	s.Equal("2", got[0].ID)
	s.Equal("4", got[1].ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestMergePurposeElements() {
	existing := []consent.PurposeElement{
		{Name: "a", Namespace: providers.NamespacePermission},
		{Name: "b", Namespace: providers.NamespacePermission},
	}
	desired := []consent.PurposeElement{
		{Name: "b", Namespace: providers.NamespacePermission},
		{Name: "c", Namespace: providers.NamespacePermission},
	}
	merged, changed := mergePurposeElements(existing, desired)
	s.True(changed)
	s.Len(merged, 3)
	s.Equal("a", merged[0].Name)
	s.Equal("b", merged[1].Name)
	s.Equal("c", merged[2].Name)

	merged2, changed2 := mergePurposeElements(existing, existing)
	s.False(changed2)
	s.Len(merged2, 2)
}

func (s *ConsentEnforcerServiceTestSuite) TestComputePermissionParents_NoParents() {
	parents := computePermissionParents([]string{"users", "groups", "roles"})
	s.Equal("", parents["users"])
	s.Equal("", parents["groups"])
	s.Equal("", parents["roles"])
}

func (s *ConsentEnforcerServiceTestSuite) TestComputePermissionParents_DotDelimitedHierarchy() {
	parents := computePermissionParents([]string{"users", "users.read", "users.read.email"})
	s.Equal("", parents["users"])
	s.Equal("users", parents["users.read"])
	s.Equal("users.read", parents["users.read.email"])
}

func (s *ConsentEnforcerServiceTestSuite) TestComputePermissionParents_PrefersLongestParent() {
	parents := computePermissionParents([]string{"a", "a.b", "a.b.c"})
	s.Equal("a.b", parents["a.b.c"], "longest matching prefix wins")
}

func (s *ConsentEnforcerServiceTestSuite) TestComputePermissionParents_PrefixWithoutDelimiterIsNotParent() {
	// "usersgroup" starts with "users" but the next char is 'g' (not a delimiter), so "users"
	// is not its parent — it's an unrelated permission that happens to share a prefix.
	parents := computePermissionParents([]string{"users", "usersgroup"})
	s.Equal("", parents["usersgroup"])
}

func (s *ConsentEnforcerServiceTestSuite) TestComputePermissionParents_AcceptsAllDelimiters() {
	// computePermissionParents must accept .  _  :  -  / as delimiter chars after the parent.
	for _, sep := range []string{".", "_", ":", "-", "/"} {
		child := "users" + sep + "read"
		parents := computePermissionParents([]string{"users", child})
		s.Equal("users", parents[child], "delimiter %q should make 'users' the parent of %q", sep, child)
	}
}

func (s *ConsentEnforcerServiceTestSuite) TestComputePermissionParents_ParentMustBeInSet() {
	// "users.read" has no parent because "users" is not in the prompted set.
	parents := computePermissionParents([]string{"users.read", "users.read.email"})
	s.Equal("", parents["users.read"])
	s.Equal("users.read", parents["users.read.email"])
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPermissionPurposePrompt_FiltersByAuthorizedAndConsented() {
	purpose := consent.ConsentPurpose{
		ID:        "perm-1",
		Name:      consent.PermissionsPurposeName("app1"),
		Namespace: providers.NamespacePermission,
		Elements: []consent.PurposeElement{
			{Name: "p1"},
			{Name: "p2"},
			{Name: "p3"},
			{Name: "p4"},
		},
	}
	// p1: authorized + unconsented → prompted
	// p2: authorized + consented   → skipped
	// p3: unauthorized              → skipped
	// p4: authorized + unconsented → prompted
	authorized := []string{"p1", "p2", "p4"}
	consented := map[string]bool{purposeElementKey(purpose.Name, "p2"): true}

	prompt, ok := buildPermissionPurposePrompt(purpose, consented, authorized)
	s.True(ok)
	s.Equal("permissions", prompt.Type)
	s.Empty(prompt.Essential)
	s.Len(prompt.Optional, 2)
	names := make([]string, 0, 2)
	for _, e := range prompt.Optional {
		names = append(names, e.Name)
	}
	s.ElementsMatch([]string{"p1", "p4"}, names)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPermissionPurposePrompt_AttachesParentLinkage() {
	purpose := consent.ConsentPurpose{
		ID:        "perm-1",
		Name:      consent.PermissionsPurposeName("app1"),
		Namespace: providers.NamespacePermission,
		Elements: []consent.PurposeElement{
			{Name: "users"},
			{Name: "users.read"},
		},
	}
	prompt, ok := buildPermissionPurposePrompt(purpose, map[string]bool{}, []string{"users", "users.read"})
	s.True(ok)
	parentByName := make(map[string]string, len(prompt.Optional))
	for _, e := range prompt.Optional {
		parentByName[e.Name] = e.Parent
	}
	s.Equal("", parentByName["users"])
	s.Equal("users", parentByName["users.read"])
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPermissionPurposePrompt_EmptyWhenNothingToPrompt() {
	purpose := consent.ConsentPurpose{
		ID:        "perm-1",
		Name:      consent.PermissionsPurposeName("app1"),
		Namespace: providers.NamespacePermission,
		Elements: []consent.PurposeElement{
			{Name: "p1"},
		},
	}
	// p1 is consented, so nothing to prompt
	consented := map[string]bool{purposeElementKey(purpose.Name, "p1"): true}
	_, ok := buildPermissionPurposePrompt(purpose, consented, []string{"p1"})
	s.False(ok)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPromptedPurposeSet_AndElementSet() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p:a", Essential: []string{"e1"}, Optional: []string{"o1", "o2"}},
			{PurposeName: "p:b", Optional: []string{"x"}},
		},
	}
	purposes := buildPromptedPurposeSet(session)
	s.True(purposes["p:a"])
	s.True(purposes["p:b"])
	s.False(purposes["p:c"])

	elems := buildPromptedElementSet(session)
	s.True(elems[purposeElementKey("p:a", "e1")])
	s.True(elems[purposeElementKey("p:a", "o1")])
	s.True(elems[purposeElementKey("p:a", "o2")])
	s.True(elems[purposeElementKey("p:b", "x")])
	s.False(elems[purposeElementKey("p:a", "missing")])
	s.False(elems[purposeElementKey("p:other", "e1")])
}

func (s *ConsentEnforcerServiceTestSuite) TestElementNames() {
	s.Nil(elementNames(nil))
	s.Nil(elementNames([]providers.PromptElement{}))
	s.Equal([]string{"a", "b"}, elementNames([]providers.PromptElement{{Name: "a"}, {Name: "b", Parent: "a"}}))
}
