// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type DefinitionServiceTestSuite struct {
	suite.Suite
}

func TestDefinitionServiceTestSuite(t *testing.T) {
	suite.Run(t, new(DefinitionServiceTestSuite))
}

// SetupTest pins the store mode to mutable so the service's declarative-mode guard has a
// runtime to read. Suites in this package reset the runtime, so it is re-initialized here.
func (suite *DefinitionServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	suite.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		OpenID4VP: config.OpenID4VPConfig{Store: string(serverconst.StoreModeMutable)},
	}))
}

// newStatefulDefinitionStore returns a definitionStoreInterface mock backed by an
// in-memory map, so service tests exercise real create/read/update round-trips.
func newStatefulDefinitionStore(t *testing.T) *definitionStoreInterfaceMock {
	t.Helper()
	m := newDefinitionStoreInterfaceMock(t)
	byID := map[string]PresentationDefinitionDTO{}
	m.EXPECT().CreatePresentationDefinition(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, dto PresentationDefinitionDTO) error {
			byID[dto.ID] = dto
			return nil
		}).Maybe()
	m.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (*PresentationDefinitionDTO, error) {
			dto, ok := byID[id]
			if !ok {
				return nil, ErrNotFound
			}
			return &dto, nil
		}).Maybe()
	m.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, handle string) (*PresentationDefinitionDTO, error) {
			for _, dto := range byID {
				if dto.Handle == handle {
					d := dto
					return &d, nil
				}
			}
			return nil, ErrNotFound
		}).Maybe()
	m.EXPECT().ListPresentationDefinitions(mock.Anything).RunAndReturn(
		func(_ context.Context) ([]PresentationDefinitionDTO, error) {
			out := make([]PresentationDefinitionDTO, 0, len(byID))
			for _, dto := range byID {
				out = append(out, dto)
			}
			return out, nil
		}).Maybe()
	m.EXPECT().ListPresentationDefinitionSummaries(mock.Anything).RunAndReturn(
		func(_ context.Context) ([]PresentationDefinitionList, error) {
			out := make([]PresentationDefinitionList, 0, len(byID))
			for _, dto := range byID {
				out = append(out, toSummary(dto))
			}
			return out, nil
		}).Maybe()
	m.EXPECT().ListPresentationDefinitionSummariesByOUID(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, ouID string) ([]PresentationDefinitionList, error) {
			out := make([]PresentationDefinitionList, 0, len(byID))
			for _, dto := range byID {
				if dto.OUID == ouID {
					out = append(out, toSummary(dto))
				}
			}
			return out, nil
		}).Maybe()
	m.EXPECT().ListPresentationDefinitionSummariesByProject(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, projectID string) ([]PresentationDefinitionList, error) {
			out := make([]PresentationDefinitionList, 0, len(byID))
			for _, dto := range byID {
				if dto.ProjectID == projectID {
					out = append(out, toSummary(dto))
				}
			}
			return out, nil
		}).Maybe()
	m.EXPECT().CountPresentationDefinitionsByProject(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, projectID string) (int, error) {
			count := 0
			for _, dto := range byID {
				if dto.ProjectID == projectID {
					count++
				}
			}
			return count, nil
		}).Maybe()
	m.EXPECT().UpdatePresentationDefinition(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, dto PresentationDefinitionDTO) error {
			byID[dto.ID] = dto
			return nil
		}).Maybe()
	m.EXPECT().DeletePresentationDefinition(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) error {
			delete(byID, id)
			return nil
		}).Maybe()
	m.EXPECT().IsPresentationDefinitionDeclarative(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (bool, error) {
			return false, nil
		}).Maybe()
	return m
}

func newTestDefinitionService(t *testing.T) (*definitionService, *definitionStoreInterfaceMock) {
	store := newStatefulDefinitionStore(t)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(t), nil, false)
	return svc, store
}

// newOUServiceMock returns an OrganizationUnitServiceInterface mock backed by the
// given maps, so tests can validate/resolve OUs without a real OU service.
func newOUServiceMock(
	t *testing.T, exists map[string]bool, byPath, handles map[string]string,
) *oumock.OrganizationUnitServiceInterfaceMock {
	t.Helper()
	m := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	m.EXPECT().IsOrganizationUnitExists(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (bool, *tidcommon.ServiceError) {
			return exists[id], nil
		}).Maybe()
	m.EXPECT().GetOrganizationUnitByPath(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, handlePath string) (ou.OrganizationUnit, *tidcommon.ServiceError) {
			id, ok := byPath[handlePath]
			if !ok {
				return ou.OrganizationUnit{}, &tidcommon.InternalServerError
			}
			return ou.OrganizationUnit{ID: id}, nil
		}).Maybe()
	m.EXPECT().GetOrganizationUnitHandlesByIDs(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, ids []string) (map[string]string, *tidcommon.ServiceError) {
			out := make(map[string]string, len(ids))
			for _, id := range ids {
				if h, ok := handles[id]; ok {
					out[id] = h
				}
			}
			return out, nil
		}).Maybe()
	return m
}

