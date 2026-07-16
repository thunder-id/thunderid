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

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/agent"
	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool {
	return &v
}

func updateOUCommon(existing map[string]providers.OrganizationUnit, updated *[]ou.OrganizationUnitRequest,
	id string, request ou.OrganizationUnitRequest) (providers.OrganizationUnit, *tidcommon.ServiceError) {
	if _, ok := existing[id]; !ok {
		return providers.OrganizationUnit{}, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "OU-1003",
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
		}
	}
	*updated = append(*updated, request)
	result := providers.OrganizationUnit{
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
	existing map[string]*providers.Application
}

func (f *fakeApplicationService) CreateApplication(
	_ context.Context, app *model.ApplicationDTO,
) (*model.ApplicationDTO, *tidcommon.ServiceError) {
	if app.ID == "" {
		app.ID = "generated-app-id"
	}
	f.created = append(f.created, app)
	if f.existing == nil {
		f.existing = map[string]*providers.Application{}
	}
	f.existing[app.ID] = &providers.Application{ID: app.ID, Name: app.Name}
	return app, nil
}

func (f *fakeApplicationService) ValidateApplication(
	_ context.Context, _ *model.ApplicationDTO,
) (*model.ApplicationProcessedDTO, *providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError) {
	return nil, nil, nil
}

func (f *fakeApplicationService) GetApplicationList(
	_ context.Context,
) (*model.ApplicationListResponse, *tidcommon.ServiceError) {
	return nil, nil
}

func (f *fakeApplicationService) GetOAuthApplication(
	_ context.Context, _ string,
) (*providers.OAuthClient, *tidcommon.ServiceError) {
	return nil, nil
}

func (f *fakeApplicationService) GetApplication(
	_ context.Context, appID string,
) (*providers.Application, *tidcommon.ServiceError) {
	if app, ok := f.existing[appID]; ok {
		return app, nil
	}
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  application.ErrorApplicationNotFound.Code,
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeApplicationService) UpdateApplication(
	_ context.Context, appID string, app *model.ApplicationDTO,
) (*model.ApplicationDTO, *tidcommon.ServiceError) {
	if _, ok := f.existing[appID]; !ok {
		return nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  application.ErrorApplicationNotFound.Code,
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
		}
	}
	app.ID = appID
	f.updated = append(f.updated, app)
	f.existing[appID] = &providers.Application{ID: app.ID, Name: app.Name}
	return app, nil
}

func (f *fakeApplicationService) DeleteApplication(_ context.Context, _ string) *tidcommon.ServiceError {
	return nil
}

type fakeIDPService struct {
	created []*providers.IDPDTO
	updated []*providers.IDPDTO
	byID    map[string]*providers.IDPDTO
	byName  map[string]*providers.IDPDTO
}

