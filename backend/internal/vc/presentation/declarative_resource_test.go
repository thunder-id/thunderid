// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type DefinitionExporterTestSuite struct {
	suite.Suite
	svc      *definitionService
	store    *definitionStoreInterfaceMock
	exporter declarativeresource.ResourceExporter
	logger   *log.Logger
}

func TestDefinitionExporterTestSuite(t *testing.T) {
	suite.Run(t, new(DefinitionExporterTestSuite))
}

func (s *DefinitionExporterTestSuite) SetupTest() {
	s.svc, s.store = newTestDefinitionService(s.T())
	s.exporter = newDefinitionExporter(s.svc)
	s.logger = log.GetLogger()
}

func (s *DefinitionExporterTestSuite) seed(id, handle, vct string) {
	s.Require().NoError(s.store.CreatePresentationDefinition(context.Background(), PresentationDefinitionDTO{
		ID:     id,
		Handle: handle,
		VCT:    vct,
		Format: DefaultCredentialFormat,
	}))
}

func (s *DefinitionExporterTestSuite) TestGetResourceType() {
	s.Equal("presentation_definition", s.exporter.GetResourceType())
}

func (s *DefinitionExporterTestSuite) TestGetParameterizerType() {
	s.Equal("PresentationDefinition", s.exporter.GetParameterizerType())
}

func (s *DefinitionExporterTestSuite) TestGetAllResourceIDs_Success() {
	s.seed("def1", "handle-1", "urn:eudi:pid:1")
	s.seed("def2", "handle-2", "urn:eudi:pid:2")

	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	s.Nil(err)
	s.Len(ids, 2)
	s.ElementsMatch([]string{"def1", "def2"}, ids)
}

func (s *DefinitionExporterTestSuite) TestGetAllResourceIDs_EmptyList() {
	ids, err := s.exporter.GetAllResourceIDs(context.Background())

	s.Nil(err)
	s.Len(ids, 0)
}

func (s *DefinitionExporterTestSuite) TestGetResourceByID_Success() {
	s.seed("def1", "handle-1", "urn:eudi:pid:1")

	resource, name, err := s.exporter.GetResourceByID(context.Background(), "def1")

	s.Nil(err)
	s.Equal("handle-1", name)
	dto, ok := resource.(*PresentationDefinitionDTO)
	s.Require().True(ok)
	s.Equal("def1", dto.ID)
	s.Equal("urn:eudi:pid:1", dto.VCT)
}

func (s *DefinitionExporterTestSuite) TestGetResourceByID_NotFound() {
	resource, name, err := s.exporter.GetResourceByID(context.Background(), "missing")

	s.Nil(resource)
	s.Empty(name)
	s.Require().NotNil(err)
	s.Equal(ErrorDefinitionNotFound.Code, err.Code)
}

func (s *DefinitionExporterTestSuite) TestValidateResource_Success() {
	dto := &PresentationDefinitionDTO{ID: "def1", Handle: "handle-1", VCT: "v"}

	name, err := s.exporter.ValidateResource(context.Background(), dto, "def1", s.logger)

	s.Nil(err)
	s.Equal("handle-1", name)
}

func (s *DefinitionExporterTestSuite) TestValidateResource_InvalidType() {
	name, err := s.exporter.ValidateResource(context.Background(), "not-a-definition", "def1", s.logger)

	s.Empty(name)
	s.Require().NotNil(err)
	s.Equal("presentation_definition", err.ResourceType)
	s.Equal("def1", err.ResourceID)
	s.Equal("INVALID_TYPE", err.Code)
}

func (s *DefinitionExporterTestSuite) TestValidateResource_EmptyHandle() {
	dto := &PresentationDefinitionDTO{ID: "def1", Handle: "", VCT: "v"}

	name, err := s.exporter.ValidateResource(context.Background(), dto, "def1", s.logger)

	s.Empty(name)
	s.Require().NotNil(err)
	s.Equal("presentation_definition", err.ResourceType)
	s.Equal("def1", err.ResourceID)
}

func (s *DefinitionExporterTestSuite) TestGetResourceRules() {
	rules := s.exporter.GetResourceRules()

	s.Require().NotNil(rules)
	s.Contains(rules.ArrayVariables, "TrustedAuthorities")
	s.Contains(rules.DynamicPropertyFields, "ClaimValues")
}