// newErroringDefinitionStore returns a store mock whose methods all return the
// given error (or ErrNotFound), so service error paths can be exercised.
func newErroringDefinitionStore(t *testing.T, err error) *definitionStoreInterfaceMock {
	t.Helper()
	m := newDefinitionStoreInterfaceMock(t)
	m.EXPECT().CreatePresentationDefinition(mock.Anything, mock.Anything).Return(err).Maybe()
	m.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().ListPresentationDefinitions(mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().ListPresentationDefinitionSummariesByProject(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().CountPresentationDefinitionsByProject(mock.Anything, mock.Anything).Return(0, err).Maybe()
	m.EXPECT().ListPresentationDefinitionSummaries(mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().ListPresentationDefinitionSummariesByOUID(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().UpdatePresentationDefinition(mock.Anything, mock.Anything).Return(err).Maybe()
	m.EXPECT().DeletePresentationDefinition(mock.Anything, mock.Anything).Return(err).Maybe()
	m.EXPECT().IsPresentationDefinitionDeclarative(mock.Anything, mock.Anything).Return(false, err).Maybe()
	return m
}

// newFailingOUServiceMock returns an OU service mock whose methods all fail,
// so service OU-resolution error paths can be exercised.
func newFailingOUServiceMock(t *testing.T) *oumock.OrganizationUnitServiceInterfaceMock {
	t.Helper()
	m := oumock.NewOrganizationUnitServiceInterfaceMock(t)
	m.EXPECT().IsOrganizationUnitExists(mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Maybe()
	m.EXPECT().GetOrganizationUnitByPath(mock.Anything, mock.Anything).
		Return(ou.OrganizationUnit{}, &tidcommon.InternalServerError).Maybe()
	m.EXPECT().GetOrganizationUnitHandlesByIDs(mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Maybe()
	return m
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceRequiresAndValidatesOUID() {
	resolver := newOUServiceMock(suite.T(),
		map[string]bool{"ou-1": true},
		map[string]string{"default": "ou-1"},
		map[string]string{"ou-1": "default"})
	svc := newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	_, err := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "eudi-pid", VCT: "urn:eudi:pid:de:1",
	})
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionInvalidOU.Code, err.Code)

	created, err := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "eudi-pid", VCT: "urn:eudi:pid:de:1", OUID: "ou-1",
	})
	suite.Require().Nil(err)
	suite.Equal("ou-1", created.OUID)

	got, err := svc.GetPresentationDefinition(ctx, created.ID)
	suite.Require().Nil(err)
	suite.Equal("ou-1", got.OUID)
	suite.Equal("default", got.OUHandle)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreatePersists() {
	svc, store := newTestDefinitionService(suite.T())
	ctx := context.Background()

	created, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle:          "eudi-pid",
		Name:            "EUDI PID",
		VCT:             "urn:eudi:pid:de:1",
		MandatoryClaims: []string{"given_name", "family_name"},
		OptionalClaims:  []string{"birthdate"},
	})
	suite.Require().Nil(svcErr)
	suite.Require().NotEmpty(created.ID)
	suite.Equal(DefaultCredentialFormat, created.Format)

	// The verifier engine resolves definitions from the same store on demand.
	stored, err := store.GetPresentationDefinitionByHandle(ctx, "eudi-pid")
	suite.Require().NoError(err)
	suite.Equal(created.ID, stored.ID)
	suite.Equal("urn:eudi:pid:de:1", stored.VCT)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateValidationAndConflict() {
	svc, _ := newTestDefinitionService(suite.T())
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "", VCT: "x"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)

	_, svcErr = svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "h", VCT: ""})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)

	_, svcErr = svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "dup", VCT: "v"})
	suite.Require().Nil(svcErr)
	_, svcErr = svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "dup", VCT: "v2"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionAlreadyExists.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceRejectsUnsupportedFormat() {
	svc, _ := newTestDefinitionService(suite.T())
	ctx := context.Background()

	// An empty format defaults to the supported SD-JWT VC format.
	created, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "default-fmt", VCT: "v",
	})
	suite.Require().Nil(svcErr)
	suite.Equal(DefaultCredentialFormat, created.Format)

	// An unsupported format is rejected.
	_, svcErr = svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "mdoc", VCT: "v", Format: "mso_mdoc",
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionUnsupportedFormat.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateRehandles() {
	svc, store := newTestDefinitionService(suite.T())
	ctx := context.Background()

	created, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "old", VCT: "v"})
	suite.Require().Nil(svcErr)
	_, err := store.GetPresentationDefinitionByHandle(ctx, "old")
	suite.Require().NoError(err)

	_, svcErr = svc.UpdatePresentationDefinition(ctx, created.ID, &PresentationDefinitionDTO{Handle: "new", VCT: "v2"})
	suite.Require().Nil(svcErr)

	_, err = store.GetPresentationDefinitionByHandle(ctx, "old")
	suite.ErrorIs(err, ErrNotFound, "old handle should no longer resolve")
	stored, err := store.GetPresentationDefinitionByHandle(ctx, "new")
	suite.Require().NoError(err)
	suite.Equal("v2", stored.VCT)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceGetByHandle() {
	svc, _ := newTestDefinitionService(suite.T())
	ctx := context.Background()

	created, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "eudi-pid", VCT: "v"})
	suite.Require().Nil(svcErr)

	got, svcErr := svc.GetPresentationDefinitionByHandle(ctx, "eudi-pid")
	suite.Require().Nil(svcErr)
	suite.Equal(created.ID, got.ID)

	_, svcErr = svc.GetPresentationDefinitionByHandle(ctx, "missing")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionNotFound.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListSummaries() {
	resolver := newOUServiceMock(suite.T(),
		map[string]bool{"ou-1": true},
		map[string]string{"default": "ou-1"},
		map[string]string{"ou-1": "default"})
	svc := newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "eudi-pid", VCT: "v", OUID: "ou-1",
	})
	suite.Require().Nil(svcErr)

	summaries, svcErr := svc.ListPresentationDefinitionSummaries(ctx)
	suite.Require().Nil(svcErr)
	suite.Require().Len(summaries, 1)
	suite.Equal("eudi-pid", summaries[0].Handle)
	// populateSummaryOUHandles resolved the OU handle for display.
	suite.Equal("default", summaries[0].OUHandle)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListSummariesNoOUService() {
	svc, _ := newTestDefinitionService(suite.T())
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().Nil(svcErr)

	summaries, svcErr := svc.ListPresentationDefinitionSummaries(ctx)
	suite.Require().Nil(svcErr)
	suite.Len(summaries, 1)
	suite.Empty(summaries[0].OUHandle)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceGetUpdateDeleteNotFound() {
	svc, store := newTestDefinitionService(suite.T())
	ctx := context.Background()

	_, svcErr := svc.GetPresentationDefinition(ctx, "missing")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionNotFound.Code, svcErr.Code)

	_, svcErr = svc.UpdatePresentationDefinition(ctx, "missing", &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionNotFound.Code, svcErr.Code)

	// Delete of a missing definition is idempotent.
	suite.Require().Nil(svc.DeletePresentationDefinition(ctx, "missing"))

	created, _ := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "todelete", VCT: "v"})
	suite.Require().Nil(svc.DeletePresentationDefinition(ctx, created.ID))
	_, err := store.GetPresentationDefinitionByHandle(ctx, "todelete")
	suite.ErrorIs(err, ErrNotFound)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceRejectsMissingOUID() {
	resolver := newFailingOUServiceMock(suite.T())
	svc := newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v",
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidOU.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceResolveOUExistsCheckFails() {
	resolver := newFailingOUServiceMock(suite.T())
	svc := newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	// OUID is supplied directly, so existence verification runs and returns an error.
	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v", OUID: "ou-x",
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceResolveOUNotExists() {
	resolver := newOUServiceMock(suite.T(),
		map[string]bool{"ou-1": false}, nil, nil)
	svc := newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v", OUID: "ou-1",
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidOU.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateKeepsSuppliedID() {
	svc, _ := newTestDefinitionService(suite.T())
	created, svcErr := svc.CreatePresentationDefinition(context.Background(),
		&PresentationDefinitionDTO{ID: "imported-id", Handle: "h", VCT: "v"})
	suite.Require().Nil(svcErr)
	suite.Equal("imported-id", created.ID)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateRejectsTakenID() {
	svc, _ := newTestDefinitionService(suite.T())
	created, svcErr := svc.CreatePresentationDefinition(context.Background(),
		&PresentationDefinitionDTO{Handle: "first", VCT: "v"})
	suite.Require().Nil(svcErr)

	_, svcErr = svc.CreatePresentationDefinition(context.Background(),
		&PresentationDefinitionDTO{ID: created.ID, Handle: "second", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionIDConflict.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateIDLookupError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound)
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, "imported-id").Return(nil, errors.New("db boom"))
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	_, svcErr := svc.CreatePresentationDefinition(context.Background(),
		&PresentationDefinitionDTO{ID: "imported-id", Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateHandleLookupError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreatePersistError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound)
	store.EXPECT().CreatePresentationDefinition(mock.Anything, mock.Anything).Return(errors.New("insert failed"))
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateUUIDError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)
	svc.uuid = func() (string, error) { return "", errors.New("uuid failed") }
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceGetInvalidRequest() {
	svc, _ := newTestDefinitionService(suite.T())
	_, svcErr := svc.GetPresentationDefinition(context.Background(), "  ")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceGetStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.GetPresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceGetByHandleStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.GetPresentationDefinitionByHandle(context.Background(), "h")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.ListPresentationDefinitions(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListResultLimitExceeded() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), ErrResultLimitExceededInCompositeMode), nil,
		allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.ListPresentationDefinitions(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionResultLimitExceeded.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListSummariesStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.ListPresentationDefinitionSummaries(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListSummariesResultLimitExceeded() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), ErrResultLimitExceededInCompositeMode), nil,
		allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.ListPresentationDefinitionSummaries(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionResultLimitExceeded.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListPopulatesOUHandles() {
	resolver := newOUServiceMock(suite.T(),
		map[string]bool{"ou-1": true},
		map[string]string{"default": "ou-1"},
		map[string]string{"ou-1": "default"})
	svc := newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v", OUID: "ou-1",
	})
	suite.Require().Nil(svcErr)

	defs, svcErr := svc.ListPresentationDefinitions(ctx)
	suite.Require().Nil(svcErr)
	suite.Require().Len(defs, 1)
	suite.Equal("default", defs[0].OUHandle)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListSummariesPopulateResolveFails() {
	resolver := newFailingOUServiceMock(suite.T())
	store := newStatefulDefinitionStore(suite.T())
	suite.Require().NoError(store.CreatePresentationDefinition(context.Background(), PresentationDefinitionDTO{
		ID: "id-1", Handle: "h", VCT: "v", OUID: "ou-1", Format: DefaultCredentialFormat,
	}))
	svc := newPresentationDefinitionService(store, resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	// Handle resolution fails, but the summaries still return with unresolved handles.
	summaries, svcErr := svc.ListPresentationDefinitionSummaries(ctx)
	suite.Require().Nil(svcErr)
	suite.Require().Len(summaries, 1)
	suite.Empty(summaries[0].OUHandle)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServicePopulateOUHandleResolveFails() {
	resolver := newFailingOUServiceMock(suite.T())
	store := newStatefulDefinitionStore(suite.T())
	suite.Require().NoError(store.CreatePresentationDefinition(context.Background(), PresentationDefinitionDTO{
		ID: "id-1", Handle: "h", VCT: "v", OUID: "ou-1", Format: DefaultCredentialFormat,
	}))
	svc := newPresentationDefinitionService(store, resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()

	// Handle resolution fails, but the get still succeeds with an unresolved handle.
	got, svcErr := svc.GetPresentationDefinition(ctx, "id-1")
	suite.Require().Nil(svcErr)
	suite.Empty(got.OUHandle)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateInvalidRequest() {
	svc, _ := newTestDefinitionService(suite.T())
	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "  ",
		&PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateValidationFails() {
	svc, _ := newTestDefinitionService(suite.T())
	created, svcErr := svc.CreatePresentationDefinition(context.Background(),
		&PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().Nil(svcErr)
	_, svcErr = svc.UpdatePresentationDefinition(context.Background(), created.ID,
		&PresentationDefinitionDTO{Handle: "", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateLoadStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "id-1",
		&PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateHandleClashLookupError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "old", VCT: "v"}, nil)
	store.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(
		nil, errors.New("db boom"))
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "id-1",
		&PresentationDefinitionDTO{Handle: "new", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateHandleClashConflict() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "old", VCT: "v"}, nil)
	store.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-2", Handle: "new", VCT: "v"}, nil)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "id-1",
		&PresentationDefinitionDTO{Handle: "new", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionAlreadyExists.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateImmutable() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "h", VCT: "v"}, nil)
	store.EXPECT().UpdatePresentationDefinition(mock.Anything, mock.Anything).Return(ErrDefinitionIsImmutable)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "id-1",
		&PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionImmutable.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdatePersistError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "h", VCT: "v"}, nil)
	store.EXPECT().UpdatePresentationDefinition(mock.Anything, mock.Anything).Return(errors.New("update failed"))
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "id-1",
		&PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceDeleteInvalidRequest() {
	svc, _ := newTestDefinitionService(suite.T())
	svcErr := svc.DeletePresentationDefinition(context.Background(), "  ")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceDeleteLoadStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	svcErr := svc.DeletePresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceDeleteImmutable() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "h", VCT: "v"}, nil)
	store.EXPECT().DeletePresentationDefinition(mock.Anything, mock.Anything).Return(ErrDefinitionIsImmutable)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	svcErr := svc.DeletePresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionImmutable.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceDeletePersistError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "h", VCT: "v"}, nil)
	store.EXPECT().DeletePresentationDefinition(mock.Anything, mock.Anything).Return(errors.New("delete failed"))
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	svcErr := svc.DeletePresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceIsDeclarativeStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	_, svcErr := svc.IsPresentationDefinitionDeclarative(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceIsDeclarativeSuccess() {
	store := newStatefulDefinitionStore(suite.T())
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)
	isDeclarative, svcErr := svc.IsPresentationDefinitionDeclarative(context.Background(), "id-1")
	suite.Require().Nil(svcErr)
	suite.False(isDeclarative)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceRejectsDuplicateClaims() {
	ctx := context.Background()

	// The same name twice within one list.
	svc, _ := newTestDefinitionService(suite.T())
	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "dup-mandatory", VCT: "v",
		MandatoryClaims: []string{"tier", "tier"},
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionDuplicateClaim.Code, svcErr.Code)
	suite.Equal("tier", svcErr.ErrorDescription.Params["claim"])

	// The same name as both mandatory and optional.
	svc, _ = newTestDefinitionService(suite.T())
	_, svcErr = svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "dup-across", VCT: "v",
		MandatoryClaims: []string{"tier"},
		OptionalClaims:  []string{"full_name", "tier"},
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionDuplicateClaim.Code, svcErr.Code)
	suite.Equal("tier", svcErr.ErrorDescription.Params["claim"])
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceRejectsEmptyClaimName() {
	svc, _ := newTestDefinitionService(suite.T())

	_, svcErr := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "blank-claim", VCT: "v",
		OptionalClaims: []string{"  "},
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionEmptyClaimName.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceAcceptsDistinctClaims() {
	svc, _ := newTestDefinitionService(suite.T())

	created, svcErr := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "distinct-claims", VCT: "v",
		MandatoryClaims: []string{"tier"},
		OptionalClaims:  []string{"full_name"},
	})
	suite.Require().Nil(svcErr)
	suite.Equal([]string{"tier"}, created.MandatoryClaims)
	suite.Equal([]string{"full_name"}, created.OptionalClaims)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceRejectsInvalidRequestedClaims() {
	ctx := context.Background()

	svc, _ := newTestDefinitionService(suite.T())
	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "dup-requested", VCT: "v",
		RequestedClaims: []string{"tier", "tier"},
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionDuplicateClaim.Code, svcErr.Code)
	suite.Equal("tier", svcErr.ErrorDescription.Params["claim"])

	svc, _ = newTestDefinitionService(suite.T())
	_, svcErr = svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "blank-requested", VCT: "v",
		RequestedClaims: []string{"  "},
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionEmptyClaimName.Code, svcErr.Code)
}

// The requested list is the full set the mandatory and optional lists partition, so repeating
// their names in it is valid and must not be read as a duplicate.
func (suite *DefinitionServiceTestSuite) TestDefinitionServiceAcceptsRequestedClaimsSupersetting() {
	svc, _ := newTestDefinitionService(suite.T())

	created, svcErr := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "requested-superset", VCT: "v",
		RequestedClaims: []string{"tier", "full_name"},
		MandatoryClaims: []string{"tier"},
		OptionalClaims:  []string{"full_name"},
	})
	suite.Require().Nil(svcErr)
	suite.Equal([]string{"tier", "full_name"}, created.RequestedClaims)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateTrustedAuthorities() {
	svc, store := newTestDefinitionService(suite.T())
	ctx := context.Background()
	enforce := true
	certA := validTestCertificatePEM(suite.T(), "root-a")

	created, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v", EnforceTrustedIssuer: &enforce,
		TrustedAuthorities: []TrustedAuthority{{Name: "root-a", Certificate: certA}},
	})
	suite.Require().Nil(svcErr)

	stored, err := store.GetPresentationDefinitionByID(ctx, created.ID)
	suite.Require().NoError(err)
	suite.Equal([]TrustedAuthority{{Name: "root-a", Certificate: certA}}, stored.TrustedAuthorities)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateTrustedAuthorityValidation() {
	enforce := true
	certA := validTestCertificatePEM(suite.T(), "root-a")
	expired := newTestCertificatePEM(suite.T(), "expired", time.Now().Add(-time.Hour))
	tooMany := make([]TrustedAuthority, 0, maxTrustedAuthorities+1)
	for i := range maxTrustedAuthorities + 1 {
		tooMany = append(tooMany, TrustedAuthority{
			Name: fmt.Sprintf("root-%d", i), Certificate: validTestCertificatePEM(suite.T(), "root"),
		})
	}

	cases := []struct {
		name        string
		authorities []TrustedAuthority
		enforce     *bool
		wantCode    string
	}{
		{"missing certificate", []TrustedAuthority{{Name: "root-a"}}, nil,
			ErrorDefinitionInvalidTrustedAuthorityCertificate.Code},
		{"invalid certificate", []TrustedAuthority{{Name: "root-a", Certificate: "not pem"}}, nil,
			ErrorDefinitionInvalidTrustedAuthorityCertificate.Code},
		{"expired certificate", []TrustedAuthority{{Name: "old", Certificate: expired}}, nil,
			ErrorDefinitionExpiredTrustedAuthorityCertificate.Code},
		{"empty name", []TrustedAuthority{{Name: "  ", Certificate: certA}}, nil,
			ErrorDefinitionEmptyTrustedAuthorityName.Code},
		{"duplicate name", []TrustedAuthority{
			{Name: "root-a", Certificate: certA},
			{Name: "root-a", Certificate: validTestCertificatePEM(suite.T(), "root-b")},
		}, nil, ErrorDefinitionDuplicateTrustedAuthority.Code},
		{"duplicate certificate", []TrustedAuthority{
			{Name: "root-a", Certificate: certA}, {Name: "root-a-again", Certificate: certA},
		}, nil, ErrorDefinitionDuplicateTrustedAuthority.Code},
		{"too many", tooMany, nil, ErrorDefinitionTooManyTrustedAuthorities.Code},
		{"enforced without authorities", nil, &enforce, ErrorDefinitionTrustedIssuerWithoutAuthorities.Code},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, _ := newTestDefinitionService(suite.T())
			_, svcErr := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
				Handle: "h", VCT: "v", EnforceTrustedIssuer: tc.enforce, TrustedAuthorities: tc.authorities,
			})
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantCode, svcErr.Code)
		})
	}
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateCarriesStoredCertificates() {
	svc, store := newTestDefinitionService(suite.T())
	ctx := context.Background()
	certA := validTestCertificatePEM(suite.T(), "root-a")
	certB := validTestCertificatePEM(suite.T(), "root-b")
	certC := validTestCertificatePEM(suite.T(), "root-c")

	created, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v",
		TrustedAuthorities: []TrustedAuthority{
			{Name: "root-a", Certificate: certA}, {Name: "root-b", Certificate: certB},
		},
	})
	suite.Require().Nil(svcErr)

	// root-a is kept by name, root-b is dropped by omission, and root-c is added.
	_, svcErr = svc.UpdatePresentationDefinition(ctx, created.ID, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v",
		TrustedAuthorities: []TrustedAuthority{{Name: "root-a"}, {Name: "root-c", Certificate: certC}},
	})
	suite.Require().Nil(svcErr)

	stored, err := store.GetPresentationDefinitionByID(ctx, created.ID)
	suite.Require().NoError(err)
	suite.Equal([]TrustedAuthority{{Name: "root-a", Certificate: certA}, {Name: "root-c", Certificate: certC}},
		stored.TrustedAuthorities)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateKeepsStoredExpiredCertificate() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	expired := newTestCertificatePEM(suite.T(), "expired", time.Now().Add(-time.Hour))
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, "id-1").Return(&PresentationDefinitionDTO{
		ID: "id-1", Handle: "h", VCT: "v",
		TrustedAuthorities: []TrustedAuthority{{Name: "old", Certificate: expired}},
	}, nil)
	store.EXPECT().UpdatePresentationDefinition(mock.Anything, mock.MatchedBy(func(dto PresentationDefinitionDTO) bool {
		return len(dto.TrustedAuthorities) == 1 && dto.TrustedAuthorities[0].Certificate == expired
	})).Return(nil)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "id-1", &PresentationDefinitionDTO{
		Handle: "h", VCT: "v", Name: "renamed", TrustedAuthorities: []TrustedAuthority{{Name: "old"}},
	})
	suite.Nil(svcErr)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateTrustedAuthorityValidation() {
	expired := newTestCertificatePEM(suite.T(), "expired", time.Now().Add(-time.Hour))
	cases := []struct {
		name        string
		authorities []TrustedAuthority
		wantCode    string
	}{
		{"unknown name without certificate", []TrustedAuthority{{Name: "unknown"}},
			ErrorDefinitionInvalidTrustedAuthorityCertificate.Code},
		{"expired new certificate", []TrustedAuthority{{Name: "old", Certificate: expired}},
			ErrorDefinitionExpiredTrustedAuthorityCertificate.Code},
		{"invalid new certificate", []TrustedAuthority{{Name: "bad", Certificate: "not pem"}},
			ErrorDefinitionInvalidTrustedAuthorityCertificate.Code},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, _ := newTestDefinitionService(suite.T())
			created, svcErr := svc.CreatePresentationDefinition(context.Background(),
				&PresentationDefinitionDTO{Handle: "h", VCT: "v"})
			suite.Require().Nil(svcErr)

			_, svcErr = svc.UpdatePresentationDefinition(context.Background(), created.ID,
				&PresentationDefinitionDTO{Handle: "h", VCT: "v", TrustedAuthorities: tc.authorities})
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.wantCode, svcErr.Code)
		})
	}
}