func (f *fakeIDPService) CreateIdentityProvider(
	_ context.Context, idpDTO *providers.IDPDTO,
) (*providers.IDPDTO, *tidcommon.ServiceError) {
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
) (*providers.IDPDTO, *tidcommon.ServiceError) {
	if v, ok := f.byID[idpID]; ok {
		return v, nil
	}
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IDP-1001",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeIDPService) GetIdentityProviderByName(
	_ context.Context, name string,
) (*providers.IDPDTO, *tidcommon.ServiceError) {
	if v, ok := f.byName[name]; ok {
		return v, nil
	}
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IDP-1001",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeIDPService) UpdateIdentityProvider(
	_ context.Context, idpID string, idpDTO *providers.IDPDTO,
) (*providers.IDPDTO, *tidcommon.ServiceError) {
	if _, ok := f.byID[idpID]; !ok {
		return nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "IDP-1001",
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
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
	byID    map[string]*providers.CompleteFlowDefinition
	byKey   map[string]*providers.CompleteFlowDefinition
}

type fakeThemeService struct {
	created  []thememgt.CreateThemeRequestWithID
	updated  []thememgt.UpdateThemeRequest
	byID     map[string]*thememgt.Theme
	byHandle map[string]*thememgt.Theme
}

func (f *fakeThemeService) CreateTheme(_ context.Context,
	theme thememgt.CreateThemeRequestWithID,
) (*thememgt.Theme, *tidcommon.ServiceError) {
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

func (f *fakeThemeService) GetTheme(_ context.Context, id string) (*thememgt.Theme, *tidcommon.ServiceError) {
	if existing, ok := f.byID[id]; ok {
		return existing, nil
	}

	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "THM-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeThemeService) UpdateTheme(_ context.Context,
	id string, theme thememgt.UpdateThemeRequest,
) (*thememgt.Theme, *tidcommon.ServiceError) {
	if _, ok := f.byID[id]; !ok {
		return nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "THM-1003",
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
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
) (*entitytype.EntityType, *tidcommon.ServiceError) {
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
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	if existing, ok := f.byID[schemaID]; ok {
		return existing, nil
	}

	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "USRS-1002",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeEntityTypeService) GetEntityTypeByName(
	_ context.Context, _ entitytype.TypeCategory, schemaName string,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	if existing, ok := f.byName[schemaName]; ok {
		return existing, nil
	}

	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "USRS-1002",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeEntityTypeService) UpdateEntityType(
	_ context.Context, _ entitytype.TypeCategory, schemaID string, request entitytype.UpdateEntityTypeRequest,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	if _, ok := f.byID[schemaID]; !ok {
		return nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "USRS-1002",
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
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
	created  []providers.OrganizationUnitRequestWithID
	updated  []ou.OrganizationUnitRequest
	existing map[string]providers.OrganizationUnit
}

func (f *fakeOUService) CreateOrganizationUnit(
	_ context.Context, request providers.OrganizationUnitRequestWithID,
) (providers.OrganizationUnit, *tidcommon.ServiceError) {
	id := request.ID
	if id == "" {
		id = "generated-ou-id"
	}

	created := providers.OrganizationUnit{
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
		f.existing = map[string]providers.OrganizationUnit{}
	}
	f.existing[created.ID] = created
	return created, nil
}

func (f *fakeOUService) GetOrganizationUnit(
	_ context.Context, id string,
) (providers.OrganizationUnit, *tidcommon.ServiceError) {
	if existing, ok := f.existing[id]; ok {
		return existing, nil
	}

	return providers.OrganizationUnit{}, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OU-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeOUService) GetOrganizationUnitByPath(
	_ context.Context, handlePath string,
) (providers.OrganizationUnit, *tidcommon.ServiceError) {
	for _, existing := range f.existing {
		if existing.Handle == handlePath {
			return existing, nil
		}
	}

	return providers.OrganizationUnit{}, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OU-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeOUService) UpdateOrganizationUnit(
	_ context.Context, id string, request providers.OrganizationUnitRequestWithID,
) (providers.OrganizationUnit, *tidcommon.ServiceError) {
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
	created []role.RoleCreationDetail
	updated []role.RoleUpdateDetail
}

func (f *fakeRoleService) CreateRole(
	_ context.Context, req role.RoleCreationDetail,
) (*role.RoleWithPermissionsAndAssignments, *tidcommon.ServiceError) {
	f.created = append(f.created, req)
	return &role.RoleWithPermissionsAndAssignments{ID: "role-1", Name: req.Name}, nil
}

func (f *fakeRoleService) GetRoleWithPermissions(
	_ context.Context, id string,
) (*role.RoleWithPermissions, *tidcommon.ServiceError) {
	if id == "role-1" {
		return &role.RoleWithPermissions{ID: id, Name: "role"}, nil
	}

	return nil, &role.ErrorRoleNotFound
}

func (f *fakeRoleService) UpdateRoleWithPermissions(
	_ context.Context, _ string, req role.RoleUpdateDetail,
) (*role.RoleWithPermissions, *tidcommon.ServiceError) {
	f.updated = append(f.updated, req)
	return &role.RoleWithPermissions{ID: "role-1", Name: req.Name}, nil
}

type fakeRoleAssignmentService struct {
	assignments   []role.RoleAssignment
	assignmentErr *tidcommon.ServiceError
}

func (f *fakeRoleAssignmentService) AddAssignments(
	_ context.Context, _ string, assignments []role.RoleAssignment,
) *tidcommon.ServiceError {
	if f.assignmentErr != nil {
		return f.assignmentErr
	}
	f.assignments = append(f.assignments, assignments...)
	return nil
}

type fakeGroupService struct {
	created   []group.CreateGroupRequest
	members   []group.Member
	memberErr *tidcommon.ServiceError
}

func (f *fakeGroupService) CreateGroup(
	_ context.Context, req group.CreateGroupRequest,
) (*group.Group, *tidcommon.ServiceError) {
	id := req.ID
	if id == "" {
		id = "generated-group-id"
	}
	f.created = append(f.created, req)
	return &group.Group{ID: id, Name: req.Name}, nil
}

func (f *fakeGroupService) GetGroup(
	_ context.Context, id string, _ bool,
) (*group.Group, *tidcommon.ServiceError) {
	if id == "group-1" {
		return &group.Group{ID: id, Name: "Admins"}, nil
	}
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  group.ErrorGroupNotFound.Code,
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeGroupService) UpdateGroup(
	_ context.Context, id string, req group.UpdateGroupRequest,
) (*group.Group, *tidcommon.ServiceError) {
	return &group.Group{ID: id, Name: req.Name}, nil
}

func (f *fakeGroupService) AddGroupMembers(
	_ context.Context, _ string, members []group.Member,
) (*group.Group, *tidcommon.ServiceError) {
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
) (*user.User, *tidcommon.ServiceError) {
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
) (*user.User, *tidcommon.ServiceError) {
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "USR-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeUserService) UpdateUser(
	_ context.Context, userID string, u *user.User,
) (*user.User, *tidcommon.ServiceError) {
	updated := *u
	updated.ID = userID
	return &updated, nil
}

func (f *fakeUserService) DeleteUser(_ context.Context, userID string) *tidcommon.ServiceError {
	f.deleted = append(f.deleted, userID)
	return nil
}

func (f *fakeUserService) UpdateUserCredentials(
	_ context.Context, _ string, _ json.RawMessage,
) *tidcommon.ServiceError {
	if f.updateCredentialsShouldFail {
		return &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "USR-2001",
			Error: tidcommon.I18nMessage{DefaultValue: "bad credentials"},
		}
	}

	return nil
}

func (f *fakeFlowService) CreateFlow(
	_ context.Context, flowDef *flowmgt.FlowDefinition,
) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if _, ok := f.byKey[string(flowDef.FlowType)+":"+flowDef.Handle]; ok {
		return nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "FLM-1013",
			Error: tidcommon.I18nMessage{DefaultValue: "Duplicate flow handle"},
		}
	}

	id := flowDef.ID
	if id == "" {
		id = "generated-flow-id"
	}
	created := &providers.CompleteFlowDefinition{
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
) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if v, ok := f.byID[flowID]; ok {
		return v, nil
	}
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "FLM-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeFlowService) GetFlowByHandle(
	_ context.Context, handle string, flowType providers.FlowType,
) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if v, ok := f.byKey[string(flowType)+":"+handle]; ok {
		return v, nil
	}
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "FLM-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeFlowService) UpdateFlow(
	_ context.Context, flowID string, flowDef *flowmgt.FlowDefinition,
) (*providers.CompleteFlowDefinition, *tidcommon.ServiceError) {
	if _, ok := f.byID[flowID]; !ok {
		return nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "FLM-1003",
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
		}
	}
	updated := &providers.CompleteFlowDefinition{
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
		appSvc = &fakeApplicationService{existing: map[string]*providers.Application{}}
	}

	return newImportService(
		appSvc,
		&fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}},
		&fakeFlowService{
			byID:  map[string]*providers.CompleteFlowDefinition{},
			byKey: map[string]*providers.CompleteFlowDefinition{},
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func runOAuthClientSecretImport(
	t *testing.T,
	content string,
) (*fakeApplicationService, *ImportResponse, *tidcommon.ServiceError) {
	t.Helper()

	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
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
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n",
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
		existing: map[string]*providers.Application{
			"app-1": {ID: "app-1", Name: "Existing App"},
		},
	})

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Summary.Imported)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
}

