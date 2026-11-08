// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

// depCtx returns a context carrying the deployment id, mirroring what the edge middleware puts on
// every request. The store resolves its scope from this, so tests must set it explicitly rather
// than relying on the runtime-config fallback.
func depCtx(id string) context.Context {
	return deployment.WithID(context.Background(), id)
}

// NotificationTemplateStoreTestSuite exercises the store methods against a mocked DB client, so the
// query wiring (arguments, row mapping, error paths) is covered, not just the row helpers.
type NotificationTemplateStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *notificationTemplateStore
}

func TestNotificationTemplateStoreTestSuite(t *testing.T) {
	suite.Run(t, new(NotificationTemplateStoreTestSuite))
}

func (s *NotificationTemplateStoreTestSuite) SetupTest() {
	s.mockDBProvider = providermock.NewDBProviderInterfaceMock(s.T())
	s.mockDBClient = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &notificationTemplateStore{dbProvider: s.mockDBProvider}
}

func (s *NotificationTemplateStoreTestSuite) TestCreateTemplate_EmailWithDesign() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateTemplate,
		"id-1", "email", "otp", "ou-1", "proj-1", "OTP", "desc",
		`{"subject":"s","body":"b"}`, `{"colorScheme":"dark"}`, "test-deployment").Return(int64(1), nil)

	err := s.store.CreateTemplate(depCtx("test-deployment"), templateDAO{
		ID: "id-1", Channel: ChannelTypeEmail, Handle: "otp", OUID: "ou-1", ProjectID: "proj-1", DisplayName: "OTP",
		Description: "desc",
		Content:     TemplateContent{Subject: "s", Body: "b"},
		Design:      &TemplateDesign{ColorScheme: colorSchemeDark},
	})
	s.Require().NoError(err)
}

func (s *NotificationTemplateStoreTestSuite) TestCreateTemplate_NoDesignStoresEmptyObject() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	// DESIGN is NOT NULL, so a template without a design persists "{}".
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateTemplate,
		"id-2", "sms", "otp", "ou-1", nil, "OTP", "", `{"body":"b"}`, "{}", "test-deployment").Return(int64(1), nil)

	err := s.store.CreateTemplate(depCtx("test-deployment"), templateDAO{
		ID: "id-2", Channel: ChannelTypeSMS, Handle: "otp", OUID: "ou-1", DisplayName: "OTP",
		Content: TemplateContent{Body: "b"},
	})
	s.Require().NoError(err)
}

func foundTemplateRow() map[string]interface{} {
	return map[string]interface{}{
		"id": "id-1", "channel": "email", "handle": "otp", "ou_id": "ou-1", "display_name": "OTP",
		"description": "desc", "content": `{"subject":"s","body":"b"}`, "design": `{"colorScheme":"dark"}`,
	}
}

func (s *NotificationTemplateStoreTestSuite) TestGetTemplate_Found() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetTemplateByID, "id-1", "email", "test-deployment").
		Return([]map[string]interface{}{foundTemplateRow()}, nil)

	dao, err := s.store.GetTemplate(depCtx("test-deployment"), ChannelTypeEmail, "id-1")
	s.Require().NoError(err)
	s.Require().Equal("otp", dao.Handle)
	s.Require().Equal("ou-1", dao.OUID)
	s.Require().Equal("s", dao.Content.Subject)
	s.Require().NotNil(dao.Design)
	s.Require().Equal(colorSchemeDark, dao.Design.ColorScheme)
}

func (s *NotificationTemplateStoreTestSuite) TestGetTemplate_NotFound() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetTemplateByID, "missing", "email", "test-deployment").
		Return([]map[string]interface{}{}, nil)

	_, err := s.store.GetTemplate(depCtx("test-deployment"), ChannelTypeEmail, "missing")
	s.Require().ErrorIs(err, errTemplateNotFound)
}

func (s *NotificationTemplateStoreTestSuite) TestGetTemplateByHandle_Found() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetTemplateByHandle, "otp", "email", "test-deployment").
		Return([]map[string]interface{}{foundTemplateRow()}, nil)

	dao, err := s.store.GetTemplateByHandle(depCtx("test-deployment"), ChannelTypeEmail, "otp")
	s.Require().NoError(err)
	s.Require().Equal("id-1", dao.ID)
	s.Require().Equal("ou-1", dao.OUID)
	s.Require().Equal("s", dao.Content.Subject)
	s.Require().NotNil(dao.Design)
	s.Require().Equal(colorSchemeDark, dao.Design.ColorScheme)
}

