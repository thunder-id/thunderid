// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

	authzdetail "github.com/thunder-id/thunderid/internal/authzdetail/common"
	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authzdetailtypeprovidermock"
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

// attributesPurpose builds an attribute consent purpose for application "app1" with the canonical
// "attributes:app1" name that the enforcer relies on to derive the prompt type.
func attributesPurpose(elements ...string) consent.ConsentPurpose {
	const appID = "app1"
	purposeElements := make([]consent.PurposeElement, 0, len(elements))
	for _, name := range elements {
		purposeElements = append(purposeElements, consent.PurposeElement{
			Name:      name,
			Namespace: consent.NamespaceAttribute,
		})
	}
	return consent.ConsentPurpose{
		ID:          "purpose-" + appID,
		Name:        consent.AttributePurposeNamePrefix + appID,
		Description: "Attribute consent purpose for application " + appID,
		GroupID:     appID,
		Elements:    purposeElements,
	}
}

func (s *ConsentEnforcerServiceTestSuite) TestNewConsentEnforcerService() {
	svc := newConsentEnforcerService(s.mockJWTSvc, nil)
	s.NotNil(svc)

	// The consent service is injected after construction.
	svc.SetConsentService(s.mockConsentSvc)
	s.Equal(s.mockConsentSvc, svc.(*consentEnforcerService).consentService)
}

// ResolveConsent tests

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_ListPurposesClientError() {
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4001",
	}

	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).Return(nil, clientErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentPurposeFetchFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_ListPurposesServerError() {
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5001",
	}

	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).Return(nil, serverErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_PassesGroupIDFilter() {
	s.mockConsentSvc.On("ListPurposes", mock.Anything,
		mock.MatchedBy(func(f consent.PurposeFilter) bool { return f.GroupID == "app1" })).
		Return([]consent.ConsentPurpose{}, nil)

	// No purposes and no authorized permissions -> consent is skipped.
	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.Nil(svcErr)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_NoPurposesConfigured() {
	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{}, nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.Nil(svcErr)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_SearchConsentsClientError() {
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4002",
	}

	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return(nil, clientErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentSearchFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_SearchConsentsServerError() {
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5002",
	}

	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return(nil, serverErr)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_AllConsentsActive() {
	purposeName := consent.AttributePurposeNamePrefix + "app1"
	existingConsents := []*consent.Consent{
		{
			ID:      "consent-1",
			GroupID: "app1",
			Purposes: []consent.ConsentPurposeItem{
				{
					Name: purposeName,
					Elements: []consent.ConsentElementApproval{
						{Name: "email", IsUserApproved: true},
					},
				},
			},
		},
	}

	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return(existingConsents, nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.Nil(svcErr)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_ForceRepromptIgnoresExistingConsent() {
	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email")}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	// forceReprompt is honored: existing active consent is ignored, the element is prompted again,
	// and SearchConsents is never called.
	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, true, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Equal(consent.AttributePurposeNamePrefix+"app1", result.Purposes[0].PurposeName)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Essential)
	s.NotEmpty(result.SessionToken)
	s.mockConsentSvc.AssertNotCalled(s.T(), "SearchConsents", mock.Anything, mock.Anything)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_PromptNeeded() {
	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email", "phone")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, []string{"phone"}, nil, nil, nil, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Equal(consent.AttributePurposeNamePrefix+"app1", result.Purposes[0].PurposeName)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "phone"}}, result.Purposes[0].Optional)
	s.NotEmpty(result.SessionToken)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_RequiredAttributesFilter() {
	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email", "phone", "address")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	// Only request "email" — "phone" and "address" should be filtered out
	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Essential)
	s.Empty(result.Purposes[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_UserProfileFilter() {
	availableAttributes := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {},
		},
	}

	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email", "phone")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	// Both "email" and "phone" are requested as optional; the user-profile filter must
	// drop "phone" because it is not present in availableAttributes.
	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		nil, []string{"email", "phone"}, nil, nil, availableAttributes, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Empty(result.Purposes[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "email"}}, result.Purposes[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_PartialConsentsExist() {
	purposeName := consent.AttributePurposeNamePrefix + "app1"
	existingConsents := []*consent.Consent{
		{
			ID: "consent-1",
			Purposes: []consent.ConsentPurposeItem{
				{
					Name: purposeName,
					Elements: []consent.ConsentElementApproval{
						{Name: "email", IsUserApproved: true},
					},
				},
			},
		},
	}

	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email", "phone")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return(existingConsents, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, []string{"phone"}, nil, nil, nil, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Empty(result.Purposes[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "phone"}}, result.Purposes[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_PermissionsPurposePrompted() {
	// No attribute purposes configured, but authorized permissions produce a dynamically-built
	// permissions purpose that requires prompting.
	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("test-session-token", int64(0), nil)

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		nil, nil, []string{"booking:read"}, nil, nil, false, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Len(result.Purposes, 1)
	s.Equal(consent.PermissionPurposeName("app1"), result.Purposes[0].PurposeName)
	s.Equal(consentPromptTypePermissions, result.Purposes[0].Type)
	s.Equal([]providers.PromptElement{{Name: "booking:read"}}, result.Purposes[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_CreateConsentSessionTokenFails() {
	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).
		Return([]consent.ConsentPurpose{attributesPurpose("email")}, nil)
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", int64(0), &tidcommon.ServiceError{
			Error: tidcommon.I18nMessage{
				Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
			},
		})

	result, svcErr := s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		[]string{"email"}, nil, nil, nil, nil, false, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// createConsentSessionToken / verifyAndDecodeConsentSession tests

// buildTestSessionToken creates a fake JWT with the given consent session payload embedded.
// The token is structured as a valid 3-part JWT so DecodeJWTPayload can parse it.
// VerifyJWT is mocked to pass, so the signature is a placeholder.
func buildTestSessionToken(purposes []consentSessionPurpose) string {
	return encodeTestSessionToken(consentSessionData{Purposes: purposes})
}

func encodeTestSessionToken(sessionData consentSessionData) string {
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)

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

	token, err := s.service.createConsentSessionToken(context.Background(), promptData, nil)

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

// RecordConsent tests

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_SessionTokenInvalid() {
	decisions := &providers.ConsentDecisions{
		Approved: true,
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
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
			// purpose2 is missing — should be filled in as denied
		},
	}
	createdConsent := &consent.Consent{ID: "consent-filled"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			// Verify purpose2 was added with phone element (filled in as denied).
			for _, p := range req.Purposes {
				if p.Name == "purpose2" {
					for _, e := range p.Elements {
						if e.Name == "phone" {
							return !e.IsUserApproved
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
		Approved: true,
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
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return(nil, clientErr)

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
		Approved: true,
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
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return(nil, serverErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_NoExisting_CreateSuccess() {
	purposeName := consent.AttributePurposeNamePrefix + "app1"
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: purposeName, Essential: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: purposeName,
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
				},
			},
		},
	}
	createdConsent := &consent.Consent{
		ID:      "consent-new",
		GroupID: "app1",
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: purposeName,
				Elements: []consent.ConsentElementApproval{
					{Name: "email", Namespace: consent.NamespaceAttribute, IsUserApproved: true},
				},
			},
		},
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
		mock.AnythingOfType("*consent.ConsentRequest")).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.Equal("consent-new", result.ID)
	s.Equal(providers.ConsentTypeAuthentication, result.Type)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_TopLevelDenial_PersistsNothingApproved() {
	purposeName := consent.AttributePurposeNamePrefix + "app1"
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: purposeName, Optional: []string{"email", "phone"}},
	})
	// The purpose and its elements claim approval, but the top level denies the whole consent
	decisions := &providers.ConsentDecisions{
		Approved: false,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: purposeName,
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "phone", Approved: true},
				},
			},
		},
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			for _, p := range req.Purposes {
				for _, e := range p.Elements {
					if e.IsUserApproved {
						return false
					}
				}
			}
			return true
		})).Return(&consent.Consent{ID: "consent-denied"}, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(svcErr)
	s.NotNil(result)
	s.mockConsentSvc.AssertExpectations(s.T())
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_NoExisting_CreateFails_ClientError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Optional: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
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
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, clientErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentCreateFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_NoExisting_CreateFails_ServerError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Optional: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
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
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, serverErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_ExistingConsent_UpdateSuccess() {
	purposeName := consent.AttributePurposeNamePrefix + "app1"
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: purposeName, Essential: []string{"email"}, Optional: []string{"phone"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: purposeName,
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "phone", Approved: true},
				},
			},
		},
	}
	existingConsent := &consent.Consent{
		ID:      "consent-existing",
		GroupID: "app1",
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: purposeName,
				Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
		},
	}
	updatedConsent := &consent.Consent{
		ID:      "consent-existing",
		GroupID: "app1",
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: purposeName,
				Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).
		Return([]*consent.Consent{existingConsent}, nil)
	s.mockConsentSvc.On("UpdateConsent", mock.Anything, "consent-existing",
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
		{PurposeName: "purpose1", Optional: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	existingConsent := &consent.Consent{ID: "consent-existing"}
	clientErr := &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType,
		Code: "CONSENT-4004",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).
		Return([]*consent.Consent{existingConsent}, nil)
	s.mockConsentSvc.On("UpdateConsent", mock.Anything, "consent-existing",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, clientErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(ErrorConsentUpdateFailed.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_ExistingConsent_UpdateFails_ServerError() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Optional: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	existingConsent := &consent.Consent{ID: "consent-existing"}
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "CONSENT-5004",
	}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).
		Return([]*consent.Consent{existingConsent}, nil)
	s.mockConsentSvc.On("UpdateConsent", mock.Anything, "consent-existing",
		mock.AnythingOfType("*consent.ConsentRequest")).Return(nil, serverErr)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(result)
	s.NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_WithValidityPeriod() {
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: "purpose1", Optional: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	createdConsent := &consent.Consent{ID: "consent-timed"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
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
		{PurposeName: "purpose1", Optional: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "purpose1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
		},
	}
	createdConsent := &consent.Consent{ID: "consent-no-expiry"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			return req.ValidityTime == 0
		})).Return(createdConsent, nil)

	result, svcErr := s.service.RecordConsent(context.Background(), "ou1", "app1", "user1",
		decisions, sessionToken, 0, nil)

	s.Nil(svcErr)
	s.NotNil(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_EssentialDenied_ReturnsError() {
	purposeName := consent.AttributePurposeNamePrefix + "app1"
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: purposeName, Essential: []string{"email"}, Optional: []string{"phone"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: purposeName,
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: false}, // user denies essential
					{Name: "phone", Approved: false},
				},
			},
		},
	}
	createdConsent := &consent.Consent{ID: "consent-essential-deny"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
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
	s.mockConsentSvc.AssertCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
}

