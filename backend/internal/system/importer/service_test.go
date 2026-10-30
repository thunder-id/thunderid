// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	authzdetailmgt "github.com/thunder-id/thunderid/internal/authzdetail/mgt"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/agent"
	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/connection"
	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	ncommon "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/user"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// testCryptoKey is the shared key used so secret property encryption works in tests
// that round-trip a connection with a secret field through the declarative parser.
const testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"

// TestMain wires cmodels' package-level config crypto provider once for the whole test
// binary, so secret Property encryption works regardless of which test's setup last
// reset the server runtime.
func TestMain(m *testing.M) {
	config.ResetServerRuntime()
	if err := config.InitializeServerRuntime("/tmp/test", &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize server runtime: %v\n", err)
		os.Exit(1)
	}
	_, cfgCryptoSvc, err := defaultkm.Initialize(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize default crypto provider: %v\n", err)
		os.Exit(1)
	}
	cmodels.SetConfigCryptoProvider(cfgCryptoSvc)
	config.ResetServerRuntime()
	os.Exit(m.Run())
}

func boolPtr(v bool) *bool {
	return &v
}

func updateOUCommon(existing map[string]ou.OrganizationUnit, updated *[]ou.OrganizationUnitRequest,
	id string, request ou.OrganizationUnitRequest) (ou.OrganizationUnit, *tidcommon.ServiceError) {
	if _, ok := existing[id]; !ok {
		return ou.OrganizationUnit{}, &tidcommon.ServiceError{
			Type:  tidcommon.ClientErrorType,
			Code:  "OU-1003",
			Error: tidcommon.I18nMessage{DefaultValue: "not found"},
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
	existing map[string]*providers.Application
	// sequence records each mutation in the order it arrived. Resources are stored by id, so a
	// create and a delete of two different ids both succeed whichever way round they run; the
	// sequence is what shows which actually went first.
	sequence []string
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
	f.sequence = append(f.sequence, "create:"+app.ID)
	return app, nil
}

func (f *fakeApplicationService) ValidateApplication(
	_ context.Context, _ *model.ApplicationDTO,
) (*model.ApplicationProcessedDTO, *providers.InboundAuthConfigWithSecret, *tidcommon.ServiceError) {
	return nil, nil, nil
}

func (f *fakeApplicationService) GetApplicationList(
	_ context.Context, _ map[string]interface{},
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
	// An update never moves an application between organization units, so the stored owner stays
	// as it was however the document names it.
	f.existing[appID] = &providers.Application{ID: app.ID, Name: app.Name, OUID: f.existing[appID].OUID}
	f.sequence = append(f.sequence, "update:"+appID)
	return app, nil
}

func (f *fakeApplicationService) DeleteApplication(_ context.Context, appID string) *tidcommon.ServiceError {
	if _, ok := f.existing[appID]; !ok {
		return &application.ErrorApplicationNotFound
	}
	delete(f.existing, appID)
	f.sequence = append(f.sequence, "delete:"+appID)
	return nil
}

type fakeIDPService struct {
	created   []*providers.IDPDTO
	updated   []*providers.IDPDTO
	byID      map[string]*providers.IDPDTO
	byName    map[string]*providers.IDPDTO
	updateErr *tidcommon.ServiceError
	getErr    *tidcommon.ServiceError
}

func (f *fakeIDPService) ResolveIdentityProviderOU(context.Context, *providers.IDPDTO) *tidcommon.ServiceError {
	return nil
}

type fakeAuthZENPDPService struct {
	created *authzenpdp.AuthZENPDPConnection
}

func (f *fakeAuthZENPDPService) ResolveConnectionOU(
	context.Context, *authzenpdp.AuthZENPDPConnection,
) *tidcommon.ServiceError {
	return nil
}

func (f *fakeAuthZENPDPService) CreateAuthZENPDPConnection(
	_ context.Context,
	request authzenpdp.ConnectionRequest,
) (*authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError) {
	if f.created != nil {
		return f.created, nil
	}
	return &authzenpdp.AuthZENPDPConnection{ID: request.ID, Name: request.Name}, nil
}

func (f *fakeAuthZENPDPService) GetAuthZENPDP(
	_ context.Context, _ string,
) (*authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError) {
	return nil, nil
}

func (f *fakeAuthZENPDPService) UpdateAuthZENPDPConnection(
	_ context.Context,
	id string,
	request authzenpdp.ConnectionRequest,
) (*authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError) {
	return &authzenpdp.AuthZENPDPConnection{ID: id, Name: request.Name}, nil
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
	if f.getErr != nil {
		return nil, f.getErr
	}
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
	if f.updateErr != nil {
		return nil, f.updateErr
	}
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

// fakeSenderService is a minimal senderAdapter fake. updateErr/createErr let a test force a
// specific failure (e.g. notification.ErrorSenderNotFound to exercise the upsert-fallback path).
type fakeSenderService struct {
	created   []ncommon.NotificationSenderDTO
	updated   []ncommon.NotificationSenderDTO
	byID      map[string]*ncommon.NotificationSenderDTO
	updateErr *tidcommon.ServiceError
	createErr *tidcommon.ServiceError
	getErr    *tidcommon.ServiceError
}

func (f *fakeSenderService) ResolveSenderOU(context.Context, *ncommon.NotificationSenderDTO) *tidcommon.ServiceError {
	return nil
}

func (f *fakeSenderService) CreateSender(
	_ context.Context, sender ncommon.NotificationSenderDTO,
) (*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	if sender.ID == "" {
		sender.ID = "generated-sender-id"
	}
	f.created = append(f.created, sender)
	f.byID[sender.ID] = &sender
	return &sender, nil
}

func (f *fakeSenderService) GetSender(
	_ context.Context, id string,
) (*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if v, ok := f.byID[id]; ok {
		return v, nil
	}
	return nil, &notification.ErrorSenderNotFound
}

func (f *fakeSenderService) UpdateSender(
	_ context.Context, id string, sender ncommon.NotificationSenderDTO,
) (*ncommon.NotificationSenderDTO, *tidcommon.ServiceError) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if _, ok := f.byID[id]; !ok {
		return nil, &notification.ErrorSenderNotFound
	}
	sender.ID = id
	f.updated = append(f.updated, sender)
	f.byID[id] = &sender
	return &sender, nil
}

type fakeFlowService struct {
	created    []*flowmgt.FlowDefinition
	updated    []*flowmgt.FlowDefinition
	byID       map[string]*providers.CompleteFlowDefinition
	byKey      map[string]*providers.CompleteFlowDefinition
	ouIDByPath map[string]string
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

type fakeLayoutService struct {
	created   []layoutmgt.CreateLayoutRequestWithID
	updated   []layoutmgt.UpdateLayoutRequest
	byID      map[string]*layoutmgt.Layout
	byHandle  map[string]*layoutmgt.Layout
	createErr *tidcommon.ServiceError
	updateErr *tidcommon.ServiceError
}

func layoutNotFoundErr() *tidcommon.ServiceError {
	return &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  layoutmgt.ErrorLayoutNotFound.Code,
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeLayoutService) CreateLayout(_ context.Context,
	layout layoutmgt.CreateLayoutRequestWithID,
) (*layoutmgt.Layout, *tidcommon.ServiceError) {
	if f.createErr != nil {
		return nil, f.createErr
	}

	id := layout.ID
	if id == "" {
		id = "generated-layout-id"
	}

	created := &layoutmgt.Layout{
		ID:          id,
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}
	f.created = append(f.created, layout)
	if f.byID == nil {
		f.byID = map[string]*layoutmgt.Layout{}
	}
	if f.byHandle == nil {
		f.byHandle = map[string]*layoutmgt.Layout{}
	}
	f.byID[created.ID] = created
	f.byHandle[created.Handle] = created
	return created, nil
}

func (f *fakeLayoutService) GetLayout(_ context.Context, id string) (*layoutmgt.Layout, *tidcommon.ServiceError) {
	if existing, ok := f.byID[id]; ok {
		return existing, nil
	}

	return nil, layoutNotFoundErr()
}

func (f *fakeLayoutService) UpdateLayout(_ context.Context,
	id string, layout layoutmgt.UpdateLayoutRequest,
) (*layoutmgt.Layout, *tidcommon.ServiceError) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}

	if _, ok := f.byID[id]; !ok {
		return nil, layoutNotFoundErr()
	}

	updated := &layoutmgt.Layout{
		ID:          id,
		Handle:      layout.Handle,
		DisplayName: layout.DisplayName,
		Description: layout.Description,
		Layout:      layout.Layout,
	}
	f.updated = append(f.updated, layout)
	f.byID[id] = updated
	f.byHandle[updated.Handle] = updated
	return updated, nil
}

type fakeEntityTypeService struct {
	created          []entitytype.CreateEntityTypeRequestWithID
	updated          []entitytype.UpdateEntityTypeRequest
	createCategories []entitytype.TypeCategory
	byID             map[string]*entitytype.EntityType
	byHandle         map[string]*entitytype.EntityType
}

func (f *fakeEntityTypeService) CreateEntityType(
	_ context.Context, category entitytype.TypeCategory, request entitytype.CreateEntityTypeRequestWithID,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	id := request.ID
	if id == "" {
		id = "generated-entity-type-id"
	}
	created := &entitytype.EntityType{
		ID:                    id,
		Handle:                request.Handle,
		DisplayName:           request.DisplayName,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      request.SystemAttributes,
		Schema:                request.Schema,
	}
	f.created = append(f.created, request)
	f.createCategories = append(f.createCategories, category)
	if f.byID == nil {
		f.byID = map[string]*entitytype.EntityType{}
	}
	if f.byHandle == nil {
		f.byHandle = map[string]*entitytype.EntityType{}
	}
	f.byID[created.ID] = created
	f.byHandle[created.Handle] = created
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

func (f *fakeEntityTypeService) GetEntityTypeByHandle(
	_ context.Context, _ entitytype.TypeCategory, handle string,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	if existing, ok := f.byHandle[handle]; ok {
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
		Handle:                request.Handle,
		DisplayName:           request.DisplayName,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      request.SystemAttributes,
		Schema:                request.Schema,
	}
	f.updated = append(f.updated, request)
	f.byID[schemaID] = updated
	f.byHandle[updated.Handle] = updated
	return updated, nil
}

type fakeOUService struct {
	created  []ou.OrganizationUnitRequestWithID
	updated  []ou.OrganizationUnitRequest
	existing map[string]ou.OrganizationUnit
}

func (f *fakeOUService) CreateOrganizationUnit(
	_ context.Context, request ou.OrganizationUnitRequestWithID,
) (ou.OrganizationUnit, *tidcommon.ServiceError) {
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
) (ou.OrganizationUnit, *tidcommon.ServiceError) {
	if existing, ok := f.existing[id]; ok {
		return existing, nil
	}

	return ou.OrganizationUnit{}, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OU-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeOUService) GetOrganizationUnitByPath(
	_ context.Context, handlePath string,
) (ou.OrganizationUnit, *tidcommon.ServiceError) {
	for _, existing := range f.existing {
		if existing.Handle == handlePath {
			return existing, nil
		}
	}

	return ou.OrganizationUnit{}, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "OU-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeOUService) UpdateOrganizationUnit(
	_ context.Context, id string, request ou.OrganizationUnitRequestWithID,
) (ou.OrganizationUnit, *tidcommon.ServiceError) {
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
	deleted []string
}

func (f *fakeRoleService) CreateRole(
	_ context.Context, req role.RoleCreationDetail,
) (*role.RoleWithPermissionsAndAssignments, *tidcommon.ServiceError) {
	f.created = append(f.created, req)
	return &role.RoleWithPermissionsAndAssignments{ID: "role-1", Name: req.Name, OUID: req.OUID}, nil
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
	return &role.RoleWithPermissions{ID: "role-1", Name: req.Name, OUID: req.OUID}, nil
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
	created                     []*providers.User
	updateCredentialsShouldFail bool
	deleted                     []string
}

func (f *fakeUserService) CreateUser(
	_ context.Context, u *providers.User,
) (*providers.User, *tidcommon.ServiceError) {
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
) (*providers.User, *tidcommon.ServiceError) {
	return nil, &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "USR-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "not found"},
	}
}

func (f *fakeUserService) UpdateUser(
	_ context.Context, userID string, u *providers.User,
) (*providers.User, *tidcommon.ServiceError) {
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
		OUID:     flowDef.OUID,
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
		OUID:     f.byID[flowID].OUID,
	}
	f.updated = append(f.updated, flowDef)
	f.byID[flowID] = updated
	f.byKey[string(flowDef.FlowType)+":"+flowDef.Handle] = updated
	return updated, nil
}

func (f *fakeFlowService) ResolveFlowOU(_ context.Context, flowDef *flowmgt.FlowDefinition) *tidcommon.ServiceError {
	if flowDef.OUID == "" {
		if flowDef.OUHandle == "" {
			return &flowmgt.ErrorFlowOURequired
		}
		ouID, ok := f.ouIDByPath[flowDef.OUHandle]
		if !ok {
			return &flowmgt.ErrorInvalidFlowOU
		}
		flowDef.OUID = ouID
	}
	flowDef.OUHandle = ""
	return nil
}

// fakeSharingService records what the importer asked it to replay.
type fakeSharingService struct {
	calls   []sharingReplayCall
	replay  sharing.PolicyReplay
	failure *sharing.PolicyReplayError
}

type sharingReplayCall struct {
	resourceType sharing.ResourceType
	resourceID   string
	owningOUID   string
	policies     []sharing.PolicyRequest
}

func (f *fakeSharingService) ReplacePoliciesForResource(
	_ context.Context, rt sharing.ResourceType, resourceID, owningOUID string,
	requests []sharing.PolicyRequest,
) (sharing.PolicyReplay, *sharing.PolicyReplayError) {
	f.calls = append(f.calls, sharingReplayCall{
		resourceType: rt, resourceID: resourceID, owningOUID: owningOUID, policies: requests})
	return f.replay, f.failure
}

// newTestImportServiceWithSharing is newTestImportService with a sharing adapter installed, the way
// Initialize installs the real one after construction.
func newTestImportServiceWithSharing(
	appSvc *fakeApplicationService, sharingSvc *fakeSharingService,
) ImportServiceInterface {
	svc := newTestImportService(appSvc)
	svc.(*importService).sharingService = sharingSvc
	return svc
}

func newTestImportService(appSvc *fakeApplicationService) ImportServiceInterface {
	if appSvc == nil {
		appSvc = &fakeApplicationService{existing: map[string]*providers.Application{}}
	}

	return newImportService(
		appSvc,
		&fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}},
		nil,
		&fakeFlowService{
			byID:  map[string]*providers.CompleteFlowDefinition{},
			byKey: map[string]*providers.CompleteFlowDefinition{},
		},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
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

