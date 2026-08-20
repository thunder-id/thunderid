// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
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

	resource, err := parseToConfigurationDTOWrapper([]byte("id: cfg-1\nhandle: h\nvct: v\nouId: ou-1\n"))
	s.Require().NoError(err)
	s.Require().NoError(validateConfigurationWrapper(resource, fileStore, nil, nil))
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
	s.Error(validateConfigurationWrapper(dto, nil, nil, nil))
}

func (s *ConfigurationExporterTestSuite) TestValidateConfigurationWrapperRejectsInvalidConfig() {
	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "", VCT: "v"}
	s.Error(validateConfigurationWrapper(dto, nil, nil, nil))
}

func (s *ConfigurationExporterTestSuite) TestValidateConfigurationWrapperRejectsWrongType() {
	s.Error(validateConfigurationWrapper("not-a-dto", nil, nil, nil))
}

// TestDeclarativeYAMLCarriesOU verifies the declarative YAML shape accepts the same
// organization unit the management API requires, by ID or by handle.
func (s *ConfigurationExporterTestSuite) TestDeclarativeYAMLCarriesOU() {
	parsed, err := parseToConfigurationDTOWrapper(
		[]byte("id: cfg-1\nhandle: h\nvct: v\nouId: ou-123\n"))
	s.Require().NoError(err)
	s.Equal("ou-123", parsed.(*CredentialConfigurationDTO).OUID)

	parsed, err = parseToConfigurationDTOWrapper(
		[]byte("id: cfg-2\nhandle: h2\nvct: v\nouHandle: root/eng\n"))
	s.Require().NoError(err)
	s.Equal("root/eng", parsed.(*CredentialConfigurationDTO).OUHandle)
}

// TestValidateResolvesOUHandle verifies an ouHandle is resolved to an ouId at load time
// and that only the resolved ID is retained on the stored resource.
func (s *ConfigurationExporterTestSuite) TestValidateResolvesOUHandle() {
	ouSvc := newOUServiceMock(s.T(), map[string]bool{"ou-123": true},
		map[string]string{"root/eng": "ou-123"}, map[string]string{"ou-123": "root/eng"})

	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "h", VCT: "v", OUHandle: "root/eng"}
	s.Require().NoError(validateConfigurationWrapper(dto, nil, nil, ouSvc))
	s.Equal("ou-123", dto.OUID)
}

// TestValidateRejectsUnknownOUHandle verifies an unresolvable handle fails the load
// rather than silently producing a configuration with no organization unit.
func (s *ConfigurationExporterTestSuite) TestValidateRejectsUnknownOUHandle() {
	ouSvc := newOUServiceMock(s.T(), map[string]bool{}, map[string]string{}, map[string]string{})

	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "h", VCT: "v", OUHandle: "no/such/ou"}
	err := validateConfigurationWrapper(dto, nil, nil, ouSvc)
	s.Require().Error(err)
	s.Contains(err.Error(), "no/such/ou")
}

// TestValidateRejectsMissingOU verifies a declarative configuration without an organization
// unit is rejected, matching what the management API enforces on create.
func (s *ConfigurationExporterTestSuite) TestValidateRejectsMissingOU() {
	dto := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "h", VCT: "v"}
	err := validateConfigurationWrapper(dto, nil, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "ouId or ouHandle is required")
}

// TestValidateRejectsDuplicateIDAndHandle verifies a second declarative file cannot
// silently overwrite or shadow a configuration already loaded from another file.
func (s *ConfigurationExporterTestSuite) TestValidateRejectsDuplicateIDAndHandle() {
	fileStore := newCredentialFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	storer := &credentialStorer{store: fileStore}

	first := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "h", VCT: "v", OUID: "ou-1"}
	s.Require().NoError(validateConfigurationWrapper(first, fileStore, nil, nil))
	s.Require().NoError(storer.Create(first.ID, first))

	dupID := &CredentialConfigurationDTO{ID: "cfg-1", Handle: "other", VCT: "v", OUID: "ou-1"}
	err := validateConfigurationWrapper(dupID, fileStore, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "duplicate credential configuration ID")

	dupHandle := &CredentialConfigurationDTO{ID: "cfg-2", Handle: "h", VCT: "v", OUID: "ou-1"}
	err = validateConfigurationWrapper(dupHandle, fileStore, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "duplicate credential configuration handle")
}

