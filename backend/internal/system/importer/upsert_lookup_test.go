// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	authzdetailmgt "github.com/thunder-id/thunderid/internal/authzdetail/mgt"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// fakeVCServices stands in for the presentation definition and credential configuration services. Each
// update is counted, a missing resource's included, as the audited services record every update asked
// for.
type fakeVCServices struct {
	definitions    map[string]bool
	configurations map[string]bool
	getErr         *tidcommon.ServiceError
	updateAttempts int
	created        int
}

func (f *fakeVCServices) CreatePresentationDefinition(_ context.Context, dto *presentation.PresentationDefinitionDTO) (
	*presentation.PresentationDefinitionDTO, *tidcommon.ServiceError) {
	f.created++
	return dto, nil
}

func (f *fakeVCServices) GetPresentationDefinition(_ context.Context, id string) (
	*presentation.PresentationDefinitionDTO, *tidcommon.ServiceError) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if !f.definitions[id] {
		return nil, &presentation.ErrorDefinitionNotFound
	}
	return &presentation.PresentationDefinitionDTO{ID: id}, nil
}

func (f *fakeVCServices) UpdatePresentationDefinition(_ context.Context, id string,
	dto *presentation.PresentationDefinitionDTO) (*presentation.PresentationDefinitionDTO, *tidcommon.ServiceError) {
	f.updateAttempts++
	if !f.definitions[id] {
		return nil, &presentation.ErrorDefinitionNotFound
	}
	return dto, nil
}

func (f *fakeVCServices) DeletePresentationDefinition(context.Context, string) *tidcommon.ServiceError {
	return nil
}

func (f *fakeVCServices) CreateCredentialConfiguration(_ context.Context, dto *credential.CredentialConfigurationDTO) (
	*credential.CredentialConfigurationDTO, *tidcommon.ServiceError) {
	f.created++
	return dto, nil
}

func (f *fakeVCServices) GetCredentialConfiguration(_ context.Context, id string) (
	*credential.CredentialConfigurationDTO, *tidcommon.ServiceError) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if !f.configurations[id] {
		return nil, &credential.ErrorConfigurationNotFound
	}
	return &credential.CredentialConfigurationDTO{ID: id}, nil
}

func (f *fakeVCServices) UpdateCredentialConfiguration(_ context.Context, id string,
	dto *credential.CredentialConfigurationDTO) (*credential.CredentialConfigurationDTO, *tidcommon.ServiceError) {
	f.updateAttempts++
	if !f.configurations[id] {
		return nil, &credential.ErrorConfigurationNotFound
	}
	return dto, nil
}

func (f *fakeVCServices) DeleteCredentialConfiguration(context.Context, string) *tidcommon.ServiceError {
	return nil
}

// UpsertLookupTestSuite checks that an upsert looks a resource up before it writes, so a create is never
// preceded by an update of a resource that does not exist, which its service would record as failed.
type UpsertLookupTestSuite struct {
	suite.Suite
}

func TestUpsertLookupTestSuite(t *testing.T) {
	suite.Run(t, new(UpsertLookupTestSuite))
}

const (
	presentationDefinitionDocument  = "resource_type: presentation_definition\nid: pd-1\nhandle: badge\nvct: Badge\n"
	credentialConfigurationDocument = "resource_type: credential_configuration\nid: vc-1\nhandle: badge\nvct: Badge\n"
)

// importOne imports one document with upsert on, and returns its outcome.
func (s *UpsertLookupTestSuite) importOne(svc ImportServiceInterface, document string) ImportItemOutcome {
	resp, err := svc.ImportResources(context.Background(), &ImportRequest{
		Content: document,
		Options: &ImportOptions{Upsert: boolPtr(true), ContinueOnError: boolPtr(true), Target: importTargetRuntime},
	})
	s.Require().Nil(err)
	s.Require().Len(resp.Results, 1)
	return resp.Results[0]
}