func TestImportConnectionAuthZENPDP_CreateUsesGeneratedConnectionID(t *testing.T) {
	service := &importService{
		authZENPDPService: &fakeAuthZENPDPService{
			created: &authzenpdp.AuthZENPDPConnection{
				ID:   "generated-pdp-id",
				Name: "Generated PDP",
			},
		},
	}

	outcome := service.importConnectionAuthZENPDP(context.Background(), &authzenpdp.AuthZENPDPConnection{
		Name:     "Imported PDP",
		Endpoint: "https://pdp.example.com/evaluate",
	}, nil, false)

	assert.Equal(t, statusSuccess, outcome.Status)
	assert.Equal(t, operationCreate, outcome.Operation)
	assert.Equal(t, "generated-pdp-id", outcome.ResourceID)
	assert.Equal(t, "Generated PDP", outcome.ResourceName)
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

// stateDocuments are a document without a state and the same document with one.
func stateDocuments(doc string) []struct {
	name     string
	content  string
	warnings int
} {
	return []struct {
		name     string
		content  string
		warnings int
	}{
		{name: "without a state", content: doc, warnings: 0},
		{name: "with a state", content: doc + "state: SUSPENDED\n", warnings: 1},
	}
}

// TestImportResources_ApplicationStateIgnored verifies that a state in an application document
// does not fail the import and is reported as ignored.
func TestImportResources_ApplicationStateIgnored(t *testing.T) {
	for _, tc := range stateDocuments("resource_type: application\nid: app-1\nname: My App\nauthFlowId: flow-1\n") {
		t.Run(tc.name, func(t *testing.T) {
			appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
			svc := newTestImportService(appSvc)

			var resp *ImportResponse
			var err *tidcommon.ServiceError
			resp, err = svc.ImportResources(context.Background(), &ImportRequest{
				Content: tc.content,
				Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true),
					Target: importTargetRuntime},
			})

			require.Nil(t, err)
			require.Len(t, resp.Results, 1)
			assert.Equal(t, statusSuccess, resp.Results[0].Status)
			assert.Equal(t, operationCreate, resp.Results[0].Operation)
			require.Len(t, appSvc.created, 1)
			assert.Equal(t, tc.warnings, strings.Count(resp.Results[0].Message, "State in the document was ignored"))
		})
	}
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
		"resource_type: connection",
		"name: idp-one",
		"type: google",
		"clientId: abc",
		"---",
		"resource_type: flow",
		"handle: login",
		"name: Login Flow",
		"flowType: AUTHENTICATION",
		"ouId: ou-1",
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
	assert.Equal(t, resourceTypeConnection, resp.Results[0].ResourceType)
	assert.Equal(t, resourceTypeFlow, resp.Results[1].ResourceType)
}

func TestImportResources_ConnectionDecodeFailure(t *testing.T) {
	svc := newTestImportService(nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nname: bad-one\ntype: unsupported-vendor\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidYAMLContent.Code, resp.Results[0].Code)
	assert.Contains(t, resp.Results[0].Message, "failed to decode connection document")
}

func TestImportResources_ConnectionIDPAdapterNotConfigured(t *testing.T) {
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nname: idp-one\ntype: google\nclientId: abc\n",
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidImportRequest.Code, resp.Results[0].Code)
	assert.Equal(t, "identity provider adapter is not configured", resp.Results[0].Message)
}

func TestImportResources_ConnectionIDPUpdateFailureNonNotFound(t *testing.T) {
	idpSvc := &fakeIDPService{
		byID: map[string]*providers.IDPDTO{"idp-1": {ID: "idp-1"}}, byName: map[string]*providers.IDPDTO{},
		updateErr: &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "IDP-5000",
			Error: tidcommon.I18nMessage{DefaultValue: "internal error"},
		},
	}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: idp-1\nname: idp-one\ntype: google\nclientId: abc\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Equal(t, "IDP-5000", resp.Results[0].Code)
}

func TestImportResources_ConnectionIDPUpsertFallsBackToCreateOnNotFound(t *testing.T) {
	idpSvc := &fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: missing-idp\nname: idp-one\ntype: google\nclientId: abc\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, idpSvc.updated, 0)
	require.Len(t, idpSvc.created, 1)
}

func TestImportResources_DryRunConnectionIDPCreateWithoutWrite(t *testing.T) {
	idpSvc := &fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nname: idp-one\ntype: google\nclientId: abc\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, idpSvc.created, 0)
	assert.Len(t, idpSvc.updated, 0)
}

func TestImportResources_DryRunConnectionIDPUpdateWithoutWrite(t *testing.T) {
	idpSvc := &fakeIDPService{
		byID: map[string]*providers.IDPDTO{"idp-1": {ID: "idp-1"}}, byName: map[string]*providers.IDPDTO{},
	}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: idp-1\nname: idp-one\ntype: google\nclientId: abc\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Len(t, idpSvc.created, 0)
	assert.Len(t, idpSvc.updated, 0)
}

func TestImportResources_DryRunConnectionIDPServiceError(t *testing.T) {
	idpSvc := &fakeIDPService{
		byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{},
		getErr: &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "IDP-5002",
			Error: tidcommon.I18nMessage{DefaultValue: "internal error"},
		},
	}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: idp-1\nname: idp-one\ntype: google\nclientId: abc\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "IDP-5002", resp.Results[0].Code)
}

func TestImportResources_CreateConnectionSender(t *testing.T) {
	senderSvc := &fakeSenderService{byID: map[string]*ncommon.NotificationSenderDTO{}}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nname: sms-one\ntype: sms-gateway\nurl: https://example.com/sms\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, resourceTypeConnection, resp.Results[0].ResourceType)
	require.Len(t, senderSvc.created, 1)
	assert.Equal(t, "sms-one", senderSvc.created[0].Name)
}

func TestImportResources_UpdateConnectionSender(t *testing.T) {
	senderSvc := &fakeSenderService{byID: map[string]*ncommon.NotificationSenderDTO{
		"sender-1": {ID: "sender-1", Name: "Old Name"},
	}}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: sender-1\nname: sms-one\ntype: sms-gateway\n" +
			"url: https://example.com/sms\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	require.Len(t, senderSvc.updated, 1)
	assert.Equal(t, "sms-one", senderSvc.updated[0].Name)
}

func TestImportResources_ConnectionSenderUpsertFallsBackToCreateOnNotFound(t *testing.T) {
	senderSvc := &fakeSenderService{byID: map[string]*ncommon.NotificationSenderDTO{}}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: missing-sender\nname: sms-one\ntype: sms-gateway\n" +
			"url: https://example.com/sms\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, senderSvc.updated, 0)
	require.Len(t, senderSvc.created, 1)
}

func TestImportResources_ConnectionSenderUpdateFailureNonNotFound(t *testing.T) {
	senderSvc := &fakeSenderService{
		byID: map[string]*ncommon.NotificationSenderDTO{"sender-1": {ID: "sender-1"}},
		updateErr: &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "MNS-5000",
			Error: tidcommon.I18nMessage{DefaultValue: "internal error"},
		},
	}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: sender-1\nname: sms-one\ntype: sms-gateway\n" +
			"url: https://example.com/sms\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Equal(t, "MNS-5000", resp.Results[0].Code)
}

func TestImportResources_ConnectionSenderCreateFailure(t *testing.T) {
	senderSvc := &fakeSenderService{
		byID: map[string]*ncommon.NotificationSenderDTO{},
		createErr: &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "MNS-5001",
			Error: tidcommon.I18nMessage{DefaultValue: "internal error"},
		},
	}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nname: sms-one\ntype: sms-gateway\nurl: https://example.com/sms\n",
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "MNS-5001", resp.Results[0].Code)
}

func TestImportResources_ConnectionSenderAdapterNotConfigured(t *testing.T) {
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nname: sms-one\ntype: sms-gateway\nurl: https://example.com/sms\n",
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidImportRequest.Code, resp.Results[0].Code)
	assert.Equal(t, "notification sender adapter is not configured", resp.Results[0].Message)
}

func TestImportResources_DryRunConnectionSenderCreateWithoutWrite(t *testing.T) {
	senderSvc := &fakeSenderService{byID: map[string]*ncommon.NotificationSenderDTO{}}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nname: sms-one\ntype: sms-gateway\nurl: https://example.com/sms\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, senderSvc.created, 0)
	assert.Len(t, senderSvc.updated, 0)
}

func TestImportResources_DryRunConnectionSenderUpdateWithoutWrite(t *testing.T) {
	senderSvc := &fakeSenderService{byID: map[string]*ncommon.NotificationSenderDTO{
		"sender-1": {ID: "sender-1"},
	}}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: sender-1\nname: sms-one\ntype: sms-gateway\n" +
			"url: https://example.com/sms\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Len(t, senderSvc.created, 0)
	assert.Len(t, senderSvc.updated, 0)
}

func TestImportResources_DryRunConnectionSenderServiceError(t *testing.T) {
	senderSvc := &fakeSenderService{
		byID: map[string]*ncommon.NotificationSenderDTO{},
		getErr: &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType, Code: "MNS-5002",
			Error: tidcommon.I18nMessage{DefaultValue: "internal error"},
		},
	}
	svc := newImportService(
		nil, nil, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: connection\nid: sender-1\nname: sms-one\ntype: sms-gateway\n" +
			"url: https://example.com/sms\n",
		DryRun:  true,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "MNS-5002", resp.Results[0].Code)
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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
		nil, nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
		nil, nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
		nil, nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
		nil, nil, nil, nil, nil, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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

func TestImportResources_OrganizationUnitCarriesDefaultFlowFields(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: organization_unit",
		"id: ou-456",
		"handle: eng",
		"name: Engineering",
		"authFlowId: auth-flow-1",
		"registrationFlowId: reg-flow-1",
		"isRegistrationFlowEnabled: true",
		"recoveryFlowId: recovery-flow-1",
		"isRecoveryFlowEnabled: true",
		"signOutFlowId: signout-flow-1",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, ouSvc.created, 1)
	assert.Equal(t, "auth-flow-1", ouSvc.created[0].AuthFlowID)
	assert.Equal(t, "reg-flow-1", ouSvc.created[0].RegistrationFlowID)
	assert.True(t, ouSvc.created[0].IsRegistrationFlowEnabled)
	assert.Equal(t, "recovery-flow-1", ouSvc.created[0].RecoveryFlowID)
	assert.True(t, ouSvc.created[0].IsRecoveryFlowEnabled)
	assert.Equal(t, "signout-flow-1", ouSvc.created[0].SignOutFlowID)
}

func TestImportResources_FlowUpsertCreatePreservesID(t *testing.T) {
	flowSvc := &fakeFlowService{
		byID:  map[string]*providers.CompleteFlowDefinition{},
		byKey: map[string]*providers.CompleteFlowDefinition{},
	}

	svc := newImportService(
		nil, nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: flow",
		"id: missing-flow-id",
		"handle: registration-flow",
		"name: Updated Registration Flow",
		"flowType: REGISTRATION",
		"ouId: ou-1",
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

func TestImportResources_FlowKeepsItsOrganizationUnit(t *testing.T) {
	flowSvc := &fakeFlowService{
		byID:  map[string]*providers.CompleteFlowDefinition{},
		byKey: map[string]*providers.CompleteFlowDefinition{},
	}
	svc := newImportService(
		nil, nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	content := strings.Join([]string{
		"resource_type: flow",
		"id: owned-flow-id",
		"handle: owned-flow",
		"name: Owned Flow",
		"flowType: AUTHENTICATION",
		"ouId: ou-1",
		"nodes: []",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, flowSvc.created, 1)
	assert.Equal(t, "ou-1", flowSvc.created[0].OUID)
}

func TestImportResources_FlowOrganizationUnit(t *testing.T) {
	flowDoc := func(owner string) string {
		lines := []string{
			"resource_type: flow",
			"id: owned-flow-id",
			"handle: owned-flow",
			"name: Owned Flow",
			"flowType: AUTHENTICATION",
		}
		if owner != "" {
			lines = append(lines, owner)
		}
		return strings.Join(append(lines, "nodes: []", ""), "\n")
	}
	existing := func() *fakeFlowService {
		flow := &providers.CompleteFlowDefinition{
			ID: "owned-flow-id", Handle: "owned-flow", Name: "Owned Flow",
			FlowType: providers.FlowTypeAuthentication, OUID: "ou-1",
		}
		return &fakeFlowService{
			byID:       map[string]*providers.CompleteFlowDefinition{"owned-flow-id": flow},
			byKey:      map[string]*providers.CompleteFlowDefinition{"AUTHENTICATION:owned-flow": flow},
			ouIDByPath: map[string]string{"default": "ou-1", "other": "ou-2"},
		}
	}

	tests := []struct {
		name         string
		owner        string
		dryRun       bool
		wantStatus   string
		wantCode     string
		wantUpdated  int
		wantOwnerSet string
	}{
		{name: "handle resolved to the id", owner: "ouHandle: default", wantStatus: statusSuccess,
			wantUpdated: 1, wantOwnerSet: "ou-1"},
		{name: "id wins over the handle", owner: "ouId: ou-1\nouHandle: other", wantStatus: statusSuccess,
			wantUpdated: 1, wantOwnerSet: "ou-1"},
		{name: "no owner is refused", owner: "", wantStatus: statusFailed, wantCode: flowmgt.ErrorFlowOURequired.Code},
		{name: "unknown handle is refused", owner: "ouHandle: nowhere", wantStatus: statusFailed,
			wantCode: flowmgt.ErrorInvalidFlowOU.Code},
		{name: "dry run without an owner is refused", owner: "", dryRun: true, wantStatus: statusFailed,
			wantCode: flowmgt.ErrorFlowOURequired.Code},
		{name: "dry run accepts the owner the flow has", owner: "ouHandle: default", dryRun: true,
			wantStatus: statusSuccess},
		{name: "dry run refuses another owner by id", owner: "ouId: ou-2", dryRun: true, wantStatus: statusFailed,
			wantCode: flowmgt.ErrorOUUpdateNotAllowed.Code},
		{name: "dry run refuses another owner by handle", owner: "ouHandle: other", dryRun: true,
			wantStatus: statusFailed, wantCode: flowmgt.ErrorOUUpdateNotAllowed.Code},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flowSvc := existing()
			svc := newImportService(
				nil, nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

			resp, err := svc.ImportResources(context.Background(), &ImportRequest{
				Content: flowDoc(tc.owner),
				DryRun:  tc.dryRun,
				Options: &ImportOptions{
					Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime,
				},
			})

			require.Nil(t, err)
			require.Len(t, resp.Results, 1)
			assert.Equal(t, tc.wantStatus, resp.Results[0].Status)
			assert.Equal(t, tc.wantCode, resp.Results[0].Code)
			assert.Len(t, flowSvc.updated, tc.wantUpdated)
			assert.Empty(t, flowSvc.created)
			if tc.wantOwnerSet != "" {
				assert.Equal(t, tc.wantOwnerSet, flowSvc.updated[0].OUID)
				assert.Empty(t, flowSvc.updated[0].OUHandle)
			}
		})
	}
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

	svc := newImportService(
		nil, nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: flow",
		"id: missing-flow-id",
		"handle: registration-flow",
		"name: Updated Registration Flow",
		"flowType: REGISTRATION",
		"ouId: ou-1",
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
	svc := newImportService(
		appSvc, nil, nil, flowSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: flow",
		"id: missing-registration-flow-id",
		"handle: registration-flow",
		"name: Updated Registration Flow",
		"flowType: REGISTRATION",
		"ouId: ou-1",
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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, themeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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

func layoutImportContent() string {
	return strings.Join([]string{
		"resource_type: layout",
		"id: lay-123",
		"handle: default-layout",
		"displayName: Default Layout",
		"layout:",
		"  head:",
		"    stylesheets: []",
		"",
	}, "\n")
}

func newLayoutImportService(layoutSvc *fakeLayoutService) ImportServiceInterface {
	return newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, layoutSvc, nil, nil, nil, nil, nil, nil, nil, nil)
}

// Upsert with an ID that does not exist falls back to a create that preserves the ID.
func TestImportResources_LayoutUpsertCreatePreservesID(t *testing.T) {
	layoutSvc := &fakeLayoutService{byID: map[string]*layoutmgt.Layout{}, byHandle: map[string]*layoutmgt.Layout{}}
	svc := newLayoutImportService(layoutSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: layoutImportContent(),
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "lay-123", resp.Results[0].ResourceID)
	require.Len(t, layoutSvc.created, 1)
	assert.Equal(t, "lay-123", layoutSvc.created[0].ID)
	assert.Empty(t, layoutSvc.updated)
}

// Upsert with an existing ID updates in place instead of creating.
func TestImportResources_LayoutUpsertUpdatesExisting(t *testing.T) {
	layoutSvc := &fakeLayoutService{
		byID:     map[string]*layoutmgt.Layout{"lay-123": {ID: "lay-123", Handle: "default-layout"}},
		byHandle: map[string]*layoutmgt.Layout{"default-layout": {ID: "lay-123", Handle: "default-layout"}},
	}
	svc := newLayoutImportService(layoutSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: layoutImportContent(),
		Options: &ImportOptions{Upsert: boolPtr(true)},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Equal(t, "lay-123", resp.Results[0].ResourceID)
	require.Len(t, layoutSvc.updated, 1)
	assert.Empty(t, layoutSvc.created)
}

// A non-not-found error from update surfaces as a failed update outcome without attempting a create.
//
//nolint:dupl // Parallel error-path tests differ only by the failing operation and error code.
func TestImportResources_LayoutUpsertUpdateError(t *testing.T) {
	layoutSvc := &fakeLayoutService{
		byID:     map[string]*layoutmgt.Layout{},
		byHandle: map[string]*layoutmgt.Layout{},
		updateErr: &tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "LAY-5000",
			Error: tidcommon.I18nMessage{DefaultValue: "update failed"},
		},
	}
	svc := newLayoutImportService(layoutSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: layoutImportContent(),
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true)},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Equal(t, "LAY-5000", resp.Results[0].Code)
	assert.Empty(t, layoutSvc.created)
}

// When update reports not-found but the fallback create fails, it surfaces as a failed create outcome.
//
//nolint:dupl // Parallel error-path tests differ only by the failing operation and error code.
func TestImportResources_LayoutUpsertCreateError(t *testing.T) {
	layoutSvc := &fakeLayoutService{
		byID:     map[string]*layoutmgt.Layout{},
		byHandle: map[string]*layoutmgt.Layout{},
		createErr: &tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "LAY-5001",
			Error: tidcommon.I18nMessage{DefaultValue: "create failed"},
		},
	}
	svc := newLayoutImportService(layoutSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: layoutImportContent(),
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true)},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "LAY-5001", resp.Results[0].Code)
	assert.Empty(t, layoutSvc.updated)
}

// Without upsert, a create that preserves the ID is issued directly.
func TestImportResources_LayoutCreatePreservesID(t *testing.T) {
	layoutSvc := &fakeLayoutService{byID: map[string]*layoutmgt.Layout{}, byHandle: map[string]*layoutmgt.Layout{}}
	svc := newLayoutImportService(layoutSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: layoutImportContent(),
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "lay-123", resp.Results[0].ResourceID)
	require.Len(t, layoutSvc.created, 1)
	assert.Equal(t, "lay-123", layoutSvc.created[0].ID)
}

// Without upsert, a create failure surfaces as a failed create outcome.
func TestImportResources_LayoutCreateError(t *testing.T) {
	layoutSvc := &fakeLayoutService{
		byID:     map[string]*layoutmgt.Layout{},
		byHandle: map[string]*layoutmgt.Layout{},
		createErr: &tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Code:  "LAY-5001",
			Error: tidcommon.I18nMessage{DefaultValue: "create failed"},
		},
	}
	svc := newLayoutImportService(layoutSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: layoutImportContent(),
		Options: &ImportOptions{Upsert: boolPtr(false), ContinueOnError: boolPtr(true)},
	})

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Equal(t, "LAY-5001", resp.Results[0].Code)
}