func TestImportResources_DryRunCreateApplicationWithoutWrite(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newTestImportService(appSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n",
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
		existing: map[string]*providers.Application{
			"app-1": {ID: "app-1", Name: "Existing App"},
		},
	}
	svc := newTestImportService(appSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n",
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
		Content: "resource_type: application\nid:\n- app-1\nname: My App\nauthFlowId: flow-1\n",
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
		"resource_type: identity_provider",
		"name: idp-one",
		"type: GOOGLE",
		"properties:",
		"- name: client_id",
		"  value: abc",
		"---",
		"resource_type: flow",
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
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newTestImportService(appSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n",
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 1, resp.Summary.Imported)
	assert.Len(t, appSvc.created, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
}

func TestImportResources_PreservesExplicitFalseOptions(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newTestImportService(appSvc)

	falseVal := false
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n" +
			"---\nresource_type: application\nid: app-2\nname: My App 2\nauthFlowId: flow-1\n",
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
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n",
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
	svc := newImportService(
		nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: role",
		"id: role-1",
		"name: Admin",
		"ouId: ou-1",
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
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: group",
		"id: group-new",
		"name: Engineers",
		"ouId: ou-1",
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
	svc := newImportService(
		nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: role",
		"id: role-new",
		"name: Viewer",
		"ouId: ou-1",
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
	svc := newImportService(
		nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: role",
		"id: role-1",
		"name: Admin",
		"ouId: ou-1",
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
	roleAssignmentSvc := &fakeRoleAssignmentService{assignmentErr: &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "ROLE-4001",
		Error: tidcommon.I18nMessage{DefaultValue: "invalid assignee"},
	}}
	svc := newImportService(
		nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	// role-1 exists in the fake → update path → AddAssignments is called separately → fails
	content := strings.Join([]string{
		"resource_type: role",
		"id: role-1",
		"name: Admin",
		"ouId: ou-1",
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
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: group",
		"id: group-new",
		"name: Empty",
		"ouId: ou-1",
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
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: group",
		"id: group-1",
		"name: Admins",
		"ouId: ou-1",
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
	groupSvc := &fakeGroupService{memberErr: &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "GRP-4001",
		Error: tidcommon.I18nMessage{DefaultValue: "invalid member"},
	}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: group",
		"id: group-new",
		"name: Engineers",
		"ouId: ou-1",
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
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: user",
		"id: user-1",
		"type: customer",
		"ouId: ou-1",
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
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: organization_unit",
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
		byID:  map[string]*providers.CompleteFlowDefinition{},
		byKey: map[string]*providers.CompleteFlowDefinition{},
	}

	svc := newImportService(nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: flow",
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
		byID: map[string]*providers.CompleteFlowDefinition{
			"existing-flow-id": {
				ID:       "existing-flow-id",
				Handle:   "registration-flow",
				Name:     "Existing Registration Flow",
				FlowType: providers.FlowTypeRegistration,
			},
		},
		byKey: map[string]*providers.CompleteFlowDefinition{},
	}
	flowSvc.byKey[string(providers.FlowTypeRegistration)+":registration-flow"] = flowSvc.byID["existing-flow-id"]

	svc := newImportService(nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: flow",
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
		byID: map[string]*providers.CompleteFlowDefinition{
			"existing-registration-flow-id": {
				ID:       "existing-registration-flow-id",
				Handle:   "registration-flow",
				Name:     "Existing Registration Flow",
				FlowType: providers.FlowTypeRegistration,
			},
		},
		byKey: map[string]*providers.CompleteFlowDefinition{},
	}
	flowSvc.byKey[string(providers.FlowTypeRegistration)+":registration-flow"] =
		flowSvc.byID["existing-registration-flow-id"]

	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(appSvc, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: flow",
		"id: missing-registration-flow-id",
		"handle: registration-flow",
		"name: Updated Registration Flow",
		"flowType: REGISTRATION",
		"nodes: []",
		"",
		"---",
		"resource_type: application",
		"name: My App",
		"authFlowId: auth-flow-1",
		"registrationFlowId: missing-registration-flow-id",
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
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, themeSvc, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: theme",
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
	svc := newImportService(
		nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: user_type",
		"id: usrs-123",
		"name: customer",
		"ouId: ou-1",
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
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	themeSvc := &fakeThemeService{byID: map[string]*thememgt.Theme{}, byHandle: map[string]*thememgt.Theme{}}
	entityTypeSvc := &fakeEntityTypeService{
		byID:   map[string]*entitytype.EntityType{},
		byName: map[string]*entitytype.EntityType{},
	}
	flowSvc := &fakeFlowService{
		byID:  map[string]*providers.CompleteFlowDefinition{},
		byKey: map[string]*providers.CompleteFlowDefinition{},
	}

	svc := newImportService(
		nil, nil, flowSvc, ouSvc, entityTypeSvc,
		nil, nil, nil, nil, themeSvc, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: organization_unit",
		"id: ou-123",
		"handle: eng",
		"name: Engineering",
		"description: Engineering OU",
		"",
		"---",
		"resource_type: theme",
		"id: thm-123",
		"handle: default-theme",
		"displayName: Default Theme",
		"theme:",
		"  colorSchemes:",
		"    light: {}",
		"",
		"---",
		"resource_type: user_type",
		"id: usrs-123",
		"name: customer",
		"ouId: ou-123",
		"schema:",
		"  type: object",
		"  properties: {}",
		"",
		"---",
		"resource_type: flow",
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
	svc := newImportService(
		nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: user_type",
		"id: usrs-123",
		"name: customer",
		"ouHandle: default",
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
		"resource_type: application",
		"id: app-1",
		"name: My App",
		"authFlowId: flow-1",
		"inboundAuthConfig:",
		"  - type: oauth2",
		"    config:",
		"      clientId: app-client",
		"      clientSecret: should-be-removed",
		"      redirectUris:",
		"        - https://localhost:3000/callback",
		"      grantTypes:",
		"        - authorization_code",
		"      responseTypes:",
		"        - code",
		"      tokenEndpointAuthMethod: none",
		"      pkceRequired: true",
		"      publicClient: true",
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
		"resource_type: application",
		"id: app-1",
		"name: My App",
		"authFlowId: flow-1",
		"inboundAuthConfig:",
		"  - type: oauth2",
		"    config:",
		"      clientId: app-client",
		"      clientSecret: keep-me",
		"      redirectUris:",
		"        - https://localhost:3000/callback",
		"      grantTypes:",
		"        - authorization_code",
		"      responseTypes:",
		"        - code",
		"      tokenEndpointAuthMethod: client_secret_basic",
		"      publicClient: false",
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

	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n",
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

	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resourceDir := filepath.Join(tempHome, "config", "resources", "applications")
	require.NoError(t, os.MkdirAll(resourceDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(resourceDir, "app-1.yaml"),
		[]byte("resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n"),
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

	_, statErr := os.Stat(filepath.Join(tempHome, "config", "resources", "applications", "app-1.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestImportResources_ApplicationOUHandlePassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"resource_type: application",
			"name: My App",
			"ouHandle: default",
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
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"resource_type: application",
			"name: My App",
			"authFlowHandle: login-flow",
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
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"resource_type: application",
			"name: My App",
			"registrationFlowHandle: reg-flow",
			"isRegistrationFlowEnabled: true",
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
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"resource_type: application",
			"name: My App",
			"recoveryFlowHandle: recovery-flow",
			"isRecoveryFlowEnabled: true",
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
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"resource_type: application",
			"name: My App",
			"ouHandle: nonexistent-ou",
			"authFlowHandle: nonexistent-flow",
			"",
		}, "\n"),
		DryRun: true,
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Len(t, appSvc.created, 0)
}

// Agent import tests

type fakeAgentService struct {
	created  []*agentmodel.Agent
	updated  []*agentmodel.UpdateAgentRequest
	existing map[string]*agentmodel.AgentGetResponse
}

func (f *fakeAgentService) CreateAgent(
	_ context.Context, req *agentmodel.Agent,
) (*agentmodel.AgentCompleteResponse, *tidcommon.ServiceError) {
	id := req.Name + "-id"
	f.created = append(f.created, req)
	if f.existing == nil {
		f.existing = map[string]*agentmodel.AgentGetResponse{}
	}
	f.existing[id] = &agentmodel.AgentGetResponse{ID: id, Name: req.Name}
	return &agentmodel.AgentCompleteResponse{ID: id, Name: req.Name}, nil
}

func (f *fakeAgentService) GetAgent(
	_ context.Context, agentID string, _ bool,
) (*agentmodel.AgentGetResponse, *tidcommon.ServiceError) {
	if a, ok := f.existing[agentID]; ok {
		return a, nil
	}
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  agent.ErrorAgentNotFound.Code,
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeAgentService) UpdateAgent(
	_ context.Context, agentID string, req *agentmodel.UpdateAgentRequest,
) (*agentmodel.AgentCompleteResponse, *tidcommon.ServiceError) {
	if _, ok := f.existing[agentID]; !ok {
		return nil, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  agent.ErrorAgentNotFound.Code,
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
		}
	}
	f.updated = append(f.updated, req)
	f.existing[agentID] = &agentmodel.AgentGetResponse{ID: agentID, Name: req.Name}
	return &agentmodel.AgentCompleteResponse{ID: agentID, Name: req.Name}, nil
}

const agentYAML = "resource_type: agent\n" +
	"id: agent-1\ntype: default\nouId: root\nname: Test Agent\ndescription: desc\n"

func TestImportAgent_Create(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: agentYAML})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, agentSvc.created, 1)
	assert.Equal(t, "Test Agent", agentSvc.created[0].Name)
}

func TestImportAgent_UpsertUpdate(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{
		"agent-1": {ID: "agent-1", Name: "Test Agent"},
	}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Content: agentYAML,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Len(t, agentSvc.updated, 1)
}

func TestImportAgent_UpsertFallbackCreate(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Content: agentYAML,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, agentSvc.created, 1)
}

func TestImportAgent_DryRunCreate(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Content: agentYAML,
		DryRun:  true,
	})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Empty(t, agentSvc.created)
}

func TestImportAgent_DryRunUpsert(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{
		"agent-1": {ID: "agent-1", Name: "Test Agent"},
	}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Content: agentYAML,
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Empty(t, agentSvc.updated)
}

func TestImportAgent_NilAdapter(t *testing.T) {
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: agentYAML})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidImportRequest.Code, resp.Results[0].Code)
}

// errAgentService wraps fakeAgentService with injectable per-method errors.
type errAgentService struct {
	inner     *fakeAgentService
	createErr *tidcommon.ServiceError
	getErr    *tidcommon.ServiceError
	updateErr *tidcommon.ServiceError
}

func (e *errAgentService) CreateAgent(
	ctx context.Context, req *agentmodel.Agent,
) (*agentmodel.AgentCompleteResponse, *tidcommon.ServiceError) {
	if e.createErr != nil {
		return nil, e.createErr
	}
	return e.inner.CreateAgent(ctx, req)
}

func (e *errAgentService) GetAgent(
	ctx context.Context, agentID string, includeDisplay bool,
) (*agentmodel.AgentGetResponse, *tidcommon.ServiceError) {
	if e.getErr != nil {
		return nil, e.getErr
	}
	return e.inner.GetAgent(ctx, agentID, includeDisplay)
}

func (e *errAgentService) UpdateAgent(
	ctx context.Context, agentID string, req *agentmodel.UpdateAgentRequest,
) (*agentmodel.AgentCompleteResponse, *tidcommon.ServiceError) {
	if e.updateErr != nil {
		return nil, e.updateErr
	}
	return e.inner.UpdateAgent(ctx, agentID, req)
}

func TestImportAgent_DecodeError(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	// ID field is a sequence, not a string — decode into AgentRequestWithID will fail.
	invalidYAML := "resource_type: agent\nid:\n  - bad\nname: Test\n"
	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: invalidYAML})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidYAMLContent.Code, resp.Results[0].Code)
}