func (s *DefinitionExporterTestSuite) TestParseToDefinitionDTO() {
	yamlDoc := []byte(`
id: def-1
handle: eudi-pid
name: EUDI PID
vct: urn:eudi:pid:de:1
format: dc+sd-jwt
mandatoryClaims:
  - given_name
  - family_name
optionalClaims:
  - birthdate
claimValues:
  address.country:
    - DE
    - AT
enforceTrustedIssuer: true
trustedAuthorities:
  - root-a
  - root-b
`)

	dto, err := parseToDefinitionDTO(yamlDoc)
	s.Require().NoError(err)
	s.Equal("def-1", dto.ID)
	s.Equal("eudi-pid", dto.Handle)
	s.Equal("urn:eudi:pid:de:1", dto.VCT)
	s.Equal(DefaultCredentialFormat, dto.Format)
	s.Equal([]string{"given_name", "family_name"}, dto.MandatoryClaims)
	s.Equal([]string{"DE", "AT"}, dto.ClaimValues["address.country"])
	s.Require().NotNil(dto.EnforceTrustedIssuer)
	s.True(*dto.EnforceTrustedIssuer)
	s.Equal([]string{"root-a", "root-b"}, dto.TrustedAuthorities)
}

func (s *DefinitionExporterTestSuite) TestLoadResourcesThroughStorer() {
	// IDExtractor + Validator wiring: parse a YAML doc and store it via the storer
	// the loader writes through, then read it back from the file store.
	fileStore := newDefinitionFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	storer := &definitionStorer{store: fileStore}

	dto, err := parseToDefinitionDTO([]byte("id: def-1\nhandle: h\nvct: v\nouId: ou-1\n"))
	s.Require().NoError(err)
	s.Require().NoError(validateDefinitionWrapper(dto, fileStore, nil, nil))
	s.Require().NoError(storer.Create(dto.ID, dto))

	got, err := fileStore.GetPresentationDefinitionByID(context.Background(), "def-1")
	s.Require().NoError(err)
	s.Equal("h", got.Handle)
}

func (s *DefinitionExporterTestSuite) TestExportImportRoundTrip() {
	// The export parameterizer serializes a resource using its struct yaml tags
	// (field.Tag.Get("yaml")), which is exactly what yaml.Marshal does. Re-importing
	// the result through the loader parser must reproduce the definition exactly —
	// proving the exported keys match the keys the importer expects. Without yaml tags
	// on the DTO, every field would be dropped and this fails.
	enforce := true
	original := &PresentationDefinitionDTO{
		ID:                   "def-1",
		Handle:               "eudi-pid",
		Name:                 "EUDI PID",
		VCT:                  "urn:eudi:pid:de:1",
		Format:               DefaultCredentialFormat,
		RequestedClaims:      []string{"given_name"},
		MandatoryClaims:      []string{"given_name", "family_name"},
		OptionalClaims:       []string{"birthdate"},
		ClaimValues:          map[string][]string{"address.country": {"DE", "AT"}},
		EnforceTrustedIssuer: &enforce,
		TrustedAuthorities:   []string{"root-a", "root-b"},
	}

	exported, err := yaml.Marshal(original)
	s.Require().NoError(err)

	reimported, err := parseToDefinitionDTO(exported)
	s.Require().NoError(err)
	s.Equal(original, reimported, "exported YAML must round-trip back to the same definition")
}

func (s *DefinitionExporterTestSuite) TestValidateDefinitionWrapperRejectsMissingID() {
	dto := &PresentationDefinitionDTO{Handle: "h", VCT: "v"}
	s.Error(validateDefinitionWrapper(dto, nil, nil, nil))
}

func (s *DefinitionExporterTestSuite) TestValidateDefinitionWrapperRejectsInvalidDefinition() {
	// ID is present, so the wrapper proceeds to validateDefinition, which rejects
	// the missing VCT.
	dto := &PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: ""}
	s.Error(validateDefinitionWrapper(dto, nil, nil, nil))
}

func (s *DefinitionExporterTestSuite) TestParseToDefinitionDTOInvalidYAML() {
	_, err := parseToDefinitionDTO([]byte("id: [unterminated"))
	s.Error(err)
}

func (s *DefinitionExporterTestSuite) TestGetAllResourceIDsListError() {
	store := newDefinitionStoreInterfaceMock(s.T())
	store.EXPECT().ListPresentationDefinitions(mock.Anything).Return(nil, errors.New("db boom"))
	svc := newPresentationDefinitionService(store, nil)
	exporter := newDefinitionExporter(svc)

	_, err := exporter.GetAllResourceIDs(context.Background())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *DefinitionExporterTestSuite) TestGetAllResourceIDsIsDeclarativeError() {
	store := newDefinitionStoreInterfaceMock(s.T())
	store.EXPECT().ListPresentationDefinitions(mock.Anything).Return(
		[]PresentationDefinitionDTO{{ID: "def-1", Handle: "h", VCT: "v"}}, nil)
	store.EXPECT().IsPresentationDefinitionDeclarative(mock.Anything, mock.Anything).Return(
		false, errors.New("db boom"))
	svc := newPresentationDefinitionService(store, nil)
	exporter := newDefinitionExporter(svc)

	_, err := exporter.GetAllResourceIDs(context.Background())
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *DefinitionExporterTestSuite) TestGetAllResourceIDsExcludesDeclarative() {
	store := newDefinitionStoreInterfaceMock(s.T())
	store.EXPECT().ListPresentationDefinitions(mock.Anything).Return(
		[]PresentationDefinitionDTO{
			{ID: "mutable", Handle: "h1", VCT: "v"},
			{ID: "declarative", Handle: "h2", VCT: "v"},
		}, nil)
	store.EXPECT().IsPresentationDefinitionDeclarative(mock.Anything, "mutable").Return(false, nil)
	store.EXPECT().IsPresentationDefinitionDeclarative(mock.Anything, "declarative").Return(true, nil)
	svc := newPresentationDefinitionService(store, nil)
	exporter := newDefinitionExporter(svc)

	ids, err := exporter.GetAllResourceIDs(context.Background())
	s.Nil(err)
	s.Equal([]string{"mutable"}, ids)
}

