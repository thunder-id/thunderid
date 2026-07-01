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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

type ConfigurationExporterTestSuite struct {
	suite.Suite
	svc      CredentialConfigurationServiceInterface
	store    *credentialStoreInterfaceMock
	exporter declarativeresource.ResourceExporter
	logger   *log.Logger
}

func TestConfigurationExporterTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigurationExporterTestSuite))
}

func (s *ConfigurationExporterTestSuite) SetupTest() {
	s.store = newStatefulCredentialStore(s.T())
	s.svc = newCredentialConfigurationService(s.store, nil)
	s.exporter = newConfigurationExporter(s.svc)
	s.logger = log.GetLogger()
}

func (s *ConfigurationExporterTestSuite) seed(id, handle, vct string) {
	s.Require().NoError(s.store.CreateCredentialConfiguration(context.Background(), CredentialConfigurationDTO{
		ID:     id,
		Handle: handle,
		VCT:    vct,
		Format: DefaultCredentialFormat,
	}))
}

func (s *ConfigurationExporterTestSuite) TestGetResourceType() {
	s.Equal("credential_configuration", s.exporter.GetResourceType())
}

func (s *ConfigurationExporterTestSuite) TestGetParameterizerType() {
	s.Equal("CredentialConfiguration", s.exporter.GetParameterizerType())
}

func (s *ConfigurationExporterTestSuite) TestGetAllResourceIDs_Success() {
	s.seed("cfg1", "handle-1", "urn:eudi:pid:1")
	s.seed("cfg2", "handle-2", "urn:eudi:pid:2")

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	s.Nil(err)
	s.Len(ids, 2)
	s.ElementsMatch([]string{"cfg1", "cfg2"}, ids)
}

func (s *ConfigurationExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	s.Nil(err)
	s.Len(ids, 0)
}

func (s *ConfigurationExporterTestSuite) TestGetAllResourceIDs_ListError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().ListCredentialConfigurations(mock.Anything).
		Return(nil, errors.New("boom")).Maybe()
	svc := newCredentialConfigurationService(m, nil)
	exporter := newConfigurationExporter(svc)

	ids, err := exporter.GetAllResourceIDs(context.Background())
	s.Nil(ids)
	s.Require().NotNil(err)
}

func (s *ConfigurationExporterTestSuite) TestGetAllResourceIDs_IsDeclarativeError() {
	m := newCredentialStoreInterfaceMock(s.T())
	m.EXPECT().ListCredentialConfigurations(mock.Anything).RunAndReturn(
		func(_ context.Context) ([]CredentialConfigurationDTO, error) {
			return []CredentialConfigurationDTO{
				{ID: "cfg-1", Handle: "h", VCT: "v", Format: DefaultCredentialFormat},
			}, nil
		}).Maybe()
	m.EXPECT().IsCredentialConfigurationDeclarative(mock.Anything, mock.Anything).
		Return(false, errors.New("boom")).Maybe()
	svc := newCredentialConfigurationService(m, nil)
	exporter := newConfigurationExporter(svc)

	ids, err := exporter.GetAllResourceIDs(context.Background())
	s.Nil(ids)
	s.Require().NotNil(err)
}

func (s *ConfigurationExporterTestSuite) TestGetResourceByID_Success() {
	s.seed("cfg1", "handle-1", "urn:eudi:pid:1")

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "cfg1")

	s.Nil(err)
	s.Equal("handle-1", name)
	dto, ok := resource.(*CredentialConfigurationDTO)
	s.Require().True(ok)
	s.Equal("cfg1", dto.ID)
	s.Equal("urn:eudi:pid:1", dto.VCT)
	s.Empty(dto.OUHandle)
}

func (s *ConfigurationExporterTestSuite) TestGetResourceByID_NotFound() {
	resource, name, err := s.exporter.GetResourceByID(context.Background(), "missing")

	s.Nil(resource)
	s.Empty(name)
	s.Require().NotNil(err)
	s.Equal(ErrorConfigurationNotFound.Code, err.Code)
}

func (s *ConfigurationExporterTestSuite) TestValidateResource_Success() {
	dto := &CredentialConfigurationDTO{ID: "cfg1", Handle: "handle-1", VCT: "v"}

	name, err := s.exporter.ValidateResource(context.Background(), dto, "cfg1", s.logger)

	s.Nil(err)
	s.Equal("handle-1", name)
}

func (s *ConfigurationExporterTestSuite) TestValidateResource_InvalidType() {
	name, err := s.exporter.ValidateResource(context.Background(), "not-a-configuration", "cfg1", s.logger)

	s.Empty(name)
	s.Require().NotNil(err)
	s.Equal("credential_configuration", err.ResourceType)
	s.Equal("cfg1", err.ResourceID)
	s.Equal("INVALID_TYPE", err.Code)
}

