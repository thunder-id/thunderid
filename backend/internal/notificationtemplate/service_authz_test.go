// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	authzOwnOUID   = "01900000-0000-7000-8000-0000000000b1"
	authzOtherOUID = "01900000-0000-7000-8000-0000000000b2"
	authzTemplate  = "01900000-0000-7000-8000-0000000000c1"
)

// TemplateAuthzTestSuite covers the organization unit checks on the template management operations.
type TemplateAuthzTestSuite struct {
	suite.Suite
	mockStore *notificationTemplateStoreInterfaceMock
	authz     *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	svc       NotificationTemplateServiceInterface
	ctx       context.Context
}

func TestTemplateAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(TemplateAuthzTestSuite))
}

func (s *TemplateAuthzTestSuite) SetupTest() {
	initMutableTestRuntime(s.T())
	s.mockStore = newNotificationTemplateStoreInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.svc = newNotificationTemplateService(s.mockStore, executingTransactioner(s.T()), nil, s.authz, nil, false)
	s.ctx = context.Background()
}

// allowOnlyOwnOU allows an action only on templates of the caller's organization unit.
func (s *TemplateAuthzTestSuite) allowOnlyOwnOU(action security.Action) {
	s.authz.On("IsActionAllowed", mock.Anything, action, mock.MatchedBy(func(c *sysauthz.ActionContext) bool {
		return c.ResourceType == security.ResourceTypeNotificationTemplate
	})).Return(func(_ context.Context, _ security.Action, c *sysauthz.ActionContext) (bool,
		*tidcommon.ServiceError) {
		return c.OUID == authzOwnOUID, nil
	})
}

func (s *TemplateAuthzTestSuite) storedTemplate(ouID string) {
	s.mockStore.On("GetTemplate", mock.Anything, ChannelTypeEmail, authzTemplate).Return(templateDAO{
		ID: authzTemplate, Channel: ChannelTypeEmail, Handle: "welcome", OUID: ouID, DisplayName: "Welcome",
		Content: TemplateContent{Subject: "Hi", Body: "<p>Hi</p>"},
	}, nil)
}

func validCreateRequest(ouID string) CreateTemplateRequest {
	return CreateTemplateRequest{
		Handle: "welcome", DisplayName: "Welcome", OUID: ouID,
		Content: TemplateContent{Subject: "Hi", Body: "<p>Hi</p>"},
	}
}

func validUpdateRequest() UpdateTemplateRequest {
	return UpdateTemplateRequest{DisplayName: "Welcome", Content: TemplateContent{Subject: "Hi", Body: "<p>Hi</p>"}}
}

func (s *TemplateAuthzTestSuite) TestGetTemplate_OwnOU() {
	s.storedTemplate(authzOwnOUID)
	s.allowOnlyOwnOU(security.ActionReadNotificationTemplate)

	tmpl, svcErr := s.svc.GetTemplate(s.ctx, ChannelTypeEmail, authzTemplate)
	s.Require().Nil(svcErr)
	s.Equal(authzTemplate, tmpl.ID)
}

func (s *TemplateAuthzTestSuite) TestGetTemplate_OtherOU() {
	s.storedTemplate(authzOtherOUID)
	s.allowOnlyOwnOU(security.ActionReadNotificationTemplate)

	tmpl, svcErr := s.svc.GetTemplate(s.ctx, ChannelTypeEmail, authzTemplate)
	s.Nil(tmpl)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *TemplateAuthzTestSuite) TestGetTemplate_AuthzFailure() {
	s.storedTemplate(authzOwnOUID)
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError)

	_, svcErr := s.svc.GetTemplate(s.ctx, ChannelTypeEmail, authzTemplate)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *TemplateAuthzTestSuite) TestCreateTemplate_OtherOU() {
	s.allowOnlyOwnOU(security.ActionCreateNotificationTemplate)

	tmpl, svcErr := s.svc.CreateTemplate(s.ctx, ChannelTypeEmail, validCreateRequest(authzOtherOUID))
	s.Nil(tmpl)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.mockStore.AssertNotCalled(s.T(), "CreateTemplate", mock.Anything, mock.Anything)
}