func (s *DefinitionExporterTestSuite) TestValidateDefinitionWrapperRejectsWrongType() {
	s.Error(validateDefinitionWrapper("not-a-dto", nil, nil, nil))
}

func (s *DefinitionExporterTestSuite) TestParseToDefinitionDTOWrapper() {
	parsed, err := parseToDefinitionDTOWrapper([]byte("id: def-1\nhandle: h\nvct: v\n"))
	s.Require().NoError(err)
	dto, ok := parsed.(*PresentationDefinitionDTO)
	s.Require().True(ok)
	s.Equal("def-1", dto.ID)
	s.Equal("h", dto.Handle)
}

func (s *DefinitionExporterTestSuite) TestLoadDeclarativeResourcesNoResources() {
	// With a server home that has no resources directory, the loader resolves to an
	// empty resource set and completes without error, exercising loadDeclarativeResources.
	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))

	fileStore := newDefinitionFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())

	s.Require().NoError(loadDeclarativeResources(fileStore, nil, nil))
}

// TestDeclarativeYAMLCarriesOU verifies the declarative YAML shape accepts the same
// organization unit the management API requires, by ID or by handle.
func (s *DefinitionExporterTestSuite) TestDeclarativeYAMLCarriesOU() {
	dto, err := parseToDefinitionDTO([]byte("id: def-1\nhandle: h\nvct: v\nouId: ou-123\n"))
	s.Require().NoError(err)
	s.Equal("ou-123", dto.OUID)

	dto, err = parseToDefinitionDTO([]byte("id: def-2\nhandle: h2\nvct: v\nouHandle: root/eng\n"))
	s.Require().NoError(err)
	s.Equal("root/eng", dto.OUHandle)
}

// TestValidateResolvesOUHandle verifies an ouHandle is resolved to an ouId at load time
// and that only the resolved ID is retained on the stored resource.
func (s *DefinitionExporterTestSuite) TestValidateResolvesOUHandle() {
	ouSvc := newOUServiceMock(s.T(), map[string]bool{"ou-123": true},
		map[string]string{"root/eng": "ou-123"}, map[string]string{"ou-123": "root/eng"})

	dto := &PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v", OUHandle: "root/eng"}
	s.Require().NoError(validateDefinitionWrapper(dto, nil, nil, ouSvc))
	s.Equal("ou-123", dto.OUID)
}

// TestValidateRejectsUnknownOUHandle verifies an unresolvable handle fails the load
// rather than silently producing a definition with no organization unit.
func (s *DefinitionExporterTestSuite) TestValidateRejectsUnknownOUHandle() {
	ouSvc := newOUServiceMock(s.T(), map[string]bool{}, map[string]string{}, map[string]string{})

	dto := &PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v", OUHandle: "no/such/ou"}
	err := validateDefinitionWrapper(dto, nil, nil, ouSvc)
	s.Require().Error(err)
	s.Contains(err.Error(), "no/such/ou")
}

// TestValidateRejectsMissingOU verifies a declarative definition without an organization
// unit is rejected, matching what the management API enforces on create.
func (s *DefinitionExporterTestSuite) TestValidateRejectsMissingOU() {
	dto := &PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v"}
	err := validateDefinitionWrapper(dto, nil, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "ouId or ouHandle is required")
}

// TestValidateRejectsDuplicateIDAndHandle verifies a second declarative file cannot
// silently overwrite or shadow a definition already loaded from another file.
func (s *DefinitionExporterTestSuite) TestValidateRejectsDuplicateIDAndHandle() {
	fileStore := newDefinitionFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	storer := &definitionStorer{store: fileStore}

	first := &PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: "v", OUID: "ou-1"}
	s.Require().NoError(validateDefinitionWrapper(first, fileStore, nil, nil))
	s.Require().NoError(storer.Create(first.ID, first))

	dupID := &PresentationDefinitionDTO{ID: "def-1", Handle: "other", VCT: "v", OUID: "ou-1"}
	err := validateDefinitionWrapper(dupID, fileStore, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "duplicate presentation definition ID")

	dupHandle := &PresentationDefinitionDTO{ID: "def-2", Handle: "h", VCT: "v", OUID: "ou-1"}
	err = validateDefinitionWrapper(dupHandle, fileStore, nil, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "duplicate presentation definition handle")
}