func (s *ConfigurationExporterTestSuite) TestValidateResource_EmptyHandle() {
	dto := &CredentialConfigurationDTO{ID: "cfg1", Handle: "", VCT: "v"}

	name, err := s.exporter.ValidateResource(context.Background(), dto, "cfg1", s.logger)

	s.Empty(name)
	s.Require().NotNil(err)
	s.Equal("credential_configuration", err.ResourceType)
	s.Equal("cfg1", err.ResourceID)
}

func (s *ConfigurationExporterTestSuite) TestGetResourceRules() {
	rules := s.exporter.GetResourceRules()

	s.Require().NotNil(rules)
	s.Empty(rules.ArrayVariables)
	s.Empty(rules.DynamicPropertyFields)
}

func (s *ConfigurationExporterTestSuite) TestParseToConfigurationDTO() {
	yamlDoc := []byte(`
id: cfg-1
handle: eudi-pid
name: EUDI PID
format: dc+sd-jwt
vct: urn:eudi:pid:de:1
claims:
  - name: given_name
    displayName: Given Name
  - name: family_name
    displayName: Family Name
display:
  locale: en-US
  logoUri: https://example.com/logo.png
validitySeconds: 3600
`)

	resource, err := parseToConfigurationDTOWrapper(yamlDoc)
	s.Require().NoError(err)
	dto, ok := resource.(*CredentialConfigurationDTO)
	s.Require().True(ok)
	s.Equal("cfg-1", dto.ID)
	s.Equal("eudi-pid", dto.Handle)
	s.Equal("urn:eudi:pid:de:1", dto.VCT)
	s.Equal(DefaultCredentialFormat, dto.Format)
	s.Require().Len(dto.Claims, 2)
	s.Equal("given_name", dto.Claims[0].Name)
	s.Equal("EUDI PID", dto.Name)
	s.Require().NotNil(dto.Display)
	s.Equal("en-US", dto.Display.Locale)
	s.Require().NotNil(dto.ValiditySeconds)
	s.Equal(3600, *dto.ValiditySeconds)
}

func (s *ConfigurationExporterTestSuite) TestParseToConfigurationDTO_InvalidYAML() {
	_, err := parseToConfigurationDTOWrapper([]byte("id: [unterminated"))
	s.Error(err)
}

func (s *ConfigurationExporterTestSuite) TestLoadResourcesThroughStorer() {
	// Parse a YAML doc and store it via the storer the loader writes through, then
	// read it back from the file store.
	fileStore := newCredentialFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	storer := &credentialStorer{store: fileStore}

	resource, err := parseToConfigurationDTOWrapper([]byte("id: cfg-1\nhandle: h\nvct: v\n"))
	s.Require().NoError(err)
	s.Require().NoError(validateConfigurationWrapper(resource))
	dto := resource.(*CredentialConfigurationDTO)
	s.Require().NoError(storer.Create(dto.ID, dto))

	got, err := fileStore.GetCredentialConfigurationByID(context.Background(), "cfg-1")
	s.Require().NoError(err)
	s.Equal("h", got.Handle)
}

func (s *ConfigurationExporterTestSuite) TestExportImportRoundTrip() {
	validity := 3600
	original := &CredentialConfigurationDTO{
		ID:     "cfg-1",
		Handle: "eudi-pid",
		Name:   "EUDI PID",
		VCT:    "urn:eudi:pid:de:1",
		Format: DefaultCredentialFormat,
		Claims: []ClaimMapping{
			{Name: "given_name", DisplayName: "Given Name"},
			{Name: "family_name", DisplayName: "Family Name"},
		},
		Display: &CredentialDisplay{
			Locale:  "en-US",
			LogoURI: "https://example.com/logo.png",
		},
		ValiditySeconds: &validity,
	}

	exported, err := yaml.Marshal(original)
	s.Require().NoError(err)

	resource, err := parseToConfigurationDTOWrapper(exported)
	s.Require().NoError(err)
	s.Equal(original, resource, "exported YAML must round-trip back to the same configuration")
}

func (s *ConfigurationExporterTestSuite) TestValidateConfigurationWrapperRejectsMissingID() {
	dto := &CredentialConfigurationDTO{Handle: "h", VCT: "v"}
	s.Error(validateConfigurationWrapper(dto))
}

func (s *ConfigurationExporterTestSuite) TestValidateConfigurationWrapperRejectsInvalidConfig() {
	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "", VCT: "v"}
	s.Error(validateConfigurationWrapper(dto))
}

func (s *ConfigurationExporterTestSuite) TestValidateConfigurationWrapperRejectsWrongType() {
	s.Error(validateConfigurationWrapper("not-a-dto"))
}