func (suite *DefinitionServiceTestSuite) TestListSummariesByOUID() {
	resolver := newOUServiceMock(suite.T(),
		map[string]bool{"ou-1": true, "ou-2": true},
		map[string]string{},
		map[string]string{"ou-1": "default"})
	svc := newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), resolver, allowAllAuthz(suite.T()), nil, false)
	ctx := context.Background()
	first := &PresentationDefinitionDTO{Handle: "first", VCT: "v1", OUID: "ou-1"}
	_, svcErr := svc.CreatePresentationDefinition(ctx, first)
	suite.Require().Nil(svcErr)
	second := &PresentationDefinitionDTO{Handle: "second", VCT: "v2", OUID: "ou-2"}
	_, svcErr = svc.CreatePresentationDefinition(ctx, second)
	suite.Require().Nil(svcErr)

	summaries, svcErr := svc.ListPresentationDefinitionSummariesByOUID(ctx, "ou-1")
	suite.Require().Nil(svcErr)
	suite.Require().Len(summaries, 1)
	suite.Equal(first.Handle, summaries[0].Handle)
	suite.Equal("default", summaries[0].OUHandle)

	summaries, svcErr = svc.ListPresentationDefinitionSummariesByOUID(ctx, "ou-missing")
	suite.Require().Nil(svcErr)
	suite.Empty(summaries)
}

