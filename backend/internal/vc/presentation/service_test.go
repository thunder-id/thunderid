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

package presentation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type DefinitionServiceTestSuite struct {
	suite.Suite
}

func TestDefinitionServiceTestSuite(t *testing.T) {
	suite.Run(t, new(DefinitionServiceTestSuite))
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
	svc := newPresentationDefinitionService(store, nil).(*definitionService)
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
		func(_ context.Context, handlePath string) (providers.OrganizationUnit, *tidcommon.ServiceError) {
			id, ok := byPath[handlePath]
			if !ok {
				return providers.OrganizationUnit{}, &tidcommon.InternalServerError
			}
			return providers.OrganizationUnit{ID: id}, nil
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
	m.EXPECT().ListPresentationDefinitionSummaries(mock.Anything).Return(nil, err).Maybe()
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
		Return(providers.OrganizationUnit{}, &tidcommon.InternalServerError).Maybe()
	m.EXPECT().GetOrganizationUnitHandlesByIDs(mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Maybe()
	return m
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceResolvesAndValidatesOU() {
	resolver := newOUServiceMock(suite.T(),
		map[string]bool{"ou-1": true},
		map[string]string{"default": "ou-1"},
		map[string]string{"ou-1": "default"})
	svc := newPresentationDefinitionService(newStatefulDefinitionStore(suite.T()), resolver)
	ctx := context.Background()

	_, err := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "eudi-pid", VCT: "urn:eudi:pid:de:1",
	})
	suite.Require().NotNil(err)
	suite.Equal(ErrorDefinitionInvalidOU.Code, err.Code)

	created, err := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "eudi-pid", VCT: "urn:eudi:pid:de:1", OUHandle: "default",
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
	svc := newPresentationDefinitionService(newStatefulDefinitionStore(suite.T()), resolver)
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

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceResolveOUByPathFails() {
	resolver := newFailingOUServiceMock(suite.T())
	svc := newPresentationDefinitionService(newStatefulDefinitionStore(suite.T()), resolver)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v", OUHandle: "unknown",
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidOU.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceResolveOUExistsCheckFails() {
	resolver := newFailingOUServiceMock(suite.T())
	svc := newPresentationDefinitionService(newStatefulDefinitionStore(suite.T()), resolver)
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
	svc := newPresentationDefinitionService(newStatefulDefinitionStore(suite.T()), resolver)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{
		Handle: "h", VCT: "v", OUID: "ou-1",
	})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidOU.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateHandleLookupError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreatePersistError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound)
	store.EXPECT().CreatePresentationDefinition(mock.Anything, mock.Anything).Return(errors.New("insert failed"))
	svc := newPresentationDefinitionService(store, nil)
	ctx := context.Background()

	_, svcErr := svc.CreatePresentationDefinition(ctx, &PresentationDefinitionDTO{Handle: "h", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceCreateUUIDError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound)
	svc := newPresentationDefinitionService(store, nil).(*definitionService)
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
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
	_, svcErr := svc.GetPresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceGetByHandleStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
	_, svcErr := svc.GetPresentationDefinitionByHandle(context.Background(), "h")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
	_, svcErr := svc.ListPresentationDefinitions(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListResultLimitExceeded() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), ErrResultLimitExceededInCompositeMode), nil)
	_, svcErr := svc.ListPresentationDefinitions(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionResultLimitExceeded.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListSummariesStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
	_, svcErr := svc.ListPresentationDefinitionSummaries(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListSummariesResultLimitExceeded() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), ErrResultLimitExceededInCompositeMode), nil)
	_, svcErr := svc.ListPresentationDefinitionSummaries(context.Background())
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionResultLimitExceeded.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceListPopulatesOUHandles() {
	resolver := newOUServiceMock(suite.T(),
		map[string]bool{"ou-1": true},
		map[string]string{"default": "ou-1"},
		map[string]string{"ou-1": "default"})
	svc := newPresentationDefinitionService(newStatefulDefinitionStore(suite.T()), resolver)
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
	svc := newPresentationDefinitionService(store, resolver)
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
	svc := newPresentationDefinitionService(store, resolver)
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
	_, svcErr := svc.UpdatePresentationDefinition(context.Background(), "id-1",
		&PresentationDefinitionDTO{Handle: "", VCT: "v"})
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionInvalidRequest.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceUpdateLoadStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
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
	svc := newPresentationDefinitionService(store, nil)

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
	svc := newPresentationDefinitionService(store, nil)

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
	svc := newPresentationDefinitionService(store, nil)

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
	svc := newPresentationDefinitionService(store, nil)

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
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
	svcErr := svc.DeletePresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceDeleteImmutable() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "h", VCT: "v"}, nil)
	store.EXPECT().DeletePresentationDefinition(mock.Anything, mock.Anything).Return(ErrDefinitionIsImmutable)
	svc := newPresentationDefinitionService(store, nil)

	svcErr := svc.DeletePresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorDefinitionImmutable.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceDeletePersistError() {
	store := newDefinitionStoreInterfaceMock(suite.T())
	store.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).Return(
		&PresentationDefinitionDTO{ID: "id-1", Handle: "h", VCT: "v"}, nil)
	store.EXPECT().DeletePresentationDefinition(mock.Anything, mock.Anything).Return(errors.New("delete failed"))
	svc := newPresentationDefinitionService(store, nil)

	svcErr := svc.DeletePresentationDefinition(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceIsDeclarativeStoreError() {
	svc := newPresentationDefinitionService(
		newErroringDefinitionStore(suite.T(), errors.New("db boom")), nil)
	_, svcErr := svc.IsPresentationDefinitionDeclarative(context.Background(), "id-1")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (suite *DefinitionServiceTestSuite) TestDefinitionServiceIsDeclarativeSuccess() {
	store := newStatefulDefinitionStore(suite.T())
	svc := newPresentationDefinitionService(store, nil)
	isDeclarative, svcErr := svc.IsPresentationDefinitionDeclarative(context.Background(), "id-1")
	suite.Require().Nil(svcErr)
	suite.False(isDeclarative)
}