func TestImportAgent_DryRunUpsertNonNotFoundError(t *testing.T) {
	internalErr := &tidcommon.ServiceError{Code: "AGT-9999", Error: tidcommon.I18nMessage{DefaultValue: "internal"}}
	agentSvc := &errAgentService{
		inner:  &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}},
		getErr: internalErr,
	}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Content: agentYAML,
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "AGT-9999", resp.Results[0].Code)
}

func TestImportAgent_UpsertUpdateError(t *testing.T) {
	updateErr := &tidcommon.ServiceError{Code: "AGT-9998", Error: tidcommon.I18nMessage{DefaultValue: "update failed"}}
	agentSvc := &errAgentService{
		inner: &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{
			"agent-1": {ID: "agent-1", Name: "Test Agent"},
		}},
		updateErr: updateErr,
	}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Content: agentYAML,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "AGT-9998", resp.Results[0].Code)
}

func TestImportAgent_UpsertGetNonNotFoundError(t *testing.T) {
	internalErr := &tidcommon.ServiceError{Code: "AGT-9997", Error: tidcommon.I18nMessage{DefaultValue: "server error"}}
	agentSvc := &errAgentService{
		inner:  &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}},
		getErr: internalErr,
	}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{
		Content: agentYAML,
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "AGT-9997", resp.Results[0].Code)
}

