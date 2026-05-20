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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool {
	return &v
}

func updateOUCommon(existing map[string]ou.OrganizationUnit, updated *[]ou.OrganizationUnitRequest,
	id string, request ou.OrganizationUnitRequest) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	if _, ok := existing[id]; !ok {
		return ou.OrganizationUnit{}, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "OU-1003",
			Error: core.I18nMessage{DefaultValue: "not found"},
		}
	}
	*updated = append(*updated, request)
	result := ou.OrganizationUnit{
		ID:              id,
		Handle:          request.Handle,
		Name:            request.Name,
		Description:     request.Description,
		Parent:          request.Parent,
		ThemeID:         request.ThemeID,
		LayoutID:        request.LayoutID,
		LogoURL:         request.LogoURL,
		TosURI:          request.TosURI,
		PolicyURI:       request.PolicyURI,
		CookiePolicyURI: request.CookiePolicyURI,
	}
	existing[id] = result
	return result, nil
}

type fakeApplicationService struct {
	created  []*model.ApplicationDTO
	updated  []*model.ApplicationDTO
	existing map[string]*model.Application
}

func (f *fakeApplicationService) CreateApplication(
	_ context.Context, app *model.ApplicationDTO,
) (*model.ApplicationDTO, *serviceerror.ServiceError) {
	if app.ID == "" {
		app.ID = "generated-app-id"
	}
	f.created = append(f.created, app)
	if f.existing == nil {
		f.existing = map[string]*model.Application{}
	}
	f.existing[app.ID] = &model.Application{ID: app.ID, Name: app.Name}
	return app, nil
}

func (f *fakeApplicationService) ValidateApplication(
	_ context.Context, _ *model.ApplicationDTO,
) (*model.ApplicationProcessedDTO, *inboundmodel.InboundAuthConfigWithSecret, *serviceerror.ServiceError) {
	return nil, nil, nil
}

func (f *fakeApplicationService) GetApplicationList(
	_ context.Context,
) (*model.ApplicationListResponse, *serviceerror.ServiceError) {
	return nil, nil
}

func (f *fakeApplicationService) GetOAuthApplication(
	_ context.Context, _ string,
) (*inboundmodel.OAuthClient, *serviceerror.ServiceError) {
	return nil, nil
}

func (f *fakeApplicationService) GetApplication(
	_ context.Context, appID string,
) (*model.Application, *serviceerror.ServiceError) {
	if app, ok := f.existing[appID]; ok {
		return app, nil
	}
	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  application.ErrorApplicationNotFound.Code,
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeApplicationService) UpdateApplication(
	_ context.Context, appID string, app *model.ApplicationDTO,
) (*model.ApplicationDTO, *serviceerror.ServiceError) {
	if _, ok := f.existing[appID]; !ok {
		return nil, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  application.ErrorApplicationNotFound.Code,
			Error: core.I18nMessage{DefaultValue: "not found"},
		}
	}
	app.ID = appID
	f.updated = append(f.updated, app)
	f.existing[appID] = &model.Application{ID: app.ID, Name: app.Name}
	return app, nil
}

func (f *fakeApplicationService) DeleteApplication(_ context.Context, _ string) *serviceerror.ServiceError {
	return nil
}

type fakeIDPService struct {
	created []*idp.IDPDTO
	updated []*idp.IDPDTO
	byID    map[string]*idp.IDPDTO
	byName  map[string]*idp.IDPDTO
}

func (f *fakeIDPService) CreateIdentityProvider(
	_ context.Context, idpDTO *idp.IDPDTO,
) (*idp.IDPDTO, *serviceerror.ServiceError) {
	if idpDTO.ID == "" {
		idpDTO.ID = "generated-idp-id"
	}
	f.created = append(f.created, idpDTO)
	f.byID[idpDTO.ID] = idpDTO
	f.byName[idpDTO.Name] = idpDTO
	return idpDTO, nil
}

func (f *fakeIDPService) GetIdentityProvider(
	_ context.Context, idpID string,
) (*idp.IDPDTO, *serviceerror.ServiceError) {
	if v, ok := f.byID[idpID]; ok {
		return v, nil
	}
	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "IDP-1001",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeIDPService) GetIdentityProviderByName(
	_ context.Context, name string,
) (*idp.IDPDTO, *serviceerror.ServiceError) {
	if v, ok := f.byName[name]; ok {
		return v, nil
	}
	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "IDP-1001",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeIDPService) UpdateIdentityProvider(
	_ context.Context, idpID string, idpDTO *idp.IDPDTO,
) (*idp.IDPDTO, *serviceerror.ServiceError) {
	if _, ok := f.byID[idpID]; !ok {
		return nil, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "IDP-1001",
			Error: core.I18nMessage{DefaultValue: "not found"},
		}
	}
	idpDTO.ID = idpID
	f.updated = append(f.updated, idpDTO)
	f.byID[idpID] = idpDTO
	f.byName[idpDTO.Name] = idpDTO
	return idpDTO, nil
}

