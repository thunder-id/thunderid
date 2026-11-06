// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entityprovider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

type DefaultEntityProviderTestSuite struct {
	suite.Suite
	mockService *entitymock.EntityServiceInterfaceMock
	mockAuthz   *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	provider    EntityProviderInterface
}

func (suite *DefaultEntityProviderTestSuite) SetupTest() {
	suite.mockService = entitymock.NewEntityServiceInterfaceMock(suite.T())
	suite.mockAuthz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	suite.provider = newDefaultEntityProvider(suite.mockService, suite.mockAuthz)
}

// allowWrites lets the caller write to the test entity, a user.
func (suite *DefaultEntityProviderTestSuite) allowWrites() {
	suite.mockService.On("GetEntityProfile", mock.MatchedBy(security.IsRuntimeContext), testEntityID).
		Return(&providers.Entity{ID: testEntityID, Category: providers.EntityCategoryUser, OUID: "ou-1"}, nil).
		Maybe()
	suite.mockAuthz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Maybe()
}

func TestDefaultEntityProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultEntityProviderTestSuite))
}

const testEntityID = "entity123"

func (suite *DefaultEntityProviderTestSuite) TestIdentifyEntity() {
	filters := map[string]interface{}{"clientId": "test-client"}
	idAddr := testEntityID

	// Test Success
	suite.mockService.On("IdentifyEntity", mock.Anything, filters).Return(&idAddr, nil).Once()

	id, err := suite.provider.IdentifyEntity(context.Background(), filters)
	suite.Nil(err)
	suite.Equal(testEntityID, *id)

	// Test Not Found
	suite.mockService.On("IdentifyEntity", mock.Anything, filters).
		Return(nil, entity.ErrEntityNotFound).Once()

	id, err = suite.provider.IdentifyEntity(context.Background(), filters)
	suite.Nil(id)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("IdentifyEntity", mock.Anything, filters).
		Return(nil, errors.New("db error")).Once()

	id, err = suite.provider.IdentifyEntity(context.Background(), filters)
	suite.Nil(id)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetLinkedEntities() {
	holders := []providers.Entity{{ID: "e1", OUHandle: "sales"}, {ID: "e2", OUHandle: "support"}}
	suite.mockService.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").Return(holders, nil).Once()

	got, err := suite.provider.GetLinkedEntities(context.Background(), "idp-a", "sub-1")
	suite.Nil(err)
	suite.Require().Len(got, 2)
	suite.Equal("sales", got[0].OUHandle)
	suite.Equal("support", got[1].OUHandle)

	suite.mockService.On("GetLinkedEntities", mock.Anything, "idp-a", "sub-1").
		Return(nil, errors.New("db error")).Once()

	got, err = suite.provider.GetLinkedEntities(context.Background(), "idp-a", "sub-1")
	suite.Nil(got)
	suite.Require().NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntity() {
	expected := &providers.Entity{
		ID:       testEntityID,
		Category: providers.EntityCategoryUser,
		Type:     "customer",
	}

	// Test Success
	suite.mockService.On("GetEntity", mock.Anything, testEntityID).Return(expected, nil).Once()

	e, err := suite.provider.GetEntity(context.Background(), testEntityID)
	suite.Nil(err)
	suite.Equal(testEntityID, e.ID)
	suite.Equal(providers.EntityCategory("user"), e.Category)

	// Test Not Found
	suite.mockService.On("GetEntity", mock.Anything, testEntityID).
		Return(nil, entity.ErrEntityNotFound).Once()

	e, err = suite.provider.GetEntity(context.Background(), testEntityID)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)
}

// A profile read goes to the entity service's profile read, so it never loads the runtime row.
func (suite *DefaultEntityProviderTestSuite) TestGetEntityProfile() {
	expected := &providers.Entity{
		ID:       testEntityID,
		Category: providers.EntityCategoryUser,
		Type:     "customer",
	}

	suite.mockService.On("GetEntityProfile", mock.Anything, testEntityID).Return(expected, nil).Once()

	e, err := suite.provider.GetEntityProfile(context.Background(), testEntityID)
	suite.Nil(err)
	suite.Equal(testEntityID, e.ID)
	suite.Equal(providers.EntityCategory("user"), e.Category)
	suite.mockService.AssertNotCalled(suite.T(), "GetEntity", mock.Anything, mock.Anything)

	suite.mockService.On("GetEntityProfile", mock.Anything, testEntityID).
		Return(nil, entity.ErrEntityNotFound).Once()

	e, err = suite.provider.GetEntityProfile(context.Background(), testEntityID)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateEntity() {
	suite.allowWrites()
	providerEntity := &providers.Entity{
		ID:   testEntityID,
		Type: "customer",
	}
	updated := &providers.Entity{
		ID:   testEntityID,
		Type: "customer",
	}

	// Test Success
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(updated, nil).Once()

	e, err := suite.provider.UpdateEntity(context.Background(), testEntityID, providerEntity)
	suite.Nil(err)
	suite.Equal(testEntityID, e.ID)

	// Test Nil Entity
	e, err = suite.provider.UpdateEntity(context.Background(), testEntityID, nil)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)

	// Test Not Found
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrEntityNotFound).Once()

	e, err = suite.provider.UpdateEntity(context.Background(), testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Attribute Conflict
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrAttributeConflict).Once()

	e, err = suite.provider.UpdateEntity(context.Background(), testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeAttributeConflict, err.Code)

	// Test Schema Validation Failed
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrSchemaValidationFailed).Once()

	e, err = suite.provider.UpdateEntity(context.Background(), testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSchemaValidationFailed, err.Code)

	// Test Bad Attributes In Request
	suite.mockService.On("UpdateEntity", mock.Anything, testEntityID, mock.Anything).
		Return(nil, entity.ErrBadAttributesInRequest).Once()

	e, err = suite.provider.UpdateEntity(context.Background(), testEntityID, providerEntity)
	suite.Nil(e)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateCredentials() {
	suite.allowWrites()
	cases := []struct {
		method string
		creds  json.RawMessage
		update func(context.Context, string, json.RawMessage) *EntityProviderError
	}{
		{"UpdateCredentials", json.RawMessage(`{"password":"newpassword"}`), suite.provider.UpdateCredentials},
		{"UpdateSystemCredentials", json.RawMessage(`{"clientSecret":"secret"}`),
			suite.provider.UpdateSystemCredentials},
	}
	for _, tc := range cases {
		suite.Run(tc.method, func() {
			// Test Success
			suite.mockService.On(tc.method, mock.Anything, testEntityID, tc.creds).Return(nil).Once()

			err := tc.update(context.Background(), testEntityID, tc.creds)
			suite.Nil(err)

			// Test Not Found
			suite.mockService.On(tc.method, mock.Anything, testEntityID, tc.creds).
				Return(entity.ErrEntityNotFound).Once()

			err = tc.update(context.Background(), testEntityID, tc.creds)
			suite.NotNil(err)
			suite.Equal(ErrorCodeEntityNotFound, err.Code)

			// Test Invalid Credential
			suite.mockService.On(tc.method, mock.Anything, testEntityID, tc.creds).
				Return(entity.ErrInvalidCredential).Once()

			err = tc.update(context.Background(), testEntityID, tc.creds)
			suite.NotNil(err)
			suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)
		})
	}
}