//nolint:dupl // Test pattern repeated across resource types to verify ID preservation behavior
func TestImportResources_EntityTypeUpsertCreatePreservesID(t *testing.T) {
	entityTypeSvc := &fakeEntityTypeService{
		byID:     map[string]*entitytype.EntityType{},
		byHandle: map[string]*entitytype.EntityType{},
	}
	svc := newImportService(
		nil, nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: user_type",
		"id: usrs-123",
		"handle: customer",
		"displayName: Customer",
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
	assert.Equal(t, "customer", entityTypeSvc.created[0].Handle)
	assert.Equal(t, "Customer", entityTypeSvc.created[0].DisplayName)
	assert.Equal(t, "Customer", resp.Results[0].ResourceName)
}

func TestImportResources_UpsertCreatePreservesIDsAcrossResourceTypes(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	themeSvc := &fakeThemeService{byID: map[string]*thememgt.Theme{}, byHandle: map[string]*thememgt.Theme{}}
	entityTypeSvc := &fakeEntityTypeService{
		byID:     map[string]*entitytype.EntityType{},
		byHandle: map[string]*entitytype.EntityType{},
	}
	flowSvc := &fakeFlowService{
		byID:  map[string]*providers.CompleteFlowDefinition{},
		byKey: map[string]*providers.CompleteFlowDefinition{},
	}

	svc := newImportService(
		nil, nil, nil, flowSvc, ouSvc, entityTypeSvc,
		nil, nil, nil, nil, themeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
		"handle: customer",
		"displayName: Customer",
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
		"ouId: ou-1",
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
		byID:     map[string]*entitytype.EntityType{},
		byHandle: map[string]*entitytype.EntityType{},
	}
	svc := newImportService(
		nil, nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: user_type",
		"id: usrs-123",
		"handle: customer",
		"displayName: Customer",
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

func TestImportResources_AgentTypeDefaultsToAgentCategory(t *testing.T) {
	entityTypeSvc := &fakeEntityTypeService{
		byID:     map[string]*entitytype.EntityType{},
		byHandle: map[string]*entitytype.EntityType{},
	}
	svc := newImportService(
		nil, nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: agent_type",
		"id: agtt-123",
		"handle: support-agent",
		"displayName: Support Agent",
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
	assert.Equal(t, "agtt-123", resp.Results[0].ResourceID)
	require.Len(t, entityTypeSvc.created, 1)
	assert.Equal(t, "agtt-123", entityTypeSvc.created[0].ID)
	require.Len(t, entityTypeSvc.createCategories, 1)
	assert.Equal(t, entitytype.TypeCategoryAgent, entityTypeSvc.createCategories[0])
}

func TestImportResources_EntityTypeExplicitCategory(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		id           string
		category     string
		want         entitytype.TypeCategory
	}{
		{
			name:         "agent type, explicit category takes precedence",
			resourceType: "agent_type", id: "agtt-124", category: "user",
			want: entitytype.TypeCategoryUser,
		},
		{
			name:         "user type, explicit agent category still works",
			resourceType: "user_type", id: "usrs-124", category: "agent",
			want: entitytype.TypeCategoryAgent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entityTypeSvc := &fakeEntityTypeService{
				byID:     map[string]*entitytype.EntityType{},
				byHandle: map[string]*entitytype.EntityType{},
			}
			svc := newImportService(
				nil, nil, nil, nil, nil, entityTypeSvc, nil, nil, nil, nil,
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

			content := strings.Join([]string{
				"resource_type: " + tt.resourceType,
				"id: " + tt.id,
				"handle: support-agent",
				"displayName: Support Agent",
				"category: " + tt.category,
				"ouId: ou-1",
				"schema:",
				"  type: object",
				"  properties: {}",
				"",
			}, "\n")

			resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

			require.Nil(t, err)
			require.Len(t, resp.Results, 1)
			assert.Equal(t, statusSuccess, resp.Results[0].Status)
			require.Len(t, entityTypeSvc.createCategories, 1)
			assert.Equal(t, tt.want, entityTypeSvc.createCategories[0])
		})
	}
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

func TestImportResources_CarriesOAuthTokenBindingFlags(t *testing.T) {
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
		"      grantTypes:",
		"        - authorization_code",
		"      tokenEndpointAuthMethod: client_secret_basic",
		"      dpopBoundAccessTokens: true",
		"      includeActClaim: true",
		"",
	}, "\n")
	appSvc, resp, err := runOAuthClientSecretImport(t, content)

	require.Nil(t, err)
	require.NotNil(t, resp)
	require.Len(t, appSvc.created, 1)
	require.Len(t, appSvc.created[0].InboundAuthConfig, 1)
	require.NotNil(t, appSvc.created[0].InboundAuthConfig[0].OAuthConfig)
	assert.True(t, appSvc.created[0].InboundAuthConfig[0].OAuthConfig.DPoPBoundAccessTokens)
	assert.True(t, appSvc.created[0].InboundAuthConfig[0].OAuthConfig.IncludeActClaim)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
}

func TestOrderDocumentsByDependencies(t *testing.T) {
	docs := []parsedDocument{
		{ResourceType: resourceTypeApplication, Sequence: 3},
		{ResourceType: resourceTypeFlow, Sequence: 2},
		{ResourceType: resourceTypeResourceServer, Sequence: 1},
		{ResourceType: resourceTypeConnection, Sequence: 0},
	}

	ordered := orderDocumentsByDependencies(docs)
	require.Len(t, ordered, 4)
	assert.Equal(t, resourceTypeConnection, ordered[0].ResourceType)
	assert.Equal(t, resourceTypeResourceServer, ordered[1].ResourceType)
	assert.Equal(t, resourceTypeFlow, ordered[2].ResourceType)
	assert.Equal(t, resourceTypeApplication, ordered[3].ResourceType)
}

func TestImportResources_FileTargetReturnsError(t *testing.T) {
	tempHome := t.TempDir()

	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime(tempHome, &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))

	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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

	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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

func TestImportResources_ApplicationTypePassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(
		appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"resource_type: application",
			"name: My App",
			"type: browser",
			"",
		}, "\n"),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, appSvc.created, 1)
	assert.Equal(t, model.ApplicationTypeBrowser, appSvc.created[0].Type)
}

func TestImportResources_ApplicationAttestationPassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(
		appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: strings.Join([]string{
			"resource_type: application",
			"name: My Mobile App",
			"type: mobile",
			"attestation:",
			"  devMode: true",
			"  android:",
			"    packageName: com.example.app",
			"    certificateSha256Digests:",
			"      - AA:BB:CC",
			"    serviceAccountCredentials: '{\"type\":\"service_account\"}'",
			"",
		}, "\n"),
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, appSvc.created, 1)

	attestation := appSvc.created[0].Attestation
	require.NotNil(t, attestation)
	assert.True(t, attestation.DevMode)
	require.NotNil(t, attestation.Android)
	assert.Equal(t, "com.example.app", attestation.Android.PackageName)
	assert.Equal(t, []string{"AA:BB:CC"}, attestation.Android.CertificateSha256Digests)
	assert.Equal(t, `{"type":"service_account"}`, attestation.Android.ServiceAccountCredentials)
}

func TestImportResources_ApplicationAuthFlowHandlePassedToService(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newImportService(
		appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		appSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	created  []*providers.Agent
	updated  []*agentmodel.UpdateAgentRequest
	existing map[string]*agentmodel.AgentGetResponse
}

func (f *fakeAgentService) CreateAgent(
	_ context.Context, req *providers.Agent,
) (*agentmodel.AgentCompleteResponse, *tidcommon.ServiceError) {
	id := testAgentAttributeName(req.Attributes) + "-id"
	f.created = append(f.created, req)
	if f.existing == nil {
		f.existing = map[string]*agentmodel.AgentGetResponse{}
	}
	f.existing[id] = &agentmodel.AgentGetResponse{ID: id, Attributes: req.Attributes}
	return &agentmodel.AgentCompleteResponse{ID: id, Attributes: req.Attributes}, nil
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
	f.existing[agentID] = &agentmodel.AgentGetResponse{ID: agentID, Attributes: req.Attributes}
	return &agentmodel.AgentCompleteResponse{ID: agentID, Attributes: req.Attributes}, nil
}

func testAgentAttributeName(attributes json.RawMessage) string {
	var attrs struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(attributes, &attrs)
	return attrs.Name
}

const testAgentLogo = "avatar:shape=circle,variant=anonymous_entity,content=bot_head,colors=0"

const agentYAML = "resource_type: agent\n" +
	"id: agent-1\ntype: default\nouId: root\nattributes:\n  name: Test Agent\ndescription: desc\n" +
	"logoUrl: \"" + testAgentLogo + "\"\n"

func TestImportAgent_Create(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: agentYAML})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Len(t, agentSvc.created, 1)
	assert.JSONEq(t, `{"name":"Test Agent"}`, string(agentSvc.created[0].Attributes))
	assert.Equal(t, testAgentLogo, agentSvc.created[0].LogoURL)
}

// TestImportAgent_StateIgnored verifies that a state in an agent document does not fail the import
// and is reported as ignored.
func TestImportAgent_StateIgnored(t *testing.T) {
	for _, tc := range stateDocuments(agentYAML) {
		t.Run(tc.name, func(t *testing.T) {
			agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
			svc := newImportService(
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

			var resp *ImportResponse
			var svcErr *tidcommon.ServiceError
			resp, svcErr = svc.ImportResources(context.Background(), &ImportRequest{Content: tc.content})

			require.Nil(t, svcErr)
			require.Len(t, resp.Results, 1)
			assert.Equal(t, statusSuccess, resp.Results[0].Status)
			require.Len(t, agentSvc.created, 1)
			assert.Equal(t, tc.warnings, strings.Count(resp.Results[0].Message, "State in the document was ignored"))
		})
	}
}

func TestImportAgent_CreateCarriesKind(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

	content := agentYAML + "owner: user-1\nkind: " + string(providers.AgentKindPersonalAssistant) + "\n"
	resp, svcErr := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, svcErr)
	require.NotNil(t, resp)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, agentSvc.created, 1)
	assert.Equal(t, providers.AgentKindPersonalAssistant, agentSvc.created[0].Kind)
	assert.Equal(t, "user-1", agentSvc.created[0].Owner)
}

func TestImportAgent_UpsertUpdate(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{
		"agent-1": {ID: "agent-1", Attributes: json.RawMessage(`{"name":"Test Agent"}`)},
	}}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
	assert.Equal(t, testAgentLogo, agentSvc.updated[0].LogoURL)
}

