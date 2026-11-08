// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const testOUID = "ou-1"

type ConfigurationServiceTestSuite struct {
	suite.Suite
}

// SetupTest pins the store mode to mutable so the service's declarative-mode guard has a
// runtime to read. Suites in this package reset the runtime, so it is re-initialized here.
func (s *ConfigurationServiceTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		OpenID4VCI: config.OpenID4VCIConfig{Store: string(serverconst.StoreModeMutable)},
	}))
}

func TestConfigurationServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationServiceTestSuite))
}

// newStatefulCredentialStore returns a credentialStoreInterface mock backed by an
// in-memory map, so service tests exercise real create/read/update round-trips.
func newStatefulCredentialStore(t *testing.T) *credentialStoreInterfaceMock {
	t.Helper()
	m := newCredentialStoreInterfaceMock(t)
	byID := map[string]CredentialConfigurationDTO{}
	m.EXPECT().CreateCredentialConfiguration(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, dto CredentialConfigurationDTO) error {
			byID[dto.ID] = dto
			return nil
		}).Maybe()
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (*CredentialConfigurationDTO, error) {
			dto, ok := byID[id]
			if !ok {
				return nil, ErrNotFound
			}
			return &dto, nil
		}).Maybe()
	m.EXPECT().GetCredentialConfigurationByHandle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, handle string) (*CredentialConfigurationDTO, error) {
			for _, dto := range byID {
				if dto.Handle == handle {
					d := dto
					return &d, nil
				}
			}
			return nil, ErrNotFound
		}).Maybe()
	m.EXPECT().ListCredentialConfigurations(mock.Anything).RunAndReturn(
		func(_ context.Context) ([]CredentialConfigurationDTO, error) {
			out := make([]CredentialConfigurationDTO, 0, len(byID))
			for _, dto := range byID {
				out = append(out, dto)
			}
			return out, nil
		}).Maybe()
	m.EXPECT().ListCredentialConfigurationSummaries(mock.Anything).RunAndReturn(
		func(_ context.Context) ([]CredentialConfigurationList, error) {
			out := make([]CredentialConfigurationList, 0, len(byID))
			for _, dto := range byID {
				out = append(out, toConfigSummary(dto))
			}
			return out, nil
		}).Maybe()
	m.EXPECT().ListCredentialConfigurationSummariesByOUID(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, ouID string) ([]CredentialConfigurationList, error) {
			out := make([]CredentialConfigurationList, 0, len(byID))
			for _, dto := range byID {
				if dto.OUID == ouID {
					out = append(out, toConfigSummary(dto))
				}
			}
			return out, nil
		}).Maybe()
	m.EXPECT().ListCredentialConfigurationSummariesByProject(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, projectID string) ([]CredentialConfigurationList, error) {
			out := make([]CredentialConfigurationList, 0, len(byID))
			for _, dto := range byID {
				if dto.ProjectID == projectID {
					out = append(out, toConfigSummary(dto))
				}
			}
			return out, nil
		}).Maybe()
	m.EXPECT().CountCredentialConfigurationsByProject(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, projectID string) (int, error) {
			count := 0
			for _, dto := range byID {
				if dto.ProjectID == projectID {
					count++
				}
			}
			return count, nil
		}).Maybe()
	m.EXPECT().UpdateCredentialConfiguration(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, dto CredentialConfigurationDTO) error {
			byID[dto.ID] = dto
			return nil
		}).Maybe()
	m.EXPECT().DeleteCredentialConfiguration(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) error {
			delete(byID, id)
			return nil
		}).Maybe()
	m.EXPECT().IsCredentialConfigurationDeclarative(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (bool, error) {
			return false, nil
		}).Maybe()
	return m
}