// TestRecordConsent_UnpromptedElementFiltered verifies that a submission's element decisions for
// elements not covered by the signed session token are dropped before persisting.
func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_UnpromptedElementFiltered() {
	purposeName := consent.AttributePurposeNamePrefix + "app1"
	sessionToken := buildTestSessionToken([]consentSessionPurpose{
		{PurposeName: purposeName, Optional: []string{"email"}},
	})
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: purposeName,
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "forged", Approved: true}, // not prompted — dropped
				},
			},
		},
	}
	createdConsent := &consent.Consent{ID: "consent-filtered"}

	s.mockJWTSvc.On("VerifyJWT", mock.Anything, sessionToken, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything,
		mock.MatchedBy(func(req *consent.ConsentRequest) bool {
			if len(req.Purposes) != 1 || req.Purposes[0].Name != purposeName {
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

// convertToProvidersConsent tests

func (s *ConsentEnforcerServiceTestSuite) TestConvertToProvidersConsent_Nil() {
	s.Nil(convertToProvidersConsent(nil))
}

func (s *ConsentEnforcerServiceTestSuite) TestConvertToProvidersConsent_MapsFields() {
	in := &consent.Consent{
		ID:           "c1",
		GroupID:      "app1",
		Status:       consent.ConsentStatusActive,
		ValidityTime: 1234,
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "attributes:app1",
				Elements: []consent.ConsentElementApproval{
					{Name: "email", Namespace: consent.NamespaceAttribute, IsUserApproved: true},
				},
			},
		},
		Authorizations: []consent.ConsentAuthorization{
			{
				ID:          "auth1",
				UserID:      "user1",
				Type:        consent.AuthorizationTypeAuthorization,
				Status:      consent.AuthorizationStatusApproved,
				UpdatedTime: 5678,
			},
		},
	}

	out := convertToProvidersConsent(in)

	s.Equal("c1", out.ID)
	s.Equal(providers.ConsentTypeAuthentication, out.Type)
	s.Equal("app1", out.GroupID)
	s.Equal(providers.ConsentStatus(consent.ConsentStatusActive), out.Status)
	s.Equal(int64(1234), out.ValidityTime)
	s.Len(out.Purposes, 1)
	s.Equal("attributes:app1", out.Purposes[0].Name)
	s.Equal(providers.Namespace(consent.NamespaceAttribute), out.Purposes[0].Elements[0].Namespace)
	s.True(out.Purposes[0].Elements[0].IsUserApproved)
	s.Len(out.Authorizations, 1)
	s.Equal("auth1", out.Authorizations[0].ID)
	s.Equal(providers.ConsentAuthorizationType(consent.AuthorizationTypeAuthorization), out.Authorizations[0].Type)
}

// buildConsentedElementSet tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentedElementSet_Empty() {
	result := buildConsentedElementSet([]*consent.Consent{})
	s.Empty(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentedElementSet_ApprovedElements() {
	consents := []*consent.Consent{
		{
			Purposes: []consent.ConsentPurposeItem{
				{
					Name: "purpose1",
					Elements: []consent.ConsentElementApproval{
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
	consents := []*consent.Consent{
		{
			Purposes: []consent.ConsentPurposeItem{
				{Name: "purpose1", Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				}},
			},
		},
		{
			Purposes: []consent.ConsentPurposeItem{
				{Name: "purpose2", Elements: []consent.ConsentElementApproval{
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

// deriveConsentPromptTypeFromPurpose / namespaceFromPurposeName tests

func (s *ConsentEnforcerServiceTestSuite) TestDeriveConsentPromptTypeFromPurpose() {
	s.Equal(consentPromptTypeAttributes,
		deriveConsentPromptTypeFromPurpose(consent.ConsentPurpose{Name: "attributes:app1"}))
	s.Equal(consentPromptTypePermissions,
		deriveConsentPromptTypeFromPurpose(consent.ConsentPurpose{Name: "permissions:app1"}))
	s.Equal("", deriveConsentPromptTypeFromPurpose(consent.ConsentPurpose{Name: "unknown"}))
}

func (s *ConsentEnforcerServiceTestSuite) TestNamespaceFromPurposeName() {
	s.Equal(providers.NamespaceAttribute, namespaceFromPurposeName("attributes:app1"))
	s.Equal(providers.NamespacePermission, namespaceFromPurposeName("permissions:app1"))
	s.Equal(providers.Namespace(""), namespaceFromPurposeName("unknown"))
}

// buildPurposePrompts tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_AllNeedConsent() {
	purposes := []consent.ConsentPurpose{
		{
			ID:          "p1",
			Name:        "attributes:app1",
			Description: "Test purpose",
			Elements: []consent.PurposeElement{
				{Name: "email"},
				{Name: "phone"},
			},
		},
	}

	result := buildPurposePrompts(purposes, nil, []string{"email", "phone"}, map[string]bool{}, nil, nil)

	s.Len(result, 1)
	s.Equal("attributes:app1", result[0].PurposeName)
	s.Equal("p1", result[0].PurposeID)
	s.Equal("Test purpose", result[0].Description)
	s.Empty(result[0].Essential)
	s.Equal([]providers.PromptElement{{Name: "email"}, {Name: "phone"}}, result[0].Optional)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_AllAlreadyConsented() {
	purposes := []consent.ConsentPurpose{
		{
			Name: "attributes:app1",
			Elements: []consent.PurposeElement{
				{Name: "email"},
			},
		},
	}
	consentedElements := map[string]bool{"attributes:app1:email": true}

	// "email" is requested but already consented; the prompt builder must drop it.
	result := buildPurposePrompts(purposes, []string{"email"}, nil, consentedElements, nil, nil)

	s.Empty(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_RequiredAttributesFilter() {
	purposes := []consent.ConsentPurpose{
		{
			Name: "attributes:app1",
			Elements: []consent.PurposeElement{
				{Name: "email"},
				{Name: "phone"},
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
			Name: "attributes:app1",
			Elements: []consent.PurposeElement{
				{Name: "email"},
				{Name: "phone"},
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
			Name: "attributes:app1",
			Elements: []consent.PurposeElement{
				{Name: "email"},
			},
		},
	}

	// email is filtered out by required attributes
	result := buildPurposePrompts(purposes, []string{"phone"}, nil, map[string]bool{}, nil, nil)

	s.Empty(result)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPurposePrompts_UnknownPrefixSkipped() {
	purposes := []consent.ConsentPurpose{
		{
			Name: "unknown:app1",
			Elements: []consent.PurposeElement{
				{Name: "email"},
			},
		},
	}

	result := buildPurposePrompts(purposes, []string{"email"}, nil, map[string]bool{}, nil, nil)

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

	result := mergeConsentPurposes(nil, incoming)

	s.Len(result, 1)
	s.Equal("purpose1", result[0].Name)
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_NewElementAddedToExistingPurpose() {
	existing := []consent.ConsentPurposeItem{
		{
			Name: "purpose1",
			Elements: []consent.ConsentElementApproval{
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

	result := mergeConsentPurposes(existing, incoming)

	s.Len(result, 1)
	s.Len(result[0].Elements, 2)
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_NewDecisionOverridesExisting() {
	existing := []consent.ConsentPurposeItem{
		{
			Name: "purpose1",
			Elements: []consent.ConsentElementApproval{
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

	result := mergeConsentPurposes(existing, incoming)

	s.Len(result, 1)
	s.Len(result[0].Elements, 1)
	s.True(result[0].Elements[0].IsUserApproved)
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_ExistingPurposePreserved() {
	existing := []consent.ConsentPurposeItem{
		{Name: "purpose1", Elements: []consent.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
		}},
		{Name: "purpose2", Elements: []consent.ConsentElementApproval{
			{Name: "address", IsUserApproved: true},
		}},
	}
	incoming := []providers.ConsentPurposeItem{
		{Name: "purpose1", Elements: []providers.ConsentElementApproval{
			{Name: "email", IsUserApproved: true},
		}},
	}

	result := mergeConsentPurposes(existing, incoming)

	s.Len(result, 2)
	purposeNames := make([]string, 0, 2)
	for _, p := range result {
		purposeNames = append(purposeNames, p.Name)
	}
	s.Contains(purposeNames, "purpose1")
	s.Contains(purposeNames, "purpose2")
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeConsentPurposes_NewPurposeAdded() {
	existing := []consent.ConsentPurposeItem{
		{Name: "purpose1", Elements: []consent.ConsentElementApproval{
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

	result := mergeConsentPurposes(existing, incoming)

	s.Len(result, 2)
	purposeNames := make([]string, 0, 2)
	for _, p := range result {
		purposeNames = append(purposeNames, p.Name)
	}
	s.Contains(purposeNames, "purpose1")
	s.Contains(purposeNames, "purpose-new")
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
			{PurposeName: "attributes:app1", Optional: []string{"email", "phone"}},
		},
	}
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "attributes:app1",
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
	s.Equal("attributes:app1", result[0].Name)
	s.Len(result[0].Elements, 2)
	s.Equal(providers.NamespaceAttribute, result[0].Elements[0].Namespace)
	s.True(result[0].Elements[0].IsUserApproved)
	s.False(result[0].Elements[1].IsUserApproved)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentElementApprovals_DropsUnpromptedPurposeAndElement() {
	// Privilege-escalation defense: extras in the submission that weren't in the signed session
	// must be dropped from the persisted record.
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "attributes:app1", Optional: []string{"email"}},
		},
	}
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:app1", Elements: []providers.ElementDecision{
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
	s.Equal("attributes:app1", result[0].Name)
	s.Len(result[0].Elements, 1)
	s.Equal("email", result[0].Elements[0].Name)
}

// purposeElementKey tests

func (s *ConsentEnforcerServiceTestSuite) TestPurposeElementKey() {
	key := purposeElementKey("purpose1", "email")
	s.Equal("purpose1:email", key)
}

// applyDecisionHierarchy tests

func (s *ConsentEnforcerServiceTestSuite) TestApplyDecisionHierarchy_TopLevelDenialDeniesEverything() {
	decisions := &providers.ConsentDecisions{
		Approved: false,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
				{Name: "phone", Approved: true},
			}},
			{PurposeName: "p2", Approved: true, Elements: []providers.ElementDecision{
				{Name: "address", Approved: true},
			}},
		},
	}

	applyDecisionHierarchy(decisions)

	for _, p := range decisions.Purposes {
		s.False(p.Approved)
		for _, e := range p.Elements {
			s.False(e.Approved)
		}
	}
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyDecisionHierarchy_PurposeDenialOnlyAffectsItsElements() {
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "denied", Approved: false, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
			}},
			{PurposeName: "approved", Approved: true, Elements: []providers.ElementDecision{
				{Name: "phone", Approved: true},
			}},
		},
	}

	applyDecisionHierarchy(decisions)

	s.False(decisions.Purposes[0].Approved)
	s.False(decisions.Purposes[0].Elements[0].Approved)
	s.True(decisions.Purposes[1].Approved)
	s.True(decisions.Purposes[1].Elements[0].Approved)
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyDecisionHierarchy_ApprovalDoesNotOverrideDeniedElements() {
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true},
				{Name: "phone", Approved: false},
			}},
		},
	}

	applyDecisionHierarchy(decisions)

	s.True(decisions.Purposes[0].Elements[0].Approved)
	s.False(decisions.Purposes[0].Elements[1].Approved)
}

func (s *ConsentEnforcerServiceTestSuite) TestApplyDecisionHierarchy_NoPurposes() {
	decisions := &providers.ConsentDecisions{Approved: false}

	applyDecisionHierarchy(decisions)

	s.Empty(decisions.Purposes)
}

// fillMissingDecisions tests

func (s *ConsentEnforcerServiceTestSuite) TestFillMissingDecisions_AllPresent() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p1", Essential: []string{"email"}},
		},
	}
	decisions := &providers.ConsentDecisions{
		Approved: true,
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
		Approved: true,
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

func (s *ConsentEnforcerServiceTestSuite) TestFillMissingDecisions_RootDeniedFillsMissingElements() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p1", Essential: []string{"email"}, Optional: []string{"phone"}},
		},
	}
	// Client sent a minimal deny payload: root approved=false and the purpose is present but its
	// element list is empty. The backend fills in every prompted element as denied.
	decisions := &providers.ConsentDecisions{
		Approved: false,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Approved: false, Elements: []providers.ElementDecision{}},
		},
	}
	fillMissingDecisions(session, decisions)

	s.Len(decisions.Purposes, 1)
	filled := decisions.Purposes[0]
	s.Len(filled.Elements, 2)
	s.Equal("email", filled.Elements[0].Name)
	s.False(filled.Elements[0].Approved)
	s.Equal("phone", filled.Elements[1].Name)
	s.False(filled.Elements[1].Approved)
}

func (s *ConsentEnforcerServiceTestSuite) TestFillMissingDecisions_PurposeDeniedFillsMissingElements() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p1", Optional: []string{"address"}},
			{PurposeName: "p2", Essential: []string{"phone"}, Optional: []string{"birthday"}},
		},
	}
	// Root is approved but p2 is denied and its elements were omitted. Missing elements under the
	// denied purpose must be filled in as denied.
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "address", Approved: true}}},
			{PurposeName: "p2", Approved: false},
		},
	}
	fillMissingDecisions(session, decisions)

	s.Len(decisions.Purposes, 2)
	// p1 is unchanged: parent approved so omitted elements stay omitted
	s.Len(decisions.Purposes[0].Elements, 1)
	s.Equal("address", decisions.Purposes[0].Elements[0].Name)
	s.True(decisions.Purposes[0].Elements[0].Approved)
	// p2 gets both prompted elements filled in as denied
	deniedPurpose := decisions.Purposes[1]
	s.Len(deniedPurpose.Elements, 2)
	s.Equal("phone", deniedPurpose.Elements[0].Name)
	s.False(deniedPurpose.Elements[0].Approved)
	s.Equal("birthday", deniedPurpose.Elements[1].Name)
	s.False(deniedPurpose.Elements[1].Approved)
}

func (s *ConsentEnforcerServiceTestSuite) TestFillMissingDecisions_ApprovedPurposeLeftUntouched() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p1", Essential: []string{"email"}, Optional: []string{"phone"}},
		},
	}
	// Approval is not propagated down, so an omitted optional element under an approved parent
	// stays omitted (treated as not-consented on subsequent reads).
	decisions := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Approved: true, Elements: []providers.ElementDecision{
				{Name: "email", Approved: true}}},
		},
	}
	fillMissingDecisions(session, decisions)

	s.Len(decisions.Purposes, 1)
	s.Len(decisions.Purposes[0].Elements, 1)
	s.Equal("email", decisions.Purposes[0].Elements[0].Name)
}