func TestImportAgent_CreateError(t *testing.T) {
	createErr := &tidcommon.ServiceError{Code: "AGT-9996", Error: tidcommon.I18nMessage{DefaultValue: "create failed"}}
	agentSvc := &errAgentService{
		inner:     &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}},
		createErr: createErr,
	}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: agentYAML})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "AGT-9996", resp.Results[0].Code)
}

func TestImportAgent_FlowAliasRemapsFlowIDs(t *testing.T) {
	tests := []struct {
		name         string
		flowID       string
		flowHandle   string
		flowName     string
		flowType     string
		agentID      string
		agentName    string
		agentFlowKey string
		getFlowID    func(*agentmodel.Agent) string
	}{
		{
			name:         "auth_flow_id remapped",
			flowID:       "old-flow-id",
			flowHandle:   "login-flow",
			flowName:     "Login",
			flowType:     "AUTHENTICATION",
			agentID:      "agent-2",
			agentName:    "Flow Agent",
			agentFlowKey: "authFlowId",
			getFlowID:    func(r *agentmodel.Agent) string { return r.AuthFlowID },
		},
		{
			name:         "registration_flow_id remapped",
			flowID:       "old-reg-flow-id",
			flowHandle:   "reg-flow",
			flowName:     "Registration",
			flowType:     "REGISTRATION",
			agentID:      "agent-3",
			agentName:    "Reg Agent",
			agentFlowKey: "registrationFlowId",
			getFlowID:    func(r *agentmodel.Agent) string { return r.RegistrationFlowID },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flowSvc := &fakeFlowService{
				byID:  map[string]*providers.CompleteFlowDefinition{},
				byKey: map[string]*providers.CompleteFlowDefinition{},
			}
			agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
			svc := newImportService(
				nil, nil, flowSvc, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, agentSvc, nil, nil, nil,
			)

			content := strings.Join([]string{
				"resource_type: flow",
				"id: " + tc.flowID,
				"handle: " + tc.flowHandle,
				"name: " + tc.flowName,
				"flowType: " + tc.flowType,
				"nodes: []",
				"---",
				"resource_type: agent",
				"id: " + tc.agentID,
				"type: default",
				"ouId: root",
				"name: " + tc.agentName,
				tc.agentFlowKey + ": " + tc.flowID,
				"",
			}, "\n")

			resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

			require.Nil(t, svcErr)
			require.Len(t, resp.Results, 2)
			assert.Equal(t, statusSuccess, resp.Results[1].Status)
			require.Len(t, agentSvc.created, 1)
			assert.Equal(t, tc.flowID, tc.getFlowID(agentSvc.created[0]))
		})
	}
}