// vcImportService builds an import service over the fake VC services.
func vcImportService(fake *fakeVCServices) ImportServiceInterface {
	svc := newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, discardingBus{})
	svc.(*importService).presentationDefinitionService = fake
	svc.(*importService).credentialConfigurationService = fake
	return svc
}

func (s *UpsertLookupTestSuite) TestAVCResourceIsCreatedWithoutAnUpdateWhenAbsent() {
	for name, document := range map[string]string{
		"presentation definition":  presentationDefinitionDocument,
		"credential configuration": credentialConfigurationDocument,
	} {
		s.Run(name, func() {
			fake := &fakeVCServices{}

			outcome := s.importOne(vcImportService(fake), document)

			s.Equal(statusSuccess, outcome.Status)
			s.Equal(operationCreate, outcome.Operation)
			s.Equal(1, fake.created)
			s.Zero(fake.updateAttempts, "a missing resource is created without an update being tried first")
		})
	}
}

func (s *UpsertLookupTestSuite) TestAVCResourceIsUpdatedWhenPresent() {
	for name, document := range map[string]string{
		"presentation definition":  presentationDefinitionDocument,
		"credential configuration": credentialConfigurationDocument,
	} {
		s.Run(name, func() {
			fake := &fakeVCServices{definitions: map[string]bool{"pd-1": true},
				configurations: map[string]bool{"vc-1": true}}

			outcome := s.importOne(vcImportService(fake), document)

			s.Equal(statusSuccess, outcome.Status)
			s.Equal(operationUpdate, outcome.Operation)
			s.Equal(1, fake.updateAttempts)
			s.Zero(fake.created)
		})
	}
}

// A lookup that fails for a reason other than a missing resource ends the upsert as a failed update,
// writing nothing.
func (s *UpsertLookupTestSuite) TestAVCLookupFailureWritesNothing() {
	for name, document := range map[string]string{
		"presentation definition":  presentationDefinitionDocument,
		"credential configuration": credentialConfigurationDocument,
	} {
		s.Run(name, func() {
			fake := &fakeVCServices{getErr: &tidcommon.InternalServerError}

			outcome := s.importOne(vcImportService(fake), document)

			s.Equal(statusFailed, outcome.Status)
			s.Equal(operationUpdate, outcome.Operation)
			s.Equal(tidcommon.InternalServerError.Code, outcome.Code)
			s.Zero(fake.updateAttempts)
			s.Zero(fake.created)
		})
	}
}

func (s *UpsertLookupTestSuite) TestAnAuthorizationDetailTypeIsCreatedWithoutAnUpdateWhenAbsent() {
	adtSvc := &fakeAuthorizationDetailTypeService{}

	outcome := s.importOne(newAuthorizationDetailTypeImportService(adtSvc), authorizationDetailTypeDocument)

	s.Equal(statusSuccess, outcome.Status)
	s.Equal(operationCreate, outcome.Operation)
	s.Len(adtSvc.created, 1)
	s.Zero(adtSvc.updateAttempts, "a missing type is created without an update being tried first")
}

// An update the service refuses for a type that exists is reported as a failed update, and nothing is
// created in its place.
func (s *UpsertLookupTestSuite) TestAnAuthorizationDetailTypeUpdateRefusalCreatesNothing() {
	adtSvc := &fakeAuthorizationDetailTypeService{
		existing:  map[string]*providers.AuthorizationDetailType{"adt-1": {ID: "adt-1"}},
		updateErr: &authzdetailmgt.ErrorImmutableType,
	}

	outcome := s.importOne(newAuthorizationDetailTypeImportService(adtSvc), authorizationDetailTypeDocument)

	s.Equal(statusFailed, outcome.Status)
	s.Equal(operationUpdate, outcome.Operation)
	s.Equal(1, adtSvc.updateAttempts)
	s.Empty(adtSvc.created)
}