// TestValidateRejectsIDAlreadyInDatabase verifies a declarative configuration cannot claim
// an ID a runtime configuration already owns, which in composite mode would otherwise be
// shadowed by the database entry on every read.
func (s *ConfigurationExporterTestSuite) TestValidateRejectsIDAlreadyInDatabase() {
	dbStore := newStatefulCredentialStore(s.T())
	s.Require().NoError(dbStore.CreateCredentialConfiguration(context.Background(),
		CredentialConfigurationDTO{ID: "cfg-db", Handle: "runtime_handle", VCT: "v", OUID: "ou-1"}))

	dto := &CredentialConfigurationDTO{ID: "cfg-db", Handle: "declarative_handle", VCT: "v", OUID: "ou-1"}
	err := validateConfigurationWrapper(dto, nil, dbStore, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "already exists in the database store")
}

// TestValidateAllowsUnusedIDWithDatabaseStore verifies the composite-mode duplicate check
// lets a fresh ID through rather than rejecting every declarative resource.
func (s *ConfigurationExporterTestSuite) TestValidateAllowsUnusedIDWithDatabaseStore() {
	dbStore := newStatefulCredentialStore(s.T())

	dto := &CredentialConfigurationDTO{ID: "cfg-fresh", Handle: "fresh_handle", VCT: "v", OUID: "ou-1"}
	s.NoError(validateConfigurationWrapper(dto, nil, dbStore, nil))
}

// TestValidateSurfacesDatabaseLookupFailure verifies a database error during the duplicate
// check fails the load instead of being mistaken for "no duplicate".
func (s *ConfigurationExporterTestSuite) TestValidateSurfacesDatabaseLookupFailure() {
	dbStore := newCredentialStoreInterfaceMock(s.T())
	dbStore.EXPECT().GetCredentialConfigurationByID(mock.Anything, mock.Anything).
		Return(nil, errors.New("db boom"))

	dto := &CredentialConfigurationDTO{ID: "cfg-x", Handle: "h", VCT: "v", OUID: "ou-1"}
	err := validateConfigurationWrapper(dto, nil, dbStore, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to check for duplicate")
}

// TestLoadDeclarativeResourcesNoResources verifies a server home without a resources
// directory loads cleanly rather than failing startup.
func (s *ConfigurationExporterTestSuite) TestLoadDeclarativeResourcesNoResources() {
	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))

	fileStore := newCredentialFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())

	s.Require().NoError(loadDeclarativeResources(fileStore, nil, nil))
}

// TestLoadDeclarativeResourcesRejectsInvalidFile verifies a malformed declarative file
// fails the load, so the server refuses to start rather than silently skipping it.
func (s *ConfigurationExporterTestSuite) TestLoadDeclarativeResourcesRejectsInvalidFile() {
	home := s.T().TempDir()
	dir := filepath.Join(home, "config", "resources", "credential_configurations")
	s.Require().NoError(os.MkdirAll(dir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("id: [unterminated"), 0o600))

	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(home, &config.Config{}))

	fileStore := newCredentialFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())

	err := loadDeclarativeResources(fileStore, nil, nil)
	s.Require().Error(err, "a malformed declarative file must fail the load")
	s.Contains(err.Error(), "failed to load credential configuration resources")
}

// TestLoadDeclarativeResourcesFromDisk exercises the whole loader path against a real file:
// parse, validate, resolve the organization unit handle, and store under the resource ID.
func (s *ConfigurationExporterTestSuite) TestLoadDeclarativeResourcesFromDisk() {
	home := s.T().TempDir()
	dir := filepath.Join(home, "config", "resources", "credential_configurations")
	s.Require().NoError(os.MkdirAll(dir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "cfg.yaml"), []byte(
		"id: cfg-disk\nhandle: disk_handle\nvct: urn:example:vct\nouHandle: root/eng\n"), 0o600))

	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(home, &config.Config{}))

	ouSvc := newOUServiceMock(s.T(), map[string]bool{"ou-123": true},
		map[string]string{"root/eng": "ou-123"}, map[string]string{"ou-123": "root/eng"})

	fileStore := newCredentialFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	s.Require().NoError(loadDeclarativeResources(fileStore, nil, ouSvc))

	got, err := fileStore.GetCredentialConfigurationByID(context.Background(), "cfg-disk")
	s.Require().NoError(err)
	s.Equal("disk_handle", got.Handle)
	s.Equal("ou-123", got.OUID, "ouHandle must be resolved to an ouId at load time")
}