func TestImportAgent_StripsClientSecretForPublicAgentWithNoneAuthMethod(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: agent",
		"id: agent-pub",
		"type: default",
		"ouId: root",
		"name: Public Agent",
		"inboundAuthConfig:",
		"  - type: oauth2",
		"    config:",
		"      clientId: pub-client",
		"      clientSecret: should-be-removed",
		"      tokenEndpointAuthMethod: none",
		"      publicClient: true",
		"",
	}, "\n")

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, svcErr)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, agentSvc.created, 1)
	require.Len(t, agentSvc.created[0].InboundAuthConfig, 1)
	require.NotNil(t, agentSvc.created[0].InboundAuthConfig[0].OAuthConfig)
	assert.Equal(t, "", agentSvc.created[0].InboundAuthConfig[0].OAuthConfig.ClientSecret)
}

func TestGetAgentOAuthConfigForImport_NilRequest(t *testing.T) {
	result := getAgentOAuthConfigForImport(nil)
	assert.Nil(t, result)
}

// fakeResourceServerService is a test double for the resource server adapter used by importer tests.
type fakeResourceServerService struct {
	created []providers.ResourceServer
	updated []providers.ResourceServer
}

func (f *fakeResourceServerService) CreateResourceServer(
	_ context.Context, rs providers.ResourceServer,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	f.created = append(f.created, rs)
	return &providers.ResourceServer{ID: rs.ID, Name: rs.Name, OUID: rs.OUID}, nil
}