func (s *TemplateAuthzTestSuite) TestValidateTemplate_OtherOU() {
	s.allowOnlyOwnOU(security.ActionCreateNotificationTemplate)

	svcErr := s.svc.ValidateTemplate(s.ctx, ChannelTypeEmail, validCreateRequest(authzOtherOUID))
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *TemplateAuthzTestSuite) TestUpdateTemplate_OwnOU() {
	s.storedTemplate(authzOwnOUID)
	s.allowOnlyOwnOU(security.ActionUpdateNotificationTemplate)
	s.mockStore.On("UpdateTemplate", mock.Anything, mock.MatchedBy(func(d templateDAO) bool {
		return d.ID == authzTemplate && d.OUID == authzOwnOUID
	})).Return(nil)

	tmpl, svcErr := s.svc.UpdateTemplate(s.ctx, ChannelTypeEmail, authzTemplate, validUpdateRequest())
	s.Require().Nil(svcErr)
	s.Equal(authzOwnOUID, tmpl.OUID)
}

func (s *TemplateAuthzTestSuite) TestUpdateTemplate_OtherOU() {
	s.storedTemplate(authzOtherOUID)
	s.allowOnlyOwnOU(security.ActionUpdateNotificationTemplate)

	tmpl, svcErr := s.svc.UpdateTemplate(s.ctx, ChannelTypeEmail, authzTemplate, validUpdateRequest())
	s.Nil(tmpl)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.mockStore.AssertNotCalled(s.T(), "UpdateTemplate", mock.Anything, mock.Anything)
}

func (s *TemplateAuthzTestSuite) TestDeleteTemplate_OtherOU() {
	s.storedTemplate(authzOtherOUID)
	s.allowOnlyOwnOU(security.ActionDeleteNotificationTemplate)

	svcErr := s.svc.DeleteTemplate(s.ctx, ChannelTypeEmail, authzTemplate)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.mockStore.AssertNotCalled(s.T(), "DeleteTemplate", mock.Anything, mock.Anything, mock.Anything)
}

func (s *TemplateAuthzTestSuite) TestDeleteTemplate_OwnOU() {
	s.storedTemplate(authzOwnOUID)
	s.allowOnlyOwnOU(security.ActionDeleteNotificationTemplate)
	s.mockStore.On("DeleteTemplate", mock.Anything, ChannelTypeEmail, authzTemplate).Return(nil)

	s.Nil(s.svc.DeleteTemplate(s.ctx, ChannelTypeEmail, authzTemplate))
}

func (s *TemplateAuthzTestSuite) accessible(result *sysauthz.AccessibleResources) {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListNotificationTemplates,
		security.ResourceTypeOU).Return(result, nil)
}