func TestImportAgent_UpsertFallbackCreate(t *testing.T) {
	agentSvc := &fakeAgentService{existing: map[string]*agentmodel.AgentGetResponse{}}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
		"agent-1": {ID: "agent-1", Attributes: json.RawMessage(`{"name":"Test Agent"}`)},
	}}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ctx context.Context, req *providers.Agent,
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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
			"agent-1": {ID: "agent-1", Attributes: json.RawMessage(`{"name":"Test Agent"}`)},
		}},
		updateErr: updateErr,
	}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

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
		getFlowID    func(*providers.Agent) string
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
			getFlowID:    func(r *providers.Agent) string { return r.AuthFlowID },
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
			getFlowID:    func(r *providers.Agent) string { return r.RegistrationFlowID },
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
				nil, nil, nil, flowSvc, nil, nil, nil, nil, nil, nil,
				nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

			content := strings.Join([]string{
				"resource_type: flow",
				"id: " + tc.flowID,
				"handle: " + tc.flowHandle,
				"name: " + tc.flowName,
				"flowType: " + tc.flowType,
				"ouId: ou-1",
				"nodes: []",
				"---",
				"resource_type: agent",
				"id: " + tc.agentID,
				"type: default",
				"ouId: root",
				"attributes:",
				"  name: " + tc.agentName,
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
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, agentSvc, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: agent",
		"id: agent-pub",
		"type: default",
		"ouId: root",
		"attributes:",
		"  name: Public Agent",
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
	// resources are the resources created under a server, in the order they were written.
	resources []providers.Resource
	// actions are the actions created under a server, in the order they were written, each with the
	// ID of the resource it was created under, empty for an action of the server itself.
	actions []importedAction
	// actionErrs are the errors CreateAction returns, by action handle.
	actionErrs map[string]*tidcommon.ServiceError
}

// importedAction is an action an import created, with the ID of the resource it was created under.
type importedAction struct {
	resourceID string
	action     providers.Action
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
	_ context.Context, _ string, res providers.Resource,
) (*providers.Resource, *tidcommon.ServiceError) {
	f.resources = append(f.resources, res)
	return &providers.Resource{ID: "id-" + res.Permission}, nil
}

func (f *fakeResourceServerService) GetResourceList(
	_ context.Context, _ string, _ *string, _, _ int,
) (*resource.ResourceList, *tidcommon.ServiceError) {
	return &resource.ResourceList{}, nil
}

func (f *fakeResourceServerService) CreateAction(
	_ context.Context, _ string, resourceID *string, action providers.Action,
) (*providers.Action, *tidcommon.ServiceError) {
	if svcErr, ok := f.actionErrs[action.Handle]; ok {
		return nil, svcErr
	}
	created := importedAction{action: action}
	if resourceID != nil {
		created.resourceID = *resourceID
	}
	f.actions = append(f.actions, created)
	return &providers.Action{}, nil
}

// TestImportRole_OUHandleResolved verifies that ou_handle on a role document is resolved
// to ou_id via the OU service before the role create request is built.
func TestImportRole_OUHandleResolved(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	roleSvc := &fakeRoleService{}
	roleAssignmentSvc := &fakeRoleAssignmentService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, roleSvc, roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	groupSvc := &fakeGroupService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	groupSvc := &fakeGroupService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	groupSvc := &fakeGroupService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, groupSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	userSvc := &fakeUserService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	userSvc := &fakeUserService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	userSvc := &fakeUserService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil, nil, nil)

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

// TestImportUser_StateIgnored verifies that a state in a user document does not fail the import, is
// not passed to the user service, and is reported as ignored.
func TestImportUser_StateIgnored(t *testing.T) {
	content := strings.Join([]string{
		"resource_type: user",
		"id: user-new",
		"type: person",
		"ouId: ou-explicit",
		"attributes:",
		"  username: alice",
		"",
	}, "\n")
	for _, tc := range stateDocuments(content) {
		t.Run(tc.name, func(t *testing.T) {
			userSvc := &fakeUserService{}
			svc := newImportService(
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil, nil, nil)

			var resp *ImportResponse
			var err *tidcommon.ServiceError
			resp, err = svc.ImportResources(context.Background(), &ImportRequest{
				Content: tc.content,
				Options: &ImportOptions{Upsert: boolPtr(false)},
			})

			require.Nil(t, err)
			require.Len(t, resp.Results, 1)
			assert.Equal(t, statusSuccess, resp.Results[0].Status)
			require.Len(t, userSvc.created, 1)
			assert.Nil(t, userSvc.created[0].Status)
			assert.NotContains(t, string(userSvc.created[0].Attributes), "SUSPENDED")
			assert.Equal(t, tc.warnings, strings.Count(resp.Results[0].Message, "State in the document was ignored"))
		})
	}
}

// TestDocumentState verifies which top-level state values the import warns about.
func TestDocumentState(t *testing.T) {
	cases := []struct {
		name    string
		content string
		state   string
		found   bool
	}{
		{"scalar", "resource_type: user\nstate: SUSPENDED\n", "SUSPENDED", true},
		{"mapping", "resource_type: agent\nstate:\n  value: SUSPENDED\n", "", true},
		{"empty", "resource_type: application\nstate: \"\"\n", "", false},
		{"absent", "resource_type: user\nid: user-1\n", "", false},
		{"nested only", "resource_type: user\nattributes:\n  state: SUSPENDED\n", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			docs, err := parseDocuments(tc.content)
			require.NoError(t, err)
			require.Len(t, docs, 1)

			state, found := documentState(docs[0])

			assert.Equal(t, tc.found, found)
			assert.Equal(t, tc.state, state)
		})
	}
}

// TestImportResourceServer_OUHandleResolved verifies that ou_handle on a resource server
// document is resolved to ou_id via the OU service before the create request is built.
func TestImportResourceServer_OUHandleResolved(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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

// TestParentsFirstEndsWithADuplicatePath verifies that two resources sharing a path are both ordered,
// and that the ordering ends rather than waiting for the second to be placed.
func TestParentsFirstEndsWithADuplicatePath(t *testing.T) {
	resources := []providers.Resource{
		{Handle: "orders", ParentHandle: "shop", Permission: "shop:orders"},
		{Handle: "shop", Permission: "shop"},
		{Handle: "orders", ParentHandle: "shop", Permission: "shop:orders"},
	}

	done := make(chan []providers.Resource, 1)
	go func() { done <- parentsFirst(resources) }()

	select {
	case ordered := <-done:
		require.Len(t, ordered, 3)
		assert.Equal(t, "shop", ordered[0].Handle)
		assert.Equal(t, "orders", ordered[1].Handle)
		assert.Equal(t, "orders", ordered[2].Handle)
	case <-time.After(2 * time.Second):
		t.Fatal("parentsFirst did not end with two resources sharing a path")
	}
}

// TestImportResourceServer_ChildBeforeParent verifies that a resource listed before its parent,
// as an export lists them newest first, is still created under that parent rather than at the top
// level: the parent is written first, whatever the document's order.
func TestImportResourceServer_ChildBeforeParent(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{
		"ou-default": {ID: "ou-default", Handle: "default"},
	}}
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: resource_server",
		"id: rs-new",
		"name: Test RS",
		"handle: test-rs",
		"identifier: test-rs",
		"ouHandle: default",
		"resources:",
		"  - name: Drafts",
		"    handle: drafts",
		"    parent: docs",
		"  - name: Documents",
		"    handle: docs",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, rsSvc.resources, 2)
	assert.Equal(t, "docs", rsSvc.resources[0].Handle)
	assert.Equal(t, "drafts", rsSvc.resources[1].Handle)
	require.NotNil(t, rsSvc.resources[1].Parent)
	assert.Equal(t, "id-docs", *rsSvc.resources[1].Parent)
}

// TestImportResourceServer_SameHandleUnderDifferentParents verifies that a repeated handle is
// imported under the parent its path names.
func TestImportResourceServer_SameHandleUnderDifferentParents(t *testing.T) {
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: resource_server",
		"id: rs-new",
		"name: Test RS",
		"identifier: test-rs",
		"ouId: ou-explicit",
		"resources:",
		"  - { name: Lead, handle: lead, parent: \"app:team:member\" }",
		"  - { name: App, handle: app }",
		"  - { name: Project, handle: project, parent: app }",
		"  - { name: Member, handle: member, parent: project }",
		"  - { name: Team, handle: team, parent: app }",
		"  - { name: Member, handle: member, parent: \"app:team\" }",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, rsSvc.resources, 6)

	parents := make(map[string]string, len(rsSvc.resources))
	for _, res := range rsSvc.resources {
		parent := ""
		if res.Parent != nil {
			parent = *res.Parent
		}
		parents[res.Permission] = parent
	}
	assert.Equal(t, map[string]string{
		"app":                  "",
		"app:project":          "id-app",
		"app:project:member":   "id-app:project",
		"app:team":             "id-app",
		"app:team:member":      "id-app:team",
		"app:team:member:lead": "id-app:team:member",
	}, parents)
}

// TestImportResourceServer_ServerActions verifies that the actions declared on a resource server
// itself are created at the server level, with or without resources, and that an action whose handle
// already exists is skipped.
func TestImportResourceServer_ServerActions(t *testing.T) {
	tests := []struct {
		name      string
		resources []string
		expected  []importedAction
	}{
		{
			name: "without resources",
			expected: []importedAction{
				{action: providers.Action{Name: "Read All", Handle: "read_all", Permission: "read_all"}},
			},
		},
		{
			name: "beside resources",
			resources: []string{
				"resources:",
				"  - name: Orders",
				"    handle: orders",
				"    actions:",
				"      - { name: Read, handle: read }",
			},
			expected: []importedAction{
				{action: providers.Action{Name: "Read All", Handle: "read_all", Permission: "read_all"}},
				{resourceID: "id-orders", action: providers.Action{
					Name: "Read", Handle: "read", Permission: "orders:read",
				}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rsSvc := &fakeResourceServerService{actionErrs: map[string]*tidcommon.ServiceError{
				"existing": &resource.ErrorHandleConflict,
			}}
			svc := newImportService(
				nil, nil, nil, nil, nil, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

			content := strings.Join(append([]string{
				"resource_type: resource_server",
				"id: rs-new",
				"name: Test RS",
				"identifier: test-rs",
				"ouId: ou-explicit",
				"actions:",
				"  - { name: Read All, handle: read_all }",
				"  - { name: Existing, handle: existing }",
			}, append(tc.resources, "")...), "\n")

			resp, err := svc.ImportResources(context.Background(), &ImportRequest{
				Content: content,
				Options: &ImportOptions{Upsert: boolPtr(false)},
			})

			require.Nil(t, err)
			require.Len(t, resp.Results, 1)
			assert.Equal(t, statusSuccess, resp.Results[0].Status, resp.Results[0].Message)
			assert.Equal(t, tc.expected, rsSvc.actions)
		})
	}
}

// TestImportResourceServer_ServerActionError verifies that a failure to create an action of the
// resource server itself fails the import of that resource server.
func TestImportResourceServer_ServerActionError(t *testing.T) {
	rsSvc := &fakeResourceServerService{actionErrs: map[string]*tidcommon.ServiceError{
		"read_all": {
			Type:  tidcommon.ClientErrorType,
			Code:  resource.ErrorInvalidRequestFormat.Code,
			Error: tidcommon.I18nMessage{DefaultValue: "invalid action"},
		},
	}}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: resource_server",
		"id: rs-new",
		"name: Test RS",
		"identifier: test-rs",
		"ouId: ou-explicit",
		"actions:",
		"  - { name: Read All, handle: read_all }",
		"",
	}, "\n")

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: content,
		Options: &ImportOptions{Upsert: boolPtr(false)},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, resource.ErrorInvalidRequestFormat.Code, resp.Results[0].Code)
	assert.Empty(t, rsSvc.resources)
}

// TestImportResourceServer_OUHandleNotFound verifies that an unknown ou_handle on a resource
// server document causes the import to fail with a clear error.
func TestImportResourceServer_OUHandleNotFound(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	rsSvc := &fakeResourceServerService{}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, rsSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

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
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
	}))

	idpSvc := &fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: connection",
		"name: google-idp",
		"type: google",
		"clientId: my-client-id",
		"clientSecret: my-client-secret",
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
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{Key: testCryptoKey}},
	}))

	existing := &providers.IDPDTO{ID: "idp-1", Name: "google-idp", Type: providers.IDPTypeGoogle}
	idpSvc := &fakeIDPService{
		byID:   map[string]*providers.IDPDTO{"idp-1": existing},
		byName: map[string]*providers.IDPDTO{"google-idp": existing},
	}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	content := strings.Join([]string{
		"resource_type: connection",
		"id: idp-1",
		"name: google-idp",
		"type: google",
		"clientId: updated-client-id",
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

type ImportNotificationTemplateTestSuite struct {
	suite.Suite
	mockAdapter *notificationTemplateAdapterMock
	svc         *importService
	ctx         context.Context
}

func TestImportNotificationTemplateTestSuite(t *testing.T) {
	suite.Run(t, new(ImportNotificationTemplateTestSuite))
}

func (s *ImportNotificationTemplateTestSuite) SetupTest() {
	s.mockAdapter = newNotificationTemplateAdapterMock(s.T())
	s.svc = s.newService(s.mockAdapter)
	s.ctx = context.Background()
}

// newService builds an importService wired only with the notification template adapter under test.
func (s *ImportNotificationTemplateTestSuite) newService(adapter notificationTemplateAdapter) *importService {
	imp, ok := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, adapter).(*importService)
	s.Require().True(ok)
	return imp
}

// expectValidateSucceeds stubs the validation the importer always runs before the handle lookup.
func (s *ImportNotificationTemplateTestSuite) expectValidateSucceeds() {
	s.mockAdapter.EXPECT().
		ValidateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail, mock.Anything).
		Return(nil).Once()
}

const emailTemplateDoc = `
resource_type: notification_template
channel: email
handle: user-invite
displayName: User Invitation Email
description: Invite.
content:
  subject: Welcome
  body: "<p>{{ctx(inviteLink)}}</p>"
`

func parseFirstDoc(t *testing.T) parsedDocument {
	t.Helper()
	docs, err := parseDocuments(emailTemplateDoc)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	return docs[0]
}

// emailTemplateDocWithOU carries an explicit ouId, used to exercise the immutable-OU rejection on the
// update path.
const emailTemplateDocWithOU = `
resource_type: notification_template
channel: email
handle: user-invite
ouId: ou-b
displayName: User Invitation Email
description: Invite.
content:
  subject: Welcome
  body: "<p>{{ctx(inviteLink)}}</p>"
`

func parseFirstDocWithOU(t *testing.T) parsedDocument {
	t.Helper()
	docs, err := parseDocuments(emailTemplateDocWithOU)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	return docs[0]
}

func (s *ImportNotificationTemplateTestSuite) TestCreate() {
	// Empty options default to upsert enabled, so the importer looks the handle up (a miss) before creating.
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(nil, &notificationtemplate.ErrorTemplateNotFound).Once()
	s.mockAdapter.EXPECT().CreateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail,
		mock.MatchedBy(func(req notificationtemplate.CreateTemplateRequest) bool {
			return req.Handle == "user-invite" &&
				req.Content.Subject == "Welcome" &&
				req.Content.Body == "<p>{{ctx(inviteLink)}}</p>"
		})).Return(&notificationtemplate.Template{ID: "new-user-invite", Handle: "user-invite"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{}, false)

	s.Equal(statusSuccess, outcome.Status)
	s.Equal(operationCreate, outcome.Operation)
}