func (f *fakeResourceServerService) GetResourceServer(
	_ context.Context, id string,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  resource.ErrorResourceServerNotFound.Code,
		Error: tidcommon.I18nMessage{DefaultValue: "not found: " + id},
	}
}

func (f *fakeResourceServerService) UpdateResourceServer(
	_ context.Context, _ string, rs providers.ResourceServer,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
	f.updated = append(f.updated, rs)
	return &providers.ResourceServer{ID: rs.ID, Name: rs.Name, OUID: rs.OUID}, nil
}

func (f *fakeResourceServerService) CreateResource(
	_ context.Context, _ string, _ providers.Resource,
) (*providers.Resource, *tidcommon.ServiceError) {
	return &providers.Resource{}, nil
}

func (f *fakeResourceServerService) GetResourceList(
	_ context.Context, _ string, _ *string, _, _ int,
) (*resource.ResourceList, *tidcommon.ServiceError) {
	return &resource.ResourceList{}, nil
}

func (f *fakeResourceServerService) CreateAction(
	_ context.Context, _ string, _ *string, _ providers.Action,
) (*providers.Action, *tidcommon.ServiceError) {
	return &providers.Action{}, nil
}

// TestImportRole_OUHandleResolved verifies that ou_handle on a role document is resolved
// to ou_id via the OU service before the role create request is built.
func TestImportRole_OUHandleResolved(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(
		nil, nil, nil, ouSvc, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: role",
		"id: role-new",
		"name: Viewer",
		"ouHandle: default",
		"permissions: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, roleSvc.created, 1)
	assert.Equal(t, "ou-default", roleSvc.created[0].OUID)
}

// TestImportRole_OUHandleNotFound verifies that an unknown ou_handle on a role document
// causes the import to fail with a clear error.
func TestImportRole_OUHandleNotFound(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(
		nil, nil, nil, ouSvc, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: role",
		"id: role-new",
		"name: Viewer",
		"ouHandle: missing",
		"permissions: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Empty(t, roleSvc.created)
}

// TestImportRole_OUIDWinsOverHandle verifies that ou_id wins when both ou_id and ou_handle
// are provided, and the OU service is never consulted.
func TestImportRole_OUIDWinsOverHandle(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(
		nil, nil, nil, ouSvc, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)

	content := strings.Join([]string{
		"resource_type: role",
		"id: role-new",
		"name: Viewer",
		"ouId: ou-explicit",
		"ouHandle: default",
		"permissions: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, roleSvc.created, 1)
	assert.Equal(t, "ou-explicit", roleSvc.created[0].OUID)
}

// TestImportGroup_OUHandleResolved verifies that ou_handle on a group document is resolved
// to ou_id via the OU service before the group create request is built.
func TestImportGroup_OUHandleResolved(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	groupSvc := &fakeGroupService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: group",
		"id: group-new",
		"name: Engineers",
		"ouHandle: default",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, groupSvc.created, 1)
	assert.Equal(t, "ou-default", groupSvc.created[0].OUID)
}

// TestImportGroup_OUHandleNotFound verifies that an unknown ou_handle on a group document
// causes the import to fail with a clear error.
func TestImportGroup_OUHandleNotFound(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	groupSvc := &fakeGroupService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: group",
		"id: group-new",
		"name: Engineers",
		"ouHandle: missing",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Empty(t, groupSvc.created)
}

// TestImportGroup_OUIDWinsOverHandle verifies that ou_id wins when both ou_id and ou_handle
// are provided, and the OU service is never consulted.
func TestImportGroup_OUIDWinsOverHandle(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	groupSvc := &fakeGroupService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: group",
		"id: group-new",
		"name: Engineers",
		"ouId: ou-explicit",
		"ouHandle: default",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, groupSvc.created, 1)
	assert.Equal(t, "ou-explicit", groupSvc.created[0].OUID)
}

// TestImportUser_OUHandleResolved verifies that ou_handle on a user document is resolved
// to ou_id via the OU service before the user create request is built.
func TestImportUser_OUHandleResolved(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	userSvc := &fakeUserService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: user",
		"id: user-new",
		"type: person",
		"ouHandle: default",
		"attributes:",
		"  username: alice",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, userSvc.created, 1)
	assert.Equal(t, "ou-default", userSvc.created[0].OUID)
}

// TestImportUser_OUHandleNotFound verifies that an unknown ou_handle on a user document
// causes the import to fail with a clear error.
func TestImportUser_OUHandleNotFound(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	userSvc := &fakeUserService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: user",
		"id: user-new",
		"type: person",
		"ouHandle: missing",
		"attributes:",
		"  username: alice",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Empty(t, userSvc.created)
}

// TestImportUser_OUIDWinsOverHandle verifies that ou_id wins when both ou_id and ou_handle
// are provided, and the OU service is never consulted.
func TestImportUser_OUIDWinsOverHandle(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	userSvc := &fakeUserService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: user",
		"id: user-new",
		"type: person",
		"ouId: ou-explicit",
		"ouHandle: default",
		"attributes:",
		"  username: alice",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, userSvc.created, 1)
	assert.Equal(t, "ou-explicit", userSvc.created[0].OUID)
}