// buildEssentialElementSet / hasEssentialDenials tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildEssentialElementSet() {
	session := &consentSessionData{
		Purposes: []consentSessionPurpose{
			{PurposeName: "p1", Essential: []string{"email"}, Optional: []string{"phone"}},
			{PurposeName: "p2", Essential: []string{"name"}},
		},
	}

	set := buildEssentialElementSet(session)

	s.True(set["p1:email"])
	s.True(set["p2:name"])
	s.False(set["p1:phone"]) // optional, not essential
}

func (s *ConsentEnforcerServiceTestSuite) TestHasEssentialDenials() {
	essentialElements := map[string]bool{"p1:email": true}

	denied := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Elements: []providers.ElementDecision{{Name: "email", Approved: false}}},
		},
	}
	s.True(hasEssentialDenials(denied, essentialElements))

	approved := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Elements: []providers.ElementDecision{{Name: "email", Approved: true}}},
		},
	}
	s.False(hasEssentialDenials(approved, essentialElements))

	// A denied element that is not essential does not count.
	optionalDenied := &providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "p1", Elements: []providers.ElementDecision{{Name: "phone", Approved: false}}},
		},
	}
	s.False(hasEssentialDenials(optionalDenied, essentialElements))
}

// buildPermissionsPurpose tests