func (suite *DefaultEntityProviderTestSuite) TestDeleteEntity() {
	suite.allowWrites()
	// Test Success
	suite.mockService.On("DeleteEntity", mock.Anything, testEntityID).Return(nil).Once()

	err := suite.provider.DeleteEntity(context.Background(), testEntityID)
	suite.Nil(err)

	// Test Not Found (returns nil — idempotent delete)
	suite.mockService.On("DeleteEntity", mock.Anything, testEntityID).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.DeleteEntity(context.Background(), testEntityID)
	suite.Nil(err)

	// Test System Error
	suite.mockService.On("DeleteEntity", mock.Anything, testEntityID).
		Return(errors.New("db error")).Once()

	err = suite.provider.DeleteEntity(context.Background(), testEntityID)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestUpdateSystemAttributes() {
	attrs := json.RawMessage(`{"name":"test"}`)

	// Test Success
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(nil).Once()

	err := suite.provider.UpdateSystemAttributes(context.Background(), testEntityID, attrs)
	suite.Nil(err)

	// Test Not Found
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrEntityNotFound).Once()

	err = suite.provider.UpdateSystemAttributes(context.Background(), testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test Attribute Conflict
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrAttributeConflict).Once()

	err = suite.provider.UpdateSystemAttributes(context.Background(), testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeAttributeConflict, err.Code)

	// Test Bad Attributes In Request
	suite.mockService.On("UpdateSystemAttributes", mock.Anything, testEntityID, attrs).
		Return(entity.ErrBadAttributesInRequest).Once()

	err = suite.provider.UpdateSystemAttributes(context.Background(), testEntityID, attrs)
	suite.NotNil(err)
	suite.Equal(ErrorCodeInvalidRequestFormat, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestMapEntityError() {
	// Verifies the centralized error mapping helper.
	cases := []struct {
		name     string
		err      error
		expected ErrorCode
	}{
		{"EntityNotFound", entity.ErrEntityNotFound, ErrorCodeEntityNotFound},
		{"AttributeConflict", entity.ErrAttributeConflict, ErrorCodeAttributeConflict},
		{"SchemaValidationFailed", entity.ErrSchemaValidationFailed, ErrorCodeSchemaValidationFailed},
		{"InvalidCredential", entity.ErrInvalidCredential, ErrorCodeInvalidRequestFormat},
		{"BadAttributesInRequest", entity.ErrBadAttributesInRequest, ErrorCodeInvalidRequestFormat},
		{"Unknown", errors.New("unexpected"), ErrorCodeSystemError},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			result := mapEntityError(tc.err)
			suite.NotNil(result)
			suite.Equal(tc.expected, result.Code)
		})
	}
}