// TestImportResourceServer_OUHandleResolved verifies that ou_handle on a resource server
// document is resolved to ou_id via the OU service before the create request is built.
func TestImportResourceServer_OUHandleResolved(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: resource_server",
		"id: rs-new",
		"name: Test RS",
		"handle: test-rs",
		"identifier: test-rs",
		"ouHandle: default",
		"resources: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, rsSvc.created, 1)
	assert.Equal(t, "ou-default", rsSvc.created[0].OUID)
}

// TestImportResourceServer_OUHandleNotFound verifies that an unknown ou_handle on a resource
// server document causes the import to fail with a clear error.
func TestImportResourceServer_OUHandleNotFound(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: resource_server",
		"id: rs-new",
		"name: Test RS",
		"handle: test-rs",
		"identifier: test-rs",
		"ouHandle: missing",
		"resources: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Empty(t, rsSvc.created)
}

// TestImportResourceServer_OUIDWinsOverHandle verifies that ou_id wins when both ou_id and
// ou_handle are provided, and the OU service is never consulted.
func TestImportResourceServer_OUIDWinsOverHandle(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]providers.OrganizationUnit{}}
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(nil, nil, nil, ouSvc, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: resource_server",
		"id: rs-new",
		"name: Test RS",
		"handle: test-rs",
		"identifier: test-rs",
		"ouId: ou-explicit",
		"ouHandle: default",
		"resources: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, rsSvc.created, 1)
	assert.Equal(t, "ou-explicit", rsSvc.created[0].OUID)
}

func TestImportResources_IDPPropertiesArePassedToService(t *testing.T) {
	idpSvc := &fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}}
	svc := newImportService(nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// is_secret with empty value avoids the encryption path while still verifying the flag is parsed.
	content := strings.Join([]string{
		"resource_type: identity_provider",
		"name: google-idp",
		"type: GOOGLE",
		"properties:",
		"- name: client_id",
		"  value: my-client-id",
		"- name: client_secret",
		"  isSecret: true",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, idpSvc.created, 1)

	created := idpSvc.created[0]
	require.Len(t, created.Properties, 2)
	assert.Equal(t, "client_id", created.Properties[0].GetName())
	assert.False(t, created.Properties[0].IsSecret())
	assert.Equal(t, "client_secret", created.Properties[1].GetName())
	assert.True(t, created.Properties[1].IsSecret())

	plainValue, err2 := created.Properties[0].GetValue()
	require.NoError(t, err2)
	assert.Equal(t, "my-client-id", plainValue)
}

func TestImportResources_IDPUpsertUpdatePropertiesArePassedToService(t *testing.T) {
	existing := &providers.IDPDTO{ID: "idp-1", Name: "google-idp", Type: providers.IDPTypeGoogle}
	idpSvc := &fakeIDPService{
		byID:   map[string]*providers.IDPDTO{"idp-1": existing},
		byName: map[string]*providers.IDPDTO{"google-idp": existing},
	}
	svc := newImportService(nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: identity_provider",
		"id: idp-1",
		"name: google-idp",
		"type: GOOGLE",
		"properties:",
		"- name: client_id",
		"  value: updated-client-id",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, idpSvc.updated, 1)

	updated := idpSvc.updated[0]
	require.Len(t, updated.Properties, 1)
	assert.Equal(t, "client_id", updated.Properties[0].GetName())

	plainValue, err2 := updated.Properties[0].GetValue()
	require.NoError(t, err2)
	assert.Equal(t, "updated-client-id", plainValue)
}