func (s *ConsentEnforcerServiceTestSuite) TestBuildPermissionsPurpose_EmptyPermissionsReturnsNil() {
	s.Nil(s.service.buildPermissionsPurpose("app1", "App 1", nil))
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildPermissionsPurpose_BuildsPurpose() {
	perms := []string{"booking:read", "booking:write"}

	purpose := s.service.buildPermissionsPurpose("app1", "App 1", perms)

	s.NotNil(purpose)
	s.Equal(consent.PermissionPurposeName("app1"), purpose.Name)
	s.Equal("app1", purpose.GroupID)
	s.Len(purpose.Elements, 2)
	for _, e := range purpose.Elements {
		s.Equal(consent.NamespacePermission, e.Namespace)
	}
}

// Helper-level tests

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
		ID:   "perm-1",
		Name: consent.PermissionPurposeName("app1"),
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
		ID:   "perm-1",
		Name: consent.PermissionPurposeName("app1"),
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
		ID:   "perm-1",
		Name: consent.PermissionPurposeName("app1"),
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

// Authorization details tests

const (
	testFlightType   = "flight"
	testFlightSchema = `{"type":"object","properties":{"route":{"type":"string"},` +
		`"maxFare":{"type":"number","policy":{"modification":"decrease","modifiableBy":["user"]}}}}`
)

// flightType is a registered flight type whose approvals a later request may reuse.
func flightType() *providers.AuthorizationDetailType {
	return &providers.AuthorizationDetailType{ID: "type-1", Type: testFlightType, Name: "Flight", Version: 2,
		Schema: json.RawMessage(testFlightSchema), Reuse: true}
}

func flightDetail(detailID, fare string) providers.AuthorizationDetail {
	value := json.RawMessage(`{"type":"` + testFlightType + `","route":"CMB-SIN","maxFare":` + fare + `}`)
	return providers.AuthorizationDetail{ID: detailID, Type: testFlightType, TypeID: "type-1", TypeVersion: 2,
		Requested: value, Value: value, Admitted: true}
}

var testDetailPurpose = consent.AuthorizationDetailPurposeName("app1")

// flightRecord is an active consent record holding an approval of a flight with the given fare cap.
func flightRecord(fare string) *consent.Consent {
	return &consent.Consent{ID: "consent-1", GroupID: "app1", Purposes: []consent.ConsentPurposeItem{{
		Name: testDetailPurpose,
		Elements: []consent.ConsentElementApproval{{Name: "d-0", Namespace: consent.NamespaceAuthorizationDetail,
			IsUserApproved: true, TypeID: "type-1", TypeVersion: 2,
			Granted: json.RawMessage(`{"type":"` + testFlightType + `","route":"CMB-SIN","maxFare":` + fare + `}`)}},
	}}}
}

func (s *ConsentEnforcerServiceTestSuite) withDetailTypes(t *providers.AuthorizationDetailType) {
	types := authzdetailtypeprovidermock.NewAuthorizationDetailTypeProviderMock(s.T())
	types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).Return(t, nil).Maybe()
	s.service.detailTypes = types
}