type fakeFlowService struct {
	created []*flowmgt.FlowDefinition
	updated []*flowmgt.FlowDefinition
	byID    map[string]*flowmgt.CompleteFlowDefinition
	byKey   map[string]*flowmgt.CompleteFlowDefinition
}

type fakeThemeService struct {
	created  []thememgt.CreateThemeRequestWithID
	updated  []thememgt.UpdateThemeRequest
	byID     map[string]*thememgt.Theme
	byHandle map[string]*thememgt.Theme
}

func (f *fakeThemeService) CreateTheme(
	theme thememgt.CreateThemeRequestWithID,
) (*thememgt.Theme, *serviceerror.ServiceError) {
	id := theme.ID
	if id == "" {
		id = "generated-theme-id"
	}

	created := &thememgt.Theme{
		ID:          id,
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
	}
	f.created = append(f.created, theme)
	if f.byID == nil {
		f.byID = map[string]*thememgt.Theme{}
	}
	if f.byHandle == nil {
		f.byHandle = map[string]*thememgt.Theme{}
	}
	f.byID[created.ID] = created
	f.byHandle[created.Handle] = created
	return created, nil
}

func (f *fakeThemeService) GetTheme(id string) (*thememgt.Theme, *serviceerror.ServiceError) {
	if existing, ok := f.byID[id]; ok {
		return existing, nil
	}

	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "THM-1003",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeThemeService) UpdateTheme(
	id string, theme thememgt.UpdateThemeRequest,
) (*thememgt.Theme, *serviceerror.ServiceError) {
	if _, ok := f.byID[id]; !ok {
		return nil, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "THM-1003",
			Error: core.I18nMessage{DefaultValue: "not found"},
		}
	}

	updated := &thememgt.Theme{
		ID:          id,
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		Theme:       theme.Theme,
	}
	f.updated = append(f.updated, theme)
	f.byID[id] = updated
	f.byHandle[updated.Handle] = updated
	return updated, nil
}

type fakeEntityTypeService struct {
	created []entitytype.CreateEntityTypeRequestWithID
	updated []entitytype.UpdateEntityTypeRequest
	byID    map[string]*entitytype.EntityType
	byName  map[string]*entitytype.EntityType
}

func (f *fakeEntityTypeService) CreateEntityType(
	_ context.Context, _ entitytype.TypeCategory, request entitytype.CreateEntityTypeRequestWithID,
) (*entitytype.EntityType, *serviceerror.ServiceError) {
	id := request.ID
	if id == "" {
		id = "generated-entity-type-id"
	}
	created := &entitytype.EntityType{
		ID:                    id,
		Name:                  request.Name,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      request.SystemAttributes,
		Schema:                request.Schema,
	}
	f.created = append(f.created, request)
	if f.byID == nil {
		f.byID = map[string]*entitytype.EntityType{}
	}
	if f.byName == nil {
		f.byName = map[string]*entitytype.EntityType{}
	}
	f.byID[created.ID] = created
	f.byName[created.Name] = created
	return created, nil
}

func (f *fakeEntityTypeService) GetEntityType(
	_ context.Context, _ entitytype.TypeCategory, schemaID string, _ bool,
) (*entitytype.EntityType, *serviceerror.ServiceError) {
	if existing, ok := f.byID[schemaID]; ok {
		return existing, nil
	}

	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "USRS-1002",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeEntityTypeService) GetEntityTypeByName(
	_ context.Context, _ entitytype.TypeCategory, schemaName string,
) (*entitytype.EntityType, *serviceerror.ServiceError) {
	if existing, ok := f.byName[schemaName]; ok {
		return existing, nil
	}

	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "USRS-1002",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeEntityTypeService) UpdateEntityType(
	_ context.Context, _ entitytype.TypeCategory, schemaID string, request entitytype.UpdateEntityTypeRequest,
) (*entitytype.EntityType, *serviceerror.ServiceError) {
	if _, ok := f.byID[schemaID]; !ok {
		return nil, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "USRS-1002",
			Error: core.I18nMessage{DefaultValue: "not found"},
		}
	}

	updated := &entitytype.EntityType{
		ID:                    schemaID,
		Name:                  request.Name,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      request.SystemAttributes,
		Schema:                request.Schema,
	}
	f.updated = append(f.updated, request)
	f.byID[schemaID] = updated
	f.byName[updated.Name] = updated
	return updated, nil
}

type fakeOUService struct {
	created  []ou.OrganizationUnitRequestWithID
	updated  []ou.OrganizationUnitRequest
	existing map[string]ou.OrganizationUnit
}

func (f *fakeOUService) CreateOrganizationUnit(
	_ context.Context, request ou.OrganizationUnitRequestWithID,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	id := request.ID
	if id == "" {
		id = "generated-ou-id"
	}

	created := ou.OrganizationUnit{
		ID:              id,
		Handle:          request.Handle,
		Name:            request.Name,
		Description:     request.Description,
		Parent:          request.Parent,
		ThemeID:         request.ThemeID,
		LayoutID:        request.LayoutID,
		LogoURL:         request.LogoURL,
		TosURI:          request.TosURI,
		PolicyURI:       request.PolicyURI,
		CookiePolicyURI: request.CookiePolicyURI,
	}
	f.created = append(f.created, request)
	if f.existing == nil {
		f.existing = map[string]ou.OrganizationUnit{}
	}
	f.existing[created.ID] = created
	return created, nil
}