func (s *ImportNotificationTemplateTestSuite) TestCreateKeepsTheDocumentID() {
	doc, err := parseDocuments(`
resource_type: notification_template
id: 01900000-0000-7000-8000-0000000000a0
channel: email
handle: user-invite
displayName: User Invitation Email
description: Invite.
content:
  subject: Welcome
  body: "<p>{{ctx(inviteLink)}}</p>"
`)
	s.Require().NoError(err)
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(nil, &notificationtemplate.ErrorTemplateNotFound).Once()
	s.mockAdapter.EXPECT().CreateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail,
		mock.MatchedBy(func(req notificationtemplate.CreateTemplateRequest) bool {
			return req.ID == "01900000-0000-7000-8000-0000000000a0"
		})).Return(&notificationtemplate.Template{ID: "01900000-0000-7000-8000-0000000000a0",
		Handle: "user-invite"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(s.ctx, doc[0], &ImportOptions{}, false)

	s.Equal(statusSuccess, outcome.Status)
	s.Equal("01900000-0000-7000-8000-0000000000a0", outcome.ResourceID)
}

func (s *ImportNotificationTemplateTestSuite) TestUpsertUpdatesExisting() {
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(&notificationtemplate.Template{ID: "existing-id", Handle: "user-invite"}, nil).Once()
	s.mockAdapter.EXPECT().
		UpdateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail, "existing-id", mock.Anything).
		Return(&notificationtemplate.Template{ID: "existing-id", Handle: "user-invite"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{Upsert: boolPtr(true)}, false)

	s.Equal(statusSuccess, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
}

func (s *ImportNotificationTemplateTestSuite) TestUpsertRejectsOUChange() {
	// The YAML moves the template to ou-b, but the stored template belongs to ou-a; ouId is immutable,
	// so the update is rejected rather than silently keeping ou-a.
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(&notificationtemplate.Template{ID: "existing-id", Handle: "user-invite", OUID: "ou-a"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDocWithOU(s.T()), &ImportOptions{Upsert: boolPtr(true)}, false)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
	s.Equal(notificationtemplate.ErrorOUImmutable.Code, outcome.Code)
}

func (s *ImportNotificationTemplateTestSuite) TestDryRunRejectsOUChange() {
	// A dry run predicts the same immutable-OU rejection the real apply would hit.
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(&notificationtemplate.Template{ID: "existing-id", Handle: "user-invite", OUID: "ou-a"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDocWithOU(s.T()), &ImportOptions{Upsert: boolPtr(true)}, true)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
	s.Equal(notificationtemplate.ErrorOUImmutable.Code, outcome.Code)
}

func (s *ImportNotificationTemplateTestSuite) TestConflictWithoutUpsertFails() {
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().CreateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail, mock.Anything).
		Return(nil, &notificationtemplate.ErrorTemplateHandleConflict).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{Upsert: boolPtr(false)}, false)

	s.Equal(statusFailed, outcome.Status)
}

func (s *ImportNotificationTemplateTestSuite) TestNoAdapter() {
	svc := s.newService(nil)

	outcome := svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{}, false)

	s.Equal(statusFailed, outcome.Status)
}

func (s *ImportNotificationTemplateTestSuite) TestDryRunReportsCreateWithoutWriting() {
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(nil, &notificationtemplate.ErrorTemplateNotFound).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{}, true)

	s.Equal(statusSuccess, outcome.Status)
	s.Equal(operationCreate, outcome.Operation)
}

func (s *ImportNotificationTemplateTestSuite) TestDryRunReportsUpdateForExisting() {
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(&notificationtemplate.Template{ID: "existing-id", Handle: "user-invite"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{Upsert: boolPtr(true)}, true)

	s.Equal(statusSuccess, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
}

func (s *ImportNotificationTemplateTestSuite) TestUpsertResolvesHandleAfterCreateConflict() {
	// The pre-create lookup misses, so the importer attempts a create. The injected conflict models a row
	// inserted concurrently; the retry lookup then resolves it and the importer falls back to an update.
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(nil, &notificationtemplate.ErrorTemplateNotFound).Once()
	s.mockAdapter.EXPECT().CreateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail, mock.Anything).
		Return(nil, &notificationtemplate.ErrorTemplateHandleConflict).Once()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(&notificationtemplate.Template{ID: "existing-user-invite", Handle: "user-invite"}, nil).Once()
	s.mockAdapter.EXPECT().
		UpdateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail, "existing-user-invite", mock.Anything).
		Return(&notificationtemplate.Template{ID: "existing-user-invite", Handle: "user-invite"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{Upsert: boolPtr(true)}, false)

	s.Equal(statusSuccess, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
}

func (s *ImportNotificationTemplateTestSuite) TestDryRunReportsConflictForExistingWithoutUpsert() {
	// The handle already exists and upsert is off, so the real apply would fail on the conflict; the
	// dry run must predict that failure rather than reporting a create success.
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(&notificationtemplate.Template{ID: "existing-id", Handle: "user-invite"}, nil).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{Upsert: boolPtr(false)}, true)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(notificationtemplate.ErrorTemplateHandleConflict.Code, outcome.Code)
}

func (s *ImportNotificationTemplateTestSuite) TestDryRunReportsValidationFailure() {
	// Validation that the real apply would run fails, so the dry run must report the failure.
	s.mockAdapter.EXPECT().
		ValidateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail, mock.Anything).
		Return(&notificationtemplate.ErrorMissingSubject).Once()

	outcome := s.svc.importNotificationTemplate(
		s.ctx, parseFirstDoc(s.T()), &ImportOptions{}, true)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(notificationtemplate.ErrorMissingSubject.Code, outcome.Code)
}

// TestImportResourcesCreatesTemplate drives a notification_template document through the public
// ImportResources entry point, covering the parse and dispatch wiring the direct
// importNotificationTemplate tests skip.
func (s *ImportNotificationTemplateTestSuite) TestImportResourcesCreatesTemplate() {
	s.expectValidateSucceeds()
	s.mockAdapter.EXPECT().
		GetTemplateByHandle(mock.Anything, notificationtemplate.ChannelTypeEmail, "user-invite").
		Return(nil, &notificationtemplate.ErrorTemplateNotFound).Once()
	s.mockAdapter.EXPECT().CreateTemplate(mock.Anything, notificationtemplate.ChannelTypeEmail,
		mock.MatchedBy(func(req notificationtemplate.CreateTemplateRequest) bool {
			return req.Handle == "user-invite"
		})).Return(&notificationtemplate.Template{ID: "new-user-invite", Handle: "user-invite"}, nil).Once()

	resp, err := s.svc.ImportResources(s.ctx, &ImportRequest{
		Content: emailTemplateDoc,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})

	s.Require().Nil(err)
	s.Require().NotNil(resp)
	s.Equal(1, resp.Summary.Imported)
	s.Require().Len(resp.Results, 1)
	s.Equal(statusSuccess, resp.Results[0].Status)
	s.Equal(operationCreate, resp.Results[0].Operation)
	s.Equal(resourceTypeNotificationTemplate, resp.Results[0].ResourceType)
}

// appDocWithPolicies is one application document carrying sharing policies.
func appDocWithPolicies(id, ouID string) string {
	return strings.Join([]string{
		"resource_type: application",
		"id: " + id,
		"name: App " + id,
		"type: m2m",
		"ouId: " + ouID,
		"sharingPolicies:",
		"  - childUnits:",
		"      mode: all",
		"",
	}, "\n")
}

// The importer applies the policies a document carries, the way it applies role assignments and
// resource server children: itself, after the resource is written.
func TestImportApplicationAppliesSharingPolicies(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	sharingSvc := &fakeSharingService{}
	svc := newTestImportServiceWithSharing(appSvc, sharingSvc)

	resp, err := svc.ImportResources(context.Background(),
		&ImportRequest{Content: appDocWithPolicies("app-1", "owner-ou")})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, sharingSvc.calls, 1, "the importer replayed the document's policies")
	assert.Equal(t, application.ApplicationSharingType, sharingSvc.calls[0].resourceType)
	assert.Equal(t, "app-1", sharingSvc.calls[0].resourceID)
	assert.Equal(t, "owner-ou", sharingSvc.calls[0].owningOUID)
	assert.Len(t, sharingSvc.calls[0].policies, 1)
}

// A document carrying no policies asks the sharing adapter nothing.
func TestImportApplicationWithoutPoliciesAsksTheAdapterNothing(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	sharingSvc := &fakeSharingService{}
	svc := newTestImportServiceWithSharing(appSvc, sharingSvc)

	content := strings.Join([]string{
		"resource_type: application", "id: app-2", "name: App Two", "type: m2m", "ouId: owner-ou", "",
	}, "\n")
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Empty(t, sharingSvc.calls)
}

// An update never moves an application between organization units, so the replay uses the stored
// owner. A document naming a different one would otherwise share it from an organization unit that
// does not own it.
func TestImportApplicationReplaysAgainstTheStoredOwner(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{
		"app-3": {ID: "app-3", Name: "App Three", OUID: "stored-owner-ou"},
	}}
	sharingSvc := &fakeSharingService{}
	svc := newTestImportServiceWithSharing(appSvc, sharingSvc)

	upsert := true
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: appDocWithPolicies("app-3", "an-ou-the-document-named"),
		Options: &ImportOptions{Upsert: &upsert},
	})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, sharingSvc.calls, 1)
	assert.Equal(t, "stored-owner-ou", sharingSvc.calls[0].owningOUID)
}

// A refused policy has to undo the create: the caller is told the document failed and gets no id
// back, so an application left behind would hold the name and client id a retry needs.
func TestImportApplicationUndoesTheCreateWhenPoliciesAreRefused(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	sharingSvc := &fakeSharingService{failure: &sharing.PolicyReplayError{
		Index: 1, InitiatingOUID: "child-ou", Err: &sharing.ErrorInvalidRequestFormat}}
	svc := newTestImportServiceWithSharing(appSvc, sharingSvc)

	resp, err := svc.ImportResources(context.Background(),
		&ImportRequest{Content: appDocWithPolicies("app-4", "owner-ou")})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, []string{"create:app-4", "delete:app-4"}, appSvc.sequence,
		"the application the refused document created was removed again")
}

// A document names its policies only by position, so the refusal says which one and whose it was.
func TestImportApplicationNamesTheRefusedPolicy(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	sharingSvc := &fakeSharingService{failure: &sharing.PolicyReplayError{
		Index: 1, InitiatingOUID: "child-ou", Err: &sharing.ErrorInvalidRequestFormat}}
	svc := newTestImportServiceWithSharing(appSvc, sharingSvc)

	resp, _ := svc.ImportResources(context.Background(),
		&ImportRequest{Content: appDocWithPolicies("app-5", "owner-ou")})

	require.Len(t, resp.Results, 1)
	assert.Equal(t, sharing.ErrorInvalidRequestFormat.Code, resp.Results[0].Code)
	assert.Contains(t, resp.Results[0].Message, "sharing policy 2 (organization unit child-ou)",
		"the result names the entry by position and issuing unit")
	assert.Contains(t, resp.Results[0].Message, sharing.ErrorInvalidRequestFormat.ErrorDescription.DefaultValue,
		"and carries why it was refused")
}

// A deployment with no sharing adapter says so rather than importing the document and dropping the
// policies it carries.
func TestImportApplicationRefusesPoliciesWithoutASharingAdapter(t *testing.T) {
	appSvc := &fakeApplicationService{existing: map[string]*providers.Application{}}
	svc := newTestImportService(appSvc)

	resp, _ := svc.ImportResources(context.Background(),
		&ImportRequest{Content: appDocWithPolicies("app-6", "owner-ou")})

	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Contains(t, resp.Results[0].Message, "sharing")
}

// userDocWithPolicies is a user document in ou-1 that carries one sharing policy.
func userDocWithPolicies(id string) string {
	return strings.Join([]string{
		"resource_type: user",
		"id: " + id,
		"type: person",
		"ouId: ou-1",
		"attributes:",
		"  username: " + id,
		"sharingPolicies:",
		"  - childUnits:",
		"      mode: all",
		"",
	}, "\n")
}

// newTestUserImportWithSharing is an import service with a user adapter and, when given, a sharing
// adapter installed the way Initialize installs the real one.
func newTestUserImportWithSharing(
	userSvc *fakeUserService, sharingSvc *fakeSharingService,
) ImportServiceInterface {
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, userSvc, nil, nil, nil, nil, nil, nil, nil)
	if sharingSvc != nil {
		svc.(*importService).sharingService = sharingSvc
	}
	return svc
}

// A user's sharing is replayed by the importer once the user exists, against the unit it lives in.
func TestImportUserAppliesSharingPolicies(t *testing.T) {
	userSvc := &fakeUserService{}
	sharingSvc := &fakeSharingService{}
	svc := newTestUserImportWithSharing(userSvc, sharingSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: userDocWithPolicies("user-1"), Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, sharingSvc.calls, 1, "the importer replayed the document's policies")
	assert.Equal(t, user.UserSharingType, sharingSvc.calls[0].resourceType)
	assert.Equal(t, "user-1", sharingSvc.calls[0].resourceID)
	assert.Equal(t, "ou-1", sharingSvc.calls[0].owningOUID)
	assert.Len(t, sharingSvc.calls[0].policies, 1)
}

// An update replays the document's policies too.
func TestImportUserReplaysPoliciesOnUpdate(t *testing.T) {
	sharingSvc := &fakeSharingService{}
	svc := newTestUserImportWithSharing(&fakeUserService{}, sharingSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: userDocWithPolicies("user-2"), Options: &ImportOptions{Upsert: boolPtr(true)}})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	require.Len(t, sharingSvc.calls, 1)
	assert.Equal(t, "user-2", sharingSvc.calls[0].resourceID)
}

// A document carrying no policies asks the sharing adapter nothing.
func TestImportUserWithoutPoliciesAsksTheAdapterNothing(t *testing.T) {
	sharingSvc := &fakeSharingService{}
	svc := newTestUserImportWithSharing(&fakeUserService{}, sharingSvc)

	content := strings.Join([]string{
		"resource_type: user", "id: user-3", "type: person", "ouId: ou-1", "attributes:", "  username: u3", "",
	}, "\n")
	resp, err := svc.ImportResources(context.Background(),
		&ImportRequest{Content: content, Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Empty(t, sharingSvc.calls)
}

// A refused policy undoes the create, so a retry does not meet a half-imported user.
func TestImportUserUndoesTheCreateWhenPoliciesAreRefused(t *testing.T) {
	userSvc := &fakeUserService{}
	sharingSvc := &fakeSharingService{failure: &sharing.PolicyReplayError{
		Index: 0, InitiatingOUID: "ou-1", Err: &sharing.ErrorInvalidRequestFormat}}
	svc := newTestUserImportWithSharing(userSvc, sharingSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: userDocWithPolicies("user-4"), Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Contains(t, resp.Results[0].Message, "sharing policy 1 (organization unit ou-1)")
	assert.Equal(t, []string{"user-4"}, userSvc.deleted, "the user the refused document created was removed")
}

// A deployment with no sharing adapter says so rather than importing the user and dropping its
// policies.
func TestImportUserRefusesPoliciesWithoutASharingAdapter(t *testing.T) {
	userSvc := &fakeUserService{}
	svc := newTestUserImportWithSharing(userSvc, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: userDocWithPolicies("user-5"), Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Contains(t, resp.Results[0].Message, "sharing")
	assert.Equal(t, []string{"user-5"}, userSvc.deleted)
}

type fakeAuthorizationDetailTypeService struct {
	existing    map[string]*providers.AuthorizationDetailType
	created     []providers.AuthorizationDetailType
	updated     []providers.AuthorizationDetailType
	updateErr   *tidcommon.ServiceError
	validateErr *tidcommon.ServiceError
	validated   []providers.AuthorizationDetailType
}

func (f *fakeAuthorizationDetailTypeService) DeleteAuthorizationDetailType(
	_ context.Context, id string,
) *tidcommon.ServiceError {
	delete(f.existing, id)
	return nil
}

func (f *fakeAuthorizationDetailTypeService) GetAuthorizationDetailTypeByValue(
	_ context.Context, value string,
) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	for _, t := range f.existing {
		if t.Type == value {
			return t, nil
		}
	}
	return nil, nil
}

func (f *fakeAuthorizationDetailTypeService) ValidateAuthorizationDetailType(
	_ context.Context, t providers.AuthorizationDetailType,
) *tidcommon.ServiceError {
	f.validated = append(f.validated, t)
	return f.validateErr
}

func (f *fakeAuthorizationDetailTypeService) CreateAuthorizationDetailType(
	_ context.Context, t providers.AuthorizationDetailType,
) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	f.created = append(f.created, t)
	return &t, nil
}

func (f *fakeAuthorizationDetailTypeService) GetAuthorizationDetailType(
	_ context.Context, id string,
) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	if t, ok := f.existing[id]; ok {
		return t, nil
	}
	return nil, &authzdetailmgt.ErrorTypeNotFound
}

func (f *fakeAuthorizationDetailTypeService) UpdateAuthorizationDetailType(
	ctx context.Context, id string, t providers.AuthorizationDetailType,
) (*providers.AuthorizationDetailType, *tidcommon.ServiceError) {
	if _, svcErr := f.GetAuthorizationDetailType(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.updated = append(f.updated, t)
	return &t, nil
}

// withAuthorizationDetailTypeStore initializes the server runtime with the given type store mode.
func withAuthorizationDetailTypeStore(t *testing.T, store string) {
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime("/tmp/test", &config.Config{
		AuthorizationDetailType: config.AuthorizationDetailTypeConfig{Store: store},
	}))
}

func newAuthorizationDetailTypeImportService(adtSvc authorizationDetailTypeAdapter) ImportServiceInterface {
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil)
	svc.(*importService).authorizationDetailTypeService = adtSvc
	return svc
}