func (suite *DefinitionServiceTestSuite) TestListSummariesByOUIDErrors() {
	testCases := []struct {
		name         string
		ouID         string
		storeErr     error
		expectedCode string
	}{
		{name: "EmptyOUID", ouID: "", expectedCode: ErrorDefinitionInvalidRequest.Code},
		{name: "StoreError", ouID: "ou-1", storeErr: errors.New("boom"),
			expectedCode: tidcommon.InternalServerError.Code},
		{name: "ResultLimitExceeded", ouID: "ou-1", storeErr: ErrResultLimitExceededInCompositeMode,
			expectedCode: ErrorDefinitionResultLimitExceeded.Code},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			svc := newPresentationDefinitionService(
				newErroringDefinitionStore(suite.T(), tc.storeErr), nil, allowAllAuthz(suite.T()), nil, false)
			_, svcErr := svc.ListPresentationDefinitionSummariesByOUID(context.Background(), tc.ouID)
			suite.Require().NotNil(svcErr)
			suite.Equal(tc.expectedCode, svcErr.Code)
		})
	}
}

func (suite *DefinitionServiceTestSuite) TestGetResourceDependencies_OUReportsOwnedDefinitions() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().ListPresentationDefinitionSummariesByOUID(mock.Anything, "ou-1").Return(
		[]PresentationDefinitionList{{ID: "pd-1", Handle: "age-check"}, {ID: "pd-2", Handle: "employment"}}, nil)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	deps, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

	suite.Require().NoError(err)
	suite.Equal([]resourcedependency.ResourceDependency{
		{
			ResourceType:     resourcedependency.ResourceTypeVPDefinition,
			ID:               "pd-1",
			DisplayName:      "age-check",
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		},
		{
			ResourceType:     resourcedependency.ResourceTypeVPDefinition,
			ID:               "pd-2",
			DisplayName:      "employment",
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		},
	}, deps)
}