func (f *fakeOUService) GetOrganizationUnit(
	_ context.Context, id string,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	if existing, ok := f.existing[id]; ok {
		return existing, nil
	}

	return ou.OrganizationUnit{}, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "OU-1003",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeOUService) GetOrganizationUnitByPath(
	_ context.Context, handlePath string,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	for _, existing := range f.existing {
		if existing.Handle == handlePath {
			return existing, nil
		}
	}

	return ou.OrganizationUnit{}, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "OU-1003",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeOUService) UpdateOrganizationUnit(
	_ context.Context, id string, request ou.OrganizationUnitRequestWithID,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	updateReq := ou.OrganizationUnitRequest{
		Handle:          request.Handle,
		Name:            request.Name,
		Description:     request.Description,
		Parent:          request.Parent,
		ThemeID:         request.ThemeID,
		LayoutID:        request.LayoutID,
		LogoURL:         request.LogoURL,
		TosURI:          request.TosURI,
		PolicyURI:       request.PolicyURI,
		CookiePolicyURI: request.CookiePolicyURI,
	}
	return updateOUCommon(f.existing, &f.updated, id, updateReq)
}

type fakeRoleService struct {
	updated []role.RoleUpdateDetail
}

func (f *fakeRoleService) CreateRole(
	_ context.Context, req role.RoleCreationDetail,
) (*role.RoleWithPermissionsAndAssignments, *serviceerror.ServiceError) {
	return &role.RoleWithPermissionsAndAssignments{ID: "role-1", Name: req.Name}, nil
}

func (f *fakeRoleService) GetRoleWithPermissions(
	_ context.Context, id string,
) (*role.RoleWithPermissions, *serviceerror.ServiceError) {
	if id == "role-1" {
		return &role.RoleWithPermissions{ID: id, Name: "role"}, nil
	}

	return nil, &role.ErrorRoleNotFound
}

func (f *fakeRoleService) UpdateRoleWithPermissions(
	_ context.Context, _ string, req role.RoleUpdateDetail,
) (*role.RoleWithPermissions, *serviceerror.ServiceError) {
	f.updated = append(f.updated, req)
	return &role.RoleWithPermissions{ID: "role-1", Name: req.Name}, nil
}

type fakeRoleAssignmentService struct {
	assignments   []role.RoleAssignment
	assignmentErr *serviceerror.ServiceError
}

func (f *fakeRoleAssignmentService) AddAssignments(
	_ context.Context, _ string, assignments []role.RoleAssignment,
) *serviceerror.ServiceError {
	if f.assignmentErr != nil {
		return f.assignmentErr
	}
	f.assignments = append(f.assignments, assignments...)
	return nil
}

type fakeGroupService struct {
	created   []group.CreateGroupRequest
	members   []group.Member
	memberErr *serviceerror.ServiceError
}

func (f *fakeGroupService) CreateGroup(
	_ context.Context, req group.CreateGroupRequest,
) (*group.Group, *serviceerror.ServiceError) {
	id := req.ID
	if id == "" {
		id = "generated-group-id"
	}
	f.created = append(f.created, req)
	return &group.Group{ID: id, Name: req.Name}, nil
}

func (f *fakeGroupService) GetGroup(
	_ context.Context, id string, _ bool,
) (*group.Group, *serviceerror.ServiceError) {
	if id == "group-1" {
		return &group.Group{ID: id, Name: "Admins"}, nil
	}
	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  group.ErrorGroupNotFound.Code,
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeGroupService) UpdateGroup(
	_ context.Context, id string, req group.UpdateGroupRequest,
) (*group.Group, *serviceerror.ServiceError) {
	return &group.Group{ID: id, Name: req.Name}, nil
}

func (f *fakeGroupService) AddGroupMembers(
	_ context.Context, _ string, members []group.Member,
) (*group.Group, *serviceerror.ServiceError) {
	if f.memberErr != nil {
		return nil, f.memberErr
	}
	f.members = append(f.members, members...)
	return &group.Group{}, nil
}

type fakeUserService struct {
	created                     []*user.User
	updateCredentialsShouldFail bool
	deleted                     []string
}

func (f *fakeUserService) CreateUser(
	_ context.Context, u *user.User,
) (*user.User, *serviceerror.ServiceError) {
	id := u.ID
	if id == "" {
		id = "generated-user-id"
	}
	created := *u
	created.ID = id
	f.created = append(f.created, &created)
	return &created, nil
}

func (f *fakeUserService) GetUser(
	_ context.Context, _ string, _ bool,
) (*user.User, *serviceerror.ServiceError) {
	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "USR-1003",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeUserService) UpdateUser(
	_ context.Context, userID string, u *user.User,
) (*user.User, *serviceerror.ServiceError) {
	updated := *u
	updated.ID = userID
	return &updated, nil
}