const authorizationDetailTypeDocument = `resource_type: authorization_detail_type
id: adt-1
type: payment_initiation
name: Payment initiation
version: 2
schema: '{"type":"object","properties":{"amount":{"type":"string"}}}'
consentTemplate: Pay {{amount}}
processor:
  endpoint: https://bank.example.com/rar
reuse: false
`

func TestImportAuthorizationDetailType_CreatesWhenAbsent(t *testing.T) {
	adtSvc := &fakeAuthorizationDetailTypeService{}
	svc := newAuthorizationDetailTypeImportService(adtSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: authorizationDetailTypeDocument})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	require.Len(t, adtSvc.created, 1)
	created := adtSvc.created[0]
	assert.Equal(t, "adt-1", created.ID, "the type keeps the id it was exported with")
	assert.Equal(t, "Pay {{amount}}", created.ConsentTemplate, "consent placeholders survive template resolution")
	assert.Equal(t, "https://bank.example.com/rar", created.Processor.Endpoint)
	assert.False(t, created.Reuse)
	assert.JSONEq(t, `{"type":"object","properties":{"amount":{"type":"string"}}}`, string(created.Schema))
}

func TestImportAuthorizationDetailType_UpdatesWhenPresent(t *testing.T) {
	adtSvc := &fakeAuthorizationDetailTypeService{existing: map[string]*providers.AuthorizationDetailType{
		"adt-1": {ID: "adt-1", Type: "payment_initiation"},
	}}
	svc := newAuthorizationDetailTypeImportService(adtSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: authorizationDetailTypeDocument})

	require.Nil(t, err)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	require.Len(t, adtSvc.updated, 1)
	assert.Empty(t, adtSvc.created)

	resp, err = svc.ImportResources(context.Background(), &ImportRequest{
		Content: authorizationDetailTypeDocument, DryRun: true,
	})
	require.Nil(t, err)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Len(t, adtSvc.updated, 1, "a dry run changes nothing")
}

func TestImportAuthorizationDetailType_ReportsDeclarativeType(t *testing.T) {
	adtSvc := &fakeAuthorizationDetailTypeService{
		existing: map[string]*providers.AuthorizationDetailType{
			"adt-1": {ID: "adt-1", IsReadOnly: true},
		},
		updateErr: &authzdetailmgt.ErrorImmutableType,
	}
	svc := newAuthorizationDetailTypeImportService(adtSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: authorizationDetailTypeDocument})

	require.Nil(t, err)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, authzdetailmgt.ErrorImmutableType.Code, resp.Results[0].Code)
	assert.Empty(t, adtSvc.created)
}

func TestImportAuthorizationDetailType_DryRunCreateAndFailures(t *testing.T) {
	withAuthorizationDetailTypeStore(t, "")
	adtSvc := &fakeAuthorizationDetailTypeService{}
	svc := newAuthorizationDetailTypeImportService(adtSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: authorizationDetailTypeDocument, DryRun: true,
	})
	require.Nil(t, err)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	assert.Empty(t, adtSvc.created)

	resp, err = svc.ImportResources(context.Background(), &ImportRequest{
		Content: "resource_type: authorization_detail_type\nid: adt-1\nname: Bad\nschema: 'not json'\n",
	})
	require.Nil(t, err)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, ErrorInvalidYAMLContent.Code, resp.Results[0].Code)

	resp, err = newAuthorizationDetailTypeImportService(nil).ImportResources(context.Background(),
		&ImportRequest{Content: authorizationDetailTypeDocument})
	require.Nil(t, err)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
}

func TestImportAuthorizationDetailType_DeletesAType(t *testing.T) {
	adtSvc := existingTypes(&providers.AuthorizationDetailType{ID: "adt-1", Type: "payment_initiation"},
		&providers.AuthorizationDetailType{ID: "adt-2", Type: "other"})
	svc := newAuthorizationDetailTypeImportService(adtSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeAuthorizationDetailType, ID: "adt-1"}},
	})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, 1, resp.Summary.Deleted)
	assert.NotContains(t, adtSvc.existing, "adt-1")
	assert.Contains(t, adtSvc.existing, "adt-2", "unrelated types must be left alone")
}

// existingTypes returns an authorization details type service holding the given types.
func existingTypes(types ...*providers.AuthorizationDetailType) *fakeAuthorizationDetailTypeService {
	existing := make(map[string]*providers.AuthorizationDetailType, len(types))
	for _, t := range types {
		existing[t.ID] = t
	}
	return &fakeAuthorizationDetailTypeService{existing: existing}
}

// A dry run refuses what the real import would, reporting the operation that import would perform.
func TestImportAuthorizationDetailType_DryRunPredictsRefusals(t *testing.T) {
	invalidUpdate := existingTypes(&providers.AuthorizationDetailType{ID: "adt-1", Type: "payment_initiation"})
	invalidUpdate.validateErr = &authzdetailmgt.ErrorInvalidSchema
	cases := map[string]struct {
		service   *fakeAuthorizationDetailTypeService
		options   *ImportOptions
		store     string
		code      string
		operation string
	}{
		"invalid type": {&fakeAuthorizationDetailTypeService{validateErr: &authzdetailmgt.ErrorInvalidSchema}, nil,
			"", authzdetailmgt.ErrorInvalidSchema.Code, operationCreate},
		"invalid update": {invalidUpdate, nil, "", authzdetailmgt.ErrorInvalidSchema.Code, operationUpdate},
		"type value taken": {existingTypes(&providers.AuthorizationDetailType{ID: "adt-other",
			Type: "payment_initiation"}), nil, "", authzdetailmgt.ErrorTypeConflict.Code, operationCreate},
		"id taken without upsert": {existingTypes(&providers.AuthorizationDetailType{ID: "adt-1", Type: "other"}),
			&ImportOptions{Upsert: boolPtr(false)}, "", authzdetailmgt.ErrorTypeIDConflict.Code, operationCreate},
		"declarative type": {existingTypes(&providers.AuthorizationDetailType{ID: "adt-1",
			Type: "payment_initiation", IsReadOnly: true}), nil, "", authzdetailmgt.ErrorImmutableType.Code,
			operationUpdate},
		"type value changed": {existingTypes(&providers.AuthorizationDetailType{ID: "adt-1", Type: "other"}), nil,
			"", authzdetailmgt.ErrorTypeValueImmutable.Code, operationUpdate},
		"create in declarative-only mode": {&fakeAuthorizationDetailTypeService{}, nil, "declarative",
			authzdetailmgt.ErrorDeclarativeModeCreateNotAllowed.Code, operationCreate},
	}
	for name, tc := range cases {
		withAuthorizationDetailTypeStore(t, tc.store)
		resp, err := newAuthorizationDetailTypeImportService(tc.service).ImportResources(context.Background(),
			&ImportRequest{Content: authorizationDetailTypeDocument, DryRun: true, Options: tc.options})
		require.Nil(t, err, name)
		assert.Equal(t, statusFailed, resp.Results[0].Status, name)
		assert.Equal(t, tc.code, resp.Results[0].Code, name)
		assert.Equal(t, tc.operation, resp.Results[0].Operation, name)
		assert.Empty(t, tc.service.created, name)
		assert.Empty(t, tc.service.updated, name)
	}
}