func (s *NotificationTemplateStoreTestSuite) TestGetTemplateByHandle_NotFound() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetTemplateByHandle, "missing", "email", "test-deployment").
		Return([]map[string]interface{}{}, nil)

	_, err := s.store.GetTemplateByHandle(depCtx("test-deployment"), ChannelTypeEmail, "missing")
	s.Require().ErrorIs(err, errTemplateNotFound)
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplates() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryListTemplates, "email", 10, 0, "test-deployment").
		Return([]map[string]interface{}{
			{"id": "a", "channel": "email", "handle": "a", "ou_id": "ou-1", "display_name": "A"},
			{"id": "b", "channel": "email", "handle": "b", "ou_id": "ou-1", "display_name": "B"},
		}, nil)

	list, err := s.store.ListTemplates(depCtx("test-deployment"), ChannelTypeEmail, 10, 0)
	s.Require().NoError(err)
	s.Require().Len(list, 2)
	s.Require().Equal("a", list[0].Handle)
}

func (s *NotificationTemplateStoreTestSuite) TestCountTemplates() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryCountTemplates, "email", "test-deployment").
		Return([]map[string]interface{}{{"total": int64(3)}}, nil)

	n, err := s.store.CountTemplates(depCtx("test-deployment"), ChannelTypeEmail)
	s.Require().NoError(err)
	s.Require().Equal(3, n)
}

func (s *NotificationTemplateStoreTestSuite) TestIsHandleExists() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryCheckHandleExists, "email", "otp", "test-deployment").
		Return([]map[string]interface{}{{"total": int64(1)}}, nil)

	exists, err := s.store.IsHandleExists(depCtx("test-deployment"), ChannelTypeEmail, "otp")
	s.Require().NoError(err)
	s.Require().True(exists)
}

func (s *NotificationTemplateStoreTestSuite) TestUpdateTemplate() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateTemplate,
		"OTP", "desc", `{"subject":"s","body":"b"}`, `{"colorScheme":"dark"}`, "proj-1",
		"id-1", "email", "test-deployment").Return(int64(1), nil)

	err := s.store.UpdateTemplate(depCtx("test-deployment"), templateDAO{
		ID: "id-1", Channel: ChannelTypeEmail, ProjectID: "proj-1", DisplayName: "OTP", Description: "desc",
		Content: TemplateContent{Subject: "s", Body: "b"},
		Design:  &TemplateDesign{ColorScheme: colorSchemeDark},
	})
	s.Require().NoError(err)
}

func (s *NotificationTemplateStoreTestSuite) TestDeleteTemplate() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteTemplate, "id-1", "email", "test-deployment").
		Return(int64(1), nil)

	err := s.store.DeleteTemplate(depCtx("test-deployment"), ChannelTypeEmail, "id-1")
	s.Require().NoError(err)
}

func (s *NotificationTemplateStoreTestSuite) TestGetTemplate_DBClientError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	_, err := s.store.GetTemplate(depCtx("test-deployment"), ChannelTypeEmail, "id-1")
	s.Require().Error(err)
}