func (suite *DefinitionServiceTestSuite) TestGetResourceDependencies_OUWithoutDefinitions() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().ListPresentationDefinitionSummariesByOUID(mock.Anything, "ou-1").
		Return([]PresentationDefinitionList{}, nil)
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	deps, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

	suite.Require().NoError(err)
	suite.NotNil(deps)
	suite.Empty(deps)
}

func (suite *DefinitionServiceTestSuite) TestGetResourceDependencies_OUStoreError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().ListPresentationDefinitionSummariesByOUID(mock.Anything, "ou-1").
		Return(nil, errors.New("boom"))
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	deps, err := svc.GetResourceDependencies(context.Background(), resourcedependency.ResourceTypeOU, "ou-1")

	suite.Require().EqualError(err, "boom")
	suite.Nil(deps)
}

func (suite *DefinitionServiceTestSuite) TestGetResourceDependencies_OtherResourceTypes() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	svc := newPresentationDefinitionService(store, nil, allowAllAuthz(suite.T()), nil, false)

	deps, err := svc.GetResourceDependencies(
		context.Background(), resourcedependency.ResourceTypeVPDefinition, "pd-1")

	suite.Require().NoError(err)
	suite.NotNil(deps)
	suite.Empty(deps)
	store.AssertNotCalled(suite.T(), "ListPresentationDefinitionSummariesByOUID", mock.Anything, mock.Anything)
}