func (s *ConsentEnforcerServiceTestSuite) resolveDetails(forceReprompt bool,
	details ...providers.AuthorizationDetail) (*providers.ConsentPromptData, *tidcommon.ServiceError) {
	s.mockConsentSvc.On("ListPurposes", mock.Anything, mock.Anything).Return([]consent.ConsentPurpose{}, nil)
	return s.service.ResolveConsent(context.Background(), "ou1", "app1", "App 1", "user1",
		nil, nil, nil, details, nil, forceReprompt, nil)
}

// expectSessionToken captures the consent session the enforcer signs.
func (s *ConsentEnforcerServiceTestSuite) expectSessionToken() *consentSessionData {
	session := &consentSessionData{}
	s.mockJWTSvc.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			raw, err := json.Marshal(claims[consentSessionClaimKey])
			return err == nil && json.Unmarshal(raw, session) == nil
		}), mock.Anything, mock.Anything).Return("session-token", int64(0), nil).Once()
	return session
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_AuthorizationDetailsPrompted() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).
		Return([]*consent.Consent{flightRecord("500")}, nil)
	session := s.expectSessionToken()

	result, svcErr := s.resolveDetails(false, flightDetail("d-1", "450"), flightDetail("d-2", "900"))

	s.Require().Nil(svcErr)
	s.Require().NotNil(result)
	s.Require().Len(result.Purposes, 1)
	purpose := result.Purposes[0]
	s.Equal(testDetailPurpose, purpose.PurposeName)
	s.Equal(consentPromptTypeAuthorizationDetails, purpose.Type)
	s.Require().Len(purpose.Optional, 1, "d-1 is covered by the earlier approval")
	s.Equal("d-2", purpose.Optional[0].Name)
	s.Equal(testFlightType, purpose.Optional[0].DetailType)
	s.NotEmpty(purpose.Optional[0].Rows)
	s.Equal("session-token", result.SessionToken)
	s.Require().Len(session.Purposes, 1)
	s.Equal([]string{"d-2"}, session.Purposes[0].Optional)
	s.Require().Len(session.Purposes[0].Details, 1, "the session holds the detail it prompts for")
	s.Equal("d-2", session.Purposes[0].Details[0].ID)
	s.JSONEq(string(flightDetail("d-2", "900").Value), string(session.Purposes[0].Details[0].Value))
	s.Require().Len(session.CoveredDetails, 1, "the session holds the detail the earlier approval covers")
	s.Equal("d-1", session.CoveredDetails[0].ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_AuthorizationDetailsAllCovered() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).
		Return([]*consent.Consent{flightRecord("500")}, nil)

	result, svcErr := s.resolveDetails(false, flightDetail("d-1", "450"))

	s.Nil(svcErr)
	s.Nil(result, "covered details need no prompt")
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_AuthorizationDetailsForcedReprompt() {
	s.withDetailTypes(flightType())
	s.expectSessionToken()

	result, svcErr := s.resolveDetails(true, flightDetail("d-1", "450"))

	s.Require().Nil(svcErr)
	s.Require().NotNil(result)
	s.Len(result.Purposes, 1, "a forced prompt ignores earlier approvals")
	s.mockConsentSvc.AssertNotCalled(s.T(), "SearchConsents", mock.Anything, mock.Anything)
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_AuthorizationDetailsFailures() {
	notAdmitted := flightDetail("d-1", "450")
	notAdmitted.Admitted = false
	edited := flightType()
	edited.Version++
	recreated := flightType()
	recreated.ID = "type-1-recreated"
	typeChanged := authzdetail.ErrorAuthorizationDetailTypeChanged.Code

	cases := []struct {
		name   string
		types  *providers.AuthorizationDetailType
		detail providers.AuthorizationDetail
		code   string
	}{
		{"not admitted", flightType(), notAdmitted, ErrorAuthorizationDetailsNotAdmitted.Code},
		{"type edited", edited, flightDetail("d-1", "450"), typeChanged},
		{"type deleted", nil, flightDetail("d-1", "450"), typeChanged},
		{"type created again", recreated, flightDetail("d-1", "450"), typeChanged},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.withDetailTypes(tc.types)
			s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)

			result, svcErr := s.resolveDetails(false, tc.detail)

			s.Nil(result)
			s.Require().NotNil(svcErr)
			s.Equal(tc.code, svcErr.Code)
		})
	}
}