// A dry run create checks the credential as sent, not against a stored type that holds the id, as the
// real create does.
func TestImportAuthorizationDetailType_DryRunCreateIgnoresTheStoredType(t *testing.T) {
	withAuthorizationDetailTypeStore(t, "")
	adtSvc := existingTypes(&providers.AuthorizationDetailType{ID: "adt-1", Type: "other"})

	resp, err := newAuthorizationDetailTypeImportService(adtSvc).ImportResources(context.Background(),
		&ImportRequest{Content: authorizationDetailTypeDocument, DryRun: true,
			Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Nil(t, err)
	assert.Equal(t, authzdetailmgt.ErrorTypeIDConflict.Code, resp.Results[0].Code)
	require.Len(t, adtSvc.validated, 1)
	assert.Empty(t, adtSvc.validated[0].ID)
}

// An id wider than the store's ID column is refused before anything is written.
func TestImportAuthorizationDetailType_RefusesAnOverlongID(t *testing.T) {
	adtSvc := &fakeAuthorizationDetailTypeService{}
	content := strings.Replace(authorizationDetailTypeDocument, "id: adt-1", "id: "+strings.Repeat("a", 256), 1)

	for _, dryRun := range []bool{false, true} {
		resp, err := newAuthorizationDetailTypeImportService(adtSvc).ImportResources(context.Background(),
			&ImportRequest{Content: content, DryRun: dryRun})
		require.Nil(t, err)
		assert.Equal(t, statusFailed, resp.Results[0].Status)
		assert.Equal(t, ErrorInvalidYAMLContent.Code, resp.Results[0].Code)
	}
	assert.Empty(t, adtSvc.created)
}

func TestDeleteResource_RemovesAuthorizationDetailTypeFile(t *testing.T) {
	tempHome := t.TempDir()
	config.ResetServerRuntime()
	t.Cleanup(config.ResetServerRuntime)
	require.NoError(t, config.InitializeServerRuntime(tempHome, &config.Config{}))

	resourceDir := filepath.Join(tempHome, "config", "resources", "authorization_detail_types")
	require.NoError(t, os.MkdirAll(resourceDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(resourceDir, "payment.yaml"),
		[]byte(authorizationDetailTypeDocument), 0o600))

	resp, svcErr := newAuthorizationDetailTypeImportService(nil).DeleteResource(context.Background(),
		&DeleteResourceRequest{ResourceType: resourceTypeAuthorizationDetailType, ResourceKey: "adt-1"})

	require.Nil(t, svcErr)
	assert.Equal(t, "payment.yaml", resp.DeletedFile)
	_, statErr := os.Stat(filepath.Join(resourceDir, "payment.yaml"))
	assert.True(t, os.IsNotExist(statErr))
}

// connectionKind describes how the importer drives the service of one kind of connection, so a scenario runs against
// each kind.
type connectionKind struct {
	name string
	// doc is a connection document of the kind with the given id, naming its owner by the given lines.
	doc func(id, owner string) string
	// service returns the mock of the service of the kind, and the methods the importer calls on it.
	service                                func() *mock.Mock
	resolveMethod, createMethod, getMethod string
	// created is what the service answers a create call with, and stored what it answers a get call with for a
	// connection owned by the given organization unit.
	created any
	stored  func(ouID string) any
	// updateNotAllowedCode is the code of the error the service answers an update to another owner with.
	updateNotAllowedCode string
}

// ownerCall is a call to the service to resolve the owner of a connection document.
type ownerCall struct {
	// wantOUID and wantHandle are the owner the document names, which the importer hands to the service.
	wantOUID, wantHandle string
	// setOUID is the organization unit the service resolves the owner to, when it is not empty.
	setOUID string
	// svcErr is the error the service answers with.
	svcErr *tidcommon.ServiceError
}

type ImportConnectionOUTestSuite struct {
	suite.Suite
	idpService    *idpAdapterMock
	senderService *senderAdapterMock
	pdpService    *authZENPDPAdapterMock
	svc           ImportServiceInterface
}

func TestImportConnectionOUTestSuite(t *testing.T) {
	suite.Run(t, new(ImportConnectionOUTestSuite))
}

func (s *ImportConnectionOUTestSuite) SetupTest() {
	s.idpService = newIdpAdapterMock(s.T())
	s.senderService = newSenderAdapterMock(s.T())
	s.pdpService = newAuthZENPDPAdapterMock(s.T())
	s.svc = newImportService(
		nil, s.idpService, s.senderService, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, s.pdpService)
}

// ownerFields returns the pointers to the ouId and the ouHandle of the connection a resolve call is made with, which
// every kind of connection holds in fields of those names.
func ownerFields(arg any) (ouID, ouHandle *string) {
	connection := reflect.ValueOf(arg).Elem()
	return connection.FieldByName("OUID").Addr().Interface().(*string),
		connection.FieldByName("OUHandle").Addr().Interface().(*string)
}

// createdOU returns the organization unit the argument of a create call names, which is a connection or a request
// that holds it in a field named OUID.
func createdOU(arg any) string {
	return reflect.Indirect(reflect.ValueOf(arg)).FieldByName("OUID").String()
}

// expectResolve expects the service of the kind to be asked to resolve the owner of the document.
func (s *ImportConnectionOUTestSuite) expectResolve(kind connectionKind, call ownerCall) {
	kind.service().On(kind.resolveMethod, mock.Anything, mock.MatchedBy(func(arg any) bool {
		ouID, ouHandle := ownerFields(arg)
		return *ouID == call.wantOUID && *ouHandle == call.wantHandle
	})).Run(func(args mock.Arguments) {
		if call.setOUID != "" {
			ouID, _ := ownerFields(args.Get(1))
			*ouID = call.setOUID
		}
	}).Return(call.svcErr).Once()
}

// expectCreate expects the connection to be created under the given organization unit.
func (s *ImportConnectionOUTestSuite) expectCreate(kind connectionKind, ouID string) {
	kind.service().On(kind.createMethod, mock.Anything, mock.MatchedBy(func(arg any) bool {
		return createdOU(arg) == ouID
	})).Return(kind.created, (*tidcommon.ServiceError)(nil)).Once()
}

// expectExisting expects the service to hold the connection of the document, owned by the given organization unit.
func (s *ImportConnectionOUTestSuite) expectExisting(kind connectionKind, ouID string) {
	kind.service().On(kind.getMethod, mock.Anything, "conn-1").
		Return(kind.stored(ouID), (*tidcommon.ServiceError)(nil)).Once()
}

// kinds returns the three kinds of connection the importer drives, each over the suite's mocks.
func (s *ImportConnectionOUTestSuite) kinds() []connectionKind {
	return []connectionKind{
		{
			name: "identity provider",
			doc: func(id, owner string) string {
				return "resource_type: connection\n" + idLine(id) + "name: idp-one\ntype: google\n" + owner +
					"clientId: abc\n"
			},
			service:       func() *mock.Mock { return &s.idpService.Mock },
			resolveMethod: "ResolveIdentityProviderOU", createMethod: "CreateIdentityProvider",
			getMethod: "GetIdentityProvider",
			created:   &providers.IDPDTO{ID: "idp-new", Name: "idp-one"},
			stored: func(ouID string) any {
				return &providers.IDPDTO{ID: "conn-1", Name: "idp-one", OUID: ouID}
			},
			updateNotAllowedCode: idp.ErrorIDPOUUpdateNotAllowed.Code,
		},
		{
			name: "notification sender",
			doc: func(id, owner string) string {
				return "resource_type: connection\n" + idLine(id) + "name: sms\ntype: twilio\n" + owner +
					"accountSid: AC1\nauthToken: t\nsenderId: x\n"
			},
			service:       func() *mock.Mock { return &s.senderService.Mock },
			resolveMethod: "ResolveSenderOU", createMethod: "CreateSender", getMethod: "GetSender",
			created: &ncommon.NotificationSenderDTO{ID: "sender-new", Name: "sms"},
			stored: func(ouID string) any {
				return &ncommon.NotificationSenderDTO{ID: "conn-1", Name: "sms", OUID: ouID}
			},
			updateNotAllowedCode: notification.ErrorSenderOUUpdateNotAllowed.Code,
		},
		{
			name: "AuthZEN PDP",
			doc: func(id, owner string) string {
				return "resource_type: connection\n" + idLine(id) + "name: pdp\ntype: authzen-pdp\n" + owner +
					"endpoint: https://pdp.example.com\n"
			},
			service:       func() *mock.Mock { return &s.pdpService.Mock },
			resolveMethod: "ResolveConnectionOU", createMethod: "CreateAuthZENPDPConnection",
			getMethod: "GetAuthZENPDP",
			created:   &authzenpdp.AuthZENPDPConnection{ID: "pdp-new", Name: "pdp"},
			stored: func(ouID string) any {
				return &authzenpdp.AuthZENPDPConnection{ID: "conn-1", Name: "pdp", OUID: ouID}
			},
			updateNotAllowedCode: authzenpdp.ErrorOUUpdateNotAllowed.Code,
		},
	}
}

// idLine is the id line of a connection document, which is empty for a document that has no id.
func idLine(id string) string {
	if id == "" {
		return ""
	}
	return "id: " + id + "\n"
}

// importOne imports a document that holds one connection and returns its outcome.
func (s *ImportConnectionOUTestSuite) importOne(content string, dryRun bool) ImportItemOutcome {
	resp, err := s.svc.ImportResources(context.Background(), &ImportRequest{Content: content, DryRun: dryRun})
	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	return resp.Results[0]
}

func (s *ImportConnectionOUTestSuite) TestOwnerIsResolvedByTheServiceOfTheKind() {
	for _, kind := range s.kinds() {
		s.Run(kind.name, func() {
			s.SetupTest()
			s.expectResolve(kind, ownerCall{wantHandle: "default", setOUID: "ou-default"})
			s.expectCreate(kind, "ou-default")

			outcome := s.importOne(kind.doc("", "ouHandle: default\n"), false)

			s.Equal(statusSuccess, outcome.Status)
			s.Equal(operationCreate, outcome.Operation)
		})
	}
}

func (s *ImportConnectionOUTestSuite) TestOUIDAndHandleAreBothHandedToTheService() {
	for _, kind := range s.kinds() {
		s.Run(kind.name, func() {
			s.SetupTest()
			s.expectResolve(kind, ownerCall{wantOUID: "ou-other", wantHandle: "default"})
			s.expectCreate(kind, "ou-other")

			outcome := s.importOne(kind.doc("", "ouId: ou-other\nouHandle: default\n"), false)

			s.Equal(statusSuccess, outcome.Status)
		})
	}
}

func (s *ImportConnectionOUTestSuite) TestOwnerTheServiceCannotResolveFailsTheItem() {
	for _, kind := range s.kinds() {
		for _, dryRun := range []bool{false, true} {
			s.Run(fmt.Sprintf("%s dry run %t", kind.name, dryRun), func() {
				s.SetupTest()
				s.expectResolve(kind, ownerCall{wantHandle: "missing", svcErr: &authzenpdp.ErrorInvalidOU})

				outcome := s.importOne(kind.doc("", "ouHandle: missing\n"), dryRun)

				s.Equal(statusFailed, outcome.Status)
				s.Equal(authzenpdp.ErrorInvalidOU.Code, outcome.Code)
			})
		}
	}
}

func (s *ImportConnectionOUTestSuite) TestDryRunRefusesToMoveAnExistingConnection() {
	for _, kind := range s.kinds() {
		s.Run(kind.name, func() {
			s.SetupTest()
			s.expectResolve(kind, ownerCall{wantOUID: "ou-2"})
			s.expectExisting(kind, "ou-1")

			outcome := s.importOne(kind.doc("conn-1", "ouId: ou-2\n"), true)

			s.Equal(statusFailed, outcome.Status)
			s.Equal(kind.updateNotAllowedCode, outcome.Code)
		})
	}
}

func (s *ImportConnectionOUTestSuite) TestDryRunAcceptsTheOwnerAnExistingConnectionHas() {
	for _, kind := range s.kinds() {
		s.Run(kind.name, func() {
			s.SetupTest()
			s.expectResolve(kind, ownerCall{wantOUID: "ou-1"})
			s.expectExisting(kind, "ou-1")

			outcome := s.importOne(kind.doc("conn-1", "ouId: ou-1\n"), true)

			s.Equal(statusSuccess, outcome.Status)
			s.Equal(operationUpdate, outcome.Operation)
		})
	}
}

const httpConnectionImportDoc = `resource_type: connection
id: http-1
type: http
name: Risk Engine
url: %s
timeoutMs: 5000
authentication:
  type: basic
  properties:
    username: svc-user
%s`

const (
	httpImportURL          = "https://risk.example.com/evaluate"
	httpImportPasswordLine = "    password: doc-pw\n"
)

type ImportHTTPConnectionTestSuite struct {
	suite.Suite
	adapter *httpConnectionAdapterMock
	svc     ImportServiceInterface
}

func TestImportHTTPConnectionTestSuite(t *testing.T) {
	suite.Run(t, new(ImportHTTPConnectionTestSuite))
}

func (s *ImportHTTPConnectionTestSuite) SetupTest() {
	s.adapter = newHttpConnectionAdapterMock(s.T())
	s.svc = s.newService(nil, nil, s.adapter)
}

// newService builds an import service over the connection adapters an HTTP connection test reaches.
func (s *ImportHTTPConnectionTestSuite) newService(idpSvc idpAdapter, senderSvc senderAdapter,
	httpSvc httpConnectionAdapter) ImportServiceInterface {
	svc := newImportService(newAppService(), idpSvc, senderSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil)
	svc.(*importService).httpConnectionService = httpSvc
	return svc
}

func (s *ImportHTTPConnectionTestSuite) importHTTPConnection(url, passwordLine string, dryRun bool) ImportItemOutcome {
	s.T().Helper()
	s.adapter.EXPECT().ResolveConnectionOU(mock.Anything, mock.Anything).Return(nil).Maybe()
	resp, err := s.svc.ImportResources(context.Background(), &ImportRequest{
		Content: fmt.Sprintf(httpConnectionImportDoc, url, passwordLine),
		DryRun:  dryRun,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})
	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	return resp.Results[0]
}

func (s *ImportHTTPConnectionTestSuite) storedPassword(props []cmodels.Property) string {
	s.T().Helper()
	authConfig, err := outboundauth.FromProperties(props)
	s.Require().NoError(err)
	return authConfig.Get(outboundauth.FieldBasicPassword)
}

func (s *ImportHTTPConnectionTestSuite) stored() *httpconnection.HTTPConnection {
	authProps, err := outboundauth.ToProperties(outboundauth.Config{Type: outboundauth.TypeBasic,
		Properties: map[string]string{
			outboundauth.FieldBasicUsername: "svc-user",
			outboundauth.FieldBasicPassword: "stored-pw",
		}})
	s.Require().NoError(err)
	return &httpconnection.HTTPConnection{ID: "http-1", Name: "Risk Engine", URL: httpImportURL,
		AuthProperties: authProps}
}

// httpSharingImportDoc is an HTTP connection document that names its owner by handle only and carries one
// sharing policy.
const httpSharingImportDoc = `resource_type: connection
id: http-1
type: http
name: Risk Engine
url: https://risk.example.com/evaluate
ouHandle: root
sharingPolicies:
  - childUnits:
      mode: all
`

func (s *ImportHTTPConnectionTestSuite) importSharedHTTPConnection(sharingSvc *fakeSharingService) ImportItemOutcome {
	s.T().Helper()
	s.svc.(*importService).sharingService = sharingSvc
	resp, err := s.svc.ImportResources(context.Background(), &ImportRequest{
		Content: httpSharingImportDoc,
		Options: &ImportOptions{Upsert: boolPtr(false), Target: importTargetRuntime},
	})
	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	return resp.Results[0]
}

// A document that names its owner by handle is shared against the unit the handle resolves to.
func (s *ImportHTTPConnectionTestSuite) TestSharingIsReplayedAgainstTheResolvedOwner() {
	sharingSvc := &fakeSharingService{}
	s.adapter.EXPECT().ResolveConnectionOU(mock.Anything, mock.Anything).
		Run(func(_ context.Context, connection *httpconnection.HTTPConnection) { connection.OUID = "ou-root" }).
		Return(nil)
	s.adapter.EXPECT().CreateHTTPConnection(mock.Anything, mock.Anything).
		Return(&httpconnection.HTTPConnection{ID: "http-1", Name: "Risk Engine"}, nil)

	outcome := s.importSharedHTTPConnection(sharingSvc)

	s.Equal(statusSuccess, outcome.Status, outcome.Message)
	s.Require().Len(sharingSvc.calls, 1)
	s.Equal(connection.ConnectionSharingType, sharingSvc.calls[0].resourceType)
	s.Equal("http-1", sharingSvc.calls[0].resourceID)
	s.Equal("ou-root", sharingSvc.calls[0].owningOUID)
	s.Len(sharingSvc.calls[0].policies, 1)
}

// A refused policy fails the document, and a connection the import just created is removed.
func (s *ImportHTTPConnectionTestSuite) TestRefusedPoliciesRemoveTheCreatedConnection() {
	sharingSvc := &fakeSharingService{failure: &sharing.PolicyReplayError{
		Index: 0, InitiatingOUID: "ou-root", Err: &sharing.ErrorPolicyExists}}
	s.adapter.EXPECT().ResolveConnectionOU(mock.Anything, mock.Anything).
		Run(func(_ context.Context, connection *httpconnection.HTTPConnection) { connection.OUID = "ou-root" }).
		Return(nil)
	s.adapter.EXPECT().CreateHTTPConnection(mock.Anything, mock.Anything).
		Return(&httpconnection.HTTPConnection{ID: "http-1", Name: "Risk Engine"}, nil)
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").
		Return(&httpconnection.HTTPConnection{ID: "http-1"}, nil)
	s.adapter.EXPECT().DeleteHTTPConnection(mock.Anything, "http-1").Return(nil)

	outcome := s.importSharedHTTPConnection(sharingSvc)

	s.Equal(statusFailed, outcome.Status)
}

// An owner that cannot be resolved fails the document before anything is written.
func (s *ImportHTTPConnectionTestSuite) TestAnUnresolvableOwnerWritesNothing() {
	sharingSvc := &fakeSharingService{}
	s.adapter.EXPECT().ResolveConnectionOU(mock.Anything, mock.Anything).
		Return(&httpconnection.ErrorInvalidOU)

	outcome := s.importSharedHTTPConnection(sharingSvc)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(httpconnection.ErrorInvalidOU.Code, outcome.Code)
	s.Empty(sharingSvc.calls)
}

// Flows reference a connection by ID, so an imported one must keep the document's.
func (s *ImportHTTPConnectionTestSuite) TestCreateKeepsTheDocumentID() {
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").Return(nil, &httpconnection.ErrorNotFound)
	var created httpconnection.HTTPConnection
	s.adapter.EXPECT().CreateHTTPConnection(mock.Anything, mock.Anything).
		Run(func(_ context.Context, connection httpconnection.HTTPConnection) { created = connection }).
		Return(&httpconnection.HTTPConnection{ID: "http-1", Name: "Risk Engine"}, nil)

	outcome := s.importHTTPConnection(httpImportURL, httpImportPasswordLine, false)

	s.Equal(statusSuccess, outcome.Status, outcome.Message)
	s.Equal(operationCreate, outcome.Operation)
	s.Equal("http-1", created.ID)
	s.NotContains(fmt.Sprint(created.AuthProperties), "doc-pw")
	s.Equal("doc-pw", s.storedPassword(created.AuthProperties))
}

// An upsert hands the writer the connection it read, so the retention rule is applied against it.
func (s *ImportHTTPConnectionTestSuite) TestAnUpsertUpdatesTheConnectionItRead() {
	stored := s.stored()
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").Return(stored, nil)
	var updated httpconnection.HTTPConnection
	s.adapter.EXPECT().UpdateHTTPConnection(mock.Anything, stored, mock.Anything).
		Run(func(_ context.Context, _ *httpconnection.HTTPConnection, connection httpconnection.HTTPConnection) {
			updated = connection
		}).Return(stored, nil)

	outcome := s.importHTTPConnection("https://risk.example.com/v2", "", false)

	s.Equal(statusSuccess, outcome.Status, outcome.Message)
	s.Equal(operationUpdate, outcome.Operation)
	s.Equal("https://risk.example.com/v2", updated.URL)
	s.Empty(s.storedPassword(updated.AuthProperties), "the document's omission is the writer's to resolve")
}

func (s *ImportHTTPConnectionTestSuite) TestDryRunWritesNothing() {
	stored := s.stored()
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").Return(stored, nil)
	s.adapter.EXPECT().ValidateHTTPConnectionUpdate(mock.Anything, stored, mock.Anything).Return(nil)

	outcome := s.importHTTPConnection(httpImportURL, httpImportPasswordLine, true)

	s.Equal(statusSuccess, outcome.Status, outcome.Message)
	s.Equal(operationUpdate, outcome.Operation)
}

func (s *ImportHTTPConnectionTestSuite) TestADryRunUpsertReportsTheUpdateRefusal() {
	stored := s.stored()
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").Return(stored, nil)
	s.adapter.EXPECT().ValidateHTTPConnectionUpdate(mock.Anything, stored, mock.Anything).
		Return(&declarativeresource.ErrorDeclarativeResourceUpdateOperation)

	outcome := s.importHTTPConnection(httpImportURL, httpImportPasswordLine, true)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
	s.Equal(declarativeresource.ErrorDeclarativeResourceUpdateOperation.Code, outcome.Code)
}

func (s *ImportHTTPConnectionTestSuite) TestADryRunCreateReportsTheValidationRefusal() {
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").Return(nil, &httpconnection.ErrorNotFound)
	s.adapter.EXPECT().ValidateHTTPConnectionCreate(mock.Anything, mock.Anything).
		Return(&httpconnection.ErrorInvalidURL)

	outcome := s.importHTTPConnection("http://risk.example.com/evaluate", httpImportPasswordLine, true)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(operationCreate, outcome.Operation)
	s.Equal(httpconnection.ErrorInvalidURL.Code, outcome.Code)
}

func (s *ImportHTTPConnectionTestSuite) TestFailsWithoutTheAdapter() {
	s.svc = s.newService(nil, nil, nil)

	outcome := s.importHTTPConnection(httpImportURL, httpImportPasswordLine, false)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(ErrorInvalidImportRequest.Code, outcome.Code)
}

func (s *ImportHTTPConnectionTestSuite) TestReportsTheServiceRefusal() {
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").Return(nil, &httpconnection.ErrorNotFound)
	s.adapter.EXPECT().CreateHTTPConnection(mock.Anything, mock.Anything).
		Return(nil, &httpconnection.ErrorInvalidURL)

	outcome := s.importHTTPConnection("http://risk.example.com/evaluate", httpImportPasswordLine, false)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(httpconnection.ErrorInvalidURL.Code, outcome.Code)
}

func (s *ImportHTTPConnectionTestSuite) TestALookupFailureIsReportedWithoutWriting() {
	s.adapter.EXPECT().GetHTTPConnection(mock.Anything, "http-1").Return(nil, &tidcommon.InternalServerError)

	outcome := s.importHTTPConnection(httpImportURL, httpImportPasswordLine, false)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
	s.Equal(tidcommon.InternalServerError.Code, outcome.Code)
}

// fakeProjectService records the projects it is asked to import.
type fakeProjectService struct {
	projects  map[string]projectDocument
	imported  []string
	upserts   []bool
	importErr *tidcommon.ServiceError
}

func (f *fakeProjectService) ProjectExists(_ context.Context, id string) (bool, *tidcommon.ServiceError) {
	_, exists := f.projects[id]
	return exists, nil
}

func (f *fakeProjectService) ImportProject(_ context.Context, id, rootOUID, handle, displayName,
	description string, upsert bool) (bool, *tidcommon.ServiceError) {
	if f.importErr != nil {
		return false, f.importErr
	}
	_, exists := f.projects[id]
	if f.projects == nil {
		f.projects = map[string]projectDocument{}
	}
	f.projects[id] = projectDocument{
		ID: id, RootOUID: rootOUID, Handle: handle, DisplayName: displayName, Description: description,
	}
	f.imported = append(f.imported, id)
	f.upserts = append(f.upserts, upsert)
	return !exists, nil
}

func newImportServiceWithProjects(ouSvc *fakeOUService, projectSvc *fakeProjectService) ImportServiceInterface {
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.(*importService).projectService = projectSvc
	return svc
}

func TestImportResources_RootOrganizationUnitIsImportedWithoutAProject(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	projectSvc := &fakeProjectService{}
	upsert := true
	content := strings.Join([]string{
		"resource_type: organization_unit",
		"id: root-ou",
		"handle: shop",
		"name: Shop",
		"",
	}, "\n")

	resp, err := newImportServiceWithProjects(ouSvc, projectSvc).ImportResources(context.Background(),
		&ImportRequest{Content: content, Options: &ImportOptions{Upsert: &upsert}})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, operationCreate, resp.Results[0].Operation)
	require.Len(t, ouSvc.created, 1)
	assert.Equal(t, "root-ou", ouSvc.created[0].ID)
	assert.Empty(t, projectSvc.imported, "only a declared project is imported")
}

const importedProjectDocuments = `resource_type: project
id: 019a3000-0000-7000-8000-0000000000p1
rootOuId: root-ou
handle: shop
displayName: Shop
description: Online shop
---
resource_type: organization_unit
id: root-ou
handle: shop
name: Shop
themeId: theme-1
`

func TestImportResources_ProjectIsSkippedWhereProjectsAreNotServed(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	svc := newImportService(
		nil, nil, nil, nil, ouSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{Content: importedProjectDocuments})

	require.Nil(t, err)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, 0, resp.Summary.Failed)
	assert.Equal(t, resourceTypeOrganizationUnit, resp.Results[0].ResourceType)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Equal(t, resourceTypeProject, resp.Results[1].ResourceType)
	assert.Equal(t, statusSuccess, resp.Results[1].Status)
	assert.Equal(t, operationSkip, resp.Results[1].Operation)
	assert.Contains(t, resp.Results[1].Message, "Control Plane")
	require.Len(t, ouSvc.created, 1, "the root organization unit is still imported")
	assert.Equal(t, "root-ou", ouSvc.created[0].ID)
}