func (suite *DefaultEntityProviderTestSuite) TestGetTransitiveEntityGroups() {
	groups := []providers.EntityGroup{
		{ID: "g1", Name: "Group 1", OUID: "ou1"},
		{ID: "g2", Name: "Group 2", OUID: "ou1"},
	}

	// Test Success
	suite.mockService.On("GetTransitiveEntityGroups", mock.Anything, testEntityID).
		Return(groups, nil).Once()

	result, err := suite.provider.GetTransitiveEntityGroups(context.Background(), testEntityID)
	suite.Nil(err)
	suite.Len(result, 2)
	suite.Equal("g1", result[0].ID)

	// Test Not Found
	suite.mockService.On("GetTransitiveEntityGroups", mock.Anything, testEntityID).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.GetTransitiveEntityGroups(context.Background(), testEntityID)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("GetTransitiveEntityGroups", mock.Anything, testEntityID).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.GetTransitiveEntityGroups(context.Background(), testEntityID)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestValidateEntityIDs() {
	ids := []string{"id1", "id2"}

	// Test Success - all valid
	suite.mockService.On("ValidateEntityIDs", mock.Anything, ids).
		Return([]string{}, nil).Once()

	invalid, err := suite.provider.ValidateEntityIDs(context.Background(), ids)
	suite.Nil(err)
	suite.Empty(invalid)

	// Test Success - some invalid
	suite.mockService.On("ValidateEntityIDs", mock.Anything, ids).
		Return([]string{"id2"}, nil).Once()

	invalid, err = suite.provider.ValidateEntityIDs(context.Background(), ids)
	suite.Nil(err)
	suite.Equal([]string{"id2"}, invalid)

	// Test System Error
	suite.mockService.On("ValidateEntityIDs", mock.Anything, ids).
		Return(nil, errors.New("db error")).Once()

	invalid, err = suite.provider.ValidateEntityIDs(context.Background(), ids)
	suite.Nil(invalid)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

func (suite *DefaultEntityProviderTestSuite) TestGetEntitiesByIDs() {
	ids := []string{"id1"}
	entities := []providers.Entity{
		{ID: "id1", Category: providers.EntityCategoryUser, Type: "customer"},
	}

	// Test Success
	suite.mockService.On("GetEntitiesByIDs", mock.Anything, ids).Return(entities, nil).Once()

	result, err := suite.provider.GetEntitiesByIDs(context.Background(), ids)
	suite.Nil(err)
	suite.Len(result, 1)
	suite.Equal("id1", result[0].ID)

	// Test Not Found
	suite.mockService.On("GetEntitiesByIDs", mock.Anything, ids).
		Return(nil, entity.ErrEntityNotFound).Once()

	result, err = suite.provider.GetEntitiesByIDs(context.Background(), ids)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeEntityNotFound, err.Code)

	// Test System Error
	suite.mockService.On("GetEntitiesByIDs", mock.Anything, ids).
		Return(nil, errors.New("db error")).Once()

	result, err = suite.provider.GetEntitiesByIDs(context.Background(), ids)
	suite.Nil(result)
	suite.NotNil(err)
	suite.Equal(ErrorCodeSystemError, err.Code)
}

// writeCall invokes one of the writes the provider authorizes.
type writeCall func(p EntityProviderInterface, ctx context.Context) *EntityProviderError

func (suite *DefaultEntityProviderTestSuite) writes() map[string]writeCall {
	attrs := json.RawMessage(`{"name":"test"}`)
	return map[string]writeCall{
		"UpdateEntity": func(p EntityProviderInterface, ctx context.Context) *EntityProviderError {
			_, err := p.UpdateEntity(ctx, testEntityID, &providers.Entity{ID: testEntityID})
			return err
		},
		"DeleteEntity": func(p EntityProviderInterface, ctx context.Context) *EntityProviderError {
			return p.DeleteEntity(ctx, testEntityID)
		},
		"UpdateCredentials": func(p EntityProviderInterface, ctx context.Context) *EntityProviderError {
			return p.UpdateCredentials(ctx, testEntityID, attrs)
		},
		"UpdateAttributes": func(p EntityProviderInterface, ctx context.Context) *EntityProviderError {
			return p.UpdateAttributes(ctx, testEntityID, attrs)
		},
	}
}

// expectWrite expects the named write on the entity service, run with the caller's own context.
func (suite *DefaultEntityProviderTestSuite) expectWrite(method string, ctx context.Context) {
	switch method {
	case "UpdateEntity":
		suite.mockService.On(method, ctx, testEntityID, mock.Anything).
			Return(&providers.Entity{ID: testEntityID}, nil).Once()
	case "DeleteEntity":
		suite.mockService.On(method, ctx, testEntityID).Return(nil).Once()
	default:
		suite.mockService.On(method, ctx, testEntityID, mock.Anything).Return(nil).Once()
	}
}

// The entity service mock is strict, so a write that runs after a refusal fails the test.
func (suite *DefaultEntityProviderTestSuite) TestWritesAuthorizeTheCallerForTheEntityCategory() {
	categories := []struct {
		category     providers.EntityCategory
		update       security.Action
		del          security.Action
		resourceType security.ResourceType
		ouID         string
	}{
		{providers.EntityCategoryUser, security.ActionUpdateUser, security.ActionDeleteUser,
			security.ResourceTypeUser, "ou-1"},
		{providers.EntityCategoryAgent, security.ActionUpdateAgent, security.ActionDeleteAgent,
			security.ResourceTypeAgent, "ou-1"},
		{providers.EntityCategoryApp, security.ActionUpdateApplication, security.ActionDeleteApplication,
			security.ResourceTypeApplication, ""},
	}
	decisions := []struct {
		name    string
		allowed bool
		svcErr  *tidcommon.ServiceError
		code    ErrorCode
	}{
		{"allowed", true, nil, ""},
		{"denied", false, nil, ErrorCodeUnauthorized},
		{"authz error", false, &tidcommon.InternalServerError, ErrorCodeSystemError},
	}
	ctx := context.Background()
	for _, c := range categories {
		for method, call := range suite.writes() {
			for _, d := range decisions {
				suite.Run(string(c.category)+"/"+method+"/"+d.name, func() {
					suite.SetupTest()
					suite.mockService.On("GetEntityProfile", mock.MatchedBy(security.IsRuntimeContext), testEntityID).
						Return(&providers.Entity{ID: testEntityID, Category: c.category, OUID: "ou-1"}, nil).Once()
					action := c.update
					if method == "DeleteEntity" {
						action = c.del
					}
					suite.mockAuthz.On("IsActionAllowed", ctx, action, &sysauthz.ActionContext{
						OUID: c.ouID, ResourceType: c.resourceType, ResourceID: testEntityID,
					}).Return(d.allowed, d.svcErr).Once()
					if d.allowed {
						suite.expectWrite(method, ctx)
					}

					err := call(suite.provider, ctx)

					if d.code == "" {
						suite.Nil(err)
						return
					}
					suite.Require().NotNil(err)
					suite.Equal(d.code, err.Code)
				})
			}
		}
	}
}

func (suite *DefaultEntityProviderTestSuite) TestWritesToAnUngovernedCategoryNeedRuntime() {
	for method, call := range suite.writes() {
		suite.Run(method, func() {
			suite.SetupTest()
			suite.mockService.On("GetEntityProfile", mock.Anything, testEntityID).
				Return(&providers.Entity{ID: testEntityID, Category: "device"}, nil)

			err := call(suite.provider, context.Background())

			suite.Require().NotNil(err)
			suite.Equal(ErrorCodeUnauthorized, err.Code)

			runtimeCtx := security.WithRuntimeContext(context.Background())
			suite.expectWrite(method, runtimeCtx)
			suite.Nil(call(suite.provider, runtimeCtx))
			suite.mockAuthz.AssertNotCalled(suite.T(), "IsActionAllowed", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func (suite *DefaultEntityProviderTestSuite) TestWritesFailWhenTheEntityCannotBeRead() {
	for method, call := range suite.writes() {
		suite.Run(method, func() {
			suite.SetupTest()
			suite.mockService.On("GetEntityProfile", mock.Anything, testEntityID).
				Return(nil, errors.New("db error")).Once()

			err := call(suite.provider, context.Background())

			suite.Require().NotNil(err)
			suite.Equal(ErrorCodeSystemError, err.Code)
		})
	}
}

func (suite *DefaultEntityProviderTestSuite) TestDeleteEntity_MissingEntityIsNotDeleted() {
	suite.mockService.On("GetEntityProfile", mock.Anything, testEntityID).Return(nil, entity.ErrEntityNotFound).Once()

	suite.Nil(suite.provider.DeleteEntity(context.Background(), testEntityID))
}

// The system authorization service decides runtime callers and self-service owners, and the provider defers to it.
func (suite *DefaultEntityProviderTestSuite) TestWritesDeferToTheSystemAuthorizationRules() {
	security.InitSystemPermissions("")
	authz, err := sysauthz.Initialize()
	suite.Require().NoError(err)
	provider := newDefaultEntityProvider(suite.mockService, authz)
	suite.mockService.On("GetEntityProfile", mock.Anything, testEntityID).
		Return(&providers.Entity{ID: testEntityID, Category: providers.EntityCategoryUser, OUID: "ou-1"}, nil)
	attrs := json.RawMessage(`{"name":"test"}`)

	runtimeCtx := security.WithRuntimeContext(context.Background())
	suite.mockService.On("UpdateAttributes", runtimeCtx, testEntityID, attrs).Return(nil).Once()
	suite.Nil(provider.UpdateAttributes(runtimeCtx, testEntityID, attrs))

	ownerCtx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testEntityID, "ou-1", "token", nil, nil))
	suite.mockService.On("UpdateAttributes", ownerCtx, testEntityID, attrs).Return(nil).Once()
	suite.Nil(provider.UpdateAttributes(ownerCtx, testEntityID, attrs))

	otherCtx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("someone-else", "ou-1", "token", nil, nil))
	denied := provider.UpdateAttributes(otherCtx, testEntityID, attrs)
	suite.Require().NotNil(denied)
	suite.Equal(ErrorCodeUnauthorized, denied.Code)
}