// testProjectID is the project the project tests place presentation definitions in.
const testProjectID = "proj-1"

// newProjectDefinitionService builds a service on a deployment that serves projects, where the system
// organization unit is sys-ou and the given validator checks projects.
func (suite *DefinitionServiceTestSuite) newProjectDefinitionService(
	validator *projectOUValidatorMock,
) *definitionService {
	ouMock := newOUServiceMock(suite.T(), map[string]bool{"sys-ou": true, "ou-1": true}, nil, nil)
	ouMock.EXPECT().GetSystemOUID(mock.Anything).Return("sys-ou", nil).Maybe()
	return newPresentationDefinitionService(
		newStatefulDefinitionStore(suite.T()), ouMock, allowAllAuthz(suite.T()), validator, true)
}

func (suite *DefinitionServiceTestSuite) TestCreateInProjectPlacesTheDefinitionInTheSystemOU() {
	validator := newProjectOUValidatorMock(suite.T())
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "sys-ou").Return(nil)
	svc := suite.newProjectDefinitionService(validator)

	created, err := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "pid", VCT: "urn:pid", ProjectID: testProjectID,
	})
	suite.Require().Nil(err)
	suite.Equal("sys-ou", created.OUID)
	suite.Equal(testProjectID, created.ProjectID)

	summaries, err := svc.ListPresentationDefinitionSummariesByProject(context.Background(), testProjectID)
	suite.Require().Nil(err)
	suite.Require().Len(summaries, 1)
	suite.Equal(testProjectID, summaries[0].ProjectID)

	has, err := svc.ProjectHasPresentationDefinitions(context.Background(), testProjectID)
	suite.Require().Nil(err)
	suite.True(has)
	has, err = svc.ProjectHasPresentationDefinitions(context.Background(), "proj-2")
	suite.Require().Nil(err)
	suite.False(has)
}