func TestImportResources_ProjectIsImportedAfterItsRootOrganizationUnit(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	projectSvc := &fakeProjectService{}

	resp, err := newImportServiceWithProjects(ouSvc, projectSvc).ImportResources(context.Background(),
		&ImportRequest{Content: importedProjectDocuments})

	require.Nil(t, err)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, 0, resp.Summary.Failed)
	assert.Equal(t, resourceTypeOrganizationUnit, resp.Results[0].ResourceType)
	require.Len(t, ouSvc.created, 1)
	assert.Equal(t, resourceTypeProject, resp.Results[1].ResourceType)
	assert.Equal(t, operationCreate, resp.Results[1].Operation)
	assert.Equal(t, "019a3000-0000-7000-8000-0000000000p1", resp.Results[1].ResourceID)
	assert.Equal(t, projectDocument{
		ID: "019a3000-0000-7000-8000-0000000000p1", RootOUID: "root-ou", Handle: "shop", DisplayName: "Shop",
		Description: "Online shop",
	}, projectSvc.projects["019a3000-0000-7000-8000-0000000000p1"])
	assert.Equal(t, []bool{true}, projectSvc.upserts)
}

func TestImportResources_ExistingProjectIsUpdated(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	projectSvc := &fakeProjectService{projects: map[string]projectDocument{
		"019a3000-0000-7000-8000-0000000000p1": {ID: "019a3000-0000-7000-8000-0000000000p1"},
	}}

	resp, err := newImportServiceWithProjects(ouSvc, projectSvc).ImportResources(context.Background(),
		&ImportRequest{Content: importedProjectDocuments})

	require.Nil(t, err)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, operationUpdate, resp.Results[1].Operation)
}

func TestImportResources_ProjectDryRun(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	projectSvc := &fakeProjectService{projects: map[string]projectDocument{
		"019a3000-0000-7000-8000-0000000000p1": {ID: "019a3000-0000-7000-8000-0000000000p1"},
	}}
	content := "resource_type: project\nid: 019a3000-0000-7000-8000-0000000000p1\nrootOuId: root-ou\n" +
		"handle: shop\ndisplayName: Shop\n---\nresource_type: project\nid: 019a3000-0000-7000-8000-0000000000p2\n" +
		"handle: other\ndisplayName: Other\n"

	resp, err := newImportServiceWithProjects(ouSvc, projectSvc).ImportResources(context.Background(),
		&ImportRequest{Content: content, DryRun: true})

	require.Nil(t, err)
	require.Len(t, resp.Results, 2)
	assert.Equal(t, operationUpdate, resp.Results[0].Operation)
	assert.Equal(t, operationCreate, resp.Results[1].Operation)
	assert.Empty(t, projectSvc.imported, "a dry run imports nothing")
}

func TestImportResources_ProjectFailureIsReported(t *testing.T) {
	ouSvc := &fakeOUService{existing: map[string]ou.OrganizationUnit{}}
	projectSvc := &fakeProjectService{importErr: &tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType, Code: "PRJ-1012",
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "in use"},
	}}
	content := "resource_type: project\nid: 019a3000-0000-7000-8000-0000000000p1\nrootOuId: root-ou\n" +
		"handle: shop\ndisplayName: Shop\n"

	resp, err := newImportServiceWithProjects(ouSvc, projectSvc).ImportResources(context.Background(),
		&ImportRequest{Content: content})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Equal(t, "PRJ-1012", resp.Results[0].Code)
}

type ImportRoleSharingTestSuite struct {
	suite.Suite
	roleSvc           *fakeRoleService
	roleAssignmentSvc *fakeRoleAssignmentService
	sharingSvc        *fakeSharingService
	svc               ImportServiceInterface
}

func TestImportRoleSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ImportRoleSharingTestSuite))
}

func (s *ImportRoleSharingTestSuite) SetupTest() {
	s.roleSvc = &fakeRoleService{}
	s.roleAssignmentSvc = &fakeRoleAssignmentService{}
	s.sharingSvc = &fakeSharingService{}
	svc := newImportService(
		nil, nil, nil, nil, nil, nil, s.roleSvc, s.roleAssignmentSvc,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.(*importService).sharingService = s.sharingSvc
	s.svc = svc
}

// roleDocWithPolicies is one role document carrying a sharing policy and its assignments.
func roleDocWithPolicies(id string) string {
	return strings.Join([]string{
		"resource_type: role",
		"id: " + id,
		"name: Support Agent",
		"ouId: ou-owner",
		"permissions: []",
		"sharingPolicies:",
		"  - id: " + id + "-policy-owner",
		"    childUnits:",
		"      mode: all",
		"assignments:",
		"  - type: group",
		"    id: group1",
		"  - type: user",
		"    id: user1",
		"",
	}, "\n")
}

// connectionDocWithPolicies is an identity provider connection document in ou-1 that carries one sharing
// policy.
func connectionDocWithPolicies(id string) string {
	return strings.Join([]string{
		"resource_type: connection",
		"id: " + id,
		"name: idp-one",
		"type: google",
		"clientId: abc",
		"ouId: ou-1",
		"sharingPolicies:",
		"  - childUnits:",
		"      mode: all",
		"",
	}, "\n")
}

// The importer replays the policies a role document carries once the role exists, against the
// organization unit that owns it.
func (s *ImportRoleSharingTestSuite) TestPoliciesAreReplayedOnCreate() {
	resp, err := s.svc.ImportResources(context.Background(), &ImportRequest{Content: roleDocWithPolicies("role-new")})

	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	s.Equal(statusSuccess, resp.Results[0].Status)
	s.Require().Len(s.roleSvc.created, 1)
	s.Len(s.roleSvc.created[0].Assignments, 2, "the role is created with its assignments, as before")

	s.Require().Len(s.sharingSvc.calls, 1)
	call := s.sharingSvc.calls[0]
	s.Equal(role.RoleSharingType, call.resourceType)
	s.Equal("role-1", call.resourceID)
	s.Equal("ou-owner", call.owningOUID)
	s.Require().Len(call.policies, 1)
	s.Require().NotNil(call.policies[0].ChildUnits)
	s.Empty(s.roleSvc.deleted)
}

// A refused policy has to undo the create: the caller is told the document failed and gets no id
// back, so a role left behind would hold the name a retry needs.
func (s *ImportRoleSharingTestSuite) TestRefusedPoliciesUndoTheCreate() {
	s.sharingSvc.failure = &sharing.PolicyReplayError{
		Index: 0, InitiatingOUID: "ou-owner", Err: &sharing.ErrorInvalidRequestFormat}

	resp, _ := s.svc.ImportResources(context.Background(), &ImportRequest{Content: roleDocWithPolicies("role-new")})

	s.Require().Len(resp.Results, 1)
	s.Equal(statusFailed, resp.Results[0].Status)
	s.Contains(resp.Results[0].Message, "sharing policy 1 (organization unit ou-owner)")
	s.Equal([]string{"role-1"}, s.roleSvc.deleted)
}

// An update replays the document's policies against the role as stored, and adds its assignments as
// before.
func (s *ImportRoleSharingTestSuite) TestUpsertReplaysPolicies() {
	upsert := true
	resp, err := s.svc.ImportResources(context.Background(), &ImportRequest{
		Content: roleDocWithPolicies("role-1"),
		Options: &ImportOptions{Upsert: &upsert},
	})

	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	s.Equal(statusSuccess, resp.Results[0].Status)
	s.Equal(operationUpdate, resp.Results[0].Operation)
	s.Require().Len(s.sharingSvc.calls, 1)
	s.Equal("role-1", s.sharingSvc.calls[0].resourceID)
	s.Len(s.roleAssignmentSvc.assignments, 2)
	s.Empty(s.roleSvc.deleted, "an update is not undone")
}

// A document carrying no policies asks the sharing adapter nothing.
func (s *ImportRoleSharingTestSuite) TestDocumentWithoutPoliciesAsksTheAdapterNothing() {
	content := strings.Join([]string{
		"resource_type: role", "id: role-plain", "name: Plain", "ouId: ou-owner", "permissions: []", "",
	}, "\n")

	resp, err := s.svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	s.Require().Nil(err)
	s.Equal(statusSuccess, resp.Results[0].Status)
	s.Empty(s.sharingSvc.calls)
}

// A misspelled policy field fails the document rather than being dropped, which would share the
// role with the organization unit the document meant to exclude.
func (s *ImportRoleSharingTestSuite) TestMisspelledPolicyFieldFailsTheDocument() {
	content := strings.Join([]string{
		"resource_type: role", "id: role-typo", "name: Typo", "ouId: ou-owner", "permissions: []",
		"sharingPolicies:", "  - childUnits:", "      mode: all", "      excludeOuIds: [ou-trial]", "",
	}, "\n")

	resp, err := s.svc.ImportResources(context.Background(), &ImportRequest{Content: content})

	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	s.Equal(statusFailed, resp.Results[0].Status)
	s.Contains(resp.Results[0].Message, `unknown sharing policy field "excludeOuIds"`)
	s.Empty(s.roleSvc.created, "nothing is written for a document that does not decode")
	s.Empty(s.sharingSvc.calls)
}

func newTestConnectionImportWithSharing(sharingSvc *fakeSharingService) ImportServiceInterface {
	idpSvc := &fakeIDPService{byID: map[string]*providers.IDPDTO{}, byName: map[string]*providers.IDPDTO{}}
	svc := newImportService(
		nil, idpSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if sharingSvc != nil {
		svc.(*importService).sharingService = sharingSvc
	}
	return svc
}

// A connection's sharing is replayed by the importer once the connection exists, against the unit it lives in.
func TestImportConnectionAppliesSharingPolicies(t *testing.T) {
	sharingSvc := &fakeSharingService{}
	svc := newTestConnectionImportWithSharing(sharingSvc)

	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: connectionDocWithPolicies("idp-1"), Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Nil(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	require.Len(t, sharingSvc.calls, 1)
	assert.Equal(t, connection.ConnectionSharingType, sharingSvc.calls[0].resourceType)
	assert.Equal(t, "idp-1", sharingSvc.calls[0].resourceID)
	assert.Equal(t, "ou-1", sharingSvc.calls[0].owningOUID)
	assert.Len(t, sharingSvc.calls[0].policies, 1)
}

// A refused policy fails the document rather than importing a connection that is not shared as declared.
func TestImportConnectionFailsWhenPoliciesAreRefused(t *testing.T) {
	sharingSvc := &fakeSharingService{failure: &sharing.PolicyReplayError{
		Index: 0, InitiatingOUID: "ou-1", Err: &sharing.ErrorPolicyExists}}
	svc := newTestConnectionImportWithSharing(sharingSvc)

	resp, _ := svc.ImportResources(context.Background(), &ImportRequest{
		Content: connectionDocWithPolicies("idp-2"), Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Contains(t, resp.Results[0].Message, "sharing policy 1")
}

func TestImportConnectionRefusesPoliciesWithoutASharingAdapter(t *testing.T) {
	svc := newTestConnectionImportWithSharing(nil)

	resp, _ := svc.ImportResources(context.Background(), &ImportRequest{
		Content: connectionDocWithPolicies("idp-3"), Options: &ImportOptions{Upsert: boolPtr(false)}})

	require.Len(t, resp.Results, 1)
	assert.Equal(t, statusFailed, resp.Results[0].Status)
	assert.Contains(t, resp.Results[0].Message, "sharing")
}

// A dry run writes nothing, so it replays nothing.
func TestImportConnectionDryRunReplaysNoPolicies(t *testing.T) {
	sharingSvc := &fakeSharingService{}
	svc := newTestConnectionImportWithSharing(sharingSvc)

	resp, _ := svc.ImportResources(context.Background(), &ImportRequest{
		Content: connectionDocWithPolicies("idp-4"), DryRun: true})

	assert.Equal(t, statusSuccess, resp.Results[0].Status)
	assert.Empty(t, sharingSvc.calls)
}