func (s *TemplateAuthzTestSuite) TestListTemplates_AllAllowed() {
	s.accessible(&sysauthz.AccessibleResources{AllAllowed: true})
	s.mockStore.On("CountTemplates", mock.Anything, ChannelTypeEmail).Return(2, nil)
	s.mockStore.On("ListTemplates", mock.Anything, ChannelTypeEmail, 10, 0).Return([]templateDAO{
		{ID: "t1", OUID: authzOwnOUID}, {ID: "t2", OUID: authzOtherOUID},
	}, nil)

	resp, svcErr := s.svc.ListTemplates(s.ctx, ChannelTypeEmail, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(2, resp.TotalResults)
}

func (s *TemplateAuthzTestSuite) TestListTemplates_OwnOUOnly() {
	s.accessible(&sysauthz.AccessibleResources{IDs: []string{authzOwnOUID}})
	s.mockStore.On("CountTemplatesByOUIDs", mock.Anything, ChannelTypeEmail, []string{authzOwnOUID}).Return(1, nil)
	s.mockStore.On("ListTemplatesByOUIDs", mock.Anything, ChannelTypeEmail, []string{authzOwnOUID}, 10, 0).
		Return([]templateDAO{{ID: "t1", OUID: authzOwnOUID}}, nil)

	resp, svcErr := s.svc.ListTemplates(s.ctx, ChannelTypeEmail, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(1, resp.TotalResults)
	s.Require().Len(resp.Templates, 1)
	s.Equal("t1", resp.Templates[0].ID)
}

func (s *TemplateAuthzTestSuite) TestListTemplates_NoAccessibleOU() {
	s.accessible(&sysauthz.AccessibleResources{IDs: []string{}})

	resp, svcErr := s.svc.ListTemplates(s.ctx, ChannelTypeEmail, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(0, resp.TotalResults)
	s.Empty(resp.Templates)
}

func (s *TemplateAuthzTestSuite) TestListTemplatesByOUID_OtherOU() {
	s.accessible(&sysauthz.AccessibleResources{IDs: []string{authzOwnOUID}})

	resp, svcErr := s.svc.ListTemplatesByOUID(s.ctx, ChannelTypeEmail, authzOtherOUID, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(0, resp.TotalResults)
	s.Empty(resp.Templates)
}

func (s *TemplateAuthzTestSuite) TestListTemplates_AuthzFailure() {
	s.authz.On("GetAccessibleResources", mock.Anything, mock.Anything, mock.Anything).
		Return((*sysauthz.AccessibleResources)(nil), &tidcommon.InternalServerError)

	_, svcErr := s.svc.ListTemplates(s.ctx, ChannelTypeEmail, 10, 0)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *TemplateAuthzTestSuite) TestListTemplates_OwnAndAncestorOUs() {
	s.accessible(&sysauthz.AccessibleResources{IDs: []string{authzOwnOUID, authzOtherOUID}})
	ouIDs := []string{authzOwnOUID, authzOtherOUID}
	s.mockStore.On("CountTemplatesByOUIDs", mock.Anything, ChannelTypeEmail, ouIDs).Return(2, nil)
	s.mockStore.On("ListTemplatesByOUIDs", mock.Anything, ChannelTypeEmail, ouIDs, 10, 0).
		Return([]templateDAO{{ID: "t1", OUID: authzOwnOUID}, {ID: "t2", OUID: authzOtherOUID}}, nil)

	resp, svcErr := s.svc.ListTemplates(s.ctx, ChannelTypeEmail, 10, 0)
	s.Require().Nil(svcErr)
	s.Equal(2, resp.TotalResults)
}

func (s *TemplateAuthzTestSuite) TestGetTemplateByHandle_OtherOU() {
	s.mockStore.On("GetTemplateByHandle", mock.Anything, ChannelTypeEmail, "welcome").
		Return(templateDAO{ID: authzTemplate, OUID: authzOtherOUID}, nil)
	s.allowOnlyOwnOU(security.ActionReadNotificationTemplate)

	tmpl, svcErr := s.svc.GetTemplateByHandle(s.ctx, ChannelTypeEmail, "welcome")
	s.Nil(tmpl)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *TemplateAuthzTestSuite) TestGetTemplateByHandle_OtherOUInRuntimeContext() {
	s.mockStore.On("GetTemplateByHandle", mock.Anything, ChannelTypeEmail, "welcome").
		Return(templateDAO{ID: authzTemplate, OUID: authzOtherOUID}, nil)
	s.authz.On("IsActionAllowed", mock.MatchedBy(security.IsRuntimeContext), security.ActionReadNotificationTemplate,
		mock.Anything).Return(true, nil)

	tmpl, svcErr := s.svc.GetTemplateByHandle(security.WithRuntimeContext(s.ctx), ChannelTypeEmail, "welcome")
	s.Require().Nil(svcErr)
	s.Equal(authzTemplate, tmpl.ID)
}
