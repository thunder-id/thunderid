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

package credential

import (
	"context"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type ConfigurationServiceTestSuite struct {
	suite.Suite
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
	return newCredentialConfigurationService(newStatefulCredentialStore(s.T()), nil)
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
		map[string]bool{"ou-1": true},
		map[string]string{"default": "ou-1"},
		map[string]string{"ou-1": "default"})
	svc := newCredentialConfigurationService(newStatefulCredentialStore(s.T()), resolver)

	dto := s.validDTO()
	dto.OUHandle = "default"
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
	svc := newCredentialConfigurationService(store, nil)
	_, err := svc.UpdateCredentialConfiguration(context.Background(), "cfg-1", s.validDTO())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationImmutable.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestDeleteImmutableMapsToImmutableError() {
	store := s.newImmutableStore()
	svc := newCredentialConfigurationService(store, nil)
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
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil)
	_, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().GetCredentialConfigurationByHandle(mock.Anything, mock.Anything).Return(nil, ErrNotFound).Maybe()
	m.EXPECT().CreateCredentialConfiguration(mock.Anything, mock.Anything).
		Return(errors.New("insert failed")).Maybe()
	svc := newCredentialConfigurationService(m, nil)
	_, err := svc.CreateCredentialConfiguration(context.Background(), s.validDTO())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateOUVerificationError() {
	resolver := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	resolver.EXPECT().IsOrganizationUnitExists(mock.Anything, mock.Anything).
		Return(false, &tidcommon.InternalServerError).Maybe()
	svc := newCredentialConfigurationService(newStatefulCredentialStore(s.T()), resolver)
	dto := s.validDTO()
	dto.OUID = "ou-1"
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestCreateOUResolveByPathError() {
	resolver := newOUServiceMock(s.T(),
		map[string]bool{}, map[string]string{}, map[string]string{})
	svc := newCredentialConfigurationService(newStatefulCredentialStore(s.T()), resolver)
	dto := s.validDTO()
	dto.OUHandle = "unknown-path"
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
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil)
	_, err := svc.GetCredentialConfiguration(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestGetByHandleStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil)
	_, err := svc.GetCredentialConfigurationByHandle(context.Background(), "h")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil)
	_, err := svc.ListCredentialConfigurations(context.Background())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListResultLimitExceeded() {
	svc := newCredentialConfigurationService(
		s.newErrorStore(ErrResultLimitExceededInCompositeMode), nil)
	_, err := svc.ListCredentialConfigurations(context.Background())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationResultLimitExceeded.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListSummariesStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil)
	_, err := svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestListSummariesResultLimitExceeded() {
	svc := newCredentialConfigurationService(
		s.newErrorStore(ErrResultLimitExceededInCompositeMode), nil)
	_, err := svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationResultLimitExceeded.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestUpdateLoadStoreError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().GetCredentialConfigurationByID(mock.Anything, mock.Anything).
		Return(nil, errors.New("boom")).Maybe()
	svc := newCredentialConfigurationService(m, nil)
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
	svc := newCredentialConfigurationService(m, nil)
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
	svc := newCredentialConfigurationService(m, nil)
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
	svc := newCredentialConfigurationService(m, nil)
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
	svc := newCredentialConfigurationService(m, nil)
	err := svc.DeleteCredentialConfiguration(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestIsDeclarativeStoreError() {
	svc := newCredentialConfigurationService(s.newErrorStore(errors.New("boom")), nil)
	_, err := svc.IsCredentialConfigurationDeclarative(context.Background(), "cfg-1")
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *ConfigurationServiceTestSuite) TestPopulateOUHandleResolveError() {
	resolver := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	resolver.EXPECT().IsOrganizationUnitExists(mock.Anything, mock.Anything).Return(true, nil).Maybe()
	resolver.EXPECT().GetOrganizationUnitByPath(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, _ string) (providers.OrganizationUnit, *tidcommon.ServiceError) {
			return providers.OrganizationUnit{ID: "ou-1"}, nil
		}).Maybe()
	resolver.EXPECT().GetOrganizationUnitHandlesByIDs(mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError).Maybe()
	svc := newCredentialConfigurationService(newStatefulCredentialStore(s.T()), resolver)

	dto := s.validDTO()
	dto.OUID = "ou-1"
	created, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().Nil(err)

	// Resolution failure is logged and swallowed; the call still succeeds.
	got, err := svc.GetCredentialConfiguration(context.Background(), created.ID)
	s.Require().Nil(err)
	s.Equal("ou-1", got.OUID)

	summaries, err := svc.ListCredentialConfigurationSummaries(context.Background())
	s.Require().Nil(err)
	s.Require().Len(summaries, 1)
}

func (s *ConfigurationServiceTestSuite) TestCreateResolvesAndValidatesOU() {
	resolver := newOUServiceMock(s.T(),
		map[string]bool{"ou-1": true},
		map[string]string{"default": "ou-1"},
		map[string]string{"ou-1": "default"})
	svc := newCredentialConfigurationService(newStatefulCredentialStore(s.T()), resolver)

	dto := s.validDTO()
	_, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationInvalidOU.Code, err.Code)

	dto = s.validDTO()
	dto.OUHandle = "default"
	created, err := svc.CreateCredentialConfiguration(context.Background(), dto)
	s.Require().Nil(err)
	s.Equal("ou-1", created.OUID)

	got, err := svc.GetCredentialConfiguration(context.Background(), created.ID)
	s.Require().Nil(err)
	s.Equal("ou-1", got.OUID)
	s.Equal("default", got.OUHandle)
}