func (f *fakeUserService) DeleteUser(_ context.Context, userID string) *serviceerror.ServiceError {
	f.deleted = append(f.deleted, userID)
	return nil
}

func (f *fakeUserService) UpdateUserCredentials(
	_ context.Context, _ string, _ json.RawMessage,
) *serviceerror.ServiceError {
	if f.updateCredentialsShouldFail {
		return &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "USR-2001",
			Error: core.I18nMessage{DefaultValue: "bad credentials"},
		}
	}

	return nil
}

func (f *fakeFlowService) CreateFlow(
	_ context.Context, flowDef *flowmgt.FlowDefinition,
) (*flowmgt.CompleteFlowDefinition, *serviceerror.ServiceError) {
	if _, ok := f.byKey[string(flowDef.FlowType)+":"+flowDef.Handle]; ok {
		return nil, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "FLM-1013",
			Error: core.I18nMessage{DefaultValue: "Duplicate flow handle"},
		}
	}

	id := flowDef.ID
	if id == "" {
		id = "generated-flow-id"
	}
	created := &flowmgt.CompleteFlowDefinition{
		ID:       id,
		Handle:   flowDef.Handle,
		Name:     flowDef.Name,
		FlowType: flowDef.FlowType,
		Nodes:    flowDef.Nodes,
	}
	f.created = append(f.created, flowDef)
	f.byID[id] = created
	f.byKey[string(flowDef.FlowType)+":"+flowDef.Handle] = created
	return created, nil
}

func (f *fakeFlowService) GetFlow(
	_ context.Context, flowID string,
) (*flowmgt.CompleteFlowDefinition, *serviceerror.ServiceError) {
	if v, ok := f.byID[flowID]; ok {
		return v, nil
	}
	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "FLM-1003",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeFlowService) GetFlowByHandle(
	_ context.Context, handle string, flowType common.FlowType,
) (*flowmgt.CompleteFlowDefinition, *serviceerror.ServiceError) {
	if v, ok := f.byKey[string(flowType)+":"+handle]; ok {
		return v, nil
	}
	return nil, &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "FLM-1003",
		Error: core.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeFlowService) UpdateFlow(
	_ context.Context, flowID string, flowDef *flowmgt.FlowDefinition,
) (*flowmgt.CompleteFlowDefinition, *serviceerror.ServiceError) {
	if _, ok := f.byID[flowID]; !ok {
		return nil, &serviceerror.ServiceError{
			Type:  serviceerror.ClientErrorType,
			Code:  "FLM-1003",
			Error: core.I18nMessage{DefaultValue: "not found"},
		}
	}
	updated := &flowmgt.CompleteFlowDefinition{
		ID:       flowID,
		Handle:   flowDef.Handle,
		Name:     flowDef.Name,
		FlowType: flowDef.FlowType,
		Nodes:    flowDef.Nodes,
	}
	f.updated = append(f.updated, flowDef)
	f.byID[flowID] = updated
	f.byKey[string(flowDef.FlowType)+":"+flowDef.Handle] = updated
	return updated, nil
}