func (s *NotificationTemplateStoreTestSuite) TestGetTemplate_QueryError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetTemplateByID, "id-1", "email", "test-deployment").
		Return(nil, errors.New("query failed"))

	_, err := s.store.GetTemplate(depCtx("test-deployment"), ChannelTypeEmail, "id-1")
	s.Require().Error(err)
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplatesByOUIDs() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, buildListTemplatesByOUIDsQuery(1), "email", "test-deployment", 10,
		0, "ou-1").
		Return([]map[string]interface{}{
			{"id": "a", "channel": "email", "handle": "a", "ou_id": "ou-1", "display_name": "A"},
		}, nil)

	list, err := s.store.ListTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{"ou-1"}, 10, 0)
	s.Require().NoError(err)
	s.Require().Len(list, 1)
	s.Require().Equal("ou-1", list[0].OUID)
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplatesByOUIDs_SeveralOUs() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, buildListTemplatesByOUIDsQuery(2), "email", "test-deployment",
		10, 0, "ou-1", "ou-2").Return([]map[string]interface{}{
		{"id": "a", "channel": "email", "handle": "a", "ou_id": "ou-1", "display_name": "A"},
		{"id": "b", "channel": "email", "handle": "b", "ou_id": "ou-2", "display_name": "B"},
	}, nil)

	list, err := s.store.ListTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail,
		[]string{"ou-1", "ou-2"}, 10, 0)
	s.Require().NoError(err)
	s.Require().Len(list, 2)
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplatesByOUIDs_NoOUs() {
	list, err := s.store.ListTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{}, 10, 0)
	s.Require().NoError(err)
	s.Empty(list)
	count, err := s.store.CountTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, nil)
	s.Require().NoError(err)
	s.Zero(count)
	s.mockDBProvider.AssertNotCalled(s.T(), "GetConfigDBClient")
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplatesByOUIDs_Errors() {
	s.Run("DBClientError", func() {
		s.SetupTest()
		s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

		_, err := s.store.ListTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{"ou-1"}, 10, 0)
		s.Require().Error(err)
	})
	s.Run("QueryError", func() {
		s.SetupTest()
		s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
		s.mockDBClient.On("QueryContext", mock.Anything, buildListTemplatesByOUIDsQuery(1), "email", "test-deployment",
			10,
			0, "ou-1").Return(nil, errors.New("query failed"))

		_, err := s.store.ListTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{"ou-1"}, 10, 0)
		s.Require().Error(err)
	})
	s.Run("InvalidRow", func() {
		s.SetupTest()
		s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
		s.mockDBClient.On("QueryContext", mock.Anything, buildListTemplatesByOUIDsQuery(1), "email", "test-deployment",
			10,
			0, "ou-1").Return([]map[string]interface{}{{"id": 1}}, nil)

		_, err := s.store.ListTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{"ou-1"}, 10, 0)
		s.Require().Error(err)
	})
}

func (s *NotificationTemplateStoreTestSuite) TestCountTemplatesByOUIDs() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, buildCountTemplatesByOUIDsQuery(1), "email",
		"test-deployment", "ou-1").Return([]map[string]interface{}{{"total": int64(4)}}, nil)

	count, err := s.store.CountTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{"ou-1"})
	s.Require().NoError(err)
	s.Require().Equal(4, count)
}

func (s *NotificationTemplateStoreTestSuite) TestCountTemplatesByOUIDs_Errors() {
	s.Run("DBClientError", func() {
		s.SetupTest()
		s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

		_, err := s.store.CountTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{"ou-1"})
		s.Require().Error(err)
	})
	s.Run("QueryError", func() {
		s.SetupTest()
		s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
		s.mockDBClient.On("QueryContext", mock.Anything, buildCountTemplatesByOUIDsQuery(1), "email",
			"test-deployment", "ou-1").Return(nil, errors.New("query failed"))

		_, err := s.store.CountTemplatesByOUIDs(depCtx("test-deployment"), ChannelTypeEmail, []string{"ou-1"})
		s.Require().Error(err)
	})
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplates_DBClientError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))

	_, err := s.store.ListTemplates(depCtx("test-deployment"), ChannelTypeEmail, 10, 0)
	s.Require().Error(err)
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplates_QueryError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryListTemplates, "email", 10, 0, "test-deployment").
		Return(nil, errors.New("query failed"))

	_, err := s.store.ListTemplates(depCtx("test-deployment"), ChannelTypeEmail, 10, 0)
	s.Require().Error(err)
}

func (s *NotificationTemplateStoreTestSuite) TestCreateTemplate_ExecError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryCreateTemplate,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), errors.New("insert failed"))

	err := s.store.CreateTemplate(depCtx("test-deployment"), templateDAO{
		ID: "id", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "b"},
	})
	s.Require().Error(err)
}

func (s *NotificationTemplateStoreTestSuite) TestDeleteTemplate_ExecError() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeleteTemplate, "id", "email", "test-deployment").
		Return(int64(0), errors.New("delete failed"))

	err := s.store.DeleteTemplate(depCtx("test-deployment"), ChannelTypeEmail, "id")
	s.Require().Error(err)
}