// TestValidateRejectsIDAlreadyInDatabase verifies a declarative definition cannot claim an
// ID a runtime definition already owns, which in composite mode would otherwise be shadowed
// by the database entry on every read.
func (s *DefinitionExporterTestSuite) TestValidateRejectsIDAlreadyInDatabase() {
	dbStore := newStatefulDefinitionStore(s.T())
	s.Require().NoError(dbStore.CreatePresentationDefinition(context.Background(),
		PresentationDefinitionDTO{ID: "def-db", Handle: "runtime_handle", VCT: "v", OUID: "ou-1"}))

	dto := &PresentationDefinitionDTO{ID: "def-db", Handle: "declarative_handle", VCT: "v", OUID: "ou-1"}
	err := validateDefinitionWrapper(dto, nil, dbStore, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "already exists in the database store")
}

// TestValidateAllowsUnusedIDWithDatabaseStore verifies the composite-mode duplicate check
// lets a fresh ID through rather than rejecting every declarative resource.
func (s *DefinitionExporterTestSuite) TestValidateAllowsUnusedIDWithDatabaseStore() {
	dbStore := newStatefulDefinitionStore(s.T())

	dto := &PresentationDefinitionDTO{ID: "def-fresh", Handle: "fresh_handle", VCT: "v", OUID: "ou-1"}
	s.NoError(validateDefinitionWrapper(dto, nil, dbStore, nil))
}

// TestValidateSurfacesDatabaseLookupFailure verifies a database error during the duplicate
// check fails the load instead of being mistaken for "no duplicate".
func (s *DefinitionExporterTestSuite) TestValidateSurfacesDatabaseLookupFailure() {
	dbStore := newDefinitionStoreInterfaceMock(s.T())
	dbStore.EXPECT().GetPresentationDefinitionByID(mock.Anything, mock.Anything).
		Return(nil, errors.New("db boom"))

	dto := &PresentationDefinitionDTO{ID: "def-x", Handle: "h", VCT: "v", OUID: "ou-1"}
	err := validateDefinitionWrapper(dto, nil, dbStore, nil)
	s.Require().Error(err)
	s.Contains(err.Error(), "failed to check for duplicate")
}

// TestLoadDeclarativeResourcesRejectsInvalidFile verifies a malformed declarative file
// fails the load, so the server refuses to start rather than silently skipping it.
func (s *DefinitionExporterTestSuite) TestLoadDeclarativeResourcesRejectsInvalidFile() {
	home := s.T().TempDir()
	dir := filepath.Join(home, "config", "resources", "presentation_definitions")
	s.Require().NoError(os.MkdirAll(dir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("id: [unterminated"), 0o600))

	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(home, &config.Config{}))

	fileStore := newDefinitionFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())

	err := loadDeclarativeResources(fileStore, nil, nil)
	s.Require().Error(err, "a malformed declarative file must fail the load")
	s.Contains(err.Error(), "failed to load presentation definition resources")
}

// TestLoadDeclarativeResourcesFromDisk exercises the whole loader path against a real file:
// parse, validate, resolve the organization unit handle, and store under the resource ID.
func (s *DefinitionExporterTestSuite) TestLoadDeclarativeResourcesFromDisk() {
	home := s.T().TempDir()
	dir := filepath.Join(home, "config", "resources", "presentation_definitions")
	s.Require().NoError(os.MkdirAll(dir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "def.yaml"), []byte(
		"id: def-disk\nhandle: disk_handle\nvct: urn:example:vct\nouHandle: root/eng\n"), 0o600))

	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(home, &config.Config{}))

	ouSvc := newOUServiceMock(s.T(), map[string]bool{"ou-123": true},
		map[string]string{"root/eng": "ou-123"}, map[string]string{"ou-123": "root/eng"})

	fileStore := newDefinitionFileBasedStore()
	s.Require().NoError(fileStore.GenericFileBasedStore.ClearByType())
	s.Require().NoError(loadDeclarativeResources(fileStore, nil, ouSvc))

	got, err := fileStore.GetPresentationDefinitionByID(context.Background(), "def-disk")
	s.Require().NoError(err)
	s.Equal("disk_handle", got.Handle)
	s.Equal("ou-123", got.OUID, "ouHandle must be resolved to an ouId at load time")
}