func newTestImportService(appSvc *fakeApplicationService) ImportServiceInterface {
	if appSvc == nil {
		appSvc = &fakeApplicationService{existing: map[string]*model.Application{}}
	}

	return newImportService(
		appSvc,
		&fakeIDPService{byID: map[string]*idp.IDPDTO{}, byName: map[string]*idp.IDPDTO{}},
		&fakeFlowService{
			byID:  map[string]*flowmgt.CompleteFlowDefinition{},
			byKey: map[string]*flowmgt.CompleteFlowDefinition{},
		},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func runOAuthClientSecretImport(
	t *testing.T,
	content string,
) (*fakeApplicationService, *ImportResponse, *serviceerror.ServiceError) {
	t.Helper()

	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newTestImportService(appSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	return appSvc, resp, err
}

func TestImportResources_CreateApplication(t *testing.T) {
	svc := newTestImportService(nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Summary.Imported)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
}

func TestImportResources_UpdateApplication(t *testing.T) {
	svc := newTestImportService(&fakeApplicationService{
		existing: map[string]*model.Application{
			"app-1": {ID: "app-1", Name: "Existing App"},
		},
	})

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Summary.Imported)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
}

func TestImportResources_DryRunCreateApplicationWithoutWrite(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newTestImportService(appSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, appSvc.created, 0)
	assert.Len(t, appSvc.updated, 0)
}

func TestImportResources_DryRunUpdateApplicationWithoutWrite(t *testing.T) {
	appSvc := &fakeApplicationService{
		existing: map[string]*model.Application{
			"app-1": {ID: "app-1", Name: "Existing App"},
		},
	}
	svc := newTestImportService(appSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Len(t, appSvc.created, 0)
	assert.Len(t, appSvc.updated, 0)
}

func TestImportResources_DryRunValidationFailure(t *testing.T) {
	svc := newTestImportService(nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id:\n- app-1\nname: My App\nauth_flow_id: flow-1\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Summary.Failed)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
}

func TestImportResources_CreateIDPAndFlow(t *testing.T) {
	svc := newTestImportService(nil)

	content := strings.Join([]string{
		"name: idp-one",
		"type: GOOGLE",
		"properties:",
		"- name: client_id",
		"  value: abc",
		"---",
		"handle: login",
		"name: Login Flow",
		"flowType: AUTHENTICATION",
		"nodes: []",
		"",
	}, "\n")
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 2, resp.Summary.Imported)
	assert.Equal(t, resourceTypeIdentityProvider, resp.Results[0].ResourceType)
	assert.Equal(t, resourceTypeFlow, resp.Results[1].ResourceType)
}

func TestImportResources_DefaultsToRuntimeTarget(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newTestImportService(appSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n",
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Summary.Imported)
	assert.Len(t, appSvc.created, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
}

func TestImportResources_PreservesExplicitFalseOptions(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newTestImportService(appSvc)

	falseVal := false
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n" +
			"---\nid: app-2\nname: My App 2\nauth_flow_id: flow-1\n",
		Options: &ImportOptions{
			Upsert:          &falseVal,
			ContinueOnError: &falseVal,
			Target:          importTargetRuntime,
		},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 2, resp.Summary.Imported)
	assert.Equal(t, 0, resp.Summary.Failed)
	assert.Len(t, appSvc.created, 2)
}

func TestImportResources_ApplicationAdapterNotConfigured(t *testing.T) {
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n",
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorAdapterNotConfigured.Code, resp.Results[0].Code)
	assert.Equal(t, "application adapter not configured", resp.Results[0].Message)
}

func TestImportResources_RoleImportIncludesAssignments(t *testing.T) {
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: role-1",
		"name: Admin",
		"ou_id: ou-1",
		"permissions:",
		"  - resource: api",
		"    actions:",
		"      - read",
		"assignments:",
		"  - type: group",
		"    id: g1",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	require.Len(t, roleAssignmentSvc.assignments, 1)
	assert.Equal(t, "g1", roleAssignmentSvc.assignments[0].ID)
	assert.Equal(t, role.AssigneeTypeGroup, roleAssignmentSvc.assignments[0].Type)
}

func TestImportResources_GroupImportIncludesMembers(t *testing.T) {
	groupSvc := &fakeGroupService{}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: group-new",
		"name: Engineers",
		"ou_id: ou-1",
		"members:",
		"  - id: user-1",
		"    type: user",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	require.Len(t, groupSvc.members, 1)
	assert.Equal(t, "user-1", groupSvc.members[0].ID)
	assert.Equal(t, group.MemberTypeUser, groupSvc.members[0].Type)
}

func TestImportResources_RoleImportNoAssignments(t *testing.T) {
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: role-new",
		"name: Viewer",
		"ou_id: ou-1",
		"permissions: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Empty(t, roleAssignmentSvc.assignments)
}

func TestImportResources_RoleUpsertUpdateIncludesAssignments(t *testing.T) {
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: role-1",
		"name: Admin",
		"ou_id: ou-1",
		"permissions: []",
		"assignments:",
		"  - type: group",
		"    id: g-99",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	require.Len(t, roleAssignmentSvc.assignments, 1)
	assert.Equal(t, "g-99", roleAssignmentSvc.assignments[0].ID)
}

func TestImportResources_RoleAssignmentFailureReturnsError(t *testing.T) {
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{assignmentErr: &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "ROLE-4001",
		Error: core.I18nMessage{DefaultValue: "invalid assignee"},
	}}
	svc := newImportService(nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc, nil, nil, nil, nil, nil, nil)

	// role-1 exists in the fake → update path → AddAssignments is called separately → fails
	content := strings.Join([]string{
		"id: role-1",
		"name: Admin",
		"ou_id: ou-1",
		"permissions: []",
		"assignments:",
		"  - type: group",
		"    id: g1",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
}

func TestImportResources_GroupImportNoMembers(t *testing.T) {
	groupSvc := &fakeGroupService{}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: group-new",
		"name: Empty",
		"ou_id: ou-1",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Empty(t, groupSvc.members)
}

func TestImportResources_GroupUpsertUpdateIncludesMembers(t *testing.T) {
	groupSvc := &fakeGroupService{}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: group-1",
		"name: Admins",
		"ou_id: ou-1",
		"members:",
		"  - id: u-99",
		"    type: user",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	require.Len(t, groupSvc.members, 1)
	assert.Equal(t, "u-99", groupSvc.members[0].ID)
	assert.Equal(t, group.MemberTypeUser, groupSvc.members[0].Type)
}

func TestImportResources_GroupMemberFailureReturnsError(t *testing.T) {
	groupSvc := &fakeGroupService{memberErr: &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "GRP-4001",
		Error: core.I18nMessage{DefaultValue: "invalid member"},
	}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: group-new",
		"name: Engineers",
		"ou_id: ou-1",
		"members:",
		"  - id: u1",
		"    type: user",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
}

func TestImportResources_UserCredentialFailureRollsBackCreate(t *testing.T) {
	userSvc := &fakeUserService{updateCredentialsShouldFail: true}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, userSvc, nil)

	content := strings.Join([]string{
		"id: user-1",
		"type: customer",
		"ou_id: ou-1",
		"attributes:",
		"  username: alice",
		"credentials:",
		"  password: secret",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Contains(t, resp.Results[0].Message, "user profile updated but credential update failed")
	assert.Empty(t, userSvc.deleted)
}

func TestImportResources_OrganizationUnitUpsertCreatePreservesID(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: ou-123",
		"handle: eng",
		"name: Engineering",
		"description: Engineering OU",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "ou-123", resp.Results[0].ResourceID)
	assert.Len(t, ouSvc.created, 1)
	assert.Equal(t, "ou-123", ouSvc.created[0].ID)
}

func TestImportResources_FlowUpsertCreatePreservesID(t *testing.T) {
	flowSvc := &fakeFlowService{
		byID:  map[string]*flowmgt.CompleteFlowDefinition{},
		byKey: map[string]*flowmgt.CompleteFlowDefinition{},
	}

	svc := newImportService(nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: missing-flow-id",
		"handle: registration-flow",
		"name: Updated Registration Flow",
		"flowType: REGISTRATION",
		"nodes: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "missing-flow-id", resp.Results[0].ResourceID)
	assert.Len(t, flowSvc.created, 1)
	assert.Equal(t, "missing-flow-id", flowSvc.created[0].ID)
	assert.Len(t, flowSvc.updated, 0)
}

func TestImportResources_FlowUpsertDuplicateHandleFallsBackToHandleUpdate(t *testing.T) {
	flowSvc := &fakeFlowService{
		byID: map[string]*flowmgt.CompleteFlowDefinition{
			"existing-flow-id": {
				ID:       "existing-flow-id",
				Handle:   "registration-flow",
				Name:     "Existing Registration Flow",
				FlowType: common.FlowTypeRegistration,
			},
		},
		byKey: map[string]*flowmgt.CompleteFlowDefinition{},
	}
	flowSvc.byKey[string(common.FlowTypeRegistration)+":registration-flow"] = flowSvc.byID["existing-flow-id"]

	svc := newImportService(nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: missing-flow-id",
		"handle: registration-flow",
		"name: Updated Registration Flow",
		"flowType: REGISTRATION",
		"nodes: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Equal(t, "existing-flow-id", resp.Results[0].ResourceID)
	assert.Len(t, flowSvc.created, 0)
	assert.Len(t, flowSvc.updated, 1)
	assert.Equal(t, "registration-flow", flowSvc.updated[0].Handle)
}

func TestImportResources_ApplicationFlowReferencesAreRemappedFromFlowAlias(t *testing.T) {
	flowSvc := &fakeFlowService{
		byID: map[string]*flowmgt.CompleteFlowDefinition{
			"existing-registration-flow-id": {
				ID:       "existing-registration-flow-id",
				Handle:   "registration-flow",
				Name:     "Existing Registration Flow",
				FlowType: common.FlowTypeRegistration,
			},
		},
		byKey: map[string]*flowmgt.CompleteFlowDefinition{},
	}
	flowSvc.byKey[string(common.FlowTypeRegistration)+":registration-flow"] =
		flowSvc.byID["existing-registration-flow-id"]

	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newImportService(appSvc, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"# resource_type: flow",
		"id: missing-registration-flow-id",
		"handle: registration-flow",
		"name: Updated Registration Flow",
		"flowType: REGISTRATION",
		"nodes: []",
		"",
		"---",
		"# resource_type: application",
		"name: My App",
		"auth_flow_id: auth-flow-1",
		"registration_flow_id: missing-registration-flow-id",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Equal(t, "existing-registration-flow-id", resp.Results[0].ResourceID)
	importedApps := append([]*model.ApplicationDTO{}, appSvc.updated...)
	importedApps = append(importedApps, appSvc.created...)
	require.Len(t, importedApps, 1)
	assert.Equal(t, "existing-registration-flow-id", importedApps[0].RegistrationFlowID)
}

//nolint:dupl // Test pattern repeated across resource types to verify ID preservation behavior
func TestImportResources_ThemeUpsertCreatePreservesID(t *testing.T) {
	themeSvc := &fakeThemeService{byID: map[string]*thememgt.Theme{}, byHandle: map[string]*thememgt.Theme{}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, themeSvc, nil, nil, nil)

	content := strings.Join([]string{
		"id: thm-123",
		"handle: default-theme",
		"displayName: Default Theme",
		"theme:",
		"  colorSchemes:",
		"    light: {}",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "thm-123", resp.Results[0].ResourceID)
	assert.Len(t, themeSvc.created, 1)
	assert.Equal(t, "thm-123", themeSvc.created[0].ID)
}

//nolint:dupl // Test pattern repeated across resource types to verify ID preservation behavior
func TestImportResources_EntityTypeUpsertCreatePreservesID(t *testing.T) {
	entityTypeSvc := &fakeEntityTypeService{
		byID:   map[string]*entitytype.EntityType{},
		byName: map[string]*entitytype.EntityType{},
	}
	svc := newImportService(nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: usrs-123",
		"name: customer",
		"organization_unit_id: ou-1",
		"schema:",
		"  type: object",
		"  properties: {}",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "usrs-123", resp.Results[0].ResourceID)
	assert.Len(t, entityTypeSvc.created, 1)
	assert.Equal(t, "usrs-123", entityTypeSvc.created[0].ID)
}

func TestImportResources_UpsertCreatePreservesIDsAcrossResourceTypes(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	themeSvc := &fakeThemeService{byID: map[string]*thememgt.Theme{}, byHandle: map[string]*thememgt.Theme{}}
	entityTypeSvc := &fakeEntityTypeService{
		byID:   map[string]*entitytype.EntityType{},
		byName: map[string]*entitytype.EntityType{},
	}
	flowSvc := &fakeFlowService{
		byID:  map[string]*flowmgt.CompleteFlowDefinition{},
		byKey: map[string]*flowmgt.CompleteFlowDefinition{},
	}

	svc := newImportService(nil, nil, flowSvc, ouSvc, entityTypeSvc, nil, nil, nil, nil, themeSvc, nil, nil, nil)

	content := strings.Join([]string{
		"id: ou-123",
		"handle: eng",
		"name: Engineering",
		"description: Engineering OU",
		"",
		"---",
		"id: thm-123",
		"handle: default-theme",
		"displayName: Default Theme",
		"theme:",
		"  colorSchemes:",
		"    light: {}",
		"",
		"---",
		"id: usrs-123",
		"name: customer",
		"organization_unit_id: ou-123",
		"schema:",
		"  type: object",
		"  properties: {}",
		"",
		"---",
		"id: flow-123",
		"handle: registration-flow",
		"name: Registration Flow",
		"flowType: REGISTRATION",
		"nodes: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Summary)
	assert.Equal(t, 4, resp.Summary.Imported)
	assert.Equal(t, 0, resp.Summary.Failed)
	require.Len(t, resp.Results, 4)

	assert.Equal(t, "ou-123", resp.Results[0].ResourceID)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)

	assert.Equal(t, "usrs-123", resp.Results[1].ResourceID)
	assert.Equal(t, statusSuccess, resp.Results[1].Status)
	assert.Equal(t, operationCreate, resp.Results[1].Operation)

	assert.Equal(t, "flow-123", resp.Results[2].ResourceID)
	assert.Equal(t, statusSuccess, resp.Results[2].Status)
	assert.Equal(t, operationCreate, resp.Results[2].Operation)

	assert.Equal(t, "thm-123", resp.Results[3].ResourceID)
	assert.Equal(t, statusSuccess, resp.Results[3].Status)
	assert.Equal(t, operationCreate, resp.Results[3].Operation)

	require.Len(t, ouSvc.created, 1)
	assert.Equal(t, "ou-123", ouSvc.created[0].ID)

	require.Len(t, entityTypeSvc.created, 1)
	assert.Equal(t, "usrs-123", entityTypeSvc.created[0].ID)

	require.Len(t, flowSvc.created, 1)
	assert.Equal(t, "flow-123", flowSvc.created[0].ID)

	require.Len(t, themeSvc.created, 1)
	assert.Equal(t, "thm-123", themeSvc.created[0].ID)
}

func TestImportResources_EntityTypeOUHandlePassedToService(t *testing.T) {
	entityTypeSvc := &fakeEntityTypeService{
		byID:   map[string]*entitytype.EntityType{},
		byName: map[string]*entitytype.EntityType{},
	}
	svc := newImportService(nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"id: usrs-123",
		"name: customer",
		"ou_handle: default",
		"schema:",
		"  type: object",
		"  properties: {}",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, entityTypeSvc.created, 1)
	assert.Equal(t, "default", entityTypeSvc.created[0].OUHandle)
	assert.Equal(t, "", entityTypeSvc.created[0].OUID)
}

func TestImportResources_StripsClientSecretForPublicClientWithNoneAuthMethod(t *testing.T) {
	content := strings.Join([]string{
		"id: app-1",
		"name: My App",
		"auth_flow_id: flow-1",
		"inbound_auth_config:",
		"  - type: oauth2",
		"    config:",
		"      client_id: app-client",
		"      client_secret: should-be-removed",
		"      redirect_uris:",
		"        - https://localhost:3000/callback",
		"      grant_types:",
		"        - authorization_code",
		"      response_types:",
		"        - code",
		"      token_endpoint_auth_method: none",
		"      pkce_required: true",
		"      public_client: true",
		"",
	}, "\n")
	appSvc, resp, err := runOAuthClientSecretImport(t, content)

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, appSvc.created, 1)
	require.Len(t, appSvc.created[0].InboundAuthConfig, 1)
	require.NotNil(t, appSvc.created[0].InboundAuthConfig[0].OAuthConfig)
	assert.Equal(t, "", appSvc.created[0].InboundAuthConfig[0].OAuthConfig.ClientSecret)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
}

func TestImportResources_KeepsClientSecretForConfidentialClient(t *testing.T) {
	content := strings.Join([]string{
		"id: app-1",
		"name: My App",
		"auth_flow_id: flow-1",
		"inbound_auth_config:",
		"  - type: oauth2",
		"    config:",
		"      client_id: app-client",
		"      client_secret: keep-me",
		"      redirect_uris:",
		"        - https://localhost:3000/callback",
		"      grant_types:",
		"        - authorization_code",
		"      response_types:",
		"        - code",
		"      token_endpoint_auth_method: client_secret_basic",
		"      public_client: false",
		"",
	}, "\n")
	appSvc, resp, err := runOAuthClientSecretImport(t, content)

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, appSvc.created, 1)
	require.Len(t, appSvc.created[0].InboundAuthConfig, 1)
	require.NotNil(t, appSvc.created[0].InboundAuthConfig[0].OAuthConfig)
	assert.Equal(t, "keep-me", appSvc.created[0].InboundAuthConfig[0].OAuthConfig.ClientSecret)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
}

func TestOrderDocumentsByDependencies(t *testing.T) {
	docs := []parsedDocument{
		{ResourceType: resourceTypeApplication, Sequence: 2},
		{ResourceType: resourceTypeFlow, Sequence: 1},
		{ResourceType: resourceTypeIdentityProvider, Sequence: 0},
	}

	ordered := orderDocumentsByDependencies(docs)
	require.Len(t, ordered, 3)
	assert.Equal(t, resourceTypeIdentityProvider, ordered[0].ResourceType)
	assert.Equal(t, resourceTypeFlow, ordered[1].ResourceType)
	assert.Equal(t, resourceTypeApplication, ordered[2].ResourceType)
}

func TestImportResources_FileTargetReturnsError(t *testing.T) {
	tempHome := t.TempDir()

	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime(tempHome, &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))

	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "id: app-1\nname: My App\nauth_flow_id: flow-1\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetFile},
	})

	require.Nil(t, resp)
	require.NotNil(t, err)
	assert.Equal(t, ErrorInvalidImportRequest.Code, err.Code)
	assert.Contains(t, err.ErrorDescription.DefaultValue, "file target is not supported")
}

func TestDeleteResource_RemovesDeclarativeFile(t *testing.T) {
	tempHome := t.TempDir()

	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime(tempHome, &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))

	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resourceDir := filepath.Join(tempHome, "repository", "resources", "applications")
	require.NoError(t, os.MkdirAll(resourceDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(resourceDir, "app-1.yaml"),
		[]byte("# resource_type: application\nid: app-1\nname: My App\nauth_flow_id: flow-1\n"),
		0o600,
	))

	deleteResp, deleteErr := svc.DeleteResource(context.Background(), &DeleteResourceRequest{
		ResourceType: resourceTypeApplication,
		ResourceKey:  "app-1",
	})

	require.Nil(t, deleteErr)
	require.NotNil(t, deleteResp)
	assert.Equal(t, resourceTypeApplication, deleteResp.ResourceType)
	assert.Equal(t, "app-1", deleteResp.ResourceKey)

	_, statErr := os.Stat(filepath.Join(tempHome, "repository", "resources", "applications", "app-1.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestImportResources_ApplicationOUHandlePassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"# resource_type: application",
			"name: My App",
			"ou_handle: default",
			"",
		}, "\n"),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, appSvc.created, 1)
	assert.Equal(t, "default", appSvc.created[0].OUHandle)
}

func TestImportResources_ApplicationAuthFlowHandlePassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"# resource_type: application",
			"name: My App",
			"auth_flow_handle: login-flow",
			"",
		}, "\n"),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, appSvc.created, 1)
	assert.Equal(t, "login-flow", appSvc.created[0].AuthFlowHandle)
}

func TestImportResources_ApplicationRegistrationFlowHandlePassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"# resource_type: application",
			"name: My App",
			"registration_flow_handle: reg-flow",
			"is_registration_flow_enabled: true",
			"",
		}, "\n"),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, appSvc.created, 1)
	assert.Equal(t, "reg-flow", appSvc.created[0].RegistrationFlowHandle)
	assert.True(t, appSvc.created[0].IsRegistrationFlowEnabled)
}

func TestImportResources_ApplicationRecoveryFlowHandlePassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"# resource_type: application",
			"name: My App",
			"recovery_flow_handle: recovery-flow",
			"is_recovery_flow_enabled: true",
			"",
		}, "\n"),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, appSvc.created, 1)
	assert.Equal(t, "recovery-flow", appSvc.created[0].RecoveryFlowHandle)
	assert.True(t, appSvc.created[0].IsRecoveryFlowEnabled)
}

func TestImportResources_DryRunSkipsApplicationHandleResolution(t *testing.T) {
	// With dry-run, handle resolution is skipped — unknown handles must not cause failure.
	appSvc := &fakeApplicationService{existing: map[string]*model.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"# resource_type: application",
			"name: My App",
			"ou_handle: nonexistent-ou",
			"auth_flow_handle: nonexistent-flow",
			"",
		}, "\n"),
		DryRun: true,
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Len(t, appSvc.created, 0)
}