// TestScopeFollowsContextDeployment proves the store scopes each query to the deployment carried on
// the request context, not a value captured at construction. Two calls on the same store with
// different context deployments must reach the DB with different DEPLOYMENT_ID arguments; this is
// what guards multi-deployment isolation and would fail if the store ever reverted to a cached id.
func (s *NotificationTemplateStoreTestSuite) TestScopeFollowsContextDeployment() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetTemplateByID, "id-1", "email", "tenant-a").
		Return([]map[string]interface{}{}, nil).Once()
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetTemplateByID, "id-1", "email", "tenant-b").
		Return([]map[string]interface{}{}, nil).Once()

	_, errA := s.store.GetTemplate(depCtx("tenant-a"), ChannelTypeEmail, "id-1")
	s.Require().ErrorIs(errA, errTemplateNotFound)
	_, errB := s.store.GetTemplate(depCtx("tenant-b"), ChannelTypeEmail, "id-1")
	s.Require().ErrorIs(errB, errTemplateNotFound)
}

func (s *NotificationTemplateStoreTestSuite) TestListTemplatesByProject() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryListTemplatesByProject, "email", "proj-1", 10, 0,
		"test-deployment").
		Return([]map[string]interface{}{
			{"id": "a", "channel": "email", "handle": "a", "ou_id": "ou-1", "project_id": "proj-1",
				"display_name": "A"},
		}, nil)

	list, err := s.store.ListTemplatesByProject(depCtx("test-deployment"), ChannelTypeEmail, "proj-1", 10, 0)
	s.Require().NoError(err)
	s.Require().Len(list, 1)
	s.Require().Equal("proj-1", list[0].ProjectID)
}

func (s *NotificationTemplateStoreTestSuite) TestCountTemplatesByProject() {
	s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryCountTemplatesByProject, "email", "proj-1",
		"test-deployment").Return([]map[string]interface{}{{"total": int64(2)}}, nil)

	count, err := s.store.CountTemplatesByProject(depCtx("test-deployment"), ChannelTypeEmail, "proj-1")
	s.Require().NoError(err)
	s.Require().Equal(2, count)
}

// TestTemplatesByProject_Errors covers the failures of the project list and count: no database client, a
// failing query, and an unreadable row.
func (s *NotificationTemplateStoreTestSuite) TestTemplatesByProject_Errors() {
	ctx := depCtx("test-deployment")
	calls := map[string]func() error{
		"list": func() error {
			_, err := s.store.ListTemplatesByProject(ctx, ChannelTypeEmail, "proj-1", 10, 0)
			return err
		},
		"count": func() error {
			_, err := s.store.CountTemplatesByProject(ctx, ChannelTypeEmail, "proj-1")
			return err
		},
	}
	for name, call := range calls {
		s.Run(name+"/DBClientError", func() {
			s.SetupTest()
			s.mockDBProvider.On("GetConfigDBClient").Return(nil, errors.New("connection error"))
			s.Require().Error(call())
		})
		s.Run(name+"/QueryError", func() {
			s.SetupTest()
			s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
			s.mockDBClient.On("QueryContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("query failed")).Maybe()
			s.mockDBClient.On("QueryContext", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
				mock.Anything).Return(nil, errors.New("query failed")).Maybe()
			s.Require().Error(call())
		})
	}
	s.Run("list/InvalidRow", func() {
		s.SetupTest()
		s.mockDBProvider.On("GetConfigDBClient").Return(s.mockDBClient, nil)
		s.mockDBClient.On("QueryContext", mock.Anything, queryListTemplatesByProject, "email", "proj-1", 10, 0,
			"test-deployment").Return([]map[string]interface{}{{"id": 1}}, nil)
		s.Require().Error(calls["list"]())
	})
}

func (s *NotificationTemplateStoreTestSuite) TestBuildTemplatesByOUIDsQueries() {
	s.Contains(buildListTemplatesByOUIDsQuery(3).Query, "OU_ID IN ($5, $6, $7)")
	s.Contains(buildCountTemplatesByOUIDsQuery(2).Query, "OU_ID IN ($3, $4)")
}