func (s *ConfigurationServiceTestSuite) newService() CredentialConfigurationServiceInterface {
	return newCredentialConfigurationService(newStatefulCredentialStore(s.T()), nil, allowAllAuthz(s.T()), nil, false)
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

func (s *ConfigurationServiceTestSuite) validDTO() *CredentialConfigurationDTO {
	return &CredentialConfigurationDTO{
		Handle: "eudi-pid",
		VCT:    "urn:eudi:pid:de:1",
		Claims: []ClaimMapping{{Name: "given_name", DisplayName: "Given Name"}},
	}
}

func (s *ConfigurationServiceTestSuite) TestCreateDefaultsFormatAndAssignsID() {
	svc := s.newService()
	created, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Nil(err)
	s.NotEmpty(created.ID)
	s.Equal(DefaultCredentialFormat, created.Format)
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsMissingFields() {
	svc := s.newService()
	_, err := svc.CreateCredentialConfiguration(context.Background(), &CredentialConfigurationDTO{Handle: "x"})
	s.NotNil(err)
	s.Equal(ErrorConfigurationInvalidRequest.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsUnsupportedFormat() {
	svc := s.newService()
	dto := s.validDTO()
	dto.Format = "mso_mdoc"
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.NotNil(err)
	s.Equal(ErrorConfigurationUnsupportedFormat.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsDuplicateHandle() {
	svc := s.newService()
	_, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Nil(err)
	_, err = svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.NotNil(err)
	s.Equal(ErrorConfigurationAlreadyExists.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateKeepsSuppliedID() {
	svc := s.newService()
	dto := s.validDTO()
	dto.ID = "imported-id"
	created, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().Nil(err)
	s.Equal("imported-id", created.ID)
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsTakenID() {
	svc := s.newService()
	created, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Require().Nil(err)

	dto := s.validDTO()
	dto.ID = created.ID
	dto.Handle = "another-handle"
	_, err = svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationIDConflict.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateIDCheckStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().GetCredentialConfigurationByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound)
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, "imported-id").Return(nil, errors.New("boom"))
	svc := newCredentialConfigurationService(m, nil, allowAllAuthz(s.T()), nil, false)
	dto := s.validDTO()
	dto.ID = "imported-id"
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestGetByHandleAndDelete() {
	svc := s.newService()
	created, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Nil(err)

	got, err := svc.GetCredentialConfigurationByHandle(context.Background(), "eudi-pid")
	s.Nil(err)
	s.Equal(created.ID, got.ID)

	s.Nil(svc.DeleteCredentialConfiguration(context.Background(), created.ID))
	_, err = svc.GetCredentialConfiguration(context.Background(), created.ID)
	s.NotNil(err)
	s.Equal(ErrorConfigurationNotFound.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestDeleteIsIdempotent() {
	svc := s.newService()
	s.Nil(svc.DeleteCredentialConfiguration(context.Background(), "missing"))
}

func (s *ConfigurationServiceTestSuite) TestListAndListSummaries() {
	resolver := newOUServiceMock(s.T(),
		map[string]bool{testOUID: true},
		map[string]string{"default": testOUID},
		map[string]string{testOUID: "default"})
	svc := newCredentialConfigurationService(
		newStatefulCredentialStore(s.T()), resolver, allowAllAuthz(s.T()), nil, false)

	dto := s.validDTO()
	dto.OUID = testOUID
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().Nil(err)

	configs, err := svc.ListCredentialConfigurations(context.Background())
	s.Require().Nil(err)
	s.Require().Len(configs, 1)
	s.Equal("default", configs[0].OUHandle)

	summaries, err := svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().Nil(err)
	s.Require().Len(summaries, 1)
	s.Equal("default", summaries[0].OUHandle)
}

func (s *ConfigurationServiceTestSuite) TestUpdate() {
	svc := s.newService()
	created, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Require().Nil(err)

	updated := s.validDTO()
	updated.Handle = "eudi-pid-v2"
	updated.VCT = "urn:eudi:pid:de:2"
	got, err := svc.UpdateCredentialConfiguration(context.Background(), created.ID, updated)
	s.Require().Nil(err)
	s.Equal("eudi-pid-v2", got.Handle)
	s.Equal("urn:eudi:pid:de:2", got.VCT)
}

func (s *ConfigurationServiceTestSuite) TestUpdateMissing() {
	svc := s.newService()
	_, err := svc.UpdateCredentialConfiguration(context.Background(), "missing", s.validDTO())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationNotFound.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateRejectsEmptyID() {
	svc := s.newService()
	_, err := svc.UpdateCredentialConfiguration(context.Background(), "  ", s.validDTO())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidRequest.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateRejectsHandleClash() {
	svc := s.newService()
	first, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Require().Nil(err)

	other := s.validDTO()
	other.Handle = "other-handle"
	_, err = svc.CreateCredentialConfiguration(context.Background(), other)
	s.Require().Nil(err)

	rename := s.validDTO()
	rename.Handle = "other-handle"
	_, err = svc.UpdateCredentialConfiguration(context.Background(), first.ID, rename)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationAlreadyExists.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestIsDeclarative() {
	svc := s.newService()
	isDeclarative, err := svc.IsCredentialConfigurationDeclarative(context.Background(), "any-id")
	s.Require().Nil(err)
	s.False(isDeclarative)
}

func (s *ConfigurationServiceTestSuite) TestGetByHandleNotFound() {
	svc := s.newService()
	_, err := svc.GetCredentialConfigurationByHandle(context.Background(), "missing")
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationNotFound.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateImmutableMapsToImmutableError() {
	store := s.newImmutableStore()
	svc := newCredentialConfigurationService(store, nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.UpdateCredentialConfiguration(context.Background(), "cfg-1", s.validDTO())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationImmutable.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestDeleteImmutableMapsToImmutableError() {
	store := s.newImmutableStore()
	svc := newCredentialConfigurationService(store, nil, allowAllAuthz(s.T()), nil, false)
	err := svc.DeleteCredentialConfiguration(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationImmutable.Code, err.Code)
}

// newImmutableStore returns a store mock whose configuration exists but rejects
// update/delete with ErrConfigurationIsImmutable.
func (s *ConfigurationServiceTestSuite) newImmutableStore() *credentialStoreInterfaceMock {
	m := newCredentialStoreInterfaceMock(s.T())
	existing := CredentialConfigurationDTO{
		ID: "cfg-1", Handle: "eudi-pid", VCT: "urn:eudi:pid:de:1", Format: DefaultCredentialFormat,
	}
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, "cfg-1").RunAndReturn(
		func(_ context.Context, _ string) (*CredentialConfigurationDTO, error) {
			d := existing
			return &d, nil
		}).Maybe()
	m.EXPECT().GetCredentialConfigurationByHandle(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (*CredentialConfigurationDTO, error) {
			return nil, ErrNotFound
		}).Maybe()
	m.EXPECT().UpdateCredentialConfiguration(mock.Anything, mock.Anything).
		Return(ErrConfigurationIsImmutable).Maybe()
	m.EXPECT().DeleteCredentialConfiguration(mock.Anything, mock.Anything).
		Return(ErrConfigurationIsImmutable).Maybe()
	return m
}

// newErrorStore returns a store mock whose every method returns the given error.
func (s *ConfigurationServiceTestSuite) newErrorStore(err error) *credentialStoreInterfaceMock {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().CreateCredentialConfiguration(mock.Anything, mock.Anything).Return(err).Maybe()
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().GetCredentialConfigurationByHandle(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().ListCredentialConfigurations(mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().ListCredentialConfigurationSummaries(mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().ListCredentialConfigurationSummariesByOUID(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().ListCredentialConfigurationSummariesByProject(mock.Anything, mock.Anything).Return(nil, err).Maybe()
	m.EXPECT().CountCredentialConfigurationsByProject(mock.Anything, mock.Anything).Return(0, err).Maybe()
	m.EXPECT().UpdateCredentialConfiguration(mock.Anything, mock.Anything).Return(err).Maybe()
	m.EXPECT().DeleteCredentialConfiguration(mock.Anything, mock.Anything).Return(err).Maybe()
	m.EXPECT().IsCredentialConfigurationDeclarative(mock.Anything, mock.Anything).Return(false, err).Maybe()
	return m
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsNegativeValidity() {
	svc := s.newService()
	negative := -1
	dto := s.validDTO()
	dto.ValiditySeconds = &negative
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidRequest.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateHandleCheckStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().GetCredentialConfigurationByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound).Maybe()
	m.EXPECT().CreateCredentialConfiguration(mock.Anything, mock.Anything).
		Return(errors.New("insert failed")).Maybe()
	svc := newCredentialConfigurationService(m, nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateOUVerificationError() {
	resolver := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	resolver.EXPECT().IsOrganizationUnitExists(mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Maybe()
	svc := newCredentialConfigurationService(
		newStatefulCredentialStore(s.T()), resolver, allowAllAuthz(s.T()), nil, false)
	dto := s.validDTO()
	dto.OUID = testOUID
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsMissingOUID() {
	resolver := newOUServiceMock(s.T(),
		map[string]bool{}, map[string]string{}, map[string]string{})
	svc := newCredentialConfigurationService(
		newStatefulCredentialStore(s.T()), resolver, allowAllAuthz(s.T()), nil, false)
	dto := s.validDTO()
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidOU.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestGetEmptyID() {
	svc := s.newService()
	_, err := svc.GetCredentialConfiguration(context.Background(), "  ")
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidRequest.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestGetStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.GetCredentialConfiguration(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestGetByHandleStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.GetCredentialConfigurationByHandle(context.Background(), "h")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.ListCredentialConfigurations(context.Background())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListResultLimitExceeded() {
	svc := newCredentialConfigurationService(
		s.newErrorStore(ErrResultLimitExceededInCompositeMode), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.ListCredentialConfigurations(context.Background())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationResultLimitExceeded.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListSummariesStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListSummariesResultLimitExceeded() {
	svc := newCredentialConfigurationService(
		s.newErrorStore(ErrResultLimitExceededInCompositeMode), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationResultLimitExceeded.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateLoadStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, mock.Anything).
		Return(nil, errors.New("boom")).Maybe()
	svc := newCredentialConfigurationService(m, nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.UpdateCredentialConfiguration(context.Background(), "cfg-1", s.validDTO())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateHandleClashCheckStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	existing := CredentialConfigurationDTO{
		ID: "cfg-1", Handle: "old-handle", VCT: "v", Format: DefaultCredentialFormat,
	}
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, "cfg-1").RunAndReturn(
		func(_ context.Context, _ string) (*CredentialConfigurationDTO, error) {
			d := existing
			return &d, nil
		}).Maybe()
	m.EXPECT().GetCredentialConfigurationByHandle(mock.Anything, mock.Anything).
		Return(nil, errors.New("boom")).Maybe()
	svc := newCredentialConfigurationService(m, nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.UpdateCredentialConfiguration(context.Background(), "cfg-1", s.validDTO())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	existing := CredentialConfigurationDTO{
		ID: "cfg-1", Handle: "eudi-pid", VCT: "v", Format: DefaultCredentialFormat,
	}
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, "cfg-1").RunAndReturn(
		func(_ context.Context, _ string) (*CredentialConfigurationDTO, error) {
			d := existing
			return &d, nil
		}).Maybe()
	m.EXPECT().UpdateCredentialConfiguration(mock.Anything, mock.Anything).
		Return(errors.New("update failed")).Maybe()
	svc := newCredentialConfigurationService(m, nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.UpdateCredentialConfiguration(context.Background(), "cfg-1", s.validDTO())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateInvalidConfig() {
	svc := s.newService()
	_, err := svc.UpdateCredentialConfiguration(
		context.Background(), "cfg-1", &CredentialConfigurationDTO{Handle: "x"})
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidRequest.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestDeleteEmptyID() {
	svc := s.newService()
	err := svc.DeleteCredentialConfiguration(context.Background(), "  ")
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidRequest.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestDeleteLoadStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, mock.Anything).
		Return(nil, errors.New("boom")).Maybe()
	svc := newCredentialConfigurationService(m, nil, allowAllAuthz(s.T()), nil, false)
	err := svc.DeleteCredentialConfiguration(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestDeleteStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	existing := CredentialConfigurationDTO{ID: "cfg-1", Handle: "h", VCT: "v", Format: DefaultCredentialFormat}
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, "cfg-1").RunAndReturn(
		func(_ context.Context, _ string) (*CredentialConfigurationDTO, error) {
			d := existing
			return &d, nil
		}).Maybe()
	m.EXPECT().DeleteCredentialConfiguration(mock.Anything, mock.Anything).
		Return(errors.New("delete failed")).Maybe()
	svc := newCredentialConfigurationService(m, nil, allowAllAuthz(s.T()), nil, false)
	err := svc.DeleteCredentialConfiguration(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestIsDeclarativeStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil, allowAllAuthz(s.T()), nil, false)
	_, err := svc.IsCredentialConfigurationDeclarative(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestPopulateOUHandleResolveError() {
	resolver := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	resolver.EXPECT().IsOrganizationUnitExists(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	resolver.EXPECT().GetOrganizationUnitByPath(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (ou.OrganizationUnit, *tidcommon.ServiceError) {
			return ou.OrganizationUnit{ID: testOUID}, nil
		}).Maybe()
	resolver.EXPECT().GetOrganizationUnitHandlesByIDs(mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Maybe()
	svc := newCredentialConfigurationService(
		newStatefulCredentialStore(s.T()), resolver, allowAllAuthz(s.T()), nil, false)

	dto := s.validDTO()
	dto.OUID = testOUID
	created, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().Nil(err)

	// Resolution failure is logged and swallowed; the call still succeeds.
	got, err := svc.GetCredentialConfiguration(context.Background(), created.ID)
	s.Require().Nil(err)
	s.Equal(testOUID, got.OUID)

	summaries, err := svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().Nil(err)
	s.Require().Len(summaries, 1)
}

func (s *ConfigurationServiceTestSuite) TestCreateResolvesAndValidatesOU() {
	resolver := newOUServiceMock(s.T(),
		map[string]bool{testOUID: true},
		map[string]string{"default": testOUID},
		map[string]string{testOUID: "default"})
	svc := newCredentialConfigurationService(
		newStatefulCredentialStore(s.T()), resolver, allowAllAuthz(s.T()), nil, false)

	dto := s.validDTO()
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidOU.Code, err.Code)

	dto = s.validDTO()
	dto.OUID = testOUID
	created, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().Nil(err)
	s.Equal(testOUID, created.OUID)

	got, err := svc.GetCredentialConfiguration(context.Background(), created.ID)
	s.Require().Nil(err)
	s.Equal(testOUID, got.OUID)
	s.Equal("default", got.OUHandle)
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsDuplicateClaimName() {
	svc := s.newService()
	dto := s.validDTO()
	dto.Claims = []ClaimMapping{
		{Name: "full_name", DisplayName: "Full Name"},
		{Name: "tier", DisplayName: "Tier"},
		{Name: "full_name", DisplayName: "Full Name Again"},
	}

	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationDuplicateClaim.Code, err.Code)
	s.Equal("full_name", err.ErrorDescription.Params["claim"])
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsReservedClaimName() {
	reserved := []string{
		"iss", "nbf", "exp", "cnf", "vct", "status",
		"_sd", "_sd_alg", "...",
		"sub", "iat",
	}
	for _, name := range reserved {
		svc := s.newService()
		dto := s.validDTO()
		dto.Claims = []ClaimMapping{{Name: name, DisplayName: "Reserved"}}

		_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
		s.Require().NotNil(err, "claim %q must be rejected", name)
		s.Equal(ErrorConfigurationReservedClaim.Code, err.Code)
		s.Equal(name, err.ErrorDescription.Params["claim"])
	}
}

func (s *ConfigurationServiceTestSuite) TestCreateRejectsEmptyClaimName() {
	svc := s.newService()
	dto := s.validDTO()
	dto.Claims = []ClaimMapping{{Name: "   ", DisplayName: "Blank"}}

	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationEmptyClaimName.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateAcceptsDistinctClaimNames() {
	svc := s.newService()
	dto := s.validDTO()
	dto.Claims = []ClaimMapping{
		{Name: "full_name", DisplayName: "Full Name"},
		{Name: "Full_Name", DisplayName: "Case Differs"},
		{Name: "tier", DisplayName: "Tier"},
	}

	created, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().Nil(err)
	s.Len(created.Claims, 3)
}

func (s *ConfigurationServiceTestSuite) TestListSummariesByOUID() {
	resolver := newOUServiceMock(s.T(),
		map[string]bool{"ou-1": true, "ou-2": true},
		map[string]string{},
		map[string]string{"ou-1": "default"})
	svc := newCredentialConfigurationService(
		newStatefulCredentialStore(s.T()), resolver, allowAllAuthz(s.T()), nil, false)
	ctx := context.Background()
	first := s.validDTO()
	first.OUID = "ou-1"
	_, svcErr := svc.CreateCredentialConfiguration(ctx, first)
	s.Require().Nil(svcErr)
	second := s.validDTO()
	second.Handle = "second-handle"
	second.VCT = "urn:second"
	second.OUID = "ou-2"
	_, svcErr = svc.CreateCredentialConfiguration(ctx, second)
	s.Require().Nil(svcErr)

	summaries, svcErr := svc.ListCredentialConfigurationSummariesByOUID(ctx, "ou-1")
	s.Require().Nil(svcErr)
	s.Require().Len(summaries, 1)
	s.Equal(first.Handle, summaries[0].Handle)
	s.Equal("default", summaries[0].OUHandle)

	summaries, svcErr = svc.ListCredentialConfigurationSummariesByOUID(ctx, "ou-missing")
	s.Require().Nil(svcErr)
	s.Empty(summaries)
}

func (s *ConfigurationServiceTestSuite) TestListSummariesByOUIDErrors() {
	testCases := []struct {
		name         string
		ouID         string
		storeErr     error
		expectedCode string
	}{
		{name: "EmptyOUID", ouID: "", expectedCode: ErrorConfigurationInvalidRequest.Code},
		{name: "StoreError", ouID: "ou-1", storeErr: errors.New("boom"),
			expectedCode: tidcommon.InternalServerError.Code},
		{name: "ResultLimitExceeded", ouID: "ou-1", storeErr: ErrResultLimitExceededInCompositeMode,
			expectedCode: ErrorConfigurationResultLimitExceeded.Code},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			svc := newCredentialConfigurationService(
				s.newErrorStore(tc.storeErr), nil, allowAllAuthz(s.T()), nil, false)
			_, svcErr := svc.ListCredentialConfigurationSummariesByOUID(context.Background(), tc.ouID)
			s.Require().NotNil(svcErr)
			s.Equal(tc.expectedCode, svcErr.Code)
		})
	}
}

// testProjectID is the project the project tests place credential configurations in.
const testProjectID = "proj-1"

// newProjectService builds a service on a deployment that serves projects, where the system organization unit
// is sys-ou and the given validator checks projects.
func (s *ConfigurationServiceTestSuite) newProjectService(validator *projectOUValidatorMock) *configurationService {
	ouMock := newOUServiceMock(s.T(), map[string]bool{"sys-ou": true, testOUID: true}, nil, nil)
	ouMock.EXPECT().GetSystemOUID(mock.Anything).Return("sys-ou", nil).Maybe()
	return newCredentialConfigurationService(
		newStatefulCredentialStore(s.T()), ouMock, allowAllAuthz(s.T()), validator, true)
}

// inProject returns a valid configuration in the test project.
func (s *ConfigurationServiceTestSuite) inProject(projectID string) *CredentialConfigurationDTO {
	dto := s.validDTO()
	dto.ProjectID = projectID
	return dto
}

func (s *ConfigurationServiceTestSuite) TestCreateInProjectPlacesTheConfigurationInTheSystemOU() {
	validator := newProjectOUValidatorMock(s.T())
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "sys-ou").Return(nil)
	svc := s.newProjectService(validator)

	created, err := svc.CreateCredentialConfiguration(context.Background(), s.inProject(testProjectID))
	s.Require().Nil(err)
	s.Equal("sys-ou", created.OUID)
	s.Equal(testProjectID, created.ProjectID)

	summaries, err := svc.ListCredentialConfigurationSummariesByProject(context.Background(), testProjectID)
	s.Require().Nil(err)
	s.Require().Len(summaries, 1)
	s.Equal(testProjectID, summaries[0].ProjectID)

	has, err := svc.ProjectHasCredentialConfigurations(context.Background(), testProjectID)
	s.Require().Nil(err)
	s.True(has)
	has, err = svc.ProjectHasCredentialConfigurations(context.Background(), "proj-2")
	s.Require().Nil(err)
	s.False(has)
}

func (s *ConfigurationServiceTestSuite) TestCreateRefusesTheProjectsTheValidatorRefuses() {
	for code, expected := range map[string]string{
		"PRJ-1002": ErrorConfigurationInvalidProject.Code,
		"PRJ-1010": ErrorConfigurationOUNotInProject.Code,
		"PRJ-9999": tidcommon.InternalServerError.Code,
	} {
		validator := newProjectOUValidatorMock(s.T())
		validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID).
			Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: code})
		svc := s.newProjectService(validator)

		dto := s.inProject(testProjectID)
		dto.OUID = testOUID
		_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
		s.Require().NotNil(err, code)
		s.Equal(expected, err.Code, code)
	}
}

func (s *ConfigurationServiceTestSuite) TestCreateRequiresAProjectWhereProjectsAreServed() {
	svc := s.newProjectService(newProjectOUValidatorMock(s.T()))

	dto := s.validDTO()
	dto.OUID = testOUID
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationProjectRequired.Code, err.Code)

	err = svc.ResolveCredentialConfigurationOUHandle(context.Background(), &CredentialConfigurationDTO{OUID: testOUID})
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationProjectRequired.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateFailsWhenTheSystemOUCannotBeRead() {
	ouMock := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouMock.EXPECT().GetSystemOUID(mock.Anything).Return("", &tidcommon.InternalServerError)
	svc := newCredentialConfigurationService(newStatefulCredentialStore(s.T()), ouMock, allowAllAuthz(s.T()),
		newProjectOUValidatorMock(s.T()), true)

	_, err := svc.CreateCredentialConfiguration(context.Background(), s.inProject(testProjectID))
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateWithoutProjectsKeepsTheProjectUnchecked() {
	svc := s.newService()

	created, err := svc.CreateCredentialConfiguration(context.Background(), s.inProject(testProjectID))
	s.Require().Nil(err)
	s.Equal(testProjectID, created.ProjectID)

	overlong := s.inProject(fmt.Sprintf("%037d", 0))
	overlong.Handle = "eudi-pid-2"
	_, err = svc.CreateCredentialConfiguration(context.Background(), overlong)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidProject.Code, err.Code)
}

// An update that moves a configuration to another organization unit of the same project checks the project again.
func (s *ConfigurationServiceTestSuite) TestUpdateToAnotherOUInTheProjectIsChecked() {
	validator := newProjectOUValidatorMock(s.T())
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "sys-ou").Return(nil).Once()
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, testOUID).
		Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: projectscope.OUNotInProjectErrorCode}).
		Once()
	svc := s.newProjectService(validator)
	ctx := context.Background()
	created, err := svc.CreateCredentialConfiguration(ctx, s.inProject(testProjectID))
	s.Require().Nil(err)

	moved := s.inProject(testProjectID)
	moved.OUID = testOUID
	_, err = svc.UpdateCredentialConfiguration(ctx, created.ID, moved)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationOUNotInProject.Code, err.Code)

	stored, err := svc.GetCredentialConfiguration(ctx, created.ID)
	s.Require().Nil(err)
	s.Equal("sys-ou", stored.OUID)
}

func (s *ConfigurationServiceTestSuite) TestUpdateKeepsOrMovesTheProject() {
	validator := newProjectOUValidatorMock(s.T())
	validator.EXPECT().ValidateProjectOU(mock.Anything, testProjectID, "sys-ou").Return(nil).Once()
	validator.EXPECT().ValidateProjectOU(mock.Anything, "proj-2", "sys-ou").Return(nil).Once()
	validator.EXPECT().ValidateProjectOU(mock.Anything, "proj-gone", "sys-ou").
		Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "PRJ-1002"}).Once()
	svc := s.newProjectService(validator)
	ctx := context.Background()
	created, err := svc.CreateCredentialConfiguration(ctx, s.inProject(testProjectID))
	s.Require().Nil(err)

	renamed := s.validDTO()
	renamed.Name = "Renamed"
	updated, err := svc.UpdateCredentialConfiguration(ctx, created.ID, renamed)
	s.Require().Nil(err)
	s.Equal(testProjectID, updated.ProjectID, "an update without a project keeps it")
	s.Equal("sys-ou", updated.OUID, "an update without an organization unit keeps it")

	updated, err = svc.UpdateCredentialConfiguration(ctx, created.ID, s.inProject("proj-2"))
	s.Require().Nil(err)
	s.Equal("proj-2", updated.ProjectID)

	_, err = svc.UpdateCredentialConfiguration(ctx, created.ID, s.inProject("proj-gone"))
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidProject.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListByProject() {
	svc := s.newService()

	summaries, err := svc.ListCredentialConfigurationSummariesByProject(context.Background(), "")
	s.Require().Nil(err)
	s.Empty(summaries, "an empty project lists no configuration")

	errSvc := newCredentialConfigurationService(
		s.newErrorStore(errors.New("db boom")), nil, allowAllAuthz(s.T()), nil, false)
	_, err = errSvc.ListCredentialConfigurationSummariesByProject(context.Background(), testProjectID)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
	_, err = errSvc.ProjectHasCredentialConfigurations(context.Background(), testProjectID)
	s.Require().NotNil(err)

	limitSvc := newCredentialConfigurationService(
		s.newErrorStore(ErrResultLimitExceededInCompositeMode), nil, allowAllAuthz(s.T()), nil, false)
	_, err = limitSvc.ListCredentialConfigurationSummariesByProject(context.Background(), testProjectID)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationResultLimitExceeded.Code, err.Code)
}