func (s *ConsentEnforcerServiceTestSuite) TestResolveConsent_AuthorizationDetailsTypeLookupFails() {
	types := authzdetailtypeprovidermock.NewAuthorizationDetailTypeProviderMock(s.T())
	types.EXPECT().GetAuthorizationDetailTypeByValue(mock.Anything, testFlightType).
		Return(nil, &tidcommon.InternalServerError).Once()
	s.service.detailTypes = types
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)

	_, svcErr := s.resolveDetails(false, flightDetail("d-1", "450"))

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func detailSessionToken(details ...providers.AuthorizationDetail) string {
	optional := make([]string, 0, len(details))
	for _, detail := range details {
		optional = append(optional, detail.ID)
	}
	return buildTestSessionToken([]consentSessionPurpose{{PurposeName: testDetailPurpose, Optional: optional,
		Details: details}})
}

func detailDecision(detailID string, approved bool) providers.PurposeDecision {
	return providers.PurposeDecision{PurposeName: testDetailPurpose,
		Approved: approved, Elements: []providers.ElementDecision{{Name: detailID, Approved: approved}}}
}

func (s *ConsentEnforcerServiceTestSuite) recordDetails(decisions *providers.ConsentDecisions,
	details ...providers.AuthorizationDetail) (*providers.Consent, *tidcommon.ServiceError) {
	token := detailSessionToken(details...)
	s.mockJWTSvc.On("VerifyJWT", mock.Anything, token, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil).Maybe()
	return s.service.RecordConsent(context.Background(), "ou1", "app1", "user1", decisions, token, 0, nil)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_KeepsReusableApprovalsInTheSameRecord() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).
		Return([]*consent.Consent{flightRecord("500")}, nil)
	var written *consent.ConsentRequest
	captureWrite := func(r *consent.ConsentRequest) bool {
		written = r
		return true
	}
	s.mockConsentSvc.On("UpdateConsent", mock.Anything, "consent-1", mock.MatchedBy(captureWrite)).
		Return(&consent.Consent{ID: "consent-1"}, nil).Once()

	result, svcErr := s.recordDetails(&providers.ConsentDecisions{Approved: true, Purposes: []providers.PurposeDecision{
		detailDecision("d-1", true), detailDecision("d-2", false)}},
		flightDetail("d-1", "450"), flightDetail("d-2", "900"))

	s.Require().Nil(svcErr)
	s.Equal("consent-1", result.ID)
	s.Require().Len(result.GrantedAuthorizationDetails, 1)
	s.Equal("d-1", result.GrantedAuthorizationDetails[0].ID)
	s.Require().Len(written.Purposes, 1)
	elements := written.Purposes[0].Elements
	s.Require().Len(elements, 2, "the earlier approval is kept beside the new one, and the denial is not kept")
	s.Equal("d-1", elements[1].Name)
	s.Equal("type-1", elements[1].TypeID)
	s.Equal(2, elements[1].TypeVersion)
	s.JSONEq(string(flightDetail("d-1", "450").Value), string(elements[1].Granted))
	s.JSONEq(string(flightDetail("d-1", "450").Requested), string(elements[1].Requested))
	s.mockConsentSvc.AssertNotCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_CreatesARecordForApprovedDetails() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything, mock.MatchedBy(func(r *consent.ConsentRequest) bool {
		return len(r.Purposes) == 1 && r.Purposes[0].Name == testDetailPurpose
	})).Return(&consent.Consent{ID: "consent-new"}, nil).Once()

	result, svcErr := s.recordDetails(&providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}}, flightDetail("d-1", "450"))

	s.Require().Nil(svcErr)
	s.Equal("consent-new", result.ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_DetailsKeepingNothingWriteNothing() {
	oneOff := flightType()
	oneOff.Reuse = false
	cases := map[string]struct {
		types     *providers.AuthorizationDetailType
		decisions *providers.ConsentDecisions
		granted   int
	}{
		"denied": {flightType(), &providers.ConsentDecisions{Approved: false,
			Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}}, 0},
		"declined": {flightType(), &providers.ConsentDecisions{Approved: true,
			Purposes: []providers.PurposeDecision{detailDecision("d-1", false)}}, 0},
		"approved on a type that is never reused": {oneOff, &providers.ConsentDecisions{Approved: true,
			Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}}, 1},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			s.SetupTest()
			s.withDetailTypes(tc.types)
			s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)

			result, svcErr := s.recordDetails(tc.decisions, flightDetail("d-1", "450"))

			s.Require().Nil(svcErr)
			s.Empty(result.ID)
			s.Len(result.GrantedAuthorizationDetails, tc.granted)
			s.mockConsentSvc.AssertNotCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
			s.mockConsentSvc.AssertNotCalled(s.T(), "UpdateConsent", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func modifiedDecision(detailID string, modified map[string]json.RawMessage) providers.PurposeDecision {
	decision := detailDecision(detailID, true)
	decision.Elements[0].Modified = modified
	return decision
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_KeepsAModifiedApprovalAtItsModifiedValue() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	var written *consent.ConsentRequest
	s.mockConsentSvc.On("CreateConsent", mock.Anything, mock.MatchedBy(func(r *consent.ConsentRequest) bool {
		written = r
		return true
	})).Return(&consent.Consent{ID: "consent-new"}, nil).Once()

	result, svcErr := s.recordDetails(&providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{modifiedDecision("d-1",
			map[string]json.RawMessage{"maxFare": json.RawMessage(`300`)})}}, flightDetail("d-1", "450"))

	s.Require().Nil(svcErr)
	s.Require().Len(result.GrantedAuthorizationDetails, 1)
	s.JSONEq(string(flightDetail("d-1", "300").Value), string(result.GrantedAuthorizationDetails[0].Value))
	s.JSONEq(string(flightDetail("d-1", "450").Requested), string(result.GrantedAuthorizationDetails[0].Requested),
		"the request is kept as the client sent it")
	s.Require().Len(written.Purposes, 1)
	s.Require().Len(written.Purposes[0].Elements, 1)
	s.JSONEq(string(flightDetail("d-1", "300").Value), string(written.Purposes[0].Elements[0].Granted))
	s.JSONEq(string(flightDetail("d-1", "450").Requested), string(written.Purposes[0].Elements[0].Requested))
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_RefusesChangesTheUserMayNotMake() {
	cases := map[string]map[string]json.RawMessage{
		"raises the value":               {"maxFare": json.RawMessage(`900`)},
		"changes a field with no policy": {"route": json.RawMessage(`"CMB-LHR"`)},
		"adds a field":                   {"note": json.RawMessage(`"x"`)},
		"breaks the schema":              {"maxFare": json.RawMessage(`"low"`)},
		"path through a value":           {"route.code": json.RawMessage(`"x"`)},
		"changes the type":               {"type": json.RawMessage(`"other"`)},
	}
	for name, change := range cases {
		s.Run(name, func() {
			s.SetupTest()
			s.withDetailTypes(flightType())

			_, svcErr := s.recordDetails(&providers.ConsentDecisions{Approved: true,
				Purposes: []providers.PurposeDecision{modifiedDecision("d-1", change)}}, flightDetail("d-1", "450"))

			s.Require().NotNil(svcErr)
			s.Equal(ErrorAuthorizationDetailModificationRefused.Code, svcErr.Code)
			s.mockConsentSvc.AssertNotCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
			s.mockConsentSvc.AssertNotCalled(s.T(), "UpdateConsent", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_ApprovedDetailOfAChangedType() {
	edited := flightType()
	edited.Version++
	s.withDetailTypes(edited)

	_, svcErr := s.recordDetails(&providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}}, flightDetail("d-1", "450"))

	s.Require().NotNil(svcErr)
	s.Equal(authzdetail.ErrorAuthorizationDetailTypeChanged.Code, svcErr.Code)
	s.mockConsentSvc.AssertNotCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
}

// recordWithCovered records the given decisions on a prompt for the prompted detail, with the covered
// detail an earlier approval covers.
func (s *ConsentEnforcerServiceTestSuite) recordWithCovered(decisions *providers.ConsentDecisions,
	prompted, covered providers.AuthorizationDetail) (*providers.Consent, *tidcommon.ServiceError) {
	token := encodeTestSessionToken(consentSessionData{
		Purposes: []consentSessionPurpose{{PurposeName: testDetailPurpose, Optional: []string{prompted.ID},
			Details: []providers.AuthorizationDetail{prompted}}},
		CoveredDetails: []providers.AuthorizationDetail{covered},
	})
	s.mockJWTSvc.On("VerifyJWT", mock.Anything, token, consentSessionTokenAudience, mock.Anything).
		Return((*tidcommon.ServiceError)(nil))
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil).Maybe()
	return s.service.RecordConsent(context.Background(), "ou1", "app1", "user1", decisions, token, 0, nil)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_GrantsCoveredDetailsAsRequested() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).
		Return([]*consent.Consent{flightRecord("450")}, nil)

	result, svcErr := s.recordWithCovered(&providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-2", false)}},
		flightDetail("d-2", "900"), flightDetail("d-1", "450"))

	s.Require().Nil(svcErr)
	s.Require().Len(result.GrantedAuthorizationDetails, 1, "the declined prompted detail is not granted")
	s.Equal("d-1", result.GrantedAuthorizationDetails[0].ID)
	s.JSONEq(string(flightDetail("d-1", "450").Value), string(result.GrantedAuthorizationDetails[0].Value))
	s.mockConsentSvc.AssertNotCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
}

// An approval given on the same prompt never grants a prompted detail the user declined, even one it covers.
func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_DeclinedDetailIsNotGrantedByAnotherApproval() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)
	s.mockConsentSvc.On("CreateConsent", mock.Anything, mock.Anything).
		Return(&consent.Consent{ID: "consent-new"}, nil).Once()
	decision := providers.PurposeDecision{PurposeName: testDetailPurpose, Approved: true,
		Elements: []providers.ElementDecision{{Name: "d-1", Approved: true}, {Name: "d-2", Approved: false}}}

	result, svcErr := s.recordDetails(&providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{decision}}, flightDetail("d-1", "450"), flightDetail("d-2", "450"))

	s.Require().Nil(svcErr)
	s.Require().Len(result.GrantedAuthorizationDetails, 1)
	s.Equal("d-1", result.GrantedAuthorizationDetails[0].ID)
}