func (suite *DefinitionServiceTestSuite) TestCreateRefusesTheProjectsTheValidatorRefuses() {
	for code, expected := range map[string]string{
		"PRJ-1002": ErrorDefinitionInvalidProject.Code,
		"PRJ-1010": ErrorDefinitionOUNotInProject.Code,
		"PRJ-9999": tidcommon.InternalServerError.Code,
	} {
		validator := newProjectOUValidatorMock(suite.T())
		validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "ou-1").
			Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: code})
		svc := suite.newProjectDefinitionService(validator)

		_, err := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
			Handle: "pid", VCT: "urn:pid", ProjectID: testProjectID, OUID: "ou-1",
		})
		suite.Require().NotNil(err, code)
		suite.Equal(expected, err.Code, code)
	}
}

func (suite *DefinitionServiceTestSuite) TestCreateRequiresAProjectWhereProjectsAreServed() {
	svc := suite.newProjectDefinitionService(newProjectOUValidatorMock(suite.T()))

	_, err := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "pid", VCT: "urn:pid", OUID: "ou-1",
	})
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionProjectRequired.Code, err.Code)

	err = svc.ResolvePresentationDefinitionOUHandle(context.Background(), &PresentationDefinitionDTO{OUID: "ou-1"})
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionProjectRequired.Code, err.Code)
}

