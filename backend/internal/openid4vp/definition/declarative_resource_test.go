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

package definition

import (
	"context"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
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
displayName: EUDI PID
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

	dto, err := parseToDefinitionDTO([]byte("id: def-1\nhandle: h\nvct: v\n"))
	s.Require().NoError(err)
	s.Require().NoError(validateDefinitionWrapper(dto))
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
		DisplayName:          "EUDI PID",
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
	s.Error(validateDefinitionWrapper(dto))
}

func (s *DefinitionExporterTestSuite) TestValidateDefinitionWrapperRejectsInvalidDefinition() {
	// ID is present, so the wrapper proceeds to validateDefinition, which rejects
	// the missing VCT.
	dto := &PresentationDefinitionDTO{ID: "def-1", Handle: "h", VCT: ""}
	s.Error(validateDefinitionWrapper(dto))
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
	s.Error(validateDefinitionWrapper("not-a-dto"))
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

	s.Require().NoError(loadDeclarativeResources(&definitionStorer{store: fileStore}))
}