func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_DeniedPromptGrantsNoCoveredDetails() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)

	result, svcErr := s.recordWithCovered(&providers.ConsentDecisions{Approved: false,
		Purposes: []providers.PurposeDecision{detailDecision("d-2", true)}},
		flightDetail("d-2", "900"), flightDetail("d-1", "450"))

	s.Require().Nil(svcErr)
	s.Empty(result.GrantedAuthorizationDetails)
}

// A covered detail whose type changed after the prompt was shown fails the decision before anything is
// recorded, as a prompted one does.
func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_CoveredDetailOfAChangedType() {
	edited := flightType()
	edited.Version++
	s.withDetailTypes(edited)

	_, svcErr := s.recordWithCovered(&providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-2", false)}},
		flightDetail("d-2", "900"), flightDetail("d-1", "450"))

	s.Require().NotNil(svcErr)
	s.Equal(authzdetail.ErrorAuthorizationDetailTypeChanged.Code, svcErr.Code)
	s.mockConsentSvc.AssertNotCalled(s.T(), "CreateConsent", mock.Anything, mock.Anything)
	s.mockConsentSvc.AssertNotCalled(s.T(), "UpdateConsent", mock.Anything, mock.Anything, mock.Anything)
}

// A covered detail whose approval was revoked while the prompt was open is not granted.
func (s *ConsentEnforcerServiceTestSuite) TestRecordConsent_RevokedApprovalNoLongerCovers() {
	s.withDetailTypes(flightType())
	s.mockConsentSvc.On("SearchConsents", mock.Anything, mock.Anything).Return([]*consent.Consent{}, nil)

	result, svcErr := s.recordWithCovered(&providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-2", false)}},
		flightDetail("d-2", "900"), flightDetail("d-1", "450"))

	s.Require().Nil(svcErr)
	s.Empty(result.GrantedAuthorizationDetails)
}

func (s *ConsentEnforcerServiceTestSuite) TestBuildConsentElementApprovals_SkipsAuthorizationDetailPurposes() {
	session := &consentSessionData{Purposes: []consentSessionPurpose{{
		PurposeName: testDetailPurpose, Optional: []string{"d-1"},
		Details: []providers.AuthorizationDetail{flightDetail("d-1", "450")}}}}
	decisions := &providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}}

	s.Empty(buildConsentElementApprovals(session, decisions))
}

func (s *ConsentEnforcerServiceTestSuite) TestPromptedDetails() {
	details := []providers.AuthorizationDetail{flightDetail("d-1", "450"), flightDetail("d-2", "900")}

	prompted := promptedDetails(providers.ConsentPurposePrompt{Type: consentPromptTypeAuthorizationDetails,
		Optional: []providers.PromptElement{{Name: "d-2"}, {Name: "d-3"}}}, details)
	s.Require().Len(prompted, 1)
	s.Equal("d-2", prompted[0].ID)
	s.Nil(promptedDetails(providers.ConsentPurposePrompt{
		Optional: []providers.PromptElement{{Name: "d-2"}}}, details),
		"only an authorization details purpose prompts for details")
}