func (suite *DefinitionServiceTestSuite) TestCreateFailsWhenTheSystemOUCannotBeRead() {
	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	ouMock.EXPECT().GetSystemOUID(mock.Anything).Return("", &tidcommon.InternalServerError)
	svc := newPresentationDefinitionService(newStatefulDefinitionStore(suite.T()), ouMock, allowAllAuthz(suite.T()),
		newProjectOUValidatorMock(suite.T()), true)

	_, err := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "pid", VCT: "urn:pid", ProjectID: testProjectID,
	})
	suite.Require().NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (suite *DefinitionServiceTestSuite) TestCreateWithoutProjectsKeepsTheProjectUnchecked() {
	svc, _ := newTestDefinitionService(suite.T())

	created, err := svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "pid", VCT: "urn:pid", ProjectID: testProjectID,
	})
	suite.Require().Nil(err)
	suite.Equal(testProjectID, created.ProjectID)

	_, err = svc.CreatePresentationDefinition(context.Background(), &PresentationDefinitionDTO{
		Handle: "pid-2", VCT: "urn:pid", ProjectID: fmt.Sprintf("%037d", 0),
	})
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionInvalidProject.Code, err.Code)
}

// An update that moves a definition to another organization unit of the same project checks the project again.
func (suite *DefinitionServiceTestSuite) TestUpdateToAnotherOUInTheProjectIsChecked() {
	validator := newProjectOUValidatorMock(suite.T())
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "sys-ou").Return(nil).Once()
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "ou-1").
		Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: projectscope.OUNotInProjectErrorCode}).
		Once()
	svc := suite.newProjectDefinitionService(validator)
	ctx := context.Background()
	created, err := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "pid", VCT: "urn:pid", ProjectID: testProjectID,
	})
	suite.Require().Nil(err)

	_, err = svc.UpdatePresentationDefinition(ctx, created.ID,
		&PresentationDefinitionDTO{Handle: "pid", VCT: "urn:pid", ProjectID: testProjectID, OUID: "ou-1"})
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionOUNotInProject.Code, err.Code)

	stored, err := svc.GetPresentationDefinition(ctx, created.ID)
	suite.Require().Nil(err)
	suite.Equal("sys-ou", stored.OUID)
}

func (suite *DefinitionServiceTestSuite) TestUpdateKeepsOrMovesTheProject() {
	validator := newProjectOUValidatorMock(suite.T())
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "sys-ou").Return(nil).Once()
	validator.EXPECT().ValidateProjectOU(mock.Anything, "proj-2", "sys-ou").Return(nil).Once()
	validator.EXPECT().ValidateProjectOU(mock.Anything, "proj-gone", "sys-ou").
		Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "PRJ-1002"}).Once()
	svc := suite.newProjectDefinitionService(validator)
	ctx := context.Background()
	created, err := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "pid", VCT: "urn:pid", ProjectID: testProjectID,
	})
	suite.Require().Nil(err)

	updated, err := svc.UpdatePresentationDefinition(ctx, created.ID,
		&PresentationDefinitionDTO{Handle: "pid", VCT: "urn:pid", Name: "Renamed"})
	suite.Require().Nil(err)
	suite.Equal(testProjectID, updated.ProjectID, "an update without a project keeps it")
	suite.Equal("sys-ou", updated.OUID, "an update without an organization unit keeps it")

	updated, err = svc.UpdatePresentationDefinition(ctx, created.ID,
		&PresentationDefinitionDTO{Handle: "pid", VCT: "urn:pid", ProjectID: "proj-2"})
	suite.Require().Nil(err)
	suite.Equal("proj-2", updated.ProjectID)

	_, err = svc.UpdatePresentationDefinition(ctx, created.ID,
		&PresentationDefinitionDTO{Handle: "pid", VCT: "urn:pid", ProjectID: "proj-gone"})
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionInvalidProject.Code, err.Code)
}

func (suite *DefinitionServiceTestSuite) TestListByProject() {
	svc, _ := newTestDefinitionService(suite.T())

	summaries, err := svc.ListPresentationDefinitionSummariesByProject(context.Background(), "")
	suite.Require().Nil(err)
	suite.Empty(summaries, "an empty project lists no definition")

	errSvc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil, allowAllAuthz(suite.T()), nil, false)
	_, err = errSvc.ListPresentationDefinitionSummariesByProject(context.Background(), testProjectID)
	suite.Require().NotNil(err)
	suite.Equal(tidcommon.InternalServerError.Code, err.Code)
	_, err = errSvc.ProjectHasPresentationDefinitions(context.Background(), testProjectID)
	suite.Require().NotNil(err)

	limitSvc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), ErrResultLimitExceededInCompositeMode), nil,
		allowAllAuthz(suite.T()), nil, false)
	_, err = limitSvc.ListPresentationDefinitionSummariesByProject(context.Background(), testProjectID)
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionResultLimitExceeded.Code, err.Code)
}