func (s *ConsentEnforcerServiceTestSuite) TestApprovedAuthorizationDetails_ReadsOnlyApprovedDetailElements() {
	records := []*consent.Consent{{Purposes: []consent.ConsentPurposeItem{
		{Name: "attributes:app-1", Elements: []consent.ConsentElementApproval{{Name: "email", IsUserApproved: true}}},
		{Name: testDetailPurpose, Elements: []consent.ConsentElementApproval{
			{Name: "d1", Namespace: consent.NamespaceAuthorizationDetail, IsUserApproved: true, TypeID: "t-1",
				TypeVersion: 2, Granted: json.RawMessage(`{"type":"pay","amount":35}`)},
			{Name: "d2", Namespace: consent.NamespaceAuthorizationDetail, TypeVersion: 2},
		}},
		{Name: consent.AuthorizationDetailPurposeName("other-app"), Elements: []consent.ConsentElementApproval{
			{Name: "d3", Namespace: consent.NamespaceAuthorizationDetail, IsUserApproved: true, TypeID: "t-1",
				TypeVersion: 2, Granted: json.RawMessage(`{"type":"pay","amount":99}`)},
		}},
	}}}

	approvals := approvedAuthorizationDetails(records, testDetailPurpose)

	s.Require().Len(approvals, 1)
	s.Equal("t-1", approvals[0].TypeID)
	s.Equal(2, approvals[0].TypeVersion)
	s.JSONEq(`{"type":"pay","amount":35}`, string(approvals[0].Granted))
}

// typedElement is an authorization detail approval of the given type value, type identifier and version.
func typedElement(name, typ, typeID string, version int) consent.ConsentElementApproval {
	return consent.ConsentElementApproval{Name: name, Namespace: consent.NamespaceAuthorizationDetail,
		IsUserApproved: true, TypeID: typeID, TypeVersion: version,
		Granted: json.RawMessage(`{"type":"` + typ + `"}`)}
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeAuthorizationDetailPurposes() {
	d1 := typedElement("d1", "a", "t-a", 1)
	d1.Granted = json.RawMessage(`{"type":"a","scope":"other"}`)
	existing := []consent.ConsentPurposeItem{
		{Name: "attributes:app1", Elements: []consent.ConsentElementApproval{{Name: "email"}}},
		{Name: testDetailPurpose, Elements: []consent.ConsentElementApproval{d1}},
	}
	merged := mergeAuthorizationDetailPurposes(existing, map[string][]consent.ConsentElementApproval{
		testDetailPurpose: {typedElement("d2", "a", "t-a", 1), typedElement("d3", "b", "t-b", 1)},
	}, nil)
	s.Require().Len(merged, 2)
	s.Equal("attributes:app1", merged[0].Name)
	names := make([]string, 0, len(merged[1].Elements))
	for _, element := range merged[1].Elements {
		names = append(names, element.Name)
	}
	s.Equal([]string{"d1", "d2", "d3"}, names, "approvals of every type share the purpose")
	s.Len(existing[1].Elements, 1, "the existing record is not modified")
	s.Equal(existing, mergeAuthorizationDetailPurposes(existing, nil, nil))

	created := mergeAuthorizationDetailPurposes(existing[:1], map[string][]consent.ConsentElementApproval{
		testDetailPurpose: {typedElement("d2", "a", "t-a", 1)},
	}, nil)
	s.Require().Len(created, 2)
	s.Equal(testDetailPurpose, created[1].Name, "the purpose is added when the record has none")
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeAuthorizationDetailPurposes_DropsElementsOfOtherVersions() {
	d2 := typedElement("d2", "a", "t-1", 2)
	d2.Granted = json.RawMessage(`{"type":"a","scope":"other"}`)
	existing := []consent.ConsentPurposeItem{{Name: testDetailPurpose, Elements: []consent.ConsentElementApproval{
		typedElement("d1", "a", "t-1", 1),
		d2,
		typedElement("d4", "a", "t-0", 2),
		typedElement("d5", "b", "t-b", 1),
	}}}
	merged := mergeAuthorizationDetailPurposes(existing, map[string][]consent.ConsentElementApproval{
		testDetailPurpose: {typedElement("d3", "a", "t-1", 2)},
	}, nil)
	s.Require().Len(merged, 1)
	names := make([]string, 0, len(merged[0].Elements))
	for _, element := range merged[0].Elements {
		names = append(names, element.Name)
	}
	s.Equal([]string{"d2", "d5", "d3"}, names,
		"approvals on an earlier version, or on a deleted type of the same value, no longer count; "+
			"those of other types are kept")
	s.Len(existing[0].Elements, 4, "the existing record is not modified")
}

// flightElement is an approval of a flight with the given fare cap on the flight type's current version.
func flightElement(name, fare string) consent.ConsentElementApproval {
	return consent.ConsentElementApproval{Name: name, Namespace: consent.NamespaceAuthorizationDetail,
		IsUserApproved: true, TypeID: "type-1", TypeVersion: 2,
		Granted: json.RawMessage(`{"type":"` + testFlightType + `","route":"CMB-SIN","maxFare":` + fare + `}`)}
}

func (s *ConsentEnforcerServiceTestSuite) TestMergeAuthorizationDetailPurposes_DropsApprovalsTheAddedOneCovers() {
	existing := []consent.ConsentPurposeItem{{Name: testDetailPurpose, Elements: []consent.ConsentElementApproval{
		flightElement("d1", "300"), flightElement("d2", "900"),
	}}}
	merged := mergeAuthorizationDetailPurposes(existing, map[string][]consent.ConsentElementApproval{
		testDetailPurpose: {flightElement("d3", "500")},
	}, map[string]providers.AuthorizationDetailType{testFlightType: *flightType()})

	s.Require().Len(merged, 1)
	names := make([]string, 0, len(merged[0].Elements))
	for _, element := range merged[0].Elements {
		names = append(names, element.Name)
	}
	s.Equal([]string{"d2", "d3"}, names,
		"an approval of 300 is redundant beside one of 500; one of 900 still covers more")
}

func (s *ConsentEnforcerServiceTestSuite) TestSupersedes() {
	typ := *flightType()
	added := flightElement("d2", "500")

	s.True(supersedes(added, flightElement("d1", "500"), typ), "an equal approval is redundant")
	s.False(supersedes(added, flightElement("d1", "600"), typ))
	otherRoute := flightElement("d1", "100")
	otherRoute.Granted = json.RawMessage(`{"type":"` + testFlightType + `","route":"CMB-LHR","maxFare":100}`)
	s.False(supersedes(added, otherRoute, typ), "a field without a modification rule must be equal")
	s.False(supersedes(added, typedElement("d1", "other", "type-9", 2), typ), "other types are kept")
	s.False(supersedes(added, consent.ConsentElementApproval{Name: "email",
		Namespace: consent.NamespaceAttribute, IsUserApproved: true}, typ))
}
